package result

import (
	"encoding/json"
	"fmt"
	"os"
)

// Baseline is the on-disk shape of a saved khealth report. It is the
// same JSON the json renderer emits, so users can pipe `-o json` into a
// file and reuse it as a baseline.
type Baseline struct {
	Findings []Finding `json:"findings"`
}

// LoadBaseline reads a Baseline from path. Empty path returns a nil
// pointer with no error so callers can write the no-baseline case as
// `b == nil`.
func LoadBaseline(path string) (*Baseline, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path is user-supplied CLI flag (--baseline)
	if err != nil {
		return nil, fmt.Errorf("read baseline %q: %w", path, err)
	}
	// Accept both a bare Baseline document and a full Report JSON.
	var b Baseline
	if err := json.Unmarshal(data, &b); err == nil && b.Findings != nil {
		return &b, nil
	}
	var rep Report
	if err := json.Unmarshal(data, &rep); err != nil {
		return nil, fmt.Errorf("parse baseline %q: %w", path, err)
	}
	return &Baseline{Findings: rep.Findings}, nil
}

// SaveBaseline writes the report's findings as a Baseline JSON file.
func SaveBaseline(path string, rep Report) error {
	if path == "" {
		return nil
	}
	b := Baseline{Findings: rep.Findings}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Diff classifies each Finding in cur against base.
type Diff struct {
	New        []Finding // present in cur, absent in base (regression)
	Cleared    []Finding // present in base, absent in cur (improvement)
	Persisting []Finding // present in both
}

// Compare classifies cur against base. Findings are matched by
// (Check, Resource, Status) — message text is allowed to drift without
// counting as new.
func Compare(base, cur []Finding) Diff {
	key := func(f Finding) string {
		return string(f.Status) + "|" + f.Check + "|" + f.Resource
	}
	baseSet := make(map[string]struct{}, len(base))
	for _, f := range base {
		baseSet[key(f)] = struct{}{}
	}
	curSet := make(map[string]struct{}, len(cur))
	for _, f := range cur {
		curSet[key(f)] = struct{}{}
	}

	d := Diff{}
	for _, f := range cur {
		if _, ok := baseSet[key(f)]; ok {
			d.Persisting = append(d.Persisting, f)
		} else {
			d.New = append(d.New, f)
		}
	}
	for _, f := range base {
		if _, ok := curSet[key(f)]; !ok {
			d.Cleared = append(d.Cleared, f)
		}
	}
	return d
}
