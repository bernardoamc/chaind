package compare_test

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/bernardoamc/chaind/internal/compare"
	"github.com/bernardoamc/chaind/internal/image"
	"github.com/bernardoamc/chaind/internal/result"
)

const testPlatform = "linux/amd64"

func randomImage(t *testing.T, layers int64) v1.Image {
	t.Helper()
	img, err := random.Image(512, layers)
	if err != nil {
		t.Fatalf("random.Image(%d): %v", layers, err)
	}
	return img
}

func randomLayer(t *testing.T) v1.Layer {
	t.Helper()
	layer, err := random.Layer(512, types.DockerLayer)
	if err != nil {
		t.Fatalf("random.Layer: %v", err)
	}
	return layer
}

func appendLayer(t *testing.T, base v1.Image, layer v1.Layer) v1.Image {
	t.Helper()
	img, err := mutate.AppendLayers(base, layer)
	if err != nil {
		t.Fatalf("mutate.AppendLayers: %v", err)
	}
	return img
}

func extractOrFatal(t *testing.T, ref string, img v1.Image) *image.Metadata {
	t.Helper()
	m, err := image.Extract(ref, img)
	if err != nil {
		t.Fatalf("Extract(%q): %v", ref, err)
	}
	return m
}

func TestCompare_ConfirmedBase_SingleLayer(t *testing.T) {
	base := randomImage(t, 1)
	derived := appendLayer(t, base, randomLayer(t))

	res := compare.Compare(extractOrFatal(t, "base:latest", base), extractOrFatal(t, "derived:latest", derived), testPlatform)

	if res.Verdict != result.VerdictConfirmedBase {
		t.Errorf("verdict = %s, want CONFIRMED_BASE", res.Verdict)
	}
	if res.Base == nil || *res.Base != "base:latest" {
		t.Errorf("Base = %v, want %q", res.Base, "base:latest")
	}
	if res.Derived == nil || *res.Derived != "derived:latest" {
		t.Errorf("Derived = %v, want %q", res.Derived, "derived:latest")
	}
	if len(res.MatchedLayers) != 1 {
		t.Errorf("matched layers = %d, want 1", len(res.MatchedLayers))
	}
	if len(res.ExtraLayers) != 1 {
		t.Errorf("extra layers = %d, want 1", len(res.ExtraLayers))
	}
}

func TestCompare_ConfirmedBase_MultiLayer(t *testing.T) {
	base := randomImage(t, 3)
	derived := appendLayer(t, appendLayer(t, base, randomLayer(t)), randomLayer(t))

	res := compare.Compare(extractOrFatal(t, "base:latest", base), extractOrFatal(t, "derived:latest", derived), testPlatform)

	if res.Verdict != result.VerdictConfirmedBase {
		t.Errorf("verdict = %s, want CONFIRMED_BASE", res.Verdict)
	}
	if len(res.MatchedLayers) != 3 {
		t.Errorf("matched layers = %d, want 3", len(res.MatchedLayers))
	}
	if len(res.ExtraLayers) != 2 {
		t.Errorf("extra layers = %d, want 2", len(res.ExtraLayers))
	}
}

func TestCompare_ConfirmedBase_ReversedArgs(t *testing.T) {
	base := randomImage(t, 2)
	derived := appendLayer(t, base, randomLayer(t))

	res := compare.Compare(extractOrFatal(t, "derived:latest", derived), extractOrFatal(t, "base:latest", base), testPlatform)

	if res.Verdict != result.VerdictConfirmedBase {
		t.Errorf("verdict = %s, want CONFIRMED_BASE", res.Verdict)
	}
	// Base must always be identified correctly regardless of argument order.
	if res.Base == nil || *res.Base != "base:latest" {
		t.Errorf("Base = %v, want %q", res.Base, "base:latest")
	}
	if len(res.MatchedLayers) != 2 {
		t.Errorf("matched layers = %d, want 2", len(res.MatchedLayers))
	}
}

func TestCompare_NotBase(t *testing.T) {
	img1 := randomImage(t, 2)
	img2 := randomImage(t, 3) // independent image, different layers

	res := compare.Compare(extractOrFatal(t, "img1:latest", img1), extractOrFatal(t, "img2:latest", img2), testPlatform)

	if res.Verdict != result.VerdictNotBase {
		t.Errorf("verdict = %s, want NOT_BASE", res.Verdict)
	}
}

func TestCompare_SameImage(t *testing.T) {
	img := randomImage(t, 2)

	res := compare.Compare(extractOrFatal(t, "img:v1", img), extractOrFatal(t, "img:v1", img), testPlatform)

	if res.Verdict != result.VerdictSameImage {
		t.Errorf("verdict = %s, want SAME_IMAGE", res.Verdict)
	}
}

func TestCompare_Platform(t *testing.T) {
	img1 := randomImage(t, 1)
	img2 := randomImage(t, 1)

	res := compare.Compare(extractOrFatal(t, "a:latest", img1), extractOrFatal(t, "b:latest", img2), "linux/arm64/v8")

	if res.Platform != "linux/arm64/v8" {
		t.Errorf("Platform = %q, want %q", res.Platform, "linux/arm64/v8")
	}
}

func TestCompare_MatchedLayerIndices(t *testing.T) {
	base := randomImage(t, 2)
	derived := appendLayer(t, base, randomLayer(t))

	res := compare.Compare(extractOrFatal(t, "base:latest", base), extractOrFatal(t, "derived:latest", derived), testPlatform)

	for i, l := range res.MatchedLayers {
		if l.Index != i {
			t.Errorf("MatchedLayers[%d].Index = %d, want %d", i, l.Index, i)
		}
	}
	for i, l := range res.ExtraLayers {
		want := len(res.MatchedLayers) + i
		if l.Index != want {
			t.Errorf("ExtraLayers[%d].Index = %d, want %d", i, l.Index, want)
		}
	}
}
