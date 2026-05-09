// Package version exposes build-time metadata.
//
// Values are populated via -ldflags at build time. See the Makefile and
// goreleaser config for the canonical -X assignments.
package version

var (
	// Version is the semantic version (e.g. "v0.1.0"). "dev" for unreleased builds.
	Version = "dev"
	// Commit is the git SHA of the build.
	Commit = "none"
	// Date is the build date in RFC3339.
	Date = "unknown"
)

// Info returns the build-time metadata as a single struct, useful for JSON output.
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// Get returns the current build's Info.
func Get() Info {
	return Info{Version: Version, Commit: Commit, Date: Date}
}
