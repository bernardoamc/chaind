package compare

import (
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/bernardoamc/chaind/internal/result"
)

// Input pairs an image reference with its loaded image.
type Input struct {
	Ref string
	Img v1.Image
}

type imageData struct {
	ref       string
	mediaType string
	diffIDs   []v1.Hash
	digest    v1.Hash
	manifest  *v1.Manifest
}

// Compare checks whether either image is a base of the other.
// Argument order does not matter: both directions are tried automatically.
// In the result, ImageA is always the base and ImageB is always the derived image.
func Compare(a, b Input, platform string) (*result.CompareResult, error) {
	d1, err := loadImageData(a.Ref, a.Img)
	if err != nil {
		return nil, fmt.Errorf("loading first image: %w", err)
	}
	d2, err := loadImageData(b.Ref, b.Img)
	if err != nil {
		return nil, fmt.Errorf("loading second image: %w", err)
	}

	// Same image — symmetric, no need to check both directions.
	if d1.digest == d2.digest {
		res := &result.CompareResult{
			Platform: platform,
			Verdict:  result.VerdictSameImage,
			ImageA:   buildImageMeta(d1),
			ImageB:   buildImageMeta(d2),
		}
		return res, nil
	}

	// Try d1 → d2, then d2 → d1. base is always placed in ImageA.
	if base, derived, ok := tryDirections(d1, d2); ok {
		return buildConfirmedResult(base, derived, platform), nil
	}

	// Neither direction matched.
	res := &result.CompareResult{
		Platform: platform,
		Verdict:  result.VerdictNotBase,
		ImageA:   buildImageMeta(d1),
		ImageB:   buildImageMeta(d2),
	}
	return res, nil
}

// tryDirections tries d1→d2 then d2→d1. Returns (base, derived, true) on first match.
func tryDirections(d1, d2 *imageData) (base, derived *imageData, ok bool) {
	if isPrefixOf(d1.diffIDs, d2.diffIDs) {
		return d1, d2, true
	}
	if isPrefixOf(d2.diffIDs, d1.diffIDs) {
		return d2, d1, true
	}
	return nil, nil, false
}

func buildConfirmedResult(base, derived *imageData, platform string) *result.CompareResult {
	res := &result.CompareResult{
		Platform: platform,
		Verdict:  result.VerdictConfirmedBase,
		ImageA:   buildImageMeta(base),
		ImageB:   buildImageMeta(derived),
	}

	res.MatchedLayers = buildLayers(derived, 0, len(base.diffIDs))
	res.ExtraLayers = buildLayers(derived, len(base.diffIDs), len(derived.diffIDs))

	return res
}

func loadImageData(ref string, img v1.Image) (*imageData, error) {
	digest, err := img.Digest()
	if err != nil {
		return nil, fmt.Errorf("get digest: %w", err)
	}

	manifest, err := img.Manifest()
	if err != nil {
		return nil, fmt.Errorf("get manifest: %w", err)
	}

	config, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("get config file: %w", err)
	}

	mt, err := img.MediaType()
	if err != nil {
		return nil, fmt.Errorf("get media type: %w", err)
	}

	return &imageData{
		ref:       ref,
		mediaType: string(mt),
		diffIDs:   config.RootFS.DiffIDs,
		digest:    digest,
		manifest:  manifest,
	}, nil
}

func buildImageMeta(d *imageData) result.ImageMeta {
	return result.ImageMeta{
		Reference:  d.ref,
		Digest:     d.digest.String(),
		LayerCount: len(d.diffIDs),
		MediaType:  d.mediaType,
	}
}

// isPrefixOf returns true if a is a prefix of b (by DiffID).
func isPrefixOf(a, b []v1.Hash) bool {
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

// buildLayers returns LayerInfo for img's layers in [start, end).
func buildLayers(img *imageData, start, end int) []result.LayerInfo {
	layers := make([]result.LayerInfo, 0, end-start)
	for i := range end - start {
		idx := start + i
		var digest v1.Hash
		if idx < len(img.manifest.Layers) {
			digest = img.manifest.Layers[idx].Digest
		}
		layers = append(layers, result.LayerInfo{
			Index:  idx,
			Digest: digest,
			DiffID: img.diffIDs[idx],
		})
	}
	return layers
}

