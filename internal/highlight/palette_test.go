package highlight

import (
	"reflect"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
)

// spanSGR returns the SGR of the span covering sub on the given line, or ""
// when the text has no span (base colour).
func spanSGR(t *testing.T, src string, lines [][]Span, line int, sub string) string {
	t.Helper()
	text := []rune(strings.Split(normalizeNewlines(src), "\n")[line])
	start := -1
	for i := 0; i+len(sub) <= len(text); i++ {
		if string(text[i:i+len(sub)]) == sub {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("%q not found on line %d", sub, line)
	}
	end := start + len(sub)
	for _, sp := range lines[line] {
		if sp.Start == start && sp.End == end {
			return sp.SGR
		}
	}
	return ""
}

func TestPaletteGoSGR(t *testing.T) {
	src := "package main\n\n// note\nfunc hello() {\n\tx := 42\n\tprintln(\"hi\")\n}\n"
	p := Palette{
		Keyword:     "#ff0000",
		String:      "#00ff00",
		Number:      "#0000ff",
		Comment:     "#ffff00",
		Function:    "#ff00ff",
		Type:        "#00ffff",
		Operator:    "#ff8800",
		Constant:    "#8800ff",
		Builtin:     "#88ff00",
		Punctuation: "#888888",
	}
	h := New()
	h.SetFile("main.go")
	h.SetPalette(p)
	lines := h.Lines(src)
	checkSpans(t, src, lines)

	for _, tc := range []struct {
		line int
		text string
		want string
	}{
		{0, "package", "\x1b[38;2;255;0;0m"},
		{2, "//", "\x1b[38;2;255;255;0m"},
		{3, "func", "\x1b[38;2;255;0;0m"},
		{3, "hello", "\x1b[38;2;255;0;255m"},
		{4, "42", "\x1b[38;2;0;0;255m"},
		{5, "println", "\x1b[38;2;136;255;0m"},
		{5, "\"hi\"", "\x1b[38;2;0;255;0m"},
		{4, ":=", "\x1b[38;2;255;136;0m"},
		{3, "(", "\x1b[38;2;136;136;136m"},
	} {
		if got := spanSGR(t, src, lines, tc.line, tc.text); got != tc.want {
			t.Fatalf("%q SGR = %q, want %q", tc.text, got, tc.want)
		}
	}
	// NameOther is not in the palette mapping: the UI base colour applies.
	if got := spanSGR(t, src, lines, 0, "main"); got != "" {
		t.Fatalf("unmapped token SGR = %q, want empty", got)
	}
}

func TestPaletteTokenMapping(t *testing.T) {
	p := Palette{
		Keyword: "1", String: "2", Number: "3", Comment: "4", Function: "5",
		Type: "6", Operator: "7", Constant: "8", Builtin: "9", Punctuation: "10",
	}
	cases := []struct {
		token chroma.TokenType
		want  string
	}{
		{chroma.Keyword, colorSGR("1")},
		{chroma.KeywordConstant, colorSGR("1")},
		{chroma.KeywordDeclaration, colorSGR("1")},
		{chroma.KeywordNamespace, colorSGR("1")},
		{chroma.KeywordPseudo, colorSGR("1")},
		{chroma.KeywordReserved, colorSGR("1")},
		{chroma.KeywordType, colorSGR("6")},
		{chroma.NameClass, colorSGR("6")},
		{chroma.NameNamespace, colorSGR("6")},
		{chroma.LiteralString, colorSGR("2")},
		{chroma.LiteralStringChar, colorSGR("2")},
		{chroma.LiteralStringEscape, colorSGR("2")},
		{chroma.LiteralNumber, colorSGR("3")},
		{chroma.LiteralNumberFloat, colorSGR("3")},
		{chroma.Comment, colorSGR("4")},
		{chroma.CommentSingle, colorSGR("4")},
		{chroma.CommentPreproc, colorSGR("4")},
		{chroma.NameFunction, colorSGR("5")},
		{chroma.NameFunctionMagic, colorSGR("5")},
		{chroma.Operator, colorSGR("7")},
		{chroma.OperatorWord, colorSGR("7")},
		{chroma.NameConstant, colorSGR("8")},
		{chroma.LiteralDate, colorSGR("8")},
		{chroma.NameBuiltin, colorSGR("9")},
		{chroma.NameBuiltinPseudo, colorSGR("9")},
		{chroma.Punctuation, colorSGR("10")},
		{chroma.NameOther, ""},
		{chroma.Text, ""},
	}
	for _, tc := range cases {
		if got := p.sgr(tc.token); got != tc.want {
			t.Fatalf("%s SGR = %q, want %q", tc.token, got, tc.want)
		}
	}
}

func TestPaletteEmptyFields(t *testing.T) {
	src := "package main\nfunc hello() { println(\"hi\") }\n"
	h := New()
	h.SetFile("main.go")
	h.SetPalette(Palette{Keyword: "#ff0000"})
	lines := h.Lines(src)
	checkSpans(t, src, lines)

	if got := spanSGR(t, src, lines, 0, "package"); got != "\x1b[38;2;255;0;0m" {
		t.Fatalf("keyword SGR = %q", got)
	}
	if got := spanSGR(t, src, lines, 1, "\"hi\""); got != "" {
		t.Fatalf("empty String field produced SGR %q", got)
	}
}

func TestEmptyPaletteFallsBackToTheme(t *testing.T) {
	src := "package main\nfunc main() {}\n"
	wantH := New()
	wantH.SetFile("main.go")
	want := wantH.Lines(src)

	h := New()
	h.SetFile("main.go")
	h.SetPalette(Palette{Keyword: "#010203"})
	h.Lines(src)
	h.SetPalette(Palette{})
	if got := h.Lines(src); !reflect.DeepEqual(got, want) {
		t.Fatal("empty palette did not restore the theme")
	}
}

func TestPaletteOverridesTheme(t *testing.T) {
	src := "package main\nfunc main() {}\n"
	h := New()
	h.SetFile("main.go")
	h.SetPalette(Palette{Keyword: "#010203"})
	h.SetTheme("nord")
	h.SetTheme("plain") // palette must work without a chroma style
	lines := h.Lines(src)
	checkSpans(t, src, lines)
	if got := spanSGR(t, src, lines, 0, "package"); got != "\x1b[38;2;1;2;3m" {
		t.Fatalf("keyword SGR = %q, theme overrode palette", got)
	}
}

func TestPaletteCache(t *testing.T) {
	src := "package main\nfunc main() {}\n"
	h := New()
	h.SetFile("main.go")

	first := h.Lines(src)
	h.SetPalette(Palette{Keyword: "#ff0000"})
	second := h.Lines(src)
	if firstPtr(first) == firstPtr(second) {
		t.Fatal("SetPalette should invalidate the cache")
	}
	if firstPtr(second) != firstPtr(h.Lines(src)) {
		t.Fatal("repeated Lines after palette change was not cached")
	}

	h.SetPalette(Palette{Keyword: "#00ff00"})
	third := h.Lines(src)
	if firstPtr(second) == firstPtr(third) {
		t.Fatal("changed palette should invalidate the cache")
	}
}

func TestPaletteUnknownFile(t *testing.T) {
	h := New()
	h.SetFile("notes.xyz")
	h.SetPalette(Palette{Keyword: "#ff0000"})
	lines := h.Lines("package main\nfunc main() {}\n")
	checkSpans(t, "package main\nfunc main() {}\n", lines)
	if countSpans(lines) != 0 {
		t.Fatalf("unknown file produced %d spans", countSpans(lines))
	}
}

func TestColorSGR(t *testing.T) {
	cases := []struct{ in, want string }{
		{"#ff8000", "\x1b[38;2;255;128;0m"},
		{"1", "\x1b[31m"},
		{"9", "\x1b[91m"},
		{"196", "\x1b[38;5;196m"},
		{"", ""},
		{"nope", ""},
		{"#zzzzzz", ""},
		{"#fff", ""},
		{"-1", ""},
		{"999", ""},
	}
	for _, tc := range cases {
		if got := colorSGR(tc.in); got != tc.want {
			t.Fatalf("colorSGR(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
