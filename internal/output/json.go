package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/bernardoamc/chaind/internal/result"
)

// JSONRenderer renders a CompareResult as JSON.
type JSONRenderer struct {
	w      io.Writer
	indent bool
}

// NewJSONRenderer creates a new JSONRenderer.
func NewJSONRenderer(w io.Writer) *JSONRenderer {
	return &JSONRenderer{w: w, indent: true}
}

// Render writes JSON output to the writer.
func (r *JSONRenderer) Render(res *result.CompareResult) error {
	var (
		data []byte
		err  error
	)
	if r.indent {
		data, err = json.MarshalIndent(res, "", "  ")
	} else {
		data, err = json.Marshal(res)
	}
	if err != nil {
		return fmt.Errorf("marshal result to JSON: %w", err)
	}
	_, err = fmt.Fprintf(r.w, "%s\n", data)
	return err
}
