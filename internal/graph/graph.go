package graph

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/bernardoamc/chaind/internal/image"
	"github.com/bernardoamc/chaind/internal/platform"
	"github.com/bernardoamc/chaind/internal/result"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// Build loads all referenced images using the host platform, then finds and
// returns all base image relationships among them.
func Build(ctx context.Context, refs []string, cli *image.Client) (*result.GraphResult, error) {
	entries, err := loadEntries(ctx, refs, cli)
	if err != nil {
		return nil, err
	}

	// Sort by layer count for deterministic output ordering.
	slices.SortStableFunc(entries, func(a, b *image.Metadata) int {
		return cmp.Compare(len(a.DiffIDs), len(b.DiffIDs))
	})

	return buildGraph(entries), nil
}

func loadEntries(ctx context.Context, refs []string, cli *image.Client) ([]*image.Metadata, error) {
	plat := platform.HostPlatform()

	var mu sync.Mutex
	var entries []*image.Metadata

	var g errgroup.Group
	g.SetLimit(8)

	for _, ref := range refs {
		g.Go(func() error {
			img, err := cli.Load(ref, plat)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", ref, err)
				return nil
			}

			m, err := image.Extract(ref, img)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", ref, err)
				return nil
			}

			mu.Lock()
			entries = append(entries, m)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return entries, nil
}

func buildGraph(entries []*image.Metadata) *result.GraphResult {
	n := len(entries)
	directChildren := make([][]int, n)
	hasParent := make([]bool, n)

	// Index every image by its DiffID fingerprint for O(1) parent lookup.
	keyToIdx := make(map[string]int, n)
	for i, e := range entries {
		keyToIdx[diffIDKey(e.DiffIDs)] = i
	}

	// Entries are sorted by layer count; the first entry has the global minimum.
	// Prefix lengths shorter than minLayers have no candidate parent in the
	// dataset, so we skip building and checking those keys entirely.
	minLayers := 0
	if n > 0 {
		minLayers = len(entries[0].DiffIDs)
	}

	// For each image, find its direct parent: the longest proper DiffID prefix
	// that belongs to another image. We walk from minLayers upward, building the
	// key incrementally so each DiffID hex is appended once per image.
	// The last map hit is the longest match (direct parent).
	for j, e := range entries {
		key := make([]byte, 0, len(e.DiffIDs)*64)
		longestParent := -1

		// Pre-fill key bytes below minLayers without checking since no image is that small.
		for k := range minLayers - 1 {
			key = append(key, e.DiffIDs[k].Hex...)
		}

		for k := minLayers - 1; k < len(e.DiffIDs)-1; k++ {
			key = append(key, e.DiffIDs[k].Hex...)
			if i, ok := keyToIdx[string(key)]; ok {
				longestParent = i
			}
		}

		if longestParent >= 0 {
			directChildren[longestParent] = append(directChildren[longestParent], j)
			hasParent[j] = true
		}
	}

	chains, inChain := collectChains(entries, directChildren, hasParent)

	unrelated := []result.GraphNode{}
	for i, m := range entries {
		if !inChain[i] {
			unrelated = append(unrelated, toNode(m))
		}
	}

	return &result.GraphResult{
		Chains:    chains,
		Unrelated: unrelated,
	}
}

// collectChains performs a DFS from every root node (no parent) and emits one
// chain per leaf path of length >= 2. It also returns an inChain bitset so the
// caller can identify unrelated images.
func collectChains(entries []*image.Metadata, directChildren [][]int, hasParent []bool) ([]result.Chain, []bool) {
	n := len(entries)
	inChain := make([]bool, n)
	chains := []result.Chain{}

	for i := range n {
		if !hasParent[i] {
			chains = dfs([]int{i}, entries, directChildren, inChain, chains)
		}
	}

	return chains, inChain
}

func dfs(path []int, entries []*image.Metadata, directChildren [][]int, inChain []bool, chains []result.Chain) []result.Chain {
	current := path[len(path)-1]

	if len(directChildren[current]) == 0 {
		if len(path) < 2 {
			return chains
		}

		nodes := make([]result.GraphNode, len(path))
		for k, idx := range path {
			nodes[k] = toNode(entries[idx])
			inChain[idx] = true
		}

		return append(chains, result.Chain{Nodes: nodes})
	}

	for _, child := range directChildren[current] {
		next := make([]int, len(path)+1)
		copy(next, path)
		next[len(path)] = child
		chains = dfs(next, entries, directChildren, inChain, chains)
	}

	return chains
}

// diffIDKey returns a string that uniquely identifies a DiffID sequence.
// DiffID hex values are fixed-width (64 chars), so concatenation is
// collision-free without a separator.
func diffIDKey(ids []v1.Hash) string {
	var b strings.Builder
	b.Grow(len(ids) * 64)
	for _, id := range ids {
		b.WriteString(id.Hex)
	}
	return b.String()
}

func toNode(m *image.Metadata) result.GraphNode {
	return result.GraphNode{
		Reference:  m.Ref,
		Digest:     m.Digest.String(),
		LayerCount: len(m.DiffIDs),
	}
}
