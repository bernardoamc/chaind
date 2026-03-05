package output

import "github.com/bernardoamc/chaind/internal/result"

// Renderer renders a CompareResult to some output format.
type Renderer interface {
	Render(res *result.CompareResult) error
}
