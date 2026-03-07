# Concepts

This document explains the concepts needed to understand how `chaind` works.

---

## Container Images

A container image is not a single file. It is a collection of independently addressable pieces assembled according to a spec, that would be either the [OCI Image Spec](https://github.com/opencontainers/image-spec) or the older Docker Image Specification (v2.2). The two are structurally very similar.

An image consists of three kinds of artifacts:

- **A manifest:** an index of what the image contains
- **A config file:** metadata about the image (environment variables, entrypoint, layer history, etc.)
- **Layers:** the actual filesystem content, stored as compressed tar archives

These are stored together in a registry or on disk, and referenced by content-addressable digests (SHA256).

---

## Digests

A digest is a SHA256 hash of some content, written as `sha256:<hex>`. Because SHA256 is collision-resistant, a digest uniquely identifies a specific byte sequence. If two digests match, the content is identical.

This matters because `chaind` relies on cryptographic equality instead of filenames, tags, or labels.

---

## The Manifest

The manifest is a JSON document that describes an image. For a single-platform image it looks like:

```json
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
  "config": {
    "mediaType": "application/vnd.docker.container.image.v1+json",
    "digest": "sha256:abc123...",
    "size": 7023
  },
  "layers": [
    { "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip", "digest": "sha256:aaa...", "size": 30000000 },
    { "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip", "digest": "sha256:bbb...", "size": 5000000 }
  ]
}
```

The manifest digest (the SHA256 of this JSON document) is what a tag like `ubuntu:24.04` resolves to in a registry. Two images with the same manifest digest are byte-for-byte identical, including all layers and config.

The layer digests listed in the manifest are digests of the **compressed** tar archives. These are what get transferred over the network and stored in registries.

---

## The Config File

The config file is also JSON. It holds image metadata: creation time, author, environment variables, labels, entrypoint, and `RootFS.DiffIDs`.

```json
{
  "rootfs": {
    "type": "layers",
    "diff_ids": [
      "sha256:111...",
      "sha256:222..."
    ]
  },
  "config": {
    "Env": ["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],
    "Cmd": ["/bin/bash"]
  }
}
```

The `diff_ids` are digests of the **uncompressed** tar archives. They are produced by the image builder and stored in the config permanently.

---

## DiffIDs vs Layer Digests

There are two ways to identify a layer:

| Property | What it hashes | Where it lives |
|----------|---------------|----------------|
| **Layer digest** | Compressed tar (gzip/zstd) | Manifest `layers[]` |
| **DiffID** | Uncompressed tar | Config `rootfs.diff_ids[]` |

**Why does this distinction matter?**

When a layer is pushed to a registry it may be re-compressed using a different compression level, algorithm, or even a different implementation. Re-compression changes the compressed bytes, which changes the layer digest in the manifest. But the uncompressed filesystem content is unchanged, so the DiffID stays the same.

This means:
- Two registries may store the same layer with different manifest digests (due to re-compression)
- The same layer pulled from two different sources may have different layer digests
- **DiffIDs are stable across re-compression; layer digests are not**

`chaind` compares DiffIDs, not layer digests. This is why the prefix match is a reliable proof of derivation regardless of where the images were pushed or how they were stored.

---

## Layers and the Filesystem

Each layer is a tar archive containing a set of filesystem changes (additions, modifications, deletions) relative to the previous layer. Layers are applied in order, from bottom to top, to produce the final container filesystem via an overlay filesystem driver.

```
Layer 0: base OS files (from ubuntu:24.04)
Layer 1: package installs (apt-get install ...)
Layer 2: application code (COPY . /app)
─────────────────────────────────────────
Final filesystem seen by the container
```

Each `RUN`, `COPY`, or `ADD` instruction in a Dockerfile typically produces one layer. `FROM` itself does not add a new layer, it inherits all layers from the parent image.

---

## Base Images

When you write `FROM ubuntu:24.04` in a Dockerfile, Docker takes all of ubuntu's layers and prepends them to your new image. Your image's `diff_ids` will begin with ubuntu's `diff_ids` followed by your additional layers.

```
ubuntu:24.04  diff_ids: [A, B, C]
myapp:latest  diff_ids: [A, B, C, D, E]
```

If `A, B, C` is a prefix of `myapp`'s diff_ids, then `ubuntu:24.04` is a base image of `myapp:latest`. This is exactly what `chaind` checks.

The prefix check works because layers are immutable and content-addressed. You cannot have `A, B, C` at the start of your diff_ids without having built on top of those exact layers.

---

## Manifest Lists (Multi-Platform Images)

A manifest list (or OCI image index) is a meta-manifest that points to multiple platform-specific manifests:

```json
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.docker.distribution.manifest.list.v2+json",
  "manifests": [
    {
      "digest": "sha256:aaa...",
      "platform": { "os": "linux", "architecture": "amd64" }
    },
    {
      "digest": "sha256:bbb...",
      "platform": { "os": "linux", "architecture": "arm64", "variant": "v8" }
    }
  ]
}
```

When you `docker pull ubuntu:24.04` on an arm64 machine, Docker resolves the manifest list and fetches the `arm64/v8` entry. The tag `ubuntu:24.04` points to the manifest list, but what ends up on disk is the platform-specific image.

`chaind` handles this by detecting a manifest list media type, finding the descriptor that matches the target platform, and loading that specific image for comparison. Both images must be resolved to the same platform before their DiffIDs are compared since comparing an amd64 image against an arm64 image would always produce `NOT_BASE`, even if one was derived from the other.

---

## Why `chaind` Uses the Docker Daemon

`chaind` reads images from the local Docker daemon (`/var/run/docker.sock`) rather than pulling from a registry. This means:

- Images must be pulled or built locally before comparison (`docker pull`, `docker build`)
- No registry credentials are needed
- Comparison works offline
- The daemon provides the config file and manifest via its API; `chaind` uses the `go-containerregistry` library to access these

