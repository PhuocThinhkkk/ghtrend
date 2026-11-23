package ascii

import (
	"fmt"
	"strings"
	"unicode"
)

// PixelCell represents a single terminal cell in a 3-row pixel glyph.
type PixelCell struct {
	Char     rune
	IsShadow bool
}

// Glyph represents a 3-row-tall pixel block character.
type Glyph struct {
	Width int
	Rows  [3][]PixelCell
}

var fontMap map[rune]Glyph

func init() {
	fontMap = make(map[rune]Glyph)

	// Helper to build a glyph from 3 row templates.
	// Syntax:
	// ' ' = empty space
	// 'O' = outer full block █
	// 'S' = shadow full block █
	// 'T' = outer upper half block ▀
	// 't' = shadow upper half block ▀
	// 'B' = outer lower half block ▄
	// 'b' = shadow lower half block ▄
	// 'L' = outer left half block ▌
	// 'R' = outer right half block ▐
	addGlyph := func(r rune, row0, row1, row2 string) {
		g := Glyph{}
		w := len(row0)
		if len(row1) > w {
			w = len(row1)
		}
		if len(row2) > w {
			w = len(row2)
		}
		g.Width = w

		parseRow := func(s string) []PixelCell {
			cells := make([]PixelCell, w)
			runes := []rune(s)
			for i := 0; i < w; i++ {
				var ch rune = ' '
				if i < len(runes) {
					ch = runes[i]
				}
				switch ch {
				case 'O':
					cells[i] = PixelCell{Char: '█', IsShadow: false}
				case 'S':
					cells[i] = PixelCell{Char: '█', IsShadow: true}
				case 'T':
					cells[i] = PixelCell{Char: '▀', IsShadow: false}
				case 't':
					cells[i] = PixelCell{Char: '▀', IsShadow: true}
				case 'B':
					cells[i] = PixelCell{Char: '▄', IsShadow: false}
				case 'b':
					cells[i] = PixelCell{Char: '▄', IsShadow: true}
				case 'L':
					cells[i] = PixelCell{Char: '▌', IsShadow: false}
				case 'R':
					cells[i] = PixelCell{Char: '▐', IsShadow: false}
				default:
					cells[i] = PixelCell{Char: ch, IsShadow: false}
				}
			}
			return cells
		}

		g.Rows[0] = parseRow(row0)
		g.Rows[1] = parseRow(row1)
		g.Rows[2] = parseRow(row2)
		fontMap[r] = g
	}

	// Alphabet A-Z (3x3 or 4x3 pixel block font with two-tone depth)
	addGlyph('A',
		"BOB",
		"ObO",
		"T T",
	)
	addGlyph('B',
		"OTB",
		"OtB",
		"TTT",
	)
	addGlyph('C',
		"BTT",
		"O  ",
		"TBB",
	)
	addGlyph('D',
		"OTB",
		"O O",
		"TTT",
	)
	addGlyph('E',
		"OTT",
		"Ob ",
		"TTT",
	)
	addGlyph('F',
		"OTT",
		"Ob ",
		"T  ",
	)
	addGlyph('G',
		"BTT",
		"O B",
		"TTO",
	)
	addGlyph('H',
		"O O",
		"ObO",
		"T T",
	)
	addGlyph('I',
		"TOT",
		" O ",
		"TOT",
	)
	addGlyph('J',
		"  O",
		"  O",
		"TTT",
	)
	addGlyph('K',
		"O B",
		"OO ",
		"T T",
	)
	addGlyph('L',
		"O  ",
		"O  ",
		"TTT",
	)
	addGlyph('M',
		"OTBTO",
		"O O O",
		"T   T",
	)
	addGlyph('N',
		"OTB",
		"O O",
		"T T",
	)
	addGlyph('O',
		"BTB",
		"O O",
		"TBT",
	)
	addGlyph('P',
		"OTB",
		"Ott",
		"T  ",
	)
	addGlyph('Q',
		"BTB",
		"O O",
		"TBO",
	)
	addGlyph('R',
		"OTB",
		"OSt",
		"T T",
	)
	addGlyph('S',
		"BTT",
		" tB",
		"TTT",
	)
	addGlyph('T',
		"TOT",
		" O ",
		" T ",
	)
	addGlyph('U',
		"O O",
		"O O",
		"TBT",
	)
	addGlyph('V',
		"O O",
		"O O",
		" T ",
	)
	addGlyph('W',
		"O O O",
		"O O O",
		" T T ",
	)
	addGlyph('X',
		"T T",
		" O ",
		"T T",
	)
	addGlyph('Y',
		"O O",
		" O ",
		" T ",
	)
	addGlyph('Z',
		"TTO",
		" O ",
		"OTT",
	)

	// Digits 0-9
	addGlyph('0',
		"BTB",
		"O O",
		"TBT",
	)
	addGlyph('1',
		"BO ",
		" O ",
		"TTT",
	)
	addGlyph('2',
		"TTO",
		"BT ",
		"TTT",
	)
	addGlyph('3',
		"TTO",
		" tO",
		"TTT",
	)
	addGlyph('4',
		"O O",
		"TTO",
		"  T",
	)
	addGlyph('5',
		"OTT",
		"TTB",
		"TTT",
	)
	addGlyph('6',
		"BTT",
		"OtB",
		"TTT",
	)
	addGlyph('7',
		"TTO",
		"  O",
		"  T",
	)
	addGlyph('8',
		"BTB",
		"OtO",
		"TBT",
	)
	addGlyph('9',
		"BTB",
		"TTO",
		" TT",
	)

	// Symbols
	addGlyph(' ',
		"  ",
		"  ",
		"  ",
	)
	addGlyph('-',
		"   ",
		"bbb",
		"   ",
	)
	addGlyph('_',
		"   ",
		"   ",
		"TTT",
	)
	addGlyph('.',
		" ",
		" ",
		"B",
	)
	addGlyph('!',
		"O",
		"b",
		"B",
	)
	addGlyph('/',
		"  O",
		" O ",
		"O  ",
	)
	addGlyph('@',
		"BTB",
		"ObO",
		"TBT",
	)
}

// RenderPixelBanner renders text into 3 rows using Unicode block elements
// with two-tone shading. Outer strokes use color1 for the first half/word
// and color2 for the second half/word. Inner/shadow pixels use shadowColor.
func RenderPixelBanner(text string, color1, color2, shadowColor string) [3]string {
	var result [3]string
	text = strings.TrimSpace(text)
	if text == "" {
		return result
	}

	if color1 == "" {
		color1 = "#a9b1d6" // soft silver/taupe
	}
	if color2 == "" {
		color2 = "#7aa2f7" // bright periwinkle blue
	}
	if shadowColor == "" {
		shadowColor = "#414868" // dark muted tone
	}

	ansiColor1 := hexToANSIForeground(color1)
	ansiColor2 := hexToANSIForeground(color2)
	ansiShadow := hexToANSIForeground(shadowColor)
	ansiReset := "\x1b[0m"

	// Split text into two halves/words
	words := strings.Fields(text)
	type styledChar struct {
		r         rune
		ansiColor string
	}
	var styledChars []styledChar

	if len(words) >= 2 {
		// First word in color 1, rest in color 2
		firstWord := words[0]
		restWords := strings.Join(words[1:], " ")
		for _, r := range firstWord {
			styledChars = append(styledChars, styledChar{r: unicode.ToUpper(r), ansiColor: ansiColor1})
		}
		styledChars = append(styledChars, styledChar{r: ' ', ansiColor: ansiColor1})
		for _, r := range restWords {
			styledChars = append(styledChars, styledChar{r: unicode.ToUpper(r), ansiColor: ansiColor2})
		}
	} else {
		// Single word: split characters in half
		runes := []rune(text)
		mid := (len(runes) + 1) / 2
		for i, r := range runes {
			c := ansiColor1
			if i >= mid {
				c = ansiColor2
			}
			styledChars = append(styledChars, styledChar{r: unicode.ToUpper(r), ansiColor: c})
		}
	}

	hasRenderable := false
	for _, sc := range styledChars {
		if sc.r != ' ' {
			if _, exists := fontMap[sc.r]; exists {
				hasRenderable = true
				break
			}
		}
	}
	if !hasRenderable {
		return [3]string{"", "", ""}
	}

	var rowBuilders [3]strings.Builder

	for i, sc := range styledChars {
		g, exists := fontMap[sc.r]
		if !exists {
			// Fallback: use space or question mark
			g = fontMap[' ']
		}

		for r := 0; r < 3; r++ {
			for _, cell := range g.Rows[r] {
				if cell.Char == ' ' {
					rowBuilders[r].WriteByte(' ')
					continue
				}
				if cell.IsShadow {
					rowBuilders[r].WriteString(ansiShadow)
					rowBuilders[r].WriteRune(cell.Char)
					rowBuilders[r].WriteString(ansiReset)
				} else {
					rowBuilders[r].WriteString(sc.ansiColor)
					rowBuilders[r].WriteRune(cell.Char)
					rowBuilders[r].WriteString(ansiReset)
				}
			}
			if i < len(styledChars)-1 {
				rowBuilders[r].WriteByte(' ') // Inter-character gap
			}
		}
	}

	result[0] = rowBuilders[0].String()
	result[1] = rowBuilders[1].String()
	result[2] = rowBuilders[2].String()
	return result
}

// RenderPixelBannerString renders text into a 3-line string, truncating if it exceeds maxCols.
func RenderPixelBannerString(text string, color1, color2, shadowColor string, maxCols int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	// If maxCols is specified and tight, determine how many characters fit
	if maxCols > 0 {
		runes := []rune(text)
		// Estimate width: avg 4 cols per character + 1 col spacing = 5 cols
		for len(runes) > 0 {
			candidate := string(runes)
			lines := RenderPixelBanner(candidate, color1, color2, shadowColor)
			if lines[0] == "" {
				return ""
			}
			width := visibleWidth(lines[0])
			if width <= maxCols {
				return lines[0] + "\n" + lines[1] + "\n" + lines[2]
			}
			runes = runes[:len(runes)-1]
		}
		return ""
	}

	lines := RenderPixelBanner(text, color1, color2, shadowColor)
	if lines[0] == "" && lines[1] == "" && lines[2] == "" {
		return ""
	}
	return lines[0] + "\n" + lines[1] + "\n" + lines[2]
}

func visibleWidth(s string) int {
	inEscape := false
	width := 0
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		width++
	}
	return width
}

func hexToANSIForeground(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return "\x1b[38;2;192;202;245m" // default to #c0caf5
	}
	var r, g, b uint8
	_, _ = fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
}
