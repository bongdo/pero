package widgets

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestInputSetCollapsesLines(t *testing.T) {
	var in Input
	in.Set("a\nb\r\nc\td")
	if got, want := in.Value(), "a b c d"; got != want {
		t.Fatalf("Value = %q, want %q", got, want)
	}
	if in.Cursor != len([]rune("a b c d")) {
		t.Errorf("Cursor = %d", in.Cursor)
	}
	if strings.Contains(in.Value(), "\n") || strings.Contains(in.Value(), "\t") {
		t.Error("input kept a newline or tab")
	}
}

func TestInputInsertBackspaceDelete(t *testing.T) {
	var in Input
	in.Set("привет")
	in.MoveLeft()
	in.MoveLeft()
	in.Insert("😀")
	if got, want := in.Value(), "прив😀ет"; got != want {
		t.Fatalf("after insert = %q, want %q", got, want)
	}
	if in.Cursor != 5 {
		t.Errorf("cursor after insert = %d, want 5", in.Cursor)
	}
	in.Backspace()
	if got, want := in.Value(), "привет"; got != want {
		t.Errorf("after backspace = %q, want %q", got, want)
	}

	in.Home()
	in.Delete()
	if got, want := in.Value(), "ривет"; got != want {
		t.Errorf("after delete = %q, want %q", got, want)
	}
	in.End()
	in.Delete()
	if got, want := in.Value(), "ривет"; got != want {
		t.Errorf("delete at end changed text: %q", got)
	}

	in.Insert("x\ny")
	if got, want := in.Value(), "риветx y"; got != want {
		t.Errorf("multiline insert = %q, want %q", got, want)
	}
}

func TestInputSelectionAndClear(t *testing.T) {
	var in Input
	in.Set("старый")
	in.SelAll = true
	in.Insert("новый")
	if got := in.Value(); got != "новый" {
		t.Errorf("replace selection = %q", got)
	}

	in.SelAll = true
	in.Backspace()
	if got := in.Value(); got != "" || in.Cursor != 0 || in.SelAll {
		t.Errorf("selall backspace: %q cursor %d selall %v", got, in.Cursor, in.SelAll)
	}

	in.Set("abc")
	in.SelAll = true
	in.MoveLeft()
	if in.Value() != "abc" || in.Cursor != 0 || in.SelAll {
		t.Errorf("left on selall: cursor %d selall %v", in.Cursor, in.SelAll)
	}

	in.SelAll = true
	in.Clear()
	if in.Value() != "" || in.Cursor != 0 || in.SelAll {
		t.Errorf("clear left state: %q cursor %d selall %v", in.Value(), in.Cursor, in.SelAll)
	}
}

func TestInputStripsControlCharacters(t *testing.T) {
	var in Input
	in.Set("a\x1b[31mb\x00c\td")
	if got, want := in.Value(), "a[31mbc d"; got != want {
		t.Fatalf("Set = %q, want %q", got, want)
	}
	in.Clear()
	in.Insert("x\x1b[2Jy\u009bz")
	if got, want := in.Value(), "x[2Jyz"; got != want {
		t.Fatalf("Insert = %q, want %q", got, want)
	}
	if out := in.Render(20, Style{}); strings.ContainsRune(out, '\x1b') {
		t.Fatalf("render содержит ESC: %q", out)
	}
}

func TestInputWords(t *testing.T) {
	var in Input
	in.Set("git commit --amend")
	if in.Cursor != 18 {
		t.Fatalf("cursor = %d, want 18", in.Cursor)
	}
	for _, want := range []int{11, 4, 0} {
		in.WordLeft()
		if in.Cursor != want {
			t.Errorf("WordLeft cursor = %d, want %d", in.Cursor, want)
		}
	}
	for _, want := range []int{4, 11, 18} {
		in.WordRight()
		if in.Cursor != want {
			t.Errorf("WordRight cursor = %d, want %d", in.Cursor, want)
		}
	}
}

func TestInputRenderScrolls(t *testing.T) {
	var in Input
	in.Set(strings.Repeat("я", 30))
	out := in.Render(10, Style{})
	if got := ansi.StringWidth(out); got != 10 {
		t.Fatalf("width = %d, want 10", got)
	}
	plain := ansi.Strip(out)
	if got := strings.Count(plain, "я"); got != 9 {
		t.Errorf("visible runes = %d, want 9 (cursor cell kept): %q", got, plain)
	}

	var emoji Input
	emoji.Set("😀😀😀")
	out = emoji.Render(5, Style{})
	if got := ansi.StringWidth(out); got != 5 {
		t.Errorf("emoji width = %d, want 5", got)
	}
	if !strings.Contains(out, "😀") {
		t.Errorf("emoji render lost content: %q", out)
	}
}

func TestInputRenderWideScrollBoundary(t *testing.T) {
	var in Input
	in.Set("a😀bc")
	in.Cursor = 2 // сразу после эмодзи, offset попадёт в его середину
	out := in.Render(2, Style{})
	if got := ansi.StringWidth(out); got != 2 {
		t.Errorf("wide boundary width = %d, want 2: %q", got, out)
	}
}

func TestInputRenderPlaceholderAndSelection(t *testing.T) {
	var in Input
	in.Placeholder = "поиск файлов"
	out := in.Render(8, Style{Muted: "\x1b[90m"})
	if got := ansi.StringWidth(out); got != 8 {
		t.Errorf("placeholder width = %d, want 8", got)
	}
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "…") {
		t.Errorf("placeholder not truncated: %q", plain)
	}

	in.Set("текст")
	in.SelAll = true
	out = in.Render(8, Style{SelectedBG: "\x1b[45m", SelectedFG: "\x1b[97m"})
	if !strings.Contains(out, "\x1b[45m") {
		t.Errorf("selection style missing: %q", out)
	}
	if got := ansi.StringWidth(out); got != 8 {
		t.Errorf("selection width = %d, want 8", got)
	}
	if strings.Contains(out, "\n") {
		t.Error("input render contains newline")
	}
}
