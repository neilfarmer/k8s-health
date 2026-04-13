package checker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/client-go/rest"
)

type mockHelmLister struct {
	releases []*release.Release
	err      error
}

func (m *mockHelmLister) ListReleases(namespace string) ([]*release.Release, error) {
	if m.err != nil {
		return nil, m.err
	}
	if namespace == "" {
		return m.releases, nil
	}
	var filtered []*release.Release
	for _, r := range m.releases {
		if r.Namespace == namespace {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
}

func makeRelease(name, ns string, status release.Status) *release.Release {
	return &release.Release{
		Name:      name,
		Namespace: ns,
		Info: &release.Info{
			Status: status,
		},
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    name + "-chart",
				Version: "1.0.0",
			},
		},
	}
}

func TestHelmChecker_FailedRelease(t *testing.T) {
	c := &HelmChecker{
		Lister: &mockHelmLister{
			releases: []*release.Release{
				makeRelease("my-app", "default", release.StatusFailed),
			},
		},
	}

	result, err := c.Check(context.Background(), CheckOptions{RestConfig: &rest.Config{}})
	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, SeverityCritical, result.Findings[0].Severity)
	assert.Contains(t, result.Findings[0].Message, "failed")
}

func TestHelmChecker_PendingInstall(t *testing.T) {
	c := &HelmChecker{
		Lister: &mockHelmLister{
			releases: []*release.Release{
				makeRelease("new-app", "default", release.StatusPendingInstall),
			},
		},
	}

	result, err := c.Check(context.Background(), CheckOptions{RestConfig: &rest.Config{}})
	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, SeverityWarning, result.Findings[0].Severity)
}

func TestHelmChecker_DeployedRelease(t *testing.T) {
	c := &HelmChecker{
		Lister: &mockHelmLister{
			releases: []*release.Release{
				makeRelease("healthy-app", "default", release.StatusDeployed),
			},
		},
	}

	result, err := c.Check(context.Background(), CheckOptions{RestConfig: &rest.Config{}})
	require.NoError(t, err)
	assert.Empty(t, result.Findings)
}

func TestHelmChecker_NoRestConfig(t *testing.T) {
	c := &HelmChecker{}
	result, err := c.Check(context.Background(), CheckOptions{})
	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, SeverityInfo, result.Findings[0].Severity)
}
