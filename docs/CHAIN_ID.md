# ChainID

This document explains what a ChainID is, how it is computed, and how it differs from the DiffIDs that `chaind graph` uses internally.

---

## What is a ChainID?

A **ChainID** is a cumulative, content-addressable identifier for an ordered sequence of layers. It is defined by the [OCI Image Spec](https://github.com/opencontainers/image-spec/blob/main/config.md#layer-chainid) and used by Docker internally for layer storage and cache management.

Where a DiffID identifies a single layer in isolation (the SHA256 of its uncompressed tar archive), a ChainID identifies a layer *in the context of all the layers beneath it*. Two layers with the same content but different histories will have the same DiffID but different ChainIDs.

---

## Computation

ChainIDs are computed recursively over a DiffID sequence:

```
ChainID(L₀)  = DiffID(L₀)
ChainID(Lₙ)  = SHA256(ChainID(Lₙ₋₁) + " " + DiffID(Lₙ))
```

For an image with three layers `[L₀, L₁, L₂]`:

```
ChainID₀ = DiffID(L₀)
ChainID₁ = SHA256("sha256:" + ChainID₀ + " " + DiffID(L₁))
ChainID₂ = SHA256("sha256:" + ChainID₁ + " " + DiffID(L₂))
```

`ChainID₂` encodes the entire layer history: L₀, then L₁, then L₂, in that order. Changing any layer in the sequence, or reordering them, produces a different ChainID at every depth above the change.

---

## ChainID vs DiffID

| Property | DiffID | ChainID |
|----------|--------|---------|
| What it identifies | A single layer's uncompressed content | A layer *and* the full sequence of layers beneath it |
| Depends on history | No | Yes |
| Stable across re-compression | Yes | Yes (inherits from DiffID) |
| Used for | Ancestry comparison (`chaind graph`, `chaind compare`) | Layer storage deduplication, cache invalidation, implied ancestry grouping |

The key difference: two images can have the same DiffID at position N (same layer content) but different ChainIDs at position N if the layers beneath N differ. ChainID is therefore a stricter identity — it does not just say "this layer has this content", it says "this layer, built on exactly this history, has this content".

---

## The ancestry property

The recursive definition gives ChainIDs a useful property for ancestry analysis:

> Two images share ChainID at depth K if and only if they have the same first K+1 layers, in the same order.

This holds even if neither image has the other as a local image, and even if no image representing exactly those K+1 layers exists locally at all.

Given three images:

```
Image A  layers: [L₀, L₁, L₂, L₃]
Image B  layers: [L₀, L₁, L₂, L₄]
Image C  layers: [L₀, L₁, L₅, L₆]
```

Their ChainIDs at each depth are:

```
        depth 0   depth 1   depth 2   depth 3
Image A   C₀        C₀₁      C₀₁₂     C₀₁₂₃
Image B   C₀        C₀₁      C₀₁₂     C₀₁₂₄
Image C   C₀        C₀₁      C₀₁₅     C₀₁₅₆
```

- A and B share ChainID through depth 2 (`C₀₁₂`): they derive from the same three-layer ancestor.
- A, B, and C share ChainID through depth 1 (`C₀₁`): they derive from the same two-layer ancestor.
- The shared ancestor at depth 2 need not be a local image for this to hold.

---

## Why `chaind graph` uses DiffIDs instead

`chaind graph` needs to find the *longest DiffID prefix* that matches a known local image. ChainIDs are opaque: the ChainID at depth N reveals nothing about the ChainID at depth N-1 without recomputing it. DiffID sequences can be fingerprinted by simple concatenation, enabling O(1) prefix lookups. ChainIDs cannot.

See [GRAPH.md](GRAPH.md) for a detailed description of the prefix-lookup algorithm.

---

## ChainIDs and `chaind ancestors`

Because ChainIDs encode cumulative history, they serve as stable group identifiers for images that share a common ancestor even when that ancestor is not available locally. `chaind ancestors` (planned) will compute the ChainID sequence for every local image and group images by their longest common ChainID, surfacing implied ancestry that `chaind graph` cannot see.
