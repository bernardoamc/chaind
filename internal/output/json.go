package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/bernardoamc/chaind/internal/result"
)

// JSONRenderer renders results as indented JSON.
type JSONRenderer struct {
	w io.Writer
}

// NewJSONRenderer creates a new JSONRenderer.
func NewJSONRenderer(w io.Writer) *JSONRenderer {
	return &JSONRenderer{w: w}
}

// RenderGraph writes a GraphResult as JSON to the writer.
func (r *JSONRenderer) RenderGraph(res *result.GraphResult) error {
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal graph result to JSON: %w", err)
	}
	_, err = fmt.Fprintf(r.w, "%s\n", data)
	return err
}

// Render writes a CompareResult as JSON to the writer.
func (r *JSONRenderer) Render(res *result.CompareResult) error {
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal result to JSON: %w", err)
	}
	_, err = fmt.Fprintf(r.w, "%s\n", data)
	return err
}
