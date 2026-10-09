// snapshot renders fixed-cell PNG previews of the text actually delivered to readers.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/mattn/go-runewidth"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type block struct {
	Label string   `json:"label"`
	Line  int      `json:"line"`
	Text  []string `json:"-"`
}

type result struct {
	block
	Columns      int    `json:"columns"`
	Width        int    `json:"content_width"`
	WrappedLines []int  `json:"wrapped_lines"`
	PNG          string `json:"png"`
}

var ansi = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

// textBlocks accepts Markdown text fences, including tutor --plain output.
// Other code fences are ignored; a missing closing fence is an error.
func textBlocks(text string, raw bool) ([]block, error) {
	text = ansi.ReplaceAllString(strings.ReplaceAll(text, "\r\n", "\n"), "")
	text = strings.TrimSuffix(text, "\n")
	for _, r := range text {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return nil, fmt.Errorf("unsupported control character %U; use plain output", r)
		}
	}
	if raw {
		if strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("empty input")
		}
		return []block{{Label: "text", Line: 1, Text: strings.Split(text, "\n")}}, nil
	}
	var blocks []block
	var current block
	fence, selected, heading, marker := "", false, "", ""
	for i, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if fence != "" {
			if len(t) >= len(fence) && strings.Trim(t, string(fence[0])) == "" {
				if selected && len(current.Text) > 0 {
					blocks = append(blocks, current)
				}
				fence, selected, marker = "", false, ""
			} else if selected {
				current.Text = append(current.Text, line)
			}
			continue
		}
		if strings.HasPrefix(t, "#") {
			heading = strings.TrimSpace(strings.TrimLeft(t, "#"))
		}
		if strings.HasPrefix(t, "<!-- draw-visual:") {
			marker = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(t, "<!-- draw-visual:"), "-->"))
		}
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			n := 0
			for n < len(t) && t[n] == t[0] {
				n++
			}
			fence = t[:n]
			selected = strings.TrimSpace(t[n:]) == "text"
			label := marker
			if label == "" {
				label = heading
			}
			if label == "" {
				label = "text"
			}
			current = block{Label: label, Line: i + 2}
		}
	}
	if fence != "" {
		return nil, fmt.Errorf("unclosed Markdown fence")
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("no fenced text blocks; use --text for raw renderer output")
	}
	return blocks, nil
}

var cells = &runewidth.Condition{EastAsianWidth: false, StrictEmojiNeutral: true}

func expandTabs(s string) string {
	var b strings.Builder
	column := 0
	for _, r := range s {
		if r == '\t' {
			n := 8 - column%8
			b.WriteString(strings.Repeat(" ", n))
			column += n
		} else {
			b.WriteRune(r)
			column += cells.RuneWidth(r)
		}
	}
	return b.String()
}

func wrap(s string, columns int) []string {
	var out []string
	var b strings.Builder
	width := 0
	for _, r := range s {
		w := cells.RuneWidth(r)
		if width+w > columns {
			out = append(out, b.String())
			b.Reset()
			width = 0
		}
		b.WriteRune(r)
		width += w
	}
	return append(out, b.String())
}

func writePNG(path string, b block, columns int, face font.Face) (result, error) {
	r := result{block: b, Columns: columns, WrappedLines: []int{}, PNG: filepath.Base(path)}
	lines := append(wrap(fmt.Sprintf("%d columns | diagram at line %d", columns, b.Line), columns), "")
	for i, line := range b.Text {
		line = expandTabs(line)
		width := cells.StringWidth(line)
		if width > r.Width {
			r.Width = width
		}
		if width > columns {
			r.WrappedLines = append(r.WrappedLines, b.Line+i)
		}
		lines = append(lines, wrap(line, columns)...)
	}
	cell, height := font.MeasureString(face, "M").Ceil(), face.Metrics().Height.Ceil()
	if len(lines) > 1500 {
		return r, fmt.Errorf("diagram too tall (%d rows); split it", len(lines))
	}
	img := image.NewRGBA(image.Rect(0, 0, columns*cell+48, len(lines)*height+48))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{30, 30, 30, 255}), image.Point{}, draw.Src)
	d := font.Drawer{Dst: img, Src: image.NewUniform(color.RGBA{210, 210, 210, 255}), Face: face}
	for y, line := range lines {
		x, previous := 0, 0
		for _, ch := range line {
			w := cells.RuneWidth(ch)
			pos := x
			if w == 0 {
				pos = previous
			}
			d.Dot = fixed.P(24+pos*cell, 24+face.Metrics().Ascent.Ceil()+y*height)
			d.DrawString(string(ch))
			if w > 0 {
				previous = x
			}
			x += w
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return r, err
	}
	err = png.Encode(f, img)
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	return r, err
}

func run() error {
	input := flag.String("input", "", "Markdown/tutor --plain output file (required)")
	out := flag.String("out", "", "directory for PNGs and manifest.json (required)")
	widths := flag.String("columns", "70,80", "comma-separated terminal widths")
	raw := flag.Bool("text", false, "input is raw diagram text instead of Markdown")
	fontPath := flag.String("font", "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf", "monospaced TrueType font")
	flag.Parse()
	if *input == "" || *out == "" || flag.NArg() != 0 {
		return fmt.Errorf("use --input FILE --out DIR [--columns 70,80] [--text]")
	}
	var columns []int
	for _, value := range strings.Split(*widths, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || n < 20 || n > 300 {
			return fmt.Errorf("columns must be between 20 and 300")
		}
		columns = append(columns, n)
	}
	data, err := os.ReadFile(*input)
	if err != nil {
		return err
	}
	blocks, err := textBlocks(string(data), *raw)
	if err != nil {
		return err
	}
	fontData, err := os.ReadFile(*fontPath)
	if err != nil {
		return fmt.Errorf("read font (override with --font): %w", err)
	}
	parsed, err := opentype.Parse(fontData)
	if err != nil {
		return err
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: 20, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return err
	}
	defer face.Close()
	if err = os.MkdirAll(*out, 0755); err != nil {
		return err
	}
	var results []result
	overflow := false
	for i, b := range blocks {
		for _, c := range columns {
			path := filepath.Join(*out, fmt.Sprintf("diagram-%02d-%d.png", i+1, c))
			r, err := writePNG(path, b, c, face)
			if err != nil {
				return err
			}
			results = append(results, r)
			if len(r.WrappedLines) > 0 {
				overflow = true
			}
			fmt.Printf("%s: %d columns of content / %d available; %d wrapped lines\n", path, r.Width, c, len(r.WrappedLines))
		}
	}
	manifest := struct {
		Input   string   `json:"input"`
		SHA256  string   `json:"sha256"`
		Font    string   `json:"font"`
		Mode    string   `json:"mode"`
		Review  string   `json:"visual_review"`
		Results []result `json:"diagrams"`
	}{*input, fmt.Sprintf("%x", sha256.Sum256(data)), *fontPath, "fixed-cell terminal simulation; not a screenshot of the user's app", "pending: open every PNG; generation is not inspection", results}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(*out, "manifest.json"), append(encoded, '\n'), 0644); err != nil {
		return err
	}
	if overflow {
		return fmt.Errorf("diagram wrapping detected; inspect PNGs, revise sources, and regenerate")
	}
	fmt.Println("Width checks passed. Visual review is still required: open every PNG.")
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "snapshot:", err)
		os.Exit(1)
	}
}
