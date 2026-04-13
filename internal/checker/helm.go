package checker

import (
	"context"
	"fmt"
	"log"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
)

// HelmLister abstracts Helm release listing for testability.
type HelmLister interface {
	ListReleases(namespace string) ([]*release.Release, error)
}

// helmSDKLister implements HelmLister using the Helm SDK.
type helmSDKLister struct {
	opts CheckOptions
}

func (l *helmSDKLister) ListReleases(namespace string) ([]*release.Release, error) {
	actionConfig := new(action.Configuration)
	nopLog := func(format string, v ...interface{}) {}
	if err := actionConfig.Init(
		&restClientGetter{config: l.opts.RestConfig, namespace: namespace},
		namespace,
		"secrets",
		nopLog,
	); err != nil {
		return nil, fmt.Errorf("initializing helm action config: %w", err)
	}

	listAction := action.NewList(actionConfig)
	listAction.All = true
	listAction.AllNamespaces = namespace == ""
	listAction.StateMask = action.ListAll

	return listAction.Run()
}

// HelmChecker inspects Helm releases for failure states.
type HelmChecker struct {
	Lister HelmLister // injectable for testing
}

func (c *HelmChecker) Name() string        { return "helm" }
func (c *HelmChecker) Description() string { return "Helm Releases" }

func (c *HelmChecker) Check(ctx context.Context, opts CheckOptions) (*Result, error) {
	result := &Result{CheckerName: c.Name()}
	start := time.Now()
	defer func() { result.Duration = time.Since(start) }()

	if opts.RestConfig == nil {
		result.Findings = append(result.Findings, Finding{
			Checker:  c.Name(),
			Severity: SeverityInfo,
			Kind:     "HelmRelease",
			Name:     "cluster",
			Message:  "Helm checking requires REST config (skipped)",
		})
		return result, nil
	}

	lister := c.Lister
	if lister == nil {
		lister = &helmSDKLister{opts: opts}
	}

	var releases []*release.Release
	if opts.AllNamespaces() {
		var err error
		releases, err = lister.ListReleases("")
		if err != nil {
			log.Printf("Warning: unable to list Helm releases: %v", err)
			result.Findings = append(result.Findings, Finding{
				Checker:  c.Name(),
				Severity: SeverityInfo,
				Kind:     "HelmRelease",
				Name:     "cluster",
				Message:  fmt.Sprintf("Unable to list Helm releases: %v", err),
			})
			return result, nil
		}
	} else {
		for _, ns := range opts.Namespaces {
			nsReleases, err := lister.ListReleases(ns)
			if err != nil {
				log.Printf("Warning: unable to list Helm releases in %s: %v", ns, err)
				continue
			}
			releases = append(releases, nsReleases...)
		}
	}

	for _, rel := range releases {
		c.checkRelease(rel, result)
	}

	return result, nil
}

var criticalHelmStatuses = map[release.Status]bool{
	release.StatusFailed: true,
}

var warningHelmStatuses = map[release.Status]bool{
	release.StatusPendingInstall:  true,
	release.StatusPendingUpgrade:  true,
	release.StatusPendingRollback: true,
	release.StatusSuperseded:      true,
	release.StatusUninstalling:    true,
}

func (c *HelmChecker) checkRelease(rel *release.Release, result *Result) {
	if rel == nil || rel.Info == nil {
		return
	}

	status := rel.Info.Status
	sev := SeverityInfo
	if criticalHelmStatuses[status] {
		sev = SeverityCritical
	} else if warningHelmStatuses[status] {
		sev = SeverityWarning
	} else {
		return // deployed or unknown-but-ok status
	}

	details := map[string]string{
		"status":  string(status),
		"chart":   rel.Chart.Metadata.Name,
		"version": rel.Chart.Metadata.Version,
	}
	if rel.Chart.Metadata.AppVersion != "" {
		details["appVersion"] = rel.Chart.Metadata.AppVersion
	}

	result.Findings = append(result.Findings, Finding{
		Checker:   c.Name(),
		Severity:  sev,
		Namespace: rel.Namespace,
		Kind:      "HelmRelease",
		Name:      rel.Name,
		Message:   fmt.Sprintf("Helm release %q is in %s state", rel.Name, status),
		Details:   details,
	})
}
