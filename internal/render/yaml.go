package render

import (
	"io"

	"sigs.k8s.io/yaml"

	"github.com/neilfarmer/k8s-health/internal/result"
)

type yamlRenderer struct{}

func (yamlRenderer) Render(w io.Writer, r result.Report) error {
	b, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}
