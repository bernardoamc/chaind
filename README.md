# chaind

A CLI tool that determines base image relationships between container images by comparing their ordered layer DiffIDs (SHA256 of uncompressed tar).

The name is a nod to `containerd` and "chain derivation".

## How it works

Layer comparison uses **DiffIDs** from each image's config (`RootFS.DiffIDs`), not the compressed layer digests found in manifests. This correctly handles re-compressed layers that would otherwise produce different digests despite identical content.

A DiffID prefix match is cryptographic proof: if every layer SHA256 in image A appears in order at the start of image B's layer list, A is a base of B.

```
if images share a digest           → SAME_IMAGE
if neither's DiffIDs prefix other  → NOT_BASE
if one's DiffIDs ⊆ other's DiffIDs → CONFIRMED_BASE
```

**Argument order does not matter.** `chaind compare` tries both directions automatically. The `base` and `derived` fields in the output always reflect the detected relationship, regardless of which image was passed first.

## Installation

```bash
git clone https://github.com/bernardoamc/chaind.git
cd chaind
go build -o chaind .
```

Requires Go 1.24+ and a running Docker daemon.

## Usage

```
chaind [command]

Commands:
  compare   Determine the base image relationship between two images
  graph     Map base image relationships across all local images

Global flags:
  --socket string   Docker socket path (overrides DOCKER_HOST; default: /var/run/docker.sock)
```

The `--socket` flag is only needed when your daemon is at a non-standard path (e.g. Podman, a remote socket, or a custom Docker context). In the common case (Docker Desktop or a standard Linux install) both commands find the daemon automatically.

```
chaind compare <image1> <image2> [flags]

Flags:
  --platform string   Target platform, e.g. linux/arm64/v8 (default: host)
```

```
chaind graph
```

Both commands output JSON.

## Examples

### compare

```bash
# Is there a base relationship? (order doesn't matter)
chaind compare alpine:3.21 myapp:latest
chaind compare myapp:latest alpine:3.21   # same result

# Pipe to jq
chaind compare alpine:3.21 myapp:latest | jq .verdict

# Override platform
chaind compare --platform linux/arm64 alpine:3.21 myapp:latest
```

Output (`CONFIRMED_BASE`):

```json
{
  "verdict": "CONFIRMED_BASE",
  "platform": "linux/amd64",
  "base": "alpine:3.21",
  "derived": "myapp:latest",
  "matched_layers": [{ "index": 0, "digest": "sha256:...", "diff_id": "sha256:..." }],
  "extra_layers":   [{ "index": 1, "digest": "sha256:...", "diff_id": "sha256:..." },
                     { "index": 2, "digest": "sha256:...", "diff_id": "sha256:..." }],
  "images": {
    "alpine:3.21":  { "digest": "sha256:...", "layer_count": 1, "media_type": "..." },
    "myapp:latest": { "digest": "sha256:...", "layer_count": 3, "media_type": "..." }
  }
}
```

Output (`NOT_BASE` / `SAME_IMAGE`):

```json
{
  "verdict": "NOT_BASE",
  "platform": "linux/amd64",
  "base": null,
  "derived": null,
  "matched_layers": [],
  "extra_layers": [],
  "images": {
    "alpine:3.20":  { "digest": "sha256:...", "layer_count": 1, "media_type": "..." },
    "myapp:latest": { "digest": "sha256:...", "layer_count": 3, "media_type": "..." }
  }
}
```

### graph

Scans all images in the local Docker daemon and maps their base image relationships. Always uses the host platform.

```bash
chaind graph | jq .
```

Output:

```json
{
  "chains": [
    {
      "nodes": [
        { "reference": "alpine:3.21", "digest": "sha256:...", "layer_count": 1 },
        { "reference": "myapp:latest", "digest": "sha256:...", "layer_count": 3 }
      ]
    },
    {
      "nodes": [
        { "reference": "alpine:3.21",  "digest": "sha256:...", "layer_count": 1 },
        { "reference": "base:latest",  "digest": "sha256:...", "layer_count": 3 },
        { "reference": "derived:latest", "digest": "sha256:...", "layer_count": 4 }
      ]
    }
  ],
  "unrelated": [
    { "reference": "postgres:16", "digest": "sha256:...", "layer_count": 8 }
  ]
}
```

When a base image has multiple derived images, each derivation appears as a separate chain with the shared root repeated. Images with no detected relationship to any other local image appear in `unrelated`.

## Exit codes (`compare` only)

| Code | Meaning |
|------|---------|
| 0    | `CONFIRMED_BASE` |
| 1    | `NOT_BASE` |
| 2    | `SAME_IMAGE` |
| 10   | Fatal error |

Exit codes make `chaind compare` suitable for use in CI pipelines and shell scripts.

## Testing

```bash
# Unit tests
go test ./...

# Integration tests (requires a running Docker daemon)
./integration_tests.sh
```

## Project structure

```
chaind/
├── main.go
├── cmd/
│   ├── root.go             # Cobra dispatcher, global flags, exit code handling
│   ├── compare.go          # chaind compare subcommand
│   └── graph.go            # chaind graph subcommand
└── internal/
    ├── result/
    │   └── result.go       # Shared types: Verdict, CompareResult, GraphResult
    ├── platform/
    │   └── platform.go     # Host platform detection, --platform parsing
    ├── image/
    │   └── loader.go       # Load images from Docker daemon, list local refs
    ├── compare/
    │   └── compare.go      # DiffID prefix algorithm
    ├── graph/
    │   └── graph.go        # Graph building: concurrent loading, chain detection
    └── output/
        └── json.go         # JSON renderer
```

## Dependencies

- [`github.com/google/go-containerregistry`](https://github.com/google/go-containerregistry) — image loading from Docker daemon, OCI types
- [`github.com/docker/docker`](https://github.com/moby/moby) — listing local images via Docker client
- [`github.com/spf13/cobra`](https://github.com/spf13/cobra) — CLI framework
- [`golang.org/x/sync`](https://pkg.go.dev/golang.org/x/sync) — concurrent image loading in `graph`
