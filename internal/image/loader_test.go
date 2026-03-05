package image_test

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/bernardoamc/chaind/internal/image"
)

func TestPlatformSatisfies(t *testing.T) {
	tests := []struct {
		name      string
		candidate *v1.Platform
		target    *v1.Platform
		want      bool
	}{
		{
			name:      "exact match no variant",
			candidate: &v1.Platform{OS: "linux", Architecture: "amd64"},
			target:    &v1.Platform{OS: "linux", Architecture: "amd64"},
			want:      true,
		},
		{
			name:      "exact match with variant",
			candidate: &v1.Platform{OS: "linux", Architecture: "arm64", Variant: "v8"},
			target:    &v1.Platform{OS: "linux", Architecture: "arm64", Variant: "v8"},
			want:      true,
		},
		{
			name:      "target has no variant, candidate does — still matches",
			candidate: &v1.Platform{OS: "linux", Architecture: "arm64", Variant: "v8"},
			target:    &v1.Platform{OS: "linux", Architecture: "arm64"},
			want:      true,
		},
		{
			name:      "target requires variant, candidate has none — no match",
			candidate: &v1.Platform{OS: "linux", Architecture: "arm64"},
			target:    &v1.Platform{OS: "linux", Architecture: "arm64", Variant: "v8"},
			want:      false,
		},
		{
			name:      "target requires variant, candidate has different variant — no match",
			candidate: &v1.Platform{OS: "linux", Architecture: "arm", Variant: "v6"},
			target:    &v1.Platform{OS: "linux", Architecture: "arm", Variant: "v7"},
			want:      false,
		},
		{
			name:      "OS mismatch",
			candidate: &v1.Platform{OS: "windows", Architecture: "amd64"},
			target:    &v1.Platform{OS: "linux", Architecture: "amd64"},
			want:      false,
		},
		{
			name:      "architecture mismatch",
			candidate: &v1.Platform{OS: "linux", Architecture: "arm64"},
			target:    &v1.Platform{OS: "linux", Architecture: "amd64"},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := image.PlatformSatisfies(tt.candidate, tt.target)
			if got != tt.want {
				t.Errorf("PlatformSatisfies(%v, %v) = %v, want %v",
					tt.candidate, tt.target, got, tt.want)
			}
		})
	}
}
