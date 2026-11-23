package ascii

import (
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEnhanceContrast(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 10, 10))
	src.Set(5, 5, color.RGBA{R: 150, G: 100, B: 200, A: 255})

	enhanced := EnhanceContrast(src, 1.2)
	if enhanced == nil {
		t.Fatal("expected non-nil enhanced image")
	}

	c := enhanced.RGBAAt(5, 5)
	if c.R == 0 && c.G == 0 && c.B == 0 {
		t.Errorf("enhanced pixel should not be zero")
	}

	// Nil image safety
	if EnhanceContrast(nil, 1.2) != nil {
		t.Error("expected nil for nil input")
	}
}

func TestResizeImageCatmullRom(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			src.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}

	dst := ResizeImageCatmullRom(src, 32, 32)
	if dst.Bounds().Dx() != 32 || dst.Bounds().Dy() != 32 {
		t.Errorf("expected 32x32, got %dx%d", dst.Bounds().Dx(), dst.Bounds().Dy())
	}

	// Nil image test
	nilDst := ResizeImageCatmullRom(nil, 28, 28)
	if nilDst.Bounds().Dx() != 28 || nilDst.Bounds().Dy() != 28 {
		t.Errorf("expected 28x28 for nil, got %dx%d", nilDst.Bounds().Dx(), nilDst.Bounds().Dy())
	}
}

func TestImageToHalfBlock(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 28, 28))
	for y := 0; y < 28; y++ {
		for x := 0; x < 28; x++ {
			// Foreground color
			src.Set(x, y, color.RGBA{R: 120, G: 160, B: 240, A: 255})
		}
	}

	// 28 cols by 14 rows
	out := ImageToHalfBlock(src, 28, 14)
	lines := strings.Split(out, "\n")
	if len(lines) != 14 {
		t.Fatalf("expected 14 lines, got %d", len(lines))
	}

	// Check ANSI TrueColor code and half-block character
	if !strings.Contains(out, "▀") {
		t.Errorf("expected half-block '▀' in output")
	}
	if !strings.Contains(out, "\x1b[38;2;") {
		t.Errorf("expected ANSI 24-bit TrueColor in output")
	}

	// Test dark background clamping
	darkSrc := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			darkSrc.Set(x, y, color.RGBA{R: 5, G: 5, B: 5, A: 255}) // Below threshold 18
		}
	}
	darkOut := ImageToHalfBlock(darkSrc, 10, 5)
	// Dark pixels should clamp to empty space ' ' without ANSI color codes
	if strings.Contains(darkOut, "\x1b[38;2;") {
		t.Errorf("dark background pixels should clamp to empty spaces, got: %q", darkOut)
	}

	// Nil image should produce fallback silhouette
	nilOut := ImageToHalfBlock(nil, 28, 14)
	if len(nilOut) == 0 {
		t.Error("nil image should return fallback silhouette")
	}
}

func TestFallbackSilhouetteHalfBlock(t *testing.T) {
	out := FallbackSilhouetteHalfBlock(30, 15)
	lines := strings.Split(out, "\n")
	if len(lines) != 15 {
		t.Fatalf("expected 15 lines, got %d", len(lines))
	}
	if !strings.Contains(out, "▀") && !strings.Contains(out, "▄") {
		t.Errorf("expected half blocks in fallback silhouette")
	}
}

func TestAvatarToHalfBlock_WithMockServer(t *testing.T) {
	t.Setenv("GHTREND_CACHE_DIR", t.TempDir())

	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: 80, G: 120, B: 200, A: 255})
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_ = png.Encode(w, img)
	}))
	defer server.Close()

	out := AvatarToHalfBlock(server.URL, 28, 14)
	lines := strings.Split(out, "\n")
	if len(lines) != 14 {
		t.Errorf("expected 14 lines, got %d", len(lines))
	}

	// Empty URL test
	emptyOut := AvatarToHalfBlock("", 28, 14)
	if len(strings.Split(emptyOut, "\n")) != 14 {
		t.Errorf("empty URL should return fallback of requested height")
	}
}
