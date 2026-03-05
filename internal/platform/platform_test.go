package platform_test

import (
	"testing"

	"github.com/bernardoamc/chaind/internal/platform"
)

func TestParse(t *testing.T) {
	tests := []struct {
		input       string
		wantOS      string
		wantArch    string
		wantVariant string
		wantErr     bool
	}{
		{"linux/amd64", "linux", "amd64", "", false},
		{"linux/arm64", "linux", "arm64", "", false},
		{"linux/arm64/v8", "linux", "arm64", "v8", false},
		{"linux/arm/v7", "linux", "arm", "v7", false},
		{"windows/amd64", "windows", "amd64", "", false},
		{"invalid", "", "", "", true},
		{"too/many/parts/here", "", "", "", true},
	}
	for _, tt := range tests {
		p, err := platform.Parse(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("Parse(%q): expected error, got nil", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("Parse(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if p.OS != tt.wantOS {
			t.Errorf("Parse(%q).OS = %q, want %q", tt.input, p.OS, tt.wantOS)
		}
		if p.Architecture != tt.wantArch {
			t.Errorf("Parse(%q).Architecture = %q, want %q", tt.input, p.Architecture, tt.wantArch)
		}
		if p.Variant != tt.wantVariant {
			t.Errorf("Parse(%q).Variant = %q, want %q", tt.input, p.Variant, tt.wantVariant)
		}
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"linux/amd64", "linux/amd64"},
		{"linux/arm64/v8", "linux/arm64/v8"},
		{"windows/amd64", "windows/amd64"},
	}
	for _, tt := range tests {
		p, err := platform.Parse(tt.input)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tt.input, err)
		}
		if got := platform.String(p); got != tt.want {
			t.Errorf("String(Parse(%q)) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
