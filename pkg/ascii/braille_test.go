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

func TestImageToBraille(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 56, 56))
	for y := 0; y < 56; y++ {
		for x := 0; x < 56; x++ {
			src.Set(x, y, color.RGBA{R: 200, G: 220, B: 255, A: 255})
		}
	}

	out := ImageToBraille(src, 28, 14)
	lines := strings.Split(out, "\n")
	if len(lines) != 14 {
		t.Fatalf("expected 14 lines, got %d", len(lines))
	}

	// Verify TrueColor escape codes are present and background codes (48;2) are absent
	if !strings.Contains(out, "\x1b[38;2;") {
		t.Errorf("expected foreground ANSI TrueColor code in Braille output")
	}
	if strings.Contains(out, "48;2;") {
		t.Errorf("braille mode should not emit background color codes (48;2)")
	}

	// Nil image produces fallback silhouette
	nilOut := ImageToBraille(nil, 28, 14)
	if len(nilOut) == 0 {
		t.Error("expected non-empty output for nil image")
	}

	// Transparent image produces spaces
	trans := image.NewRGBA(image.Rect(0, 0, 20, 20))
	transOut := ImageToBraille(trans, 10, 5)
	if strings.Contains(transOut, "\x1b[38;2;") {
		t.Errorf("fully transparent image should not contain ANSI codes, got: %q", transOut)
	}
}

func TestFallbackSilhouetteBraille(t *testing.T) {
	out := FallbackSilhouetteBraille(28, 14)
	lines := strings.Split(out, "\n")
	if len(lines) != 14 {
		t.Fatalf("expected 14 lines, got %d", len(lines))
	}
	if !strings.Contains(out, "\x1b[38;2;") {
		t.Errorf("expected colors in fallback silhouette")
	}
}

func TestAvatarToBraille_WithMockServer(t *testing.T) {
	t.Setenv("GHTREND_CACHE_DIR", t.TempDir())

	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: 150, G: 180, B: 240, A: 255})
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_ = png.Encode(w, img)
	}))
	defer server.Close()

	out := AvatarToBraille(server.URL, 28, 14)
	lines := strings.Split(out, "\n")
	if len(lines) != 14 {
		t.Errorf("expected 14 lines, got %d", len(lines))
	}

	// Empty URL produces fallback
	emptyOut := AvatarToBraille("", 28, 14)
	if len(strings.Split(emptyOut, "\n")) != 14 {
		t.Errorf("empty URL should return fallback")
	}
}
