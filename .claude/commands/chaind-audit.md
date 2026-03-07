Audit a `chaind graph` JSON output file to surface useful insights about local image chain relationships.

## Input

The argument is an optional path to a JSON file containing `chaind graph` output: $ARGUMENTS

If no path was provided, generate the graph automatically:
1. Try `chaind graph > /tmp/chaind-graph.json` first.
2. If that fails with exit code 127 (binary not on PATH), try `./chaind graph > /tmp/chaind-graph.json` from the current working directory.
3. If that also fails, ask the user where the `chaind` binary is located, then run `<path>/chaind graph > /tmp/chaind-graph.json`.

Once you have a path, read the file with the Read tool.

## Analysis

Perform the following checks and present findings clearly, grouped by section:

### 1. Summary
- Total number of chains, total nodes across all chains, number of unrelated images
- Any warnings present in the output: list them and explain what they mean

### 2. Chain structure
- For each chain, describe the lineage in plain English (e.g. "alpine:3.21 → chaind-base:latest → chaind-derived:latest, 3 images deep")
- Identify root images (those with `"parent": null` inside a chain) since these are the base images everything else builds on
- Flag any chain with more than 5 images as deep and worth reviewing

### 3. Shared roots
- Identify root images that appear as the base of more than one chain since they are high-leverage: a vulnerability or update there propagates to all derived images
- List which chains each shared root anchors

### 4. Unrelated images
- List unrelated images and note that they have no detected base relationship with any other local image
- Possible reasons: pulled from a registry with no local base, built FROM scratch, or the base was removed
- Compare digests across all unrelated images. If two references share the same digest, flag them as aliases of each other. A mutable tag (e.g. `:latest`) aliasing a pinned tag means a future push to the mutable tag will silently diverge

### 5. Recommendations
- Suggest which images are safest to update first (leaf nodes, no other image depends on them)
- If any `warnings` entries describe skipped images, recommend re-running `chaind graph` after investigating those images
- If the output has chains with very large layer counts relative to their root, note that the derived image may be accumulating unnecessary layers
