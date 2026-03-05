package platform

import (
	"fmt"
	"runtime"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// HostPlatform returns the platform of the current host, normalizing macOS to linux.
func HostPlatform() *v1.Platform {
	os := runtime.GOOS
	if os == "darwin" {
		// Docker Desktop on macOS runs a Linux VM.
		os = "linux"
	}
	arch, variant := normalizeArch(runtime.GOARCH)
	return &v1.Platform{
		OS:           os,
		Architecture: arch,
		Variant:      variant,
	}
}

// Parse parses a platform string like "linux/amd64" or "linux/arm64/v8".
func Parse(s string) (*v1.Platform, error) {
	parts := strings.Split(s, "/")
	if len(parts) < 2 || len(parts) > 3 {
		return nil, fmt.Errorf("invalid platform %q: expected os/arch or os/arch/variant", s)
	}
	p := &v1.Platform{
		OS:           parts[0],
		Architecture: parts[1],
	}
	if len(parts) == 3 {
		p.Variant = parts[2]
	}
	return p, nil
}

// String returns a string representation of a platform.
func String(p *v1.Platform) string {
	if p == nil {
		return ""
	}
	if p.Variant != "" {
		return fmt.Sprintf("%s/%s/%s", p.OS, p.Architecture, p.Variant)
	}
	return fmt.Sprintf("%s/%s", p.OS, p.Architecture)
}

func normalizeArch(goarch string) (arch, variant string) {
	switch goarch {
	case "arm":
		return "arm", "v7"
	default:
		return goarch, ""
	}
}
