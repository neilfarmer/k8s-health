// Package watch provides the interactive `khealth watch` TUI: it
// re-runs a configured set of checks on an interval and highlights
// findings that are new since the previous tick.
package watch

import "github.com/neilfarmer/k8s-health/internal/result"

// FindingKey is the identity used to compare findings across ticks. Two
// findings with the same status, check ID, and resource are considered
// "the same" for diff purposes — even if their message text changes.
type FindingKey string

// Key returns the comparison key for f.
func Key(f result.Finding) FindingKey {
	return FindingKey(string(f.Status) + "|" + f.Check + "|" + f.Resource)
}

// Tracker remembers the set of finding keys observed on the previous tick
// so the next tick can label each finding NEW or PERSISTING.
//
// The zero value is ready to use; the first tick treats every finding as
// PERSISTING (since there is nothing to diff against). Callers who want
// every finding marked NEW on the first tick should construct a Tracker
// with NewTracker and set FirstTickNew=true.
type Tracker struct {
	prev         map[FindingKey]struct{}
	FirstTickNew bool
	seenAny      bool
}

// NewTracker returns a fresh Tracker.
func NewTracker() *Tracker {
	return &Tracker{prev: map[FindingKey]struct{}{}}
}

// Diff classifies each finding in fs as new or persisting relative to the
// previous tick, then advances internal state so the next call diffs
// against fs.
func (t *Tracker) Diff(fs []result.Finding) (newKeys map[FindingKey]struct{}) {
	newKeys = make(map[FindingKey]struct{}, len(fs))
	cur := make(map[FindingKey]struct{}, len(fs))
	for _, f := range fs {
		k := Key(f)
		cur[k] = struct{}{}
		if !t.seenAny && !t.FirstTickNew {
			continue
		}
		if _, ok := t.prev[k]; !ok {
			newKeys[k] = struct{}{}
		}
	}
	t.prev = cur
	t.seenAny = true
	return newKeys
}
