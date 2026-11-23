package ascii

import (
	"fmt"
	"image"
	"image/color"
	"strings"
)

// Braille dot bitmask positions according to Unicode standard (U+2800..U+28FF):
// Dot 1 (0x01): col 0, row 0    Dot 4 (0x08): col 1, row 0
// Dot 2 (0x02): col 0, row 1    Dot 5 (0x10): col 1, row 1
// Dot 3 (0x04): col 0, row 2    Dot 6 (0x20): col 1, row 2
// Dot 7 (0x40): col 0, row 3    Dot 8 (0x80): col 1, row 3
var brailleOffsets = [4][2]uint8{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

// ImageToBraille converts an image into an ultra-high-resolution ANSI truecolor string
// using Unicode 2x4 Braille dots (U+2800..U+28FF).
// A width x height character grid yields a (width*2) x (height*4) pixel resolution
// (e.g. 28x14 character cells = 56x56 resolution), offering 4x higher micro-detail
// than half-blocks with zero background color scanline artifacts.
func ImageToBraille(img image.Image, width, height int) string {
	if width <= 0 {
		width = 28
	}
	if height <= 0 {
		height = 14
	}
	if img == nil {
		return FallbackSilhouetteBraille(width, height)
	}

	pixelW := width * 2
	pixelH := height * 4

	enhanced := EnhanceContrast(img, 1.20)
	scaled := ResizeImageCatmullRom(enhanced, pixelW, pixelH)
	sharpened := SharpenImage(scaled)
	bounds := sharpened.Bounds()

	// Convert to luminance and apply Floyd-Steinberg error diffusion dithering
	// to produce crisp, photographic dot distribution.
	lum := make([][]float64, pixelH)
	for y := 0; y < pixelH; y++ {
		lum[y] = make([]float64, pixelW)
		for x := 0; x < pixelW; x++ {
			c := sharpened.RGBAAt(bounds.Min.X+x, bounds.Min.Y+y)
			if c.A < 32 {
				lum[y][x] = 0
			} else {
				// Perceived luminance
				lum[y][x] = 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
			}
		}
	}

	// Dithered dot activation map
	active := make([][]bool, pixelH)
	for y := 0; y < pixelH; y++ {
		active[y] = make([]bool, pixelW)
	}

	for y := 0; y < pixelH; y++ {
		for x := 0; x < pixelW; x++ {
			oldVal := lum[y][x]
			// Threshold at 96 for balanced dot density on dark terminal backgrounds
			var newVal float64
			if oldVal >= 96.0 {
				newVal = 255.0
				active[y][x] = true
			} else {
				newVal = 0.0
				active[y][x] = false
			}
			err := oldVal - newVal

			// Distribute quantization error to neighboring pixels (Floyd-Steinberg)
			if x+1 < pixelW {
				lum[y][x+1] += err * (7.0 / 16.0)
			}
			if y+1 < pixelH {
				if x > 0 {
					lum[y+1][x-1] += err * (3.0 / 16.0)
				}
				lum[y+1][x] += err * (5.0 / 16.0)
				if x+1 < pixelW {
					lum[y+1][x+1] += err * (1.0 / 16.0)
				}
			}
		}
	}

	var sb strings.Builder

	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			var mask uint8
			var totalR, totalG, totalB int
			var dotCount int

			for dy := 0; dy < 4; dy++ {
				py := row*4 + dy
				if py >= pixelH {
					continue
				}
				for dx := 0; dx < 2; dx++ {
					px := col*2 + dx
					if px >= pixelW {
						continue
					}

					c := sharpened.RGBAAt(bounds.Min.X+px, bounds.Min.Y+py)
					if c.A < 32 {
						continue
					}

					if active[py][px] {
						mask |= brailleOffsets[dy][dx]
						totalR += int(c.R)
						totalG += int(c.G)
						totalB += int(c.B)
						dotCount++
					}
				}
			}

			if mask == 0 || dotCount == 0 {
				sb.WriteByte(' ')
			} else {
				avgR := totalR / dotCount
				avgG := totalG / dotCount
				avgB := totalB / dotCount
				r := rune(0x2800 + int(mask))
				sb.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm%c\x1b[0m", avgR, avgG, avgB, r))
			}
		}
		if row < height-1 {
			sb.WriteByte('\n')
		}
	}

	return sb.String()
}

// FallbackSilhouetteBraille creates a geometric user silhouette in Braille dots.
func FallbackSilhouetteBraille(width, height int) string {
	if width <= 0 {
		width = 28
	}
	if height <= 0 {
		height = 14
	}

	pixelW := width * 2
	pixelH := height * 4
	canvas := image.NewRGBA(image.Rect(0, 0, pixelW, pixelH))

	centerX := float64(pixelW) / 2.0
	headCenterY := float64(pixelH) * 0.35
	headRadius := float64(pixelW) * 0.22

	shoulderCenterY := float64(pixelH) * 0.95
	shoulderRadiusX := float64(pixelW) * 0.42
	shoulderRadiusY := float64(pixelH) * 0.35

	headColor := color.RGBA{R: 0x7a, G: 0xa2, B: 0xf7, A: 255}    // #7aa2f7
	shoulderColor := color.RGBA{R: 0x56, G: 0x5f, B: 0x89, A: 255} // #565f89

	for y := 0; y < pixelH; y++ {
		for x := 0; x < pixelW; x++ {
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

	return ImageToBraille(canvas, width, height)
}

// AvatarToBraille fetches the user's avatar from avatarURL, caches it on disk,
// and converts it to a high-resolution Braille dot representation.
func AvatarToBraille(avatarURL string, width, height int) string {
	if avatarURL == "" {
		return FallbackSilhouetteBraille(width, height)
	}

	img, err := fetchCachedAvatar(avatarURL)
	if err != nil {
		return FallbackSilhouetteBraille(width, height)
	}

	return ImageToBraille(img, width, height)
}
