package image

import (
	"encoding/json"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// Load loads a single-platform image from the Docker daemon.
// If the image is a manifest list, it resolves the descriptor matching targetPlatform.
// The caller is responsible for setting DOCKER_HOST if a custom socket is needed.
func Load(ref string, targetPlatform *v1.Platform) (v1.Image, error) {
	r, err := name.ParseReference(ref)
	if err != nil {
		return nil, fmt.Errorf("parse reference %q: %w", ref, err)
	}

	img, err := daemon.Image(r)
	if err != nil {
		return nil, fmt.Errorf("load image %q from daemon: %w", ref, err)
	}

	mt, err := img.MediaType()
	if err != nil {
		return nil, fmt.Errorf("get media type for %q: %w", ref, err)
	}

	switch mt {
	case types.OCIImageIndex, types.DockerManifestList:
		return resolveIndex(r, img, targetPlatform)
	default:
		return img, nil
	}
}

// resolveIndex resolves a manifest list to a single-platform image.
func resolveIndex(r name.Reference, img v1.Image, targetPlatform *v1.Platform) (v1.Image, error) {
	raw, err := img.RawManifest()
	if err != nil {
		return nil, fmt.Errorf("get raw manifest: %w", err)
	}

	var index v1.IndexManifest
	if err := json.Unmarshal(raw, &index); err != nil {
		return nil, fmt.Errorf("unmarshal index manifest: %w", err)
	}

	for _, desc := range index.Manifests {
		if desc.Platform == nil || !PlatformSatisfies(desc.Platform, targetPlatform) {
			continue
		}

		digestRef, err := name.NewDigest(fmt.Sprintf("%s@%s", r.Context().Name(), desc.Digest.String()))
		if err != nil {
			return nil, fmt.Errorf("build digest ref: %w", err)
		}
		return daemon.Image(digestRef)
	}

	plat := targetPlatform.OS + "/" + targetPlatform.Architecture
	if targetPlatform.Variant != "" {
		plat += "/" + targetPlatform.Variant
	}
	return nil, fmt.Errorf("no manifest found for platform %s in index", plat)
}

// PlatformSatisfies returns true if candidate matches the target platform.
func PlatformSatisfies(candidate, target *v1.Platform) bool {
	if candidate.OS != target.OS {
		return false
	}
	if candidate.Architecture != target.Architecture {
		return false
	}
	if target.Variant != "" && candidate.Variant != target.Variant {
		return false
	}
	return true
}
