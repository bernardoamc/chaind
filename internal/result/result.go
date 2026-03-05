package result

import (
	"encoding/json"
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

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
type ImageMeta struct {
	Reference  string `json:"reference"`
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

// CompareResult is the full result of comparing two images.
type CompareResult struct {
	Verdict       Verdict     `json:"verdict"`
	Platform      string      `json:"platform"`
	ImageA        ImageMeta   `json:"image_a"`
	ImageB        ImageMeta   `json:"image_b"`
	MatchedLayers []LayerInfo `json:"matched_layers"`
	ExtraLayers   []LayerInfo `json:"extra_layers"`
	Warnings      []string    `json:"warnings,omitempty"`
}
