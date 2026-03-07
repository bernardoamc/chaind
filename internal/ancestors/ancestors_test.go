package ancestors

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/bernardoamc/chaind/internal/image"
)

func h(hexStr string) v1.Hash {
	return v1.Hash{Algorithm: "sha256", Hex: hexStr}
}

func makeEntry(ref string, layers ...v1.Hash) *image.Metadata {
	return &image.Metadata{
		Ref:     ref,
		Digest:  v1.Hash{Algorithm: "sha256", Hex: ref},
		DiffIDs: layers,
	}
}

// chainID computes a single ChainID value inline, mirroring the spec formula,
// so tests can derive expected values without depending on the implementation.
func chainID(prev, diffID string) string {
	input := prev + " " + diffID
	sum := sha256.Sum256([]byte(input))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// --- computeChainIDs ---

func TestComputeChainIDsEmpty(t *testing.T) {
	ids := computeChainIDs(nil)
	if len(ids) != 0 {
		t.Errorf("want 0 chain IDs, got %d", len(ids))
	}
}

func TestComputeChainIDsSingleLayer(t *testing.T) {
	// ChainID(L₀) = DiffID(L₀).
	ids := computeChainIDs([]v1.Hash{h("aaa")})
	if len(ids) != 1 {
		t.Fatalf("want 1 chain ID, got %d", len(ids))
	}
	if ids[0] != "sha256:aaa" {
		t.Errorf("want sha256:aaa, got %s", ids[0])
	}
}

func TestComputeChainIDsMultipleLayers(t *testing.T) {
	// Verify the recursive formula against a manually computed value.
	diffIDs := []v1.Hash{h("aaa"), h("bbb"), h("ccc")}
	ids := computeChainIDs(diffIDs)

	if len(ids) != 3 {
		t.Fatalf("want 3 chain IDs, got %d", len(ids))
	}

	want0 := "sha256:aaa"
	want1 := chainID("sha256:aaa", "sha256:bbb")
	want2 := chainID(want1, "sha256:ccc")

	if ids[0] != want0 {
		t.Errorf("depth 0: want %s, got %s", want0, ids[0])
	}
	if ids[1] != want1 {
		t.Errorf("depth 1: want %s, got %s", want1, ids[1])
	}
	if ids[2] != want2 {
		t.Errorf("depth 2: want %s, got %s", want2, ids[2])
	}
}

func TestComputeChainIDsSharedPrefix(t *testing.T) {
	// Two images sharing the first N DiffIDs must have identical ChainIDs
	// through depth N, regardless of what comes after.
	a := computeChainIDs([]v1.Hash{h("L1"), h("L2"), h("L3")})
	b := computeChainIDs([]v1.Hash{h("L1"), h("L2"), h("L4")})

	if a[0] != b[0] {
		t.Errorf("depth 0 should match: %s vs %s", a[0], b[0])
	}
	if a[1] != b[1] {
		t.Errorf("depth 1 should match: %s vs %s", a[1], b[1])
	}
	if a[2] == b[2] {
		t.Errorf("depth 2 should differ after diverging DiffID")
	}
}

func TestComputeChainIDsAllUnique(t *testing.T) {
	// All ChainIDs in a sequence should be distinct.
	ids := computeChainIDs([]v1.Hash{h("L1"), h("L2"), h("L3")})
	seen := make(map[string]struct{})
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			t.Errorf("duplicate chain ID: %s", id)
		}
		seen[id] = struct{}{}
	}
}

func TestComputeChainIDsFormat(t *testing.T) {
	// All ChainIDs should be formatted as "sha256:<64-char hex>".
	ids := computeChainIDs([]v1.Hash{h("L1"), h("L2")})
	for i, id := range ids {
		if !strings.HasPrefix(id, "sha256:") {
			t.Errorf("depth %d: missing sha256: prefix: %s", i, id)
		}
	}
	// depth 1 onward is a proper SHA256 (64 hex chars after the prefix).
	if len(ids) > 1 {
		hex64 := strings.TrimPrefix(ids[1], "sha256:")
		if len(hex64) != 64 {
			t.Errorf("depth 1: expected 64 hex chars, got %d: %s", len(hex64), hex64)
		}
	}
}

// --- buildAncestors ---

func TestBuildAncestorsEmpty(t *testing.T) {
	res := buildAncestors(nil, 2)
	if len(res.Groups) != 0 {
		t.Errorf("want 0 groups, got %d", len(res.Groups))
	}
	if len(res.Ungrouped) != 0 {
		t.Errorf("want 0 ungrouped, got %d", len(res.Ungrouped))
	}
}

func TestBuildAncestorsSingleImage(t *testing.T) {
	entries := []*image.Metadata{makeEntry("alpine:3.21", h("L1"))}
	res := buildAncestors(entries, 2)
	if len(res.Groups) != 0 {
		t.Errorf("want 0 groups, got %d", len(res.Groups))
	}
	if len(res.Ungrouped) != 1 || res.Ungrouped[0] != "alpine:3.21" {
		t.Errorf("want [alpine:3.21] ungrouped, got %v", res.Ungrouped)
	}
}

func TestBuildAncestorsNoSharedLayers(t *testing.T) {
	entries := []*image.Metadata{
		makeEntry("A", h("L1"), h("L2")),
		makeEntry("B", h("L3"), h("L4")),
	}
	res := buildAncestors(entries, 2)
	if len(res.Groups) != 0 {
		t.Errorf("want 0 groups, got %d", len(res.Groups))
	}
	if len(res.Ungrouped) != 2 {
		t.Errorf("want 2 ungrouped, got %d", len(res.Ungrouped))
	}
}

func TestBuildAncestorsBelowMinDepth(t *testing.T) {
	// A and B share only 1 layer; with an explicit minDepth=2 they should not be grouped.
	entries := []*image.Metadata{
		makeEntry("A", h("L1"), h("L2")),
		makeEntry("B", h("L1"), h("L3")),
	}
	res := buildAncestors(entries, 2)
	if len(res.Groups) != 0 {
		t.Errorf("want 0 groups, got %d", len(res.Groups))
	}
	if len(res.Ungrouped) != 2 {
		t.Errorf("want 2 ungrouped, got %d", len(res.Ungrouped))
	}
}

func TestBuildAncestorsZeroLayerExcludedFromGroups(t *testing.T) {
	// A zero-layer image (e.g. scratch) must not participate in any group
	// and must appear in ungrouped.
	entries := []*image.Metadata{
		makeEntry("scratch-based"),            // 0 layers — excluded from grouping
		makeEntry("app1", h("L1"), h("L2")),   // 2 layers — participates
		makeEntry("app2", h("L1"), h("L3")),   // 2 layers — participates
	}
	res := buildAncestors(entries, 0)

	if len(res.Groups) != 1 {
		t.Fatalf("want 1 group, got %d", len(res.Groups))
	}
	for _, img := range res.Groups[0].Images {
		if img == "scratch-based" {
			t.Errorf("zero-layer image 'scratch-based' must not appear in any group")
		}
	}
	if !slices.Contains(res.Ungrouped, "scratch-based") {
		t.Errorf("want 'scratch-based' in ungrouped, got %v", res.Ungrouped)
	}
}

func TestBuildAncestorsSingleLayerExcludedFromGroups(t *testing.T) {
	// A single-layer image must not participate in any group (it is a bare base),
	// but must still appear in ungrouped so callers know it exists.
	entries := []*image.Metadata{
		makeEntry("base", h("L1")),            // 1 layer — excluded from grouping
		makeEntry("app1", h("L1"), h("L2")),   // 2 layers — participates
		makeEntry("app2", h("L1"), h("L3")),   // 2 layers — participates
	}
	res := buildAncestors(entries, 0)

	// app1 and app2 share the L1 layer and should form a group.
	if len(res.Groups) != 1 {
		t.Fatalf("want 1 group, got %d", len(res.Groups))
	}
	for _, img := range res.Groups[0].Images {
		if img == "base" {
			t.Errorf("single-layer image 'base' must not appear in any group")
		}
	}

	// base must appear in ungrouped.
	if !slices.Contains(res.Ungrouped, "base") {
		t.Errorf("want 'base' in ungrouped, got %v", res.Ungrouped)
	}
}

func TestBuildAncestorsZeroMinDepthGroupsOnSingleSharedLayer(t *testing.T) {
	// With minDepth=0, images sharing even a single layer should be grouped
	// (provided they each have more than one layer total).
	entries := []*image.Metadata{
		makeEntry("A", h("L1"), h("L2")),
		makeEntry("B", h("L1"), h("L3")),
	}
	res := buildAncestors(entries, 0)
	if len(res.Groups) != 1 {
		t.Fatalf("want 1 group, got %d", len(res.Groups))
	}
	if res.Groups[0].CommonDepth != 1 {
		t.Errorf("want common_depth 1, got %d", res.Groups[0].CommonDepth)
	}
}

func TestBuildAncestorsAtMinDepth(t *testing.T) {
	// A and B share exactly minDepth layers — should form a group.
	entries := []*image.Metadata{
		makeEntry("A", h("L1"), h("L2"), h("L3")),
		makeEntry("B", h("L1"), h("L2"), h("L4")),
	}
	res := buildAncestors(entries, 2)
	if len(res.Groups) != 1 {
		t.Fatalf("want 1 group, got %d", len(res.Groups))
	}
	g := res.Groups[0]
	if g.CommonDepth != 2 {
		t.Errorf("want common_depth 2, got %d", g.CommonDepth)
	}
	slices.Sort(g.Images)
	if g.Images[0] != "A" || g.Images[1] != "B" {
		t.Errorf("want group [A B], got %v", g.Images)
	}
	if len(res.Ungrouped) != 0 {
		t.Errorf("want 0 ungrouped, got %v", res.Ungrouped)
	}
}

func TestBuildAncestorsDeepestDepthKept(t *testing.T) {
	// A and B share 3 layers. The group should report common_depth=3, not 2 or 1.
	entries := []*image.Metadata{
		makeEntry("A", h("L1"), h("L2"), h("L3"), h("L4")),
		makeEntry("B", h("L1"), h("L2"), h("L3"), h("L5")),
	}
	res := buildAncestors(entries, 1)
	if len(res.Groups) != 1 {
		t.Fatalf("want 1 group, got %d", len(res.Groups))
	}
	if res.Groups[0].CommonDepth != 3 {
		t.Errorf("want common_depth 3, got %d", res.Groups[0].CommonDepth)
	}
}

func TestBuildAncestorsMultipleGroups(t *testing.T) {
	// Two unrelated families: {A, B} and {C, D}.
	entries := []*image.Metadata{
		makeEntry("A", h("L1"), h("L2"), h("L3")),
		makeEntry("B", h("L1"), h("L2"), h("L4")),
		makeEntry("C", h("L5"), h("L6"), h("L7")),
		makeEntry("D", h("L5"), h("L6"), h("L8")),
	}
	res := buildAncestors(entries, 2)
	if len(res.Groups) != 2 {
		t.Fatalf("want 2 groups, got %d", len(res.Groups))
	}
	if len(res.Ungrouped) != 0 {
		t.Errorf("want 0 ungrouped, got %v", res.Ungrouped)
	}
}

func TestBuildAncestorsOverlappingGroups(t *testing.T) {
	// A and B share 3 layers; A, B, and C share 1 layer (minDepth=1).
	// Expected: two groups — {A,B} at depth 3 and {A,B,C} at depth 1.
	entries := []*image.Metadata{
		makeEntry("A", h("L1"), h("L2"), h("L3"), h("L4")),
		makeEntry("B", h("L1"), h("L2"), h("L3"), h("L5")),
		makeEntry("C", h("L1"), h("L9"), h("L10")),
	}
	res := buildAncestors(entries, 1)
	if len(res.Groups) != 2 {
		t.Fatalf("want 2 groups, got %d: %+v", len(res.Groups), res.Groups)
	}

	// Groups are sorted deepest-first.
	if res.Groups[0].CommonDepth != 3 {
		t.Errorf("first group: want common_depth 3, got %d", res.Groups[0].CommonDepth)
	}
	if res.Groups[1].CommonDepth != 1 {
		t.Errorf("second group: want common_depth 1, got %d", res.Groups[1].CommonDepth)
	}

	slices.Sort(res.Groups[0].Images)
	if !slices.Equal(res.Groups[0].Images, []string{"A", "B"}) {
		t.Errorf("first group images: want [A B], got %v", res.Groups[0].Images)
	}

	slices.Sort(res.Groups[1].Images)
	if !slices.Equal(res.Groups[1].Images, []string{"A", "B", "C"}) {
		t.Errorf("second group images: want [A B C], got %v", res.Groups[1].Images)
	}

	if len(res.Ungrouped) != 0 {
		t.Errorf("want 0 ungrouped, got %v", res.Ungrouped)
	}
}

func TestBuildAncestorsMixed(t *testing.T) {
	// A and B form a group; C is unrelated.
	entries := []*image.Metadata{
		makeEntry("A", h("L1"), h("L2"), h("L3")),
		makeEntry("B", h("L1"), h("L2"), h("L4")),
		makeEntry("C", h("L9"), h("L10")),
	}
	res := buildAncestors(entries, 2)
	if len(res.Groups) != 1 {
		t.Fatalf("want 1 group, got %d", len(res.Groups))
	}
	if len(res.Ungrouped) != 1 || res.Ungrouped[0] != "C" {
		t.Errorf("want [C] ungrouped, got %v", res.Ungrouped)
	}
}

func TestBuildAncestorsUngroupedSorted(t *testing.T) {
	// Ungrouped images should be returned in sorted order.
	entries := []*image.Metadata{
		makeEntry("zoo", h("L1")),
		makeEntry("alpha", h("L2")),
		makeEntry("middle", h("L3")),
	}
	res := buildAncestors(entries, 2)
	if !slices.IsSorted(res.Ungrouped) {
		t.Errorf("ungrouped not sorted: %v", res.Ungrouped)
	}
}

func TestBuildAncestorsGroupsSortedDeepestFirst(t *testing.T) {
	// Groups should be ordered deepest common_depth first.
	entries := []*image.Metadata{
		makeEntry("A", h("L1"), h("L2"), h("L3"), h("L4")),
		makeEntry("B", h("L1"), h("L2"), h("L3"), h("L5")),
		makeEntry("C", h("L6"), h("L7"), h("L8")),
		makeEntry("D", h("L6"), h("L7"), h("L9")),
	}
	res := buildAncestors(entries, 2)
	if len(res.Groups) != 2 {
		t.Fatalf("want 2 groups, got %d", len(res.Groups))
	}
	if res.Groups[0].CommonDepth < res.Groups[1].CommonDepth {
		t.Errorf("groups not sorted deepest first: %d then %d", res.Groups[0].CommonDepth, res.Groups[1].CommonDepth)
	}
}

func TestBuildAncestorsCommonChainIDStable(t *testing.T) {
	// The reported common_chain_id must be the ChainID at common_depth, not some other depth.
	entries := []*image.Metadata{
		makeEntry("A", h("L1"), h("L2"), h("L3")),
		makeEntry("B", h("L1"), h("L2"), h("L4")),
	}
	res := buildAncestors(entries, 2)
	if len(res.Groups) != 1 {
		t.Fatalf("want 1 group, got %d", len(res.Groups))
	}

	// Recompute the expected ChainID at depth 2 independently.
	c0 := "sha256:L1"
	c1 := chainID(c0, "sha256:L2")
	if res.Groups[0].CommonChainID != c1 {
		t.Errorf("want common_chain_id %s, got %s", c1, res.Groups[0].CommonChainID)
	}
}
