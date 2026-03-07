package ancestors

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"

	"github.com/bernardoamc/chaind/internal/image"
	"github.com/bernardoamc/chaind/internal/result"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// Build loads all referenced images and groups them by longest common ChainID.
// Groups whose shared depth is below minDepth are excluded.
func Build(ctx context.Context, refs []string, cli *image.Client, minDepth int) (*result.AncestorsResult, error) {
	entries, warnings, err := cli.LoadAll(ctx, refs)
	if err != nil {
		return nil, err
	}

	res := buildAncestors(entries, minDepth)
	res.Warnings = warnings

	return res, nil
}

type imageData struct {
	ref      string
	chainIDs []string
}

type chainInfo struct {
	refs  map[string]struct{}
	depth int
}

func buildAncestors(entries []*image.Metadata, minDepth int) *result.AncestorsResult {
	images := make([]imageData, len(entries))
	for i, e := range entries {
		images[i] = imageData{ref: e.Ref, chainIDs: computeChainIDs(e.DiffIDs)}
	}

	// Build inverted index: chainID → set of image refs and the depth it was
	// first seen at. Only index chainIDs at depth >= minDepth.
	// Images with a single layer are bare base images (e.g. alpine, busybox):
	// they are the roots everything else derives from and contribute no useful
	// grouping signal. They still appear in ungrouped so the caller knows they
	// exist.
	chains := make(map[string]*chainInfo)

	for _, img := range images {
		if len(img.chainIDs) <= 1 {
			continue
		}

		for i, chainID := range img.chainIDs {
			depth := i + 1

			if depth < minDepth {
				continue
			}

			if _, ok := chains[chainID]; !ok {
				chains[chainID] = &chainInfo{refs: make(map[string]struct{}), depth: depth}
			}

			chains[chainID].refs[img.ref] = struct{}{}
		}
	}

	// Retain only chainIDs shared by 2+ images.
	for chainID, info := range chains {
		if len(info.refs) < 2 {
			delete(chains, chainID)
		}
	}

	// Deduplicate: for each unique image set, keep only the deepest chainID.
	// This collapses chains of consecutive shared chainIDs (e.g. depths 3, 4, 5
	// for the same group of images) down to a single representative entry.
	type best struct {
		chainID string
		depth   int
	}

	byImageSet := make(map[string]best)

	for chainID, info := range chains {
		key := imageSetKey(info.refs)
		if b, ok := byImageSet[key]; !ok || info.depth > b.depth {
			byImageSet[key] = best{chainID: chainID, depth: info.depth}
		}
	}

	// Build groups.
	grouped := make(map[string]struct{})
	groups := make([]result.AncestorGroup, 0, len(byImageSet))

	for key, b := range byImageSet {
		refs := strings.Split(key, "\x00")
		for _, r := range refs {
			grouped[r] = struct{}{}
		}
		groups = append(groups, result.AncestorGroup{
			CommonChainID: b.chainID,
			CommonDepth:   b.depth,
			Images:        refs,
		})
	}

	// Sort groups: deepest first, then by chainID for determinism.
	slices.SortFunc(groups, func(a, b result.AncestorGroup) int {
		if a.CommonDepth != b.CommonDepth {
			return b.CommonDepth - a.CommonDepth
		}
		return strings.Compare(a.CommonChainID, b.CommonChainID)
	})

	ungrouped := []string{}
	for _, img := range images {
		if _, ok := grouped[img.ref]; !ok {
			ungrouped = append(ungrouped, img.ref)
		}
	}
	slices.Sort(ungrouped)

	return &result.AncestorsResult{
		SchemaVersion: result.SchemaVersion,
		Groups:        groups,
		Ungrouped:     ungrouped,
	}
}

// computeChainIDs returns the ChainID at each layer depth per the OCI spec:
//
//	ChainID(L₀) = DiffID(L₀)
//	ChainID(Lₙ) = SHA256(ChainID(Lₙ₋₁) + " " + DiffID(Lₙ))
func computeChainIDs(diffIDs []v1.Hash) []string {
	if len(diffIDs) == 0 {
		return nil
	}

	chainIDs := make([]string, len(diffIDs))
	chainIDs[0] = diffIDs[0].String()

	for i := 1; i < len(diffIDs); i++ {
		input := chainIDs[i-1] + " " + diffIDs[i].String()
		h := sha256.Sum256([]byte(input))
		chainIDs[i] = "sha256:" + hex.EncodeToString(h[:])
	}

	return chainIDs
}

// imageSetKey returns a stable string key for a set of image references.
// References are sorted and joined with a null byte separator.
func imageSetKey(refs map[string]struct{}) string {
	keys := make([]string, 0, len(refs))
	for r := range refs {
		keys = append(keys, r)
	}

	slices.Sort(keys)

	return strings.Join(keys, "\x00")
}
