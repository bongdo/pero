package widgets

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func makeItems(n int) []string {
	items := make([]string, n)
	for i := range items {
		items[i] = fmt.Sprintf("строка %d", i)
	}
	return items
}

func TestListNavigationClamps(t *testing.T) {
	l := &List{Items: makeItems(1000), Height: 10}

	l.End()
	if l.Sel != 999 {
		t.Fatalf("End Sel = %d, want 999", l.Sel)
	}
	rows := l.Visible()
	if len(rows) != 10 {
		t.Fatalf("Visible rows = %d, want 10", len(rows))
	}
	if rows[9] != "строка 999" {
		t.Errorf("last visible = %q", rows[9])
	}

	l.Move(1)
	if l.Sel != 999 {
		t.Errorf("Move past end = %d, want clamp 999", l.Sel)
	}
	l.Move(-1_000_000)
	if l.Sel != 0 {
		t.Errorf("Move past start = %d, want clamp 0", l.Sel)
	}
	if rows := l.Visible(); rows[0] != "строка 0" {
		t.Errorf("after clamp top visible = %q", rows[0])
	}

	l.Page(1)
	if l.Sel != 10 {
		t.Errorf("Page(1) Sel = %d, want 10", l.Sel)
	}
	l.Page(10_000)
	if l.Sel != 999 {
		t.Errorf("Page(large) Sel = %d, want 999", l.Sel)
	}
	l.Home()
	if l.Sel != 0 || l.Scroll != 0 {
		t.Errorf("Home = %d,%d, want 0,0", l.Sel, l.Scroll)
	}

	// Выбранная строка видна при любом положении.
	for i := 0; i < 100; i++ {
		l.Move(7)
		rows := l.Visible()
		idx := l.Sel - l.Scroll
		if idx < 0 || idx >= len(rows) || rows[idx] != l.Items[l.Sel] {
			t.Fatalf("selected row not visible: sel=%d scroll=%d", l.Sel, l.Scroll)
		}
	}
}

func TestListVisiblePadding(t *testing.T) {
	l := &List{Height: 4}
	rows := l.Visible()
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4", len(rows))
	}
	for i, row := range rows {
		if row != "" {
			t.Errorf("row %d = %q, want empty", i, row)
		}
	}

	l = &List{Items: makeItems(2), Height: 5}
	rows = l.Visible()
	if len(rows) != 5 || rows[0] == "" || rows[2] != "" || rows[4] != "" {
		t.Errorf("short list padding = %#v", rows)
	}
}

func TestListRender(t *testing.T) {
	l := &List{
		Items:  []string{"раз", "два", "три"},
		Height: 4,
		Sel:    1,
	}
	out := l.Render(8, Style{
		Text:       "\x1b[37m",
		SelectedBG: "\x1b[45m",
	})
	lines := strings.Split(out, "\n")
	if len(lines) != 4 {
		t.Fatalf("render lines = %d, want 4", len(lines))
	}
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != 8 {
			t.Errorf("line %d width = %d, want 8", i, got)
		}
	}
	if !strings.Contains(lines[1], "\x1b[45m") {
		t.Errorf("selected line not highlighted: %q", lines[1])
	}
	if strings.Contains(lines[0], "\x1b[45m") {
		t.Errorf("unselected line highlighted: %q", lines[0])
	}
	if !strings.Contains(lines[0], "\x1b[37m") {
		t.Errorf("text style missing: %q", lines[0])
	}
}

func TestListRenderTruncatesUnicode(t *testing.T) {
	l := &List{Items: []string{"😀😀😀😀😀"}, Height: 1}
	out := l.Render(7, Style{})
	if got := ansi.StringWidth(out); got != 7 {
		t.Errorf("width = %d, want 7", got)
	}
	if !strings.Contains(ansi.Strip(out), "…") {
		t.Errorf("truncation marker missing: %q", ansi.Strip(out))
	}

	big := &List{Items: makeItems(1000), Height: 10}
	out = big.Render(20, Style{})
	if got := len(strings.Split(out, "\n")); got != 10 {
		t.Errorf("1000-item render lines = %d, want 10", got)
	}
}
