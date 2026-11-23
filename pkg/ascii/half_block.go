package ascii

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"strings"

	"golang.org/x/image/draw"
)

// EnhanceContrast applies a subtle contrast boost to an image so facial features
// and edges stay crisp before downscaling.
func EnhanceContrast(img image.Image, factor float64) *image.RGBA {
	if img == nil {
		return nil
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))

	if factor <= 0 {
		factor = 1.15
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBAModel.Convert(img.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.RGBA)
			if c.A == 0 {
				dst.Set(x, y, color.Transparent)
				continue
			}
			r := clampFloat(((float64(c.R)-128.0)*factor)+128.0, 0, 255)
			g := clampFloat(((float64(c.G)-128.0)*factor)+128.0, 0, 255)
			b := clampFloat(((float64(c.B)-128.0)*factor)+128.0, 0, 255)
			dst.Set(x, y, color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: c.A})
		}
	}
	return dst
}

func clampFloat(v, minVal, maxVal float64) float64 {
	if v < minVal {
		return minVal
	}
	if v > maxVal {
		return maxVal
	}
	return v
}

// ResizeImageCatmullRom resizes an image to target width x height using Catmull-Rom bicubic interpolation.
func ResizeImageCatmullRom(img image.Image, width, height int) *image.RGBA {
	if width <= 0 {
		width = 28
	}
	if height <= 0 {
		height = 28
	}
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	if img == nil {
		return dst
	}
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	return dst
}

// SharpenImage applies an unsharp mask filter to restore micro-contrast and edge sharpness
// after downsampling, ensuring facial features and outlines remain crisp.
func SharpenImage(img *image.RGBA) *image.RGBA {
	if img == nil {
		return nil
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))

	// 3x3 unsharp convolution kernel with center weight 5:
	// [  0, -1,  0 ]
	// [ -1,  5, -1 ]
	// [  0, -1,  0 ]
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x == 0 || y == 0 || x == w-1 || y == h-1 {
				dst.Set(x, y, img.At(x, y))
				continue
			}
			cCenter := img.RGBAAt(x, y)
			if cCenter.A < 32 {
				dst.Set(x, y, cCenter)
				continue
			}

			cTop := img.RGBAAt(x, y-1)
			cBot := img.RGBAAt(x, y+1)
			cLeft := img.RGBAAt(x-1, y)
			cRight := img.RGBAAt(x+1, y)

			r := 5*int(cCenter.R) - int(cTop.R) - int(cBot.R) - int(cLeft.R) - int(cRight.R)
			g := 5*int(cCenter.G) - int(cTop.G) - int(cBot.G) - int(cLeft.G) - int(cRight.G)
			b := 5*int(cCenter.B) - int(cTop.B) - int(cBot.B) - int(cLeft.B) - int(cRight.B)

			dst.Set(x, y, color.RGBA{
				R: uint8(clampInt(r, 0, 255)),
				G: uint8(clampInt(g, 0, 255)),
				B: uint8(clampInt(b, 0, 255)),
				A: cCenter.A,
			})
		}
	}
	return dst
}

func clampInt(v, minVal, maxVal int) int {
	if v < minVal {
		return minVal
	}
	if v > maxVal {
		return maxVal
	}
	return v
}

// ImageToHalfBlock converts an image into a high-resolution ANSI truecolor string
// using Unicode half-blocks ('▀', U+2580). Each character cell renders 2 vertical pixels,
// meaning a width x height character grid displays a width x (height*2) square pixel resolution.
func ImageToHalfBlock(img image.Image, width, height int) string {
	if width <= 0 {
		width = 28
	}
	if height <= 0 {
		height = 14
	}
	if img == nil {
		return FallbackSilhouetteHalfBlock(width, height)
	}

	pixelW := width
	pixelH := height * 2

	enhanced := EnhanceContrast(img, 1.15)
	scaled := ResizeImageCatmullRom(enhanced, pixelW, pixelH)
	sharpened := SharpenImage(scaled)

	var sb strings.Builder

	for y := 0; y < height; y++ {
		topY := y * 2
		botY := y*2 + 1

		for x := 0; x < width; x++ {
			cTop := sharpened.RGBAAt(x, topY)
			cBot := sharpened.RGBAAt(x, botY)

			topEmpty := isPixelEmpty(cTop)
			botEmpty := isPixelEmpty(cBot)

			switch {
			case topEmpty && botEmpty:
				sb.WriteByte(' ')
			case !topEmpty && botEmpty:
				sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm▀\x1b[0m", cTop.R, cTop.G, cTop.B))
			case topEmpty && !botEmpty:
				sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm▄\x1b[0m", cBot.R, cBot.G, cBot.B))
			default:
				sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%d;48;2;%d;%d;%dm▀\x1b[0m",
					cTop.R, cTop.G, cTop.B, cBot.R, cBot.G, cBot.B))
			}
		}
		if y < height-1 {
			sb.WriteByte('\n')
		}
	}

	return sb.String()
}

func isPixelEmpty(c color.RGBA) bool {
	// Only truly transparent pixels (alpha < 32) are treated as empty spaces.
	// Dark/shadow pixels (hair, clothes, eyes) are fully rendered so no horizontal
	// stripes or missing rows occur.
	return c.A < 32
}

// FallbackSilhouetteHalfBlock generates a high-resolution, geometric user silhouette
// in TokyoNight colors when no avatar image is available.
func FallbackSilhouetteHalfBlock(width, height int) string {
	if width <= 0 {
		width = 28
	}
	if height <= 0 {
		height = 14
	}

	canvas := image.NewRGBA(image.Rect(0, 0, width, height*2))
	centerX := float64(width) / 2.0
	headCenterY := float64(height*2) * 0.35
	headRadius := float64(width) * 0.22

	shoulderCenterY := float64(height*2) * 0.95
	shoulderRadiusX := float64(width) * 0.42
	shoulderRadiusY := float64(height*2) * 0.35

	// Colors: TokyoNight accent/primary shades
	headColor := color.RGBA{R: 0x7a, G: 0xa2, B: 0xf7, A: 255}    // #7aa2f7
	shoulderColor := color.RGBA{R: 0x56, G: 0x5f, B: 0x89, A: 255} // #565f89

	for y := 0; y < height*2; y++ {
		for x := 0; x < width; x++ {
			dx := float64(x) - centerX
			dyHead := float64(y) - headCenterY
			distHead := (dx * dx) + (dyHead * dyHead)

			if distHead <= headRadius*headRadius {
				canvas.Set(x, y, headColor)
				continue
			}

			dyShoulder := float64(y) - shoulderCenterY
			normX := dx / shoulderRadiusX
			normY := dyShoulder / shoulderRadiusY
			if (normX*normX+normY*normY) <= 1.0 && y > int(headCenterY+headRadius*0.7) {
				canvas.Set(x, y, shoulderColor)
			}
		}
	}

	var sb strings.Builder
	for y := 0; y < height; y++ {
		topY := y * 2
		botY := y*2 + 1

		for x := 0; x < width; x++ {
			cTop := canvas.RGBAAt(x, topY)
			cBot := canvas.RGBAAt(x, botY)

			topEmpty := isPixelEmpty(cTop)
			botEmpty := isPixelEmpty(cBot)

			switch {
			case topEmpty && botEmpty:
				sb.WriteByte(' ')
			case !topEmpty && botEmpty:
				sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm▀\x1b[0m", cTop.R, cTop.G, cTop.B))
			case topEmpty && !botEmpty:
				sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm▄\x1b[0m", cBot.R, cBot.G, cBot.B))
			default:
				sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%d;48;2;%d;%d;%dm▀\x1b[0m",
					cTop.R, cTop.G, cTop.B, cBot.R, cBot.G, cBot.B))
			}
		}
		if y < height-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// AvatarToHalfBlock fetches the user's avatar from avatarURL, caches it on disk,
// and converts it to a high-resolution half-block representation.
func AvatarToHalfBlock(avatarURL string, width, height int) string {
	if avatarURL == "" {
		return FallbackSilhouetteHalfBlock(width, height)
	}

	img, err := fetchCachedAvatar(avatarURL)
	if err != nil {
		return FallbackSilhouetteHalfBlock(width, height)
	}

	return ImageToHalfBlock(img, width, height)
}

// SupportsKittyGraphics reports whether the terminal indicates Kitty graphics protocol support.
func SupportsKittyGraphics() bool {
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	term := os.Getenv("TERM")
	return strings.Contains(term, "kitty")
}
