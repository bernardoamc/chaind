package compare

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/bernardoamc/chaind/internal/image"
	"github.com/bernardoamc/chaind/internal/result"
)

// Compare checks whether either image is a base of the other.
// Argument order does not matter: both directions are tried automatically.
func Compare(a, b *image.Metadata, platform string) *result.CompareResult {
	if a.Digest == b.Digest {
		return &result.CompareResult{
			Verdict:       result.VerdictSameImage,
			Platform:      platform,
			MatchedLayers: []result.LayerInfo{},
			ExtraLayers:   []result.LayerInfo{},
			Images:        buildImages(a, b),
		}
	}

	if base, derived, ok := tryDirections(a, b); ok {
		return buildConfirmedResult(base, derived, platform)
	}

	return &result.CompareResult{
		Verdict:       result.VerdictNotBase,
		Platform:      platform,
		MatchedLayers: []result.LayerInfo{},
		ExtraLayers:   []result.LayerInfo{},
		Images:        buildImages(a, b),
	}
}

// tryDirections returns (base, derived, true) if one image's DiffIDs are a
// strict prefix of the other's. Equal layer counts mean neither can be a base.
func tryDirections(a, b *image.Metadata) (base, derived *image.Metadata, ok bool) {
	if len(a.DiffIDs) < len(b.DiffIDs) {
		if IsPrefixOf(a.DiffIDs, b.DiffIDs) {
			return a, b, true
		}
		return nil, nil, false
	}
	if len(b.DiffIDs) < len(a.DiffIDs) {
		if IsPrefixOf(b.DiffIDs, a.DiffIDs) {
			return b, a, true
		}
	}
	return nil, nil, false
}

func buildConfirmedResult(base, derived *image.Metadata, platform string) *result.CompareResult {
	return &result.CompareResult{
		Verdict:       result.VerdictConfirmedBase,
		Platform:      platform,
		Base:          &base.Ref,
		Derived:       &derived.Ref,
		MatchedLayers: buildLayers(derived, 0, len(base.DiffIDs)),
		ExtraLayers:   buildLayers(derived, len(base.DiffIDs), len(derived.DiffIDs)),
		Images:        buildImages(base, derived),
	}
}

func buildImages(a, b *image.Metadata) map[string]result.ImageMeta {
	return map[string]result.ImageMeta{
		a.Ref: {Digest: a.Digest.String(), LayerCount: len(a.DiffIDs), MediaType: a.MediaType},
		b.Ref: {Digest: b.Digest.String(), LayerCount: len(b.DiffIDs), MediaType: b.MediaType},
	}
}

// IsPrefixOf returns true if a is a prefix of b (by DiffID).
func IsPrefixOf(a, b []v1.Hash) bool {
	if len(a) > len(b) {
		return false
	}
	for i, h := range a {
		if h != b[i] {
			return false
		}
	}
	return true
}

// buildLayers returns LayerInfo for m's layers in [start, end).
func buildLayers(m *image.Metadata, start, end int) []result.LayerInfo {
	layers := make([]result.LayerInfo, 0, end-start)
	for i := range end - start {
		idx := start + i
		var digest v1.Hash
		if idx < len(m.Manifest.Layers) {
			digest = m.Manifest.Layers[idx].Digest
		}
		layers = append(layers, result.LayerInfo{
			Index:  idx,
			Digest: digest,
			DiffID: m.DiffIDs[idx],
		})
	}
	return layers
}
