# chaind

A CLI tool that determines whether one container image is the base of another by comparing their ordered layer DiffIDs (SHA256 of uncompressed tar). OCI annotations and Docker labels are used as supplementary evidence to annotate the verdict.

The name is a nod to `containerd` and "chain derivation".

## How it works

Layer comparison uses **DiffIDs** from each image's config (`RootFS.DiffIDs`), not the compressed layer digests found in manifests. This correctly handles re-compressed layers that would otherwise produce different digests despite identical content.

A DiffID prefix match is cryptographic proof, if every layer SHA256 in image A appears in order at the start of image B's layer list, A is a base of B.

**Argument order does not matter.** `chaind` tries both directions automatically. In the output, `image_a` is always the base and `image_b` is always the derived image, regardless of the order you passed them.

```
if images share a digest           → SAME_IMAGE
if neither's DiffIDs prefix other  → NOT_BASE
if one's DiffIDs ⊆ other's DiffIDs → CONFIRMED_BASE  (base → image_a, derived → image_b)
if the base has no layers          → EMPTY_BASE
```

OCI annotations (`org.opencontainers.image.base.name/digest`) and Docker labels are read from the derived image for informational display. If an annotation digest is present but contradicts the layer match, a warning is emitted.

## Installation

```bash
git clone https://github.com/bernardoamc/chaind.git
cd chaind
go build -o chaind .
```

Requires Go 1.23+ and a running Docker daemon.

## Usage

```
chaind <image1> <image2> [flags]

Flags:
  --platform string   Target platform, e.g. linux/arm64/v8 (default: host)
  --json              Machine-readable JSON output
  --no-color          Disable ANSI colors
  -q, --quiet         Only the verdict line and warnings
  --socket string     Docker socket path (default: DOCKER_HOST or /var/run/docker.sock)
```

Both images must be present in the local Docker daemon (`docker pull` them first).

## Examples

```bash
# Is there a base relationship between these two images? (order doesn't matter)
chaind alpine:3.21 myapp:latest
chaind myapp:latest alpine:3.21   # same result

# JSON output
chaind alpine:3.21 myapp:latest --json | jq .verdict

# Different platform
chaind --platform linux/arm64 alpine:3.21 myapp:latest

# Quiet — just the verdict line
chaind alpine:3.21 myapp:latest --quiet
```

### Text output

```
CONFIRMED BASE  alpine:3.21 is a base image of myapp:latest

  Platform      linux/amd64
  Image A       alpine:3.21          sha256:a1b2c3d4...  (1 layers)
  Image B       myapp:latest         sha256:e5f6a7b8...  (3 layers)

  Layer comparison
  ────────────────────────────────────────────────────────────────────
   #   DiffID                          Status
   0   sha256:001122334455...          matched
   1   sha256:334455667788...          extra (B only)
   2   sha256:667788990011...          extra (B only)
  ────────────────────────────────────────────────────────────────────
   Matched 1/1 layers from base. Derived image adds 2 layer(s).

  Metadata evidence
   base.name annotation:         (not set)
   base.digest annotation:       (not set)
```

### JSON output

```json
{
  "verdict": "CONFIRMED_BASE",
  "platform": "linux/amd64",
  "image_a": { "reference": "alpine:3.21", "digest": "sha256:...", "layer_count": 1, "media_type": "..." },
  "image_b": { "reference": "myapp:latest", "digest": "sha256:...", "layer_count": 3, "media_type": "..." },
  "matched_layers": [{ "index": 0, "digest": "sha256:...", "diff_id": "sha256:..." }],
  "extra_layers":   [{ "index": 1, "digest": "sha256:...", "diff_id": "sha256:..." },
                     { "index": 2, "digest": "sha256:...", "diff_id": "sha256:..." }]
}
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0    | `CONFIRMED_BASE` |
| 1    | `NOT_BASE` |
| 2    | `SAME_IMAGE` |
| 10   | Fatal error |

Exit codes make `chaind` suitable for use in CI pipelines and shell scripts.

## Testing

```bash
# Unit tests
go test ./...

# Integration tests (requires a running Docker daemon)
./test.sh
```

## Project structure

```
chaind/
├── main.go
├── cmd/
│   └── root.go             # Cobra CLI, flag wiring, exit codes
└── internal/
    ├── result/
    │   └── result.go       # Shared types: Verdict, CompareResult, LayerInfo
    ├── platform/
    │   └── platform.go     # Host platform detection, --platform parsing
    ├── image/
    │   └── loader.go       # Load images from Docker daemon, resolve manifest lists
    ├── compare/
    │   └── compare.go      # DiffID prefix algorithm, annotation evidence
    └── output/
        ├── text.go         # ANSI terminal renderer
        └── json.go         # JSON renderer
```

## Dependencies

- [`github.com/google/go-containerregistry`](https://github.com/google/go-containerregistry) — image loading from Docker daemon, OCI types
- [`github.com/spf13/cobra`](https://github.com/spf13/cobra) — CLI framework
- [`golang.org/x/term`](https://pkg.go.dev/golang.org/x/term) — TTY detection for color output
