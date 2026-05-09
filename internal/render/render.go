// Package render emits a result.Report in one of several formats. Renderers
// are selected by name (table | json | yaml) and write to an io.Writer.
package render

import (
	"fmt"
	"io"

	"github.com/neilfarmer/k8s-health/internal/result"
)

// Renderer turns a Report into bytes on w.
type Renderer interface {
	Render(w io.Writer, r result.Report) error
}

// New returns the renderer for the given name. Unknown names return an error.
func New(name string) (Renderer, error) {
	switch name {
	case "", "pretty":
		return prettyRenderer{}, nil
	case "table":
		return tableRenderer{}, nil
	case "json":
		return jsonRenderer{}, nil
	case "yaml":
		return yamlRenderer{}, nil
	}
	return nil, fmt.Errorf("render: unknown format %q (want pretty|table|json|yaml)", name)
}
