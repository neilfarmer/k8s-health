package checker

import (
	"context"
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	eventWindow    = 15 * time.Minute
	maxEventGroups = 20
)

// EventChecker inspects recent warning events.
type EventChecker struct{}

func (c *EventChecker) Name() string        { return "events" }
func (c *EventChecker) Description() string { return "Warning Events" }

func (c *EventChecker) Check(ctx context.Context, opts CheckOptions) (*Result, error) {
	result := &Result{CheckerName: c.Name()}
	start := time.Now()
	defer func() { result.Duration = time.Since(start) }()

	var events []corev1.Event
	if opts.AllNamespaces() {
		list, err := opts.Client.CoreV1().Events("").List(ctx, metav1.ListOptions{
			FieldSelector: "type=Warning",
		})
		if err != nil {
			return nil, fmt.Errorf("listing events: %w", err)
		}
		events = list.Items
	} else {
		for _, ns := range opts.Namespaces {
			list, err := opts.Client.CoreV1().Events(ns).List(ctx, metav1.ListOptions{
				FieldSelector: "type=Warning",
			})
			if err != nil {
				return nil, fmt.Errorf("listing events in namespace %s: %w", ns, err)
			}
			events = append(events, list.Items...)
		}
	}

	// Filter to recent events
	cutoff := time.Now().Add(-eventWindow)
	var recent []corev1.Event
	for i := range events {
		eventTime := events[i].LastTimestamp.Time
		if eventTime.IsZero() {
			eventTime = events[i].CreationTimestamp.Time
		}
		if eventTime.After(cutoff) {
			recent = append(recent, events[i])
		}
	}

	// Group by involved object
	type eventGroup struct {
		key       string
		namespace string
		kind      string
		name      string
		reason    string
		message   string
		count     int32
	}

	groups := make(map[string]*eventGroup)
	for i := range recent {
		e := &recent[i]
		key := fmt.Sprintf("%s/%s/%s/%s", e.Namespace, e.InvolvedObject.Kind, e.InvolvedObject.Name, e.Reason)
		if g, exists := groups[key]; exists {
			g.count += maxInt32(e.Count, 1)
		} else {
			groups[key] = &eventGroup{
				key:       key,
				namespace: e.Namespace,
				kind:      e.InvolvedObject.Kind,
				name:      e.InvolvedObject.Name,
				reason:    e.Reason,
				message:   e.Message,
				count:     maxInt32(e.Count, 1),
			}
		}
	}

	// Sort by count descending, take top N
	sorted := make([]*eventGroup, 0, len(groups))
	for _, g := range groups {
		sorted = append(sorted, g)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})
	if len(sorted) > maxEventGroups {
		sorted = sorted[:maxEventGroups]
	}

	for _, g := range sorted {
		result.Findings = append(result.Findings, Finding{
			Checker:   c.Name(),
			Severity:  SeverityWarning,
			Namespace: g.namespace,
			Kind:      g.kind,
			Name:      g.name,
			Message:   fmt.Sprintf("[%s] %s (x%d)", g.reason, g.message, g.count),
			Details: map[string]string{
				"reason": g.reason,
				"count":  fmt.Sprintf("%d", g.count),
			},
		})
	}

	return result, nil
}

func maxInt32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
