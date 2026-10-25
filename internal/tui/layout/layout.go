// Package layout measures terminal blocks and distributes bounded areas.
package layout

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Size is an area measured in terminal cells, including any block decoration.
type Size struct {
	Width, Height int
}

// Block requests its content size, or a share of the remaining area with Grow.
// Fixed blocks are allocated in declaration order when space is insufficient.
type Block struct {
	Text string
	Grow bool
}

// Measure wraps styled content before measuring it. Empty content has no size.
func Measure(text string, width int) (string, Size) {
	if text == "" || width <= 0 {
		return "", Size{}
	}
	wrapped := lipgloss.NewStyle().Width(width).Render(text)
	return wrapped, Size{Width: width, Height: lipgloss.Height(wrapped)}
}

// Column calculates vertically stacked block sizes within area, returning one
// Size per block in input order. It does not compose text or render output.
// Every block receives max(0, area.Width) columns.
//
// Fixed blocks (Grow == false) are measured with Measure at area.Width, so their
// requested height includes text wrapping. They receive space in input order,
// each limited to the height still available. Empty text requests no height.
//
// After all fixed blocks are allocated, Grow blocks share the remaining height;
// their Text is ignored. Each receives the remaining height divided by the
// number of Grow blocks still to allocate. Integer division gives any extra
// rows to later Grow blocks. Without Grow blocks, unused height stays unallocated.
//
// If space runs out, later fixed blocks and all Grow blocks receive zero height.
// A nonpositive area.Height leaves every block with zero height.
//
// For example:
//
//	Column(Size{Width: 10, Height: 8},
//	    Block{Text: "one\ntwo"},
//	    Block{Grow: true},
//	    Block{Grow: true},
//	    Block{Text: "last"},
//	)
//
// returns sizes 10x2, 10x2, 10x3, and 10x1: fixed blocks use three rows,
// and the two Grow blocks split the remaining five rows.
func Column(area Size, blocks ...Block) []Size {
	areas := make([]Size, len(blocks))
	remainingHeight, remainingGrowingBlocks := max(0, area.Height), 0
	for i, block := range blocks {
		areas[i].Width = max(0, area.Width)
		if block.Grow {
			remainingGrowingBlocks++
			continue
		}
		_, contentSize := Measure(block.Text, area.Width)
		areas[i].Height = min(remainingHeight, contentSize.Height)
		remainingHeight -= areas[i].Height
	}
	for i, block := range blocks {
		if block.Grow {
			areas[i].Height = remainingHeight / remainingGrowingBlocks
			remainingHeight -= areas[i].Height
			remainingGrowingBlocks--
		}
	}
	return areas
}

// Row calculates horizontally arranged block sizes within area, returning one
// Size per block in input order. It does not compose text or render output.
// Every block receives max(0, area.Height) rows.
//
// Fixed blocks (Grow == false) request their natural terminal width, measured
// with lipgloss.Width. For multiline text, this is the widest line; text is not
// wrapped to area.Width. Fixed blocks receive space in input order, each limited
// to the width still available.
//
// After all fixed blocks are allocated, Grow blocks share the remaining width;
// their Text is ignored. Each receives the remaining width divided by the
// number of Grow blocks still to allocate. Integer division gives any extra
// columns to later Grow blocks. Without Grow blocks, unused width stays
// unallocated.
//
// If space runs out, later fixed blocks and all Grow blocks receive zero width.
// A nonpositive area.Width leaves every block with zero width.
//
// For example:
//
//	Row(Size{Width: 12, Height: 3},
//	    Block{Text: "ABC"},
//	    Block{Grow: true},
//	    Block{Grow: true},
//	    Block{Text: "XY"},
//	)
//
// returns sizes 3x3, 3x3, 4x3, and 2x3: fixed blocks use five columns,
// and the two Grow blocks split the remaining seven columns.
func Row(area Size, blocks ...Block) []Size {
	areas := make([]Size, len(blocks))
	remainingWidth, remainingGrowingBlocks := max(0, area.Width), 0
	for i, block := range blocks {
		areas[i].Height = max(0, area.Height)
		if block.Grow {
			remainingGrowingBlocks++
			continue
		}
		areas[i].Width = min(remainingWidth, lipgloss.Width(block.Text))
		remainingWidth -= areas[i].Width
	}
	for i, block := range blocks {
		if block.Grow {
			areas[i].Width = remainingWidth / remainingGrowingBlocks
			remainingWidth -= areas[i].Width
			remainingGrowingBlocks--
		}
	}
	return areas
}

// Fit renders text within the rectangular terminal area described by area.
// Width is the number of terminal cells per row; Height is the number of rows.
// If either dimension is zero or negative, Fit returns an empty string.
//
// Existing line breaks are preserved, and Lip Gloss wraps text to Width before
// clipping. Each retained line is truncated to Width with an ellipsis ("…")
// when necessary. If the wrapped text exceeds Height, excess lines are removed
// and the last visible line is shortened to at most Width-1 cells before an
// ellipsis is appended. This marker indicates omitted content even when the
// last visible line itself would otherwise fit.
//
// Finally, Lip Gloss pads shorter lines on the right with spaces and adds blank
// rows at the bottom to fill the allocated area. For positive dimensions, an
// empty text produces an area filled entirely with spaces. Rows are separated
// by newlines; Fit does not append a trailing newline.
//
// Wrapping, measurement, and truncation use terminal display width rather than
// len: ANSI styling sequences do not occupy cells, and Unicode characters can
// occupy a different number of cells than their UTF-8 byte length. ANSI styling
// is preserved during rendering.
//
// For example:
//
//	Fit("Hello\nGo", Size{Width: 10, Height: 3})
//
// returns "Hello     \nGo        \n          ". With "|" used only to show
// the row boundaries, the result occupies:
//
//	|Hello     |
//	|Go        |
//	|          |
func Fit(text string, area Size) string {
	if area.Width <= 0 || area.Height <= 0 {
		return ""
	}
	wrapped, _ := Measure(text, area.Width)
	lines := strings.Split(wrapped, "\n")
	heightClipped := len(lines) > area.Height
	lines = lines[:min(len(lines), area.Height)]
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], area.Width, "…")
	}
	if heightClipped {
		lastLineIndex := len(lines) - 1
		lines[lastLineIndex] = ansi.Truncate(lines[lastLineIndex], max(0, area.Width-1), "") + "…"
	}
	return lipgloss.NewStyle().Width(area.Width).Height(area.Height).Render(strings.Join(lines, "\n"))
}
