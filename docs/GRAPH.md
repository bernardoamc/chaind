# Graph Algorithm

`chaind graph` discovers all base-image relationships among the images
currently loaded in the local Docker daemon and returns them as a set of
chains (ordered sequences from root base to most-derived image) plus a
list of images that belong to no chain.

---

## Background: DiffIDs and layer identity

Every Docker/OCI image stores an ordered list of **DiffIDs** in its config
file (`config.RootFS.DiffIDs`). A DiffID is the SHA256 digest of the
**uncompressed** layer tar archive. Because it is computed before
compression, the same filesystem layer always has the same DiffID
regardless of how it was compressed or re-pushed unlike the compressed
layer digest found in the manifest.

Image A is a base of image B if and only if A's DiffID list is a strict
prefix of B's DiffID list. This is cryptographic proof: SHA256 pre-image
resistance means a matching prefix cannot be forged.

### Why not ChainID?

The OCI spec also defines a **ChainID**, computed recursively:

```
ChainID(L₀)  = DiffID(L₀)
ChainID(Lₙ)  = SHA256(ChainID(Lₙ₋₁) + " " + DiffID(Lₙ))
```

ChainID compactly identifies a full layer sequence, but the ChainID of
layer N is opaque, it reveals nothing about the ChainID of layer N-1
without recomputing it. That makes ChainID unsuitable for prefix lookups.
The algorithm below relies on being able to look up any prefix of a
DiffID sequence in O(1), which requires the concatenation-based key
described next.

---

## Fingerprinting a DiffID sequence

Each image is assigned a **fingerprint** by concatenating the hex strings
of its DiffIDs:

```
fingerprint([L₀, L₁, L₂]) = hex(L₀) + hex(L₁) + hex(L₂)
```

Because every DiffID hex value is exactly 64 characters (SHA256), the
concatenation is unambiguous without a separator, no two distinct
sequences can produce the same string. All images are indexed in a map:

```
keyToIdx: fingerprint(image.DiffIDs) → image index
```

The critical property: `fingerprint(seq[:k])` for any k is also a valid
key, so any prefix of any DiffID sequence can be looked up in O(1).

---

## Finding direct parents

For each image B (processed in ascending layer-count order), the algorithm
walks down through prefix lengths from `len(B.DiffIDs)-1` to `1` and
performs a map lookup at each step:

```
for prefixLen := len(B.DiffIDs) - 1; prefixLen >= 1; prefixLen-- {
    if parent, ok := keyToIdx[fingerprint(B.DiffIDs[:prefixLen])]; ok {
        record edge parent → B
        break
    }
}
```

The walk goes from the **shortest** prefix upward, building the key
incrementally by appending one DiffID hex at a time. Each DiffID is
appended once (O(L) string work per image) and the last map hit found is
the longest (direct) parent. Walking from longest downward and breaking on
first match is equivalent but rebuilds the key from scratch on each
iteration, costing O(L²) per image.

Tracking the last match rather than breaking on the first gives transitive
reduction for free. Consider a chain A → B → C:

- When processing C, prefixLen=1 finds A (longestParent=A), then
  prefixLen=2 finds B (longestParent=B). Only the edge B → C is recorded.
- The edge A → C is never added because B superseded A as the longest match.

Without this, a naive O(n²) approach would record both B → C and A → C
and require a separate pass to remove transitive edges.

---

## Complexity

| Step | Complexity |
|---|---|
| Build fingerprint map | O(n × L) |
| Find direct parents | O(n × L) at most L map lookups per image |
| DFS chain traversal | O(n) |
| **Total** | **O(n × L)** |

Where n = number of images, L = average layer count.

The previous O(n²) approach compared every pair of images and then ran a
separate `ContainsFunc` scan to remove transitive edges. With the map the
transitive reduction emerges naturally from the prefix walk, and the
pair-wise scan is eliminated entirely.

---

## Chain construction

After the parent-finding pass the algorithm has a forest of directed edges
(each node has at most one parent). A depth-first traversal starting from
every root (node with no parent) emits one chain per leaf:

```
A → B → C    emits [A, B, C]
A → B → D    emits [A, B, D]
```

Images that are neither a parent nor a child of any other image in the
local daemon are collected separately as **unrelated** nodes.

---

## Output structure

```json
{
  "chains": [
    {
      "nodes": [
        { "reference": "ubuntu:24.04", "digest": "sha256:...", "layer_count": 3 },
        { "reference": "myapp:latest", "digest": "sha256:...", "layer_count": 5 }
      ]
    }
  ],
  "unrelated": [
    { "reference": "scratch-tool:latest", "digest": "sha256:...", "layer_count": 1 }
  ]
}
```

Each node in a chain is ordered from base (fewest layers) to most-derived
(most layers). Images that appear in multiple chains (branching bases) will
be repeated across those chains since each chain is self-contained.
