package graph

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/bernardoamc/chaind/internal/image"
)

func h(hex string) v1.Hash {
	return v1.Hash{Algorithm: "sha256", Hex: hex}
}

func makeEntry(ref string, layers ...v1.Hash) *image.Metadata {
	return &image.Metadata{
		Ref:     ref,
		Digest:  v1.Hash{Algorithm: "sha256", Hex: ref},
		DiffIDs: layers,
	}
}

func TestBuildGraphEmpty(t *testing.T) {
	res := buildGraph(nil)
	if len(res.Chains) != 0 {
		t.Errorf("want 0 chains, got %d", len(res.Chains))
	}
	if len(res.Unrelated) != 0 {
		t.Errorf("want 0 unrelated, got %d", len(res.Unrelated))
	}
}

func TestBuildGraphSingleImage(t *testing.T) {
	entries := []*image.Metadata{
		makeEntry("alpine:3.21", h("L1")),
	}
	res := buildGraph(entries)
	if len(res.Chains) != 0 {
		t.Errorf("want 0 chains, got %d", len(res.Chains))
	}
	if len(res.Unrelated) != 1 {
		t.Errorf("want 1 unrelated, got %d", len(res.Unrelated))
	}
	if res.Unrelated[0].Reference != "alpine:3.21" {
		t.Errorf("want unrelated alpine:3.21, got %s", res.Unrelated[0].Reference)
	}
}

func TestBuildGraphTwoUnrelated(t *testing.T) {
	entries := []*image.Metadata{
		makeEntry("alpine:3.21", h("L1")),
		makeEntry("ubuntu:24.04", h("L2")),
	}
	res := buildGraph(entries)
	if len(res.Chains) != 0 {
		t.Errorf("want 0 chains, got %d", len(res.Chains))
	}
	if len(res.Unrelated) != 2 {
		t.Errorf("want 2 unrelated, got %d", len(res.Unrelated))
	}
}

func TestBuildGraphEqualLayerCounts(t *testing.T) {
	entries := []*image.Metadata{
		makeEntry("imageA", h("L1"), h("L2")),
		makeEntry("imageB", h("L1"), h("L3")), // same count, different second layer
	}
	res := buildGraph(entries)
	if len(res.Chains) != 0 {
		t.Errorf("want 0 chains, got %d", len(res.Chains))
	}
	if len(res.Unrelated) != 2 {
		t.Errorf("want 2 unrelated, got %d", len(res.Unrelated))
	}
}

func TestBuildGraphSimpleChain(t *testing.T) {
	entries := []*image.Metadata{
		makeEntry("base", h("L1")),
		makeEntry("derived", h("L1"), h("L2")),
	}
	res := buildGraph(entries)
	if len(res.Chains) != 1 {
		t.Fatalf("want 1 chain, got %d", len(res.Chains))
	}
	nodes := res.Chains[0].Nodes
	if len(nodes) != 2 {
		t.Fatalf("want 2 nodes in chain, got %d", len(nodes))
	}
	if nodes[0].Reference != "base" || nodes[1].Reference != "derived" {
		t.Errorf("want chain [base, derived], got [%s, %s]", nodes[0].Reference, nodes[1].Reference)
	}
	if len(res.Unrelated) != 0 {
		t.Errorf("want 0 unrelated, got %d", len(res.Unrelated))
	}
}

func TestBuildGraphLinearChain(t *testing.T) {
	// A → B → C should produce one chain [A, B, C], not [A, B, C] + [A, C].
	entries := []*image.Metadata{
		makeEntry("A", h("L1")),
		makeEntry("B", h("L1"), h("L2")),
		makeEntry("C", h("L1"), h("L2"), h("L3")),
	}
	res := buildGraph(entries)
	if len(res.Chains) != 1 {
		t.Fatalf("want 1 chain, got %d", len(res.Chains))
	}
	nodes := res.Chains[0].Nodes
	if len(nodes) != 3 {
		t.Fatalf("want 3 nodes in chain, got %d", len(nodes))
	}
	refs := []string{nodes[0].Reference, nodes[1].Reference, nodes[2].Reference}
	if refs[0] != "A" || refs[1] != "B" || refs[2] != "C" {
		t.Errorf("want chain [A, B, C], got %v", refs)
	}
	if len(res.Unrelated) != 0 {
		t.Errorf("want 0 unrelated, got %d", len(res.Unrelated))
	}
}

func TestBuildGraphBranching(t *testing.T) {
	// A → B and A → C (different second layers).
	entries := []*image.Metadata{
		makeEntry("A", h("L1")),
		makeEntry("B", h("L1"), h("L2")),
		makeEntry("C", h("L1"), h("L3")),
	}
	res := buildGraph(entries)
	if len(res.Chains) != 2 {
		t.Fatalf("want 2 chains, got %d", len(res.Chains))
	}
	// Both chains should start with A.
	for i, chain := range res.Chains {
		if chain.Nodes[0].Reference != "A" {
			t.Errorf("chain %d: want root A, got %s", i, chain.Nodes[0].Reference)
		}
		if len(chain.Nodes) != 2 {
			t.Errorf("chain %d: want 2 nodes, got %d", i, len(chain.Nodes))
		}
	}
	if len(res.Unrelated) != 0 {
		t.Errorf("want 0 unrelated, got %d", len(res.Unrelated))
	}
}

func TestBuildGraphPlatformPropagated(t *testing.T) {
	// Platform from Metadata must appear on every GraphNode — both chain nodes
	// and unrelated nodes — regardless of position in the chain.
	entries := []*image.Metadata{
		{Ref: "base", Digest: h("base"), DiffIDs: []v1.Hash{h("L1")}, Platform: "linux/amd64"},
		{Ref: "derived", Digest: h("derived"), DiffIDs: []v1.Hash{h("L1"), h("L2")}, Platform: "linux/amd64"},
		{Ref: "unrelated", Digest: h("unrelated"), DiffIDs: []v1.Hash{h("L9")}, Platform: "linux/arm64"},
	}
	res := buildGraph(entries)

	for _, chain := range res.Chains {
		for _, node := range chain.Nodes {
			if node.Platform != "linux/amd64" {
				t.Errorf("chain node %s: want platform linux/amd64, got %q", node.Reference, node.Platform)
			}
		}
	}
	for _, node := range res.Unrelated {
		if node.Platform != "linux/arm64" {
			t.Errorf("unrelated node %s: want platform linux/arm64, got %q", node.Reference, node.Platform)
		}
	}
}

func TestBuildGraphMixed(t *testing.T) {
	// A → B, C is unrelated.
	entries := []*image.Metadata{
		makeEntry("A", h("L1")),
		makeEntry("B", h("L1"), h("L2")),
		makeEntry("C", h("L9")),
	}
	res := buildGraph(entries)
	if len(res.Chains) != 1 {
		t.Fatalf("want 1 chain, got %d", len(res.Chains))
	}
	if len(res.Unrelated) != 1 {
		t.Fatalf("want 1 unrelated, got %d", len(res.Unrelated))
	}
	if res.Unrelated[0].Reference != "C" {
		t.Errorf("want unrelated C, got %s", res.Unrelated[0].Reference)
	}
}
