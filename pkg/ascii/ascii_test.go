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

func TestResizeImage(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			src.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}

	dst := ResizeImage(src, 20, 10)
	if dst.Bounds().Dx() != 20 || dst.Bounds().Dy() != 10 {
		t.Errorf("expected 20x10, got %dx%d", dst.Bounds().Dx(), dst.Bounds().Dy())
	}

	c := dst.At(5, 5).(color.RGBA)
	if c.R != 255 || c.G != 0 || c.B != 0 {
		t.Errorf("expected red pixel, got %v", c)
	}

	// Test nil image does not panic
	nilDst := ResizeImage(nil, 15, 8)
	if nilDst.Bounds().Dx() != 15 || nilDst.Bounds().Dy() != 8 {
		t.Errorf("expected 15x8 for nil image, got %dx%d", nilDst.Bounds().Dx(), nilDst.Bounds().Dy())
	}
}

func TestImageToColoredASCII(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			src.Set(x, y, color.RGBA{R: 128, G: 128, B: 128, A: 255})
		}
	}

	out := ImageToColoredASCII(src, 10, 5)
	lines := strings.Split(out, "\n")
	if len(lines) != 5 {
		t.Errorf("expected 5 lines, got %d", len(lines))
	}

	// Verify ANSI escape code is present
	if !strings.Contains(out, "\x1b[38;2;") {
		t.Errorf("expected ANSI TrueColor codes in output, got %q", out)
	}

	// Verify zero dimensions clamp safely
	zeroOut := ImageToColoredASCII(src, 0, 0)
	if len(zeroOut) == 0 {
		t.Errorf("expected non-empty output for 0x0 dimensions")
	}

	// Verify nil image falls back to silhouette without panicking
	nilOut := ImageToColoredASCII(nil, 20, 8)
	if !strings.Contains(nilOut, "USER AVATAR") {
		t.Errorf("expected fallback silhouette for nil image, got %q", nilOut)
	}
}

func TestFallbackSilhouetteASCII(t *testing.T) {
	out := FallbackSilhouetteASCII(24, 10)
	lines := strings.Split(out, "\n")
	if len(lines) != 10 {
		t.Errorf("expected 10 lines, got %d", len(lines))
	}
	if !strings.Contains(out, "USER AVATAR") {
		t.Errorf("expected fallback to contain 'USER AVATAR', got %q", out)
	}
}

func TestAvatarToASCII_WithMockServer(t *testing.T) {
	// Generate mock 20x20 PNG image
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			img.Set(x, y, color.RGBA{R: 0, G: 255, B: 0, A: 255})
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_ = png.Encode(w, img)
	}))
	defer server.Close()

	out := AvatarToASCII(server.URL, 20, 8)
	lines := strings.Split(out, "\n")
	if len(lines) != 8 {
		t.Errorf("expected 8 lines, got %d", len(lines))
	}
	if !strings.Contains(out, "\x1b[38;2;0;255;0m") {
		t.Errorf("expected green ANSI color, got %q", out)
	}

	// Test fallback on 404
	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer errServer.Close()

	fallbackOut := AvatarToASCII(errServer.URL, 20, 8)
	if !strings.Contains(fallbackOut, "USER AVATAR") {
		t.Errorf("expected fallback silhouette on 404, got %q", fallbackOut)
	}
}
