package image

import (
	"context"
	"encoding/json"
	"fmt"

	dockerimage "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// ListRefs returns all tagged image references available in the local Docker daemon.
// It reads DOCKER_HOST from the environment, falling back to the default socket.
func ListRefs(ctx context.Context) ([]string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	defer cli.Close()

	imgs, err := cli.ImageList(ctx, dockerimage.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}

	var refs []string
	for _, img := range imgs {
		for _, tag := range img.RepoTags {
			if tag != "<none>:<none>" {
				refs = append(refs, tag)
			}
		}
	}
	return refs, nil
}

// Metadata holds the extracted metadata from a loaded image.
type Metadata struct {
	Ref       string
	Digest    v1.Hash
	DiffIDs   []v1.Hash
	MediaType string
	Manifest  *v1.Manifest
}

// Extract pulls the relevant metadata out of a loaded v1.Image.
func Extract(ref string, img v1.Image) (*Metadata, error) {
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

	return &Metadata{
		Ref:       ref,
		Digest:    digest,
		DiffIDs:   config.RootFS.DiffIDs,
		MediaType: string(mt),
		Manifest:  manifest,
	}, nil
}

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
