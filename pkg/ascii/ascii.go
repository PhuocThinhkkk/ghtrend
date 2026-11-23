package ascii

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var asciiChars = []rune(" .:-=+*#%@")

var (
	avatarClient = &http.Client{Timeout: 4 * time.Second}
	cacheDirMu   sync.Mutex
)

// ResizeImage performs nearest-neighbor downscaling of an image to width x height pixels.
func ResizeImage(img image.Image, width, height int) *image.RGBA {
	if width <= 0 {
		width = 24
	}
	if height <= 0 {
		height = 10
	}
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	if img == nil {
		return dst
	}
	bounds := img.Bounds()
	dx := bounds.Dx()
	dy := bounds.Dy()
	if dx == 0 || dy == 0 {
		return dst
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcX := bounds.Min.X + (x * dx / width)
			srcY := bounds.Min.Y + (y * dy / height)
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}
	return dst
}

// ImageToColoredASCII converts an image to a colored ASCII string with 24-bit TrueColor ANSI codes.
func ImageToColoredASCII(img image.Image, width, height int) string {
	if width <= 0 {
		width = 24
	}
	if height <= 0 {
		height = 10
	}
	if img == nil {
		return FallbackSilhouetteASCII(width, height)
	}
	resized := ResizeImage(img, width, height)
	var sb strings.Builder

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := color.RGBAModel.Convert(resized.At(x, y)).(color.RGBA)
			lum := (float64(c.R)*0.299 + float64(c.G)*0.587 + float64(c.B)*0.114) / 255.0
			idx := int(lum * float64(len(asciiChars)-1))
			if idx < 0 {
				idx = 0
			}
			if idx >= len(asciiChars) {
				idx = len(asciiChars) - 1
			}
			char := asciiChars[idx]
			sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm%c\x1b[0m", c.R, c.G, c.B, char))
		}
		if y < height-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// FallbackSilhouetteASCII returns an aesthetic ASCII silhouette when the image is unavailable.
func FallbackSilhouetteASCII(width, height int) string {
	if width <= 0 {
		width = 24
	}
	if height <= 0 {
		height = 10
	}

	silhouette := []string{
		"   ╭──────────╮   ",
		"   │  ╭────╮  │   ",
		"   │  │ ◉◉ │  │   ",
		"   │  ╰────╯  │   ",
		"  ╭┴──────────┴╮  ",
		" ╭╯ USER AVATAR ╰╮",
		" ╰──────────────╯ ",
	}

	var lines []string
	for i := 0; i < height; i++ {
		if i < len(silhouette) {
			line := silhouette[i]
			runes := []rune(line)
			if len(runes) > width {
				line = string(runes[:width])
			} else {
				pad := (width - len(runes)) / 2
				line = strings.Repeat(" ", pad) + line
			}
			lines = append(lines, line)
		} else {
			lines = append(lines, strings.Repeat(" ", width))
		}
	}
	return strings.Join(lines, "\n")
}

// AvatarToASCII fetches the user's avatar from avatarURL, caches it on disk, and converts it to ASCII.
func AvatarToASCII(avatarURL string, width, height int) string {
	if avatarURL == "" {
		return FallbackSilhouetteASCII(width, height)
	}

	img, err := fetchCachedAvatar(avatarURL)
	if err != nil {
		return FallbackSilhouetteASCII(width, height)
	}

	return ImageToColoredASCII(img, width, height)
}

func getCacheFilePath(avatarURL string) string {
	hash := sha256.Sum256([]byte(avatarURL))
	filename := hex.EncodeToString(hash[:16]) + ".img"
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = "."
	}
	return filepath.Join(cacheDir, "ghtrend", "avatars", filename)
}

func fetchCachedAvatar(avatarURL string) (image.Image, error) {
	cachePath := getCacheFilePath(avatarURL)

	// Check if cached on disk
	if f, err := os.Open(cachePath); err == nil {
		defer f.Close()
		if img, _, err := image.Decode(f); err == nil {
			return img, nil
		}
	}

	// Fetch from network with context timeout
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", avatarURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ghtrend")

	resp, err := avatarClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("avatar fetch returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, err
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	// Save to disk asynchronously and atomically
	go func(path string, fileBytes []byte) {
		cacheDirMu.Lock()
		defer cacheDirMu.Unlock()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err == nil {
			tmpFile := fmt.Sprintf("%s.tmp.%d", path, time.Now().UnixNano())
			if err := os.WriteFile(tmpFile, fileBytes, 0644); err == nil {
				_ = os.Rename(tmpFile, path)
			}
		}
	}(cachePath, data)

	return img, nil
}
