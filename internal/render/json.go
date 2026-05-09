package render

import (
	"encoding/json"
	"io"

	"github.com/neilfarmer/k8s-health/internal/result"
)

type jsonRenderer struct{}

func (jsonRenderer) Render(w io.Writer, r result.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
