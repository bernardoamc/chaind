Assess the risk profile of local Docker images using `chaind graph` output and supplementary metadata from the Docker daemon.

## Input

The argument is an optional path to a JSON file containing `chaind graph` output: $ARGUMENTS

If a path was provided, read it with the Read tool and proceed.

If no path was provided, generate the graph automatically:
1. Try `chaind graph > /tmp/chaind-graph.json` first.
2. If that fails with exit code 127 (binary not on PATH), try `./chaind graph > /tmp/chaind-graph.json` from the current working directory.
3. If that also fails, ask the user where the `chaind` binary is located, then run `<path>/chaind graph > /tmp/chaind-graph.json`.

Once generated, read `/tmp/chaind-graph.json` with the Read tool.

## Gather supplementary metadata

Collect all unique image references that appear in the graph (across chains and unrelated).

**Deduplicate by digest first.** If two references share the same digest, only inspect one of them and note the alias relationship in your analysis. This avoids redundant calls and surfaces tag aliasing as a finding.

Run a **single** `docker inspect` call for all unique-by-digest references at once:

```
docker inspect --format '{{index .RepoTags 0}}\t{{.Created}}\t{{.Os}}/{{.Architecture}}\t{{.RootFS.Type}}' <ref1> <ref2> <ref3> ...
```

The output lines are in the same order as the input arguments. Pair each line back to its reference by position.

Collect creation timestamps. Use today's date to compute the age of each image in days.

## Risk assessment

Assign each image a risk level — **LOW**, **MEDIUM**, **HIGH**, or **CRITICAL** — based on the factors below. Combine factors: an image that is old AND has a mutable tag AND is a shared root can escalate to CRITICAL.

### Factor: Age
| Age | Risk |
|-----|------|
| < 90 days | LOW |
| 90–180 days | MEDIUM |
| 180–365 days | HIGH |
| > 365 days | CRITICAL |

Flag the specific age in days next to each image.

### Factor: Mutable tags
Images tagged `:latest` or any tag that does not look like a version pin (semver, digest, or commit SHA) are mutable and a future pull can silently change them. Mark these **MEDIUM** risk at minimum.

### Factor: No detected base (unrelated images)
Images with no detected relationship to any other local image are opaque: their full provenance is unknown. Mark **MEDIUM** at minimum. If they are also old or mutably tagged, escalate.

### Factor: Blast radius
A root image that anchors multiple chains is high-leverage: any vulnerability in it propagates to every derived image. For each root, count **all images that transitively depend on it** (direct children plus their descendants). Score:
- 1 dependent: no escalation
- 2–3 dependents: +1 level
- 4+ dependents: CRITICAL regardless of other factors

### Factor: Layer accumulation
Compare `layer_count` of a derived image to its root. A ratio above 3× suggests many incremental layers that may each introduce packages or secrets. Flag and mark **MEDIUM**.

### Factor: Known risky base patterns
Without pulling CVE data, flag the following based on reference name alone:
- Any image referencing a distribution known to be EOL (e.g. ubuntu:18.04, debian:stretch, alpine:3.15 or older) — **HIGH**
- Scratch-based images (0 layers), note they carry no OS attack surface but also no standard tooling for debugging

## Output format

### Risk summary table
Present a table with columns: Image | Age (days) | Tag type | Base known? | Dependents | Layer ratio | Risk level

### Critical and high findings
For each CRITICAL or HIGH image, explain in plain English why it scored that way and what the concrete consequence is (e.g. "alpine:3.20 is 210 days old and is the root of 3 chains, so a vulnerability discovered in this base would affect all 3 derived images").

### Recommended actions
Prioritise by impact × urgency:
1. Update shared root images first since one pull fixes multiple chains
2. Pin mutable tags to digests or commit SHAs to prevent silent drift
3. Rebuild derived images after any base update to inherit the fix
4. Investigate unrelated images to confirm their provenance
5. Consolidate chains with excessive layer counts using multi-stage builds
