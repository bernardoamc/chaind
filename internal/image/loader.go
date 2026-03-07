package image

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"

	dockerimage "github.com/docker/docker/api/types/image"
	dockerclient "github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/bernardoamc/chaind/internal/platform"
)

// Client wraps the Docker daemon connection and exposes domain-level image
// operations. All daemon calls share the same underlying connection, avoiding
// per-call connection churn that can cause transient "No such image" errors
// under Docker Desktop.
type Client struct {
	cli *dockerclient.Client
	opt daemon.Option // pre-built WithClient option, reused for every Load call
}

// NewClient creates a Docker client configured from the environment.
// The caller is responsible for calling Close when done.
func NewClient() (*Client, error) {
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}

	return &Client{cli: cli, opt: daemon.WithClient(cli)}, nil
}

// Close releases the underlying Docker client connection.
func (c *Client) Close() error {
	return c.cli.Close()
}

// ListRefs returns all tagged image references available in the local Docker daemon.
func (c *Client) ListRefs(ctx context.Context) ([]string, error) {
	imgs, err := c.cli.ImageList(ctx, dockerimage.ListOptions{})
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

// Load loads a single-platform image from the Docker daemon.
// If the image is a manifest list, it resolves the descriptor matching targetPlatform.
func (c *Client) Load(ref string, targetPlatform *v1.Platform) (v1.Image, error) {
	r, err := name.ParseReference(ref)
	if err != nil {
		return nil, fmt.Errorf("parse reference %q: %w", ref, err)
	}

	img, err := daemon.Image(r, c.opt)
	if err != nil {
		return nil, fmt.Errorf("load image %q from daemon: %w", ref, err)
	}

	mt, err := img.MediaType()
	if err != nil {
		return nil, fmt.Errorf("get media type for %q: %w", ref, err)
	}

	switch mt {
	case types.OCIImageIndex, types.DockerManifestList:
		return resolveIndex(r, img, targetPlatform, c.opt)
	default:
		return img, nil
	}
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

// LoadAll loads all referenced images concurrently from the daemon, retrying
// any that fail in the first pass. It returns the successfully loaded entries
// and a warnings slice for images that could not be loaded after retrying.
//
// Retry logic exists because Docker Desktop can transiently reject concurrent
// ImageSave streams; by the time the first wave finishes the daemon has
// settled and individual retries almost always succeed.
func (c *Client) LoadAll(ctx context.Context, refs []string) ([]*Metadata, []string, error) {
	plat := platform.HostPlatform()

	var mu sync.Mutex
	var entries []*Metadata
	var failed []string

	var g errgroup.Group
	g.SetLimit(8)

	for _, ref := range refs {
		g.Go(func() error {
			img, err := c.Load(ref, plat)
			if err != nil {
				mu.Lock()
				failed = append(failed, ref)
				mu.Unlock()
				return nil
			}

			m, err := Extract(ref, img)
			if err != nil {
				mu.Lock()
				failed = append(failed, ref)
				mu.Unlock()
				return nil
			}

			mu.Lock()
			entries = append(entries, m)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, nil, err
	}

	warnings := []string{}
	for _, ref := range failed {
		img, err := c.Load(ref, plat)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skipping %s: %v", ref, err))
			continue
		}
		m, err := Extract(ref, img)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skipping %s: %v", ref, err))
			continue
		}
		entries = append(entries, m)
	}

	return entries, warnings, nil
}

// resolveIndex resolves a manifest list to a single-platform image.
func resolveIndex(r name.Reference, img v1.Image, targetPlatform *v1.Platform, opt daemon.Option) (v1.Image, error) {
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
		return daemon.Image(digestRef, opt)
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
