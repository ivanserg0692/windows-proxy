package layout

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestMeasureStyledWrappedContent(t *testing.T) {
	text := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Border(lipgloss.NormalBorder()).Padding(1).Width(12).Render("hello 世界 wide text")
	wrapped, size := Measure(text, 8)
	if size.Height <= lipgloss.Height(text) || size.Height != lipgloss.Height(wrapped) {
		t.Fatalf("unmeasured wrapping: %+v", size)
	}
	if _, empty := Measure("", 8); empty.Height != 0 {
		t.Fatal("empty block consumed space")
	}
}

func TestDistribution(t *testing.T) {
	blocks := []Block{{Text: "one\ntwo"}, {Grow: true}, {Text: "last"}, {Grow: true}}
	areas := Column(Size{Width: 12, Height: 10}, blocks...)
	want := []int{2, 3, 1, 4}
	for i, a := range areas {
		if a.Width != 12 || a.Height != want[i] {
			t.Fatalf("column %d: %+v", i, a)
		}
	}
	row := Row(Size{Width: 12, Height: 3}, Block{Text: "世界"}, Block{Grow: true}, Block{Text: "end"})
	if row[0].Width != 4 || row[1].Width != 5 || row[2].Width != 3 {
		t.Fatalf("row: %+v", row)
	}
	for _, height := range []int{-1, 0, 1, 2} {
		total := 0
		for _, a := range Column(Size{Width: 12, Height: height}, blocks...) {
			if a.Height < 0 {
				t.Fatal("negative allocation")
			}
			total += a.Height
		}
		if total > max(0, height) {
			t.Fatal("overflow")
		}
	}
}

func TestFitBoundsAndClipping(t *testing.T) {
	for _, text := range []string{"", "longwordlongwordlongword", "世界界\nnext\nlast", lipgloss.NewStyle().Bold(true).Render("one two three four five six")} {
		for w := 0; w < 15; w++ {
			for h := 0; h < 8; h++ {
				got := Fit(text, Size{Width: w, Height: h})
				if w == 0 || h == 0 {
					if got != "" {
						t.Fatal("nonempty zero area")
					}
					continue
				}
				if lipgloss.Width(got) != w || lipgloss.Height(got) != h {
					t.Fatalf("fit %dx%d: %q (%dx%d)", w, h, got, lipgloss.Width(got), lipgloss.Height(got))
				}
			}
		}
	}
	if !strings.Contains(ansi.Strip(Fit("first\nsecond", Size{Width: 8, Height: 1})), "…") {
		t.Fatal("clipping was invisible")
	}
}
