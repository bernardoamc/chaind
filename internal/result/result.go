package result

import (
	"encoding/json"
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// SchemaVersion is the current output schema version.
// Increment on any breaking change to field names, types, or semantics.
const SchemaVersion = 1

// Verdict represents the result of a base image comparison.
type Verdict int

const (
	VerdictConfirmedBase Verdict = iota
	VerdictNotBase
	VerdictSameImage
)

var verdictStrings = map[Verdict]string{
	VerdictConfirmedBase: "CONFIRMED_BASE",
	VerdictNotBase:       "NOT_BASE",
	VerdictSameImage:     "SAME_IMAGE",
}

var stringToVerdict = func() map[string]Verdict {
	m := make(map[string]Verdict, len(verdictStrings))
	for v, s := range verdictStrings {
		m[s] = v
	}
	return m
}()

func (v Verdict) String() string {
	if s, ok := verdictStrings[v]; ok {
		return s
	}
	return fmt.Sprintf("UNKNOWN(%d)", int(v))
}

func (v Verdict) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.String())
}

func (v *Verdict) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	verdict, ok := stringToVerdict[s]
	if !ok {
		return fmt.Errorf("unknown verdict: %s", s)
	}
	*v = verdict
	return nil
}

// ImageMeta holds metadata about a single image.
// Reference is omitted since it is the key in CompareResult.Images.
type ImageMeta struct {
	Digest     string `json:"digest"`
	LayerCount int    `json:"layer_count"`
	MediaType  string `json:"media_type"`
}

// LayerInfo holds information about a single layer.
type LayerInfo struct {
	Index  int     `json:"index"`
	Digest v1.Hash `json:"digest"`
	DiffID v1.Hash `json:"diff_id"`
}

// GraphNode is a single image in a graph chain.
type GraphNode struct {
	Reference       string  `json:"reference"`
	Digest          string  `json:"digest"`
	LayerCount      int     `json:"layer_count"`
	ParentReference *string `json:"parent"`
}

// Chain is an ordered sequence of images where each is a base of the next.
type Chain struct {
	Nodes []GraphNode `json:"nodes"`
}

// GraphResult is the full result of a graph traversal across local images.
type GraphResult struct {
	SchemaVersion int         `json:"schema_version"`
	Chains        []Chain     `json:"chains"`
	Unrelated     []GraphNode `json:"unrelated"`
	Warnings      []string    `json:"warnings"`
}

// AncestorGroup is a set of images that share a common implied ancestor,
// identified by the deepest ChainID they all hold in common.
type AncestorGroup struct {
	CommonChainID string   `json:"common_chain_id"`
	CommonDepth   int      `json:"common_depth"`
	Images        []string `json:"images"`
}

// AncestorsResult is the full result of an implied ancestry analysis.
type AncestorsResult struct {
	SchemaVersion int             `json:"schema_version"`
	Groups        []AncestorGroup `json:"groups"`
	Ungrouped     []string        `json:"ungrouped"`
	Warnings      []string        `json:"warnings"`
}

// CompareResult is the full result of comparing two images.
//
// Images is always populated with the full metadata for both inputs, keyed by
// reference. Base and Derived are non-null only for CONFIRMED_BASE, holding
// the reference strings that identify which image plays each role.
type CompareResult struct {
	SchemaVersion int                  `json:"schema_version"`
	Verdict       Verdict              `json:"verdict"`
	Platform      string               `json:"platform"`
	Base          *string              `json:"base"`
	Derived       *string              `json:"derived"`
	MatchedLayers []LayerInfo          `json:"matched_layers"`
	ExtraLayers   []LayerInfo          `json:"extra_layers"`
	Images        map[string]ImageMeta `json:"images"`
	Warnings      []string             `json:"warnings"`
}
