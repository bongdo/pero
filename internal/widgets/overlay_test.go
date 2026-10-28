package widgets

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func baseRows(n, width int) []string {
	rows := make([]string, n)
	for i := range rows {
		rows[i] = strings.Repeat(string(rune('a'+i)), width)
	}
	return rows
}

func TestOverlayEdges(t *testing.T) {
	const width = 12
	base := baseRows(6, width)
	box := []string{"╭──╮", "│xy│", "╰──╯"}

	for _, pos := range []struct{ x, y int }{{0, 0}, {8, 0}, {0, 3}, {8, 3}, {4, 2}} {
		out := Overlay(base, box, pos.x, pos.y, width)
		if len(out) != len(base) {
			t.Fatalf("pos %v: rows = %d, want %d", pos, len(out), len(base))
		}
		for i, line := range out {
			if got := ansi.StringWidth(line); got != width {
				t.Errorf("pos %v: row %d width = %d, want %d", pos, i, got, width)
			}
		}
		if !strings.Contains(ansi.Strip(out[pos.y+1]), "xy") {
			t.Errorf("pos %v: box content missing: %q", pos, ansi.Strip(out[pos.y+1]))
		}
	}

	out := Overlay(base, box, 2, 2, width)
	if got, want := ansi.Strip(out[2]), "cc╭──╮cccccc"; got != want {
		t.Errorf("top-left placement = %q, want %q", got, want)
	}
}

func TestOverlayClamps(t *testing.T) {
	const width = 12
	base := baseRows(4, width)
	box := []string{"╭──╮", "│xy│", "╰──╯"}

	out := Overlay(base, box, 100, 100, width)
	if !strings.Contains(ansi.Strip(out[3]), "╰──╯") {
		t.Errorf("bottom clamp failed: %q", ansi.Strip(out[3]))
	}
	for i, line := range out {
		if got := ansi.StringWidth(line); got != width {
			t.Errorf("row %d width = %d, want %d", i, got, width)
		}
	}

	out = Overlay(base, box, -5, -5, width)
	if got, want := ansi.Strip(out[0]), "╭──╮aaaaaaaa"; got != want {
		t.Errorf("negative clamp = %q, want %q", got, want)
	}

	wide := []string{strings.Repeat("Z", 30)}
	out = Overlay(base, wide, 0, 0, width)
	if got := ansi.StringWidth(out[0]); got != width {
		t.Errorf("oversized box width = %d, want %d", got, width)
	}
	if got := ansi.Strip(out[0]); got != strings.Repeat("Z", width) {
		t.Errorf("oversized box = %q", got)
	}
}

func TestOverlayWideBaseBoundary(t *testing.T) {
	base := []string{strings.Repeat("😀", 4)} // 8 колонок
	out := Overlay(base, []string{"X"}, 2, 0, 8)
	if got := ansi.StringWidth(out[0]); got != 8 {
		t.Errorf("wide boundary width = %d, want 8: %q", got, out[0])
	}
}

func TestOverlayKeepsANSI(t *testing.T) {
	base := []string{"\x1b[41m" + strings.Repeat("a", 12) + "\x1b[0m"}
	out := Overlay(base, []string{"XY"}, 5, 0, 12)
	if got := ansi.StringWidth(out[0]); got != 12 {
		t.Fatalf("width = %d, want 12", got)
	}
	if got, want := ansi.Strip(out[0]), "aaaaaXYaaaaa"; got != want {
		t.Errorf("overlay = %q, want %q", got, want)
	}
}

func TestOverlayDerivesWidthAndClipsRows(t *testing.T) {
	base := []string{"aaaaaaaa", "bbbbbbbb"}
	out := Overlay(base, []string{"XY"}, 10, 0, 0)
	if got, want := ansi.Strip(out[0]), "aaaaaaXY"; got != want {
		t.Errorf("derived width = %q, want %q", got, want)
	}
	if got := ansi.StringWidth(out[1]); got != 8 {
		t.Errorf("untouched row width = %d, want 8", got)
	}

	short := Overlay(base[:1], []string{"1", "2", "3"}, 0, 0, 8)
	if len(short) != 1 {
		t.Fatalf("rows = %d, want 1", len(short))
	}
}

func TestOverlayPadsUntouchedRows(t *testing.T) {
	base := []string{"ab", "cd", "ef"}
	out := Overlay(base, []string{"XY"}, 0, 0, 6)
	if len(out) != len(base) {
		t.Fatalf("rows = %d, want %d", len(out), len(base))
	}
	for i, line := range out {
		if got := ansi.StringWidth(line); got != 6 {
			t.Errorf("row %d width = %d, want 6: %q", i, got, line)
		}
	}
	if got, want := ansi.Strip(out[1]), "cd    "; got != want {
		t.Errorf("untouched row = %q, want %q", got, want)
	}
}
