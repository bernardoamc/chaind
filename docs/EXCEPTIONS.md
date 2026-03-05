# Exceptions: When Base Detection Does Not Work

`chaind` detects base image relationships by checking whether image A's DiffIDs are a prefix of image B's DiffIDs. This is a cryptographic proof when it holds, but there are cases where the relationship exists conceptually yet the DiffID prefix is absent. `chaind` will correctly report `NOT_BASE` in all of these cases since they are true negatives.

---

## Multi-Stage Builds

The most common case. A multi-stage Dockerfile uses multiple `FROM` instructions, but only the final stage's layers appear in the output image. Layers from intermediate stages are discarded.

```dockerfile
FROM golang:1.24 AS builder
RUN go build -o /app .

FROM ubuntu:24.04
COPY --from=builder /app /app
```

The resulting image starts fresh from `ubuntu:24.04`. The `golang:1.24` layers are completely absent from the final image. Running `chaind golang:1.24 myapp:latest` returns `NOT_BASE`, even though golang was used during the build.

`chaind ubuntu:24.04 myapp:latest` would return `CONFIRMED_BASE` since that relationship is real and detectable.

---

## Layer Squashing

Some tools collapse all layers into one, discarding the layer history:

- `docker export <container> | docker import -`: exports and re-imports the filesystem as a single layer
- `docker build --squash`: merges all new layers into one after the build
- Image optimization tools like `docker-slim`

The resulting image contains a single layer with no DiffID relationship to the original base. Even if the filesystem content is identical to a layered build, the DiffID is a hash of a different tar archive.

---

## `FROM scratch` with a Static Binary

Images built on `scratch` have no base layers at all. This is common for Go microservices:

```dockerfile
FROM scratch
COPY myapp /myapp
ENTRYPOINT ["/myapp"]
```

There is nothing to match a prefix against. `chaind scratch myapp:latest` would not be a meaningful comparison in any case, since `scratch` cannot be loaded from the Docker daemon.

---

## Rebasing

Tools like `crane rebase` swap out the base layers of an image with a newer version. After rebasing:

- The old base DiffIDs are replaced with the new base's DiffIDs
- The prefix relationship with the original base is broken
- The relationship with the new base is established

This is intentional since the image has been updated to a new base, and `chaind` reflects that correctly.

---

## Hermetic Builds That Reconstruct the Filesystem

Build systems like Bazel can construct images in two fundamentally different ways:

**Composing on top of a pulled base (DiffIDs preserved)**

If the build pulls `ubuntu:24.04` via `oci_pull` and adds layers on top, the ubuntu layers pass through unchanged. Their DiffIDs are identical to the originals. `chaind` will correctly identify the base relationship.

**Assembling the entire image from scratch**

If the build assembles the full filesystem content independently without pulling and reusing the original layer tar archives then the DiffIDs will differ even if the visible file content is byte-for-byte identical.

This is because DiffIDs hash the tar *serialization* of a layer, not the logical filesystem content. The same files can be serialized into a tar archive in many ways (different file ordering, layer grouping, metadata handling), each producing a different hash. Two independently-assembled images that look identical to a user will have unrelated DiffIDs.

The practical rule: if a build tool *pulls and passes through* existing layers, the DiffIDs are preserved. If it *reconstructs* layers from source, they are not.
