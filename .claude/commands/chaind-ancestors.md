Analyse a `chaind ancestors` JSON output file to surface useful insights about implied image ancestry and shared layer exposure.

## Input

The argument is an optional path to a JSON file containing `chaind ancestors` output: $ARGUMENTS

If no path was provided, generate the output automatically:
1. Try `chaind ancestors > /tmp/chaind-ancestors.json` first.
2. If that fails with exit code 127 (binary not on PATH), try `./chaind ancestors > /tmp/chaind-ancestors.json` from the current working directory.
3. If that also fails, ask the user where the `chaind` binary is located, then run `<path>/chaind ancestors > /tmp/chaind-ancestors.json`.

Once you have a path, read the file with the Read tool.

## Analysis

Perform the following checks and present findings clearly, grouped by section.

### 1. Summary
- Check `schema_version` — if it is not `1`, warn that the output may not match expectations and stop analysis
- Total number of groups, total images appearing in at least one group, total ungrouped images
- Any warnings present in the output: list them and explain what they mean
- Note whether any images appear in more than one group (this is expected — it means they have both a close relative and a more distant shared ancestor)

### 2. Family groups
For each group, describe it in plain English:
- How many images share this ancestry and at what depth (e.g. "3 images share a 4-layer implied ancestor")
- The `common_chain_id` is the fingerprint of the deepest shared layer sequence — it identifies an ancestor image that may not be present locally
- Characterise depth: `common_depth == 1` is shallow (shared OS base only); `2–4` is moderate (shared intermediate base); `5+` is deep (tightly coupled, likely same application base)
- Flag any group with 4 or more images as high blast-radius: a vulnerability in the implied ancestor affects all of them

### 3. Overlapping groups
- Build an explicit map: for each image that appears in more than one group, list those groups sorted by `common_depth` descending
- For each such image, describe the hierarchy: the deepest group is the closest relationship and the actionable one for updates; shallower groups show wider family context
- Explain that this is informative, not an error

### 4. Implied ancestors
- The `common_chain_id` of each group is the fingerprint of an ancestor that may not be present locally
- Check whether any two groups share the same `common_chain_id` — if so, they are the same implied ancestor at different membership sizes (one group is a subset of the other)
- Check whether the depth-1 group's `common_chain_id` (the shallowest shared layer) is the same across multiple families — if so, those families share a common OS base even though they form separate groups at deeper depths; if they differ, the families are built on distinct base images
- Suggest running `chaind graph` after pulling suspected base images to confirm the implied relationships with known local images

### 5. Ungrouped images
- List ungrouped images, separating them into two buckets:
  - **Intentionally excluded**: single-layer images (e.g. `alpine`, `ubuntu`) — they are bare base images that contribute no grouping signal
  - **Potentially interesting**: multi-layer ungrouped images — their base is not represented locally; possible reasons are genuinely unique layer history, built FROM scratch, or the only image in their family loaded locally
- For each potentially interesting ungrouped image, suggest pulling known related images and re-running to surface the relationship

### 6. Recommendations
- For each group, compute blast radius score = image count × common_depth and present a ranked table: Group description | Images | Depth | Score
- Classify scores: 1–5 = monitor; 6–15 = plan an update; 16+ = act promptly
- For groups scoring 6 or above, recommend pulling the suspected base image locally and running `chaind graph` to identify it precisely
- If multi-layer ungrouped images exist, recommend investigating their provenance
- If any warnings describe skipped images, recommend investigating and re-running once resolved
