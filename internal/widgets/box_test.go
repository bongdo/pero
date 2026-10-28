package widgets

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderBoxFrame(t *testing.T) {
	b := Box{
		Title:  "Заголовок",
		Footer: "Enter · Esc",
		Rows:   []string{"привет", "😀 emoji"},
		Width:  40,
	}
	lines := RenderBox(b, Style{})
	if len(lines) != 4 {
		t.Fatalf("lines = %d, want 4", len(lines))
	}
	const wantW = 26 // max(24, 8) + 2 рамки
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != wantW {
			t.Errorf("line %d width = %d, want %d: %q", i, got, wantW, line)
		}
	}

	top := ansi.Strip(lines[0])
	if !strings.HasPrefix(top, "╭─ Заголовок ") || !strings.HasSuffix(top, "╮") {
		t.Errorf("top border = %q", top)
	}
	body := ansi.Strip(lines[1])
	if !strings.HasPrefix(body, "│привет") || !strings.HasSuffix(body, "│") {
		t.Errorf("body row = %q", body)
	}
	last := ansi.Strip(lines[3])
	if !strings.HasPrefix(last, "╰ Enter · Esc ") || !strings.HasSuffix(last, "╯") {
		t.Errorf("bottom border = %q", last)
	}
	if !strings.Contains(ansi.Strip(lines[2]), "😀 emoji") {
		t.Errorf("emoji row lost: %q", ansi.Strip(lines[2]))
	}
}

func TestRenderBoxNarrow(t *testing.T) {
	long := strings.Repeat("文字", 40)
	for _, width := range []int{-5, 0, 1, 2, 3, 4, 5, 6, 10, 20, 28, 29, 30, 40, 80} {
		lines := RenderBox(Box{
			Title:  "Заголовок",
			Footer: "футер",
			Rows:   []string{long, "😀"},
			Width:  width,
		}, Style{})
		if len(lines) != 4 {
			t.Fatalf("width %d: lines = %d, want 4", width, len(lines))
		}
		total := ansi.StringWidth(lines[0])
		for i, line := range lines {
			if got := ansi.StringWidth(line); got != total {
				t.Errorf("width %d: line %d width = %d, want %d", width, i, got, total)
			}
		}
		if width > 0 && total > max(2, width-2) {
			t.Errorf("width %d: frame width %d exceeds limit", width, total)
		}
	}
}

func TestRenderBoxANSITruncate(t *testing.T) {
	row := "\x1b[31m" + strings.Repeat("宽", 30) + "tail\x1b[0m"
	lines := RenderBox(Box{Rows: []string{row}, Width: 20}, Style{})
	if len(lines) != 3 {
		t.Fatalf("lines = %d, want 3", len(lines))
	}
	if got := ansi.StringWidth(lines[1]); got != 18 {
		t.Errorf("body width = %d, want 18", got)
	}
	plain := ansi.Strip(lines[1])
	if !strings.HasPrefix(plain, "│宽宽宽宽宽宽宽宽│") {
		t.Errorf("ANSI cut broke cell alignment: %q", plain)
	}
}

func TestRenderBoxStyles(t *testing.T) {
	lines := RenderBox(Box{Title: "t", Rows: []string{"x"}}, Style{
		Border: "\x1b[36m",
		Text:   "\x1b[37m",
	})
	for _, line := range lines {
		if !strings.HasPrefix(line, "\x1b[36m") && !strings.HasPrefix(line, "\x1b[37m") {
			t.Errorf("style prefix missing: %q", line)
		}
	}
}
