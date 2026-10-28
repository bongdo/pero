package highlight

import (
	"reflect"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
)

func checkSpans(t *testing.T, text string, lines [][]Span) {
	t.Helper()
	text = normalizeNewlines(text)
	want := strings.Split(text, "\n")
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(lines), len(want))
	}
	for i, spans := range lines {
		runes := []rune(want[i])
		prevEnd := 0
		for _, sp := range spans {
			if sp.Start < 0 || sp.Start >= sp.End {
				t.Fatalf("line %d: bad span [%d,%d)", i, sp.Start, sp.End)
			}
			if sp.End > len(runes) {
				t.Fatalf("line %d: span end %d > rune count %d", i, sp.End, len(runes))
			}
			if sp.Start < prevEnd {
				t.Fatalf("line %d: spans overlap at %d", i, sp.Start)
			}
			for _, r := range runes[sp.Start:sp.End] {
				if unicode.IsSpace(r) {
					t.Fatalf("line %d: whitespace inside span [%d,%d)", i, sp.Start, sp.End)
				}
			}
			prevEnd = sp.End
		}
	}
}

func countSpans(lines [][]Span) int {
	n := 0
	for _, spans := range lines {
		n += len(spans)
	}
	return n
}

func TestLanguagesProduceSpans(t *testing.T) {
	files := []struct {
		path string
		text string
	}{
		{"README.md", "# Title\n\nText with **bold** and `code`.\n"},
		{"main.go", "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n"},
		{"init.lua", "local function f(x)\n  return x + 1\nend\n"},
		{"flake.nix", "{ pkgs ? import <nixpkgs> {} }:\n{ x = 1; }\n"},
		{"data.json", "{\"a\": 1, \"b\": [true, null]}\n"},
		{"config.toml", "name = \"pero\"\n[section]\nkey = true\n"},
		{"config.yaml", "key: value\nlist:\n  - one\n"},
		{"script.sh", "#!/bin/sh\necho \"hi\"\n"},
		{"main.py", "def f(x):\n    return x + 1\n"},
		{"app.js", "const x = 1;\nconsole.log(x);\n"},
		{"app.ts", "const x: number = 1;\n"},
		{"main.rs", "fn main() { println!(\"hi\"); }\n"},
		{"main.c", "#include <stdio.h>\nint main(void) { return 0; }\n"},
		{"main.nim", "proc main() =\n  echo \"hi\"\n"},
	}
	for _, tc := range files {
		t.Run(tc.path, func(t *testing.T) {
			h := New()
			h.SetFile(tc.path)
			lines := h.Lines(tc.text)
			checkSpans(t, tc.text, lines)
			if countSpans(lines) == 0 {
				t.Fatalf("%s: no spans", tc.path)
			}
			for _, spans := range lines {
				for _, sp := range spans {
					if sp.SGR == "" {
						t.Fatalf("%s: empty SGR at [%d,%d)", tc.path, sp.Start, sp.End)
					}
					if strings.Contains(sp.SGR, "\x1b[0m") {
						t.Fatalf("%s: SGR contains reset: %q", tc.path, sp.SGR)
					}
				}
			}
		})
	}
}

func TestUnknownFileIsEmpty(t *testing.T) {
	h := New()
	h.SetFile("notes.xyz")
	lines := h.Lines("package main\nfunc main() {}\n")
	checkSpans(t, "package main\nfunc main() {}\n", lines)
	if countSpans(lines) != 0 {
		t.Fatalf("unknown file produced %d spans", countSpans(lines))
	}

	h.SetFile("")
	if lines := h.Lines("x"); len(lines) != 1 || len(lines[0]) != 0 {
		t.Fatalf("empty path should produce empty spans, got %#v", lines)
	}
}

func TestCache(t *testing.T) {
	h := New()
	h.SetFile("main.go")
	src := "package main\n\nfunc main() {}\n"

	first := h.Lines(src)
	second := h.Lines(src)
	a, b := firstPtr(first), firstPtr(second)
	if a == nil || b == nil {
		t.Fatal("expected non-empty spans")
	}
	if a != b {
		t.Fatal("repeated Lines did not return cached spans")
	}
	if firstPtr(h.Lines(strings.Clone(src))) != a {
		t.Fatal("equal text with different backing was not cached")
	}
	h.SetFile("main.go")
	if firstPtr(h.Lines(src)) != a {
		t.Fatal("setting the same file should keep the cache")
	}

	h.SetTheme("nord")
	third := h.Lines(src)
	c := firstPtr(third)
	if c == nil || c == a {
		t.Fatal("SetTheme should invalidate the cache")
	}
	if c != firstPtr(h.Lines(src)) {
		t.Fatal("repeated Lines after theme change was not cached")
	}

	h.SetFile("data.json")
	fourth := h.Lines(src)
	d := firstPtr(fourth)
	if d == nil || d == c {
		t.Fatal("SetFile should invalidate the cache")
	}

	changed := h.Lines(src + "\nvar x int\n")
	if firstPtr(changed) == d {
		t.Fatal("changed text should be tokenised again")
	}
}

func firstPtr(lines [][]Span) *Span {
	for _, spans := range lines {
		if len(spans) > 0 {
			return &spans[0]
		}
	}
	return nil
}

func TestPlainTheme(t *testing.T) {
	h := New()
	h.SetFile("main.go")
	h.SetTheme("plain")
	lines := h.Lines("package main\nfunc main() {}\n")
	checkSpans(t, "package main\nfunc main() {}\n", lines)
	if countSpans(lines) != 0 {
		t.Fatalf("plain theme produced %d spans", countSpans(lines))
	}

	h.SetTheme("no-such-theme")
	if lines := h.Lines("package main\n"); countSpans(lines) != 0 {
		t.Fatalf("unknown theme should be plain, got %d spans", countSpans(lines))
	}

	h.SetTheme("everforest")
	if countSpans(h.Lines("package main\n")) == 0 {
		t.Fatal("everforest theme produced no spans")
	}
}

func TestCRLF(t *testing.T) {
	h := New()
	h.SetFile("main.go")
	lf := h.Lines("package main\nvar x = 1\n")

	h2 := New()
	h2.SetFile("main.go")
	crlf := h2.Lines("package main\r\nvar x = 1\r\n")
	checkSpans(t, "package main\r\nvar x = 1\r\n", crlf)
	if !reflect.DeepEqual(lf, crlf) {
		t.Fatalf("CRLF differs from LF:\n%#v\n%#v", lf, crlf)
	}
}

func TestUnicodeIndexesAreRunes(t *testing.T) {
	cases := []struct{ path, text string }{
		{"x.json", "{\"a\":\"😀\",\"b\":1}\n"},
		{"x.json", "{\"ключ\":\"значение\"}\n"},
		{"x.go", "var привет = \"ёж\"\n"},
	}
	for _, tc := range cases {
		h := New()
		h.SetFile(tc.path)
		lines := h.Lines(tc.text)
		checkSpans(t, tc.text, lines)
		if countSpans(lines) == 0 {
			t.Fatalf("%q: no spans", tc.text)
		}
		line := strings.Split(normalizeNewlines(tc.text), "\n")[0]
		if utf8.RuneCountInString(line) >= len(line) {
			t.Fatalf("%q: test line has no multibyte runes", tc.text)
		}
	}
}

func TestEmptyAndWhitespace(t *testing.T) {
	h := New()
	h.SetFile("main.go")

	lines := h.Lines("")
	checkSpans(t, "", lines)
	if len(lines) != 1 || len(lines[0]) != 0 {
		t.Fatalf("empty text: %#v", lines)
	}

	for _, text := range []string{"\n\n", "   \n\t\n", "\r\n\r\n"} {
		lines := h.Lines(text)
		checkSpans(t, text, lines)
		if countSpans(lines) != 0 {
			t.Fatalf("%q: whitespace produced spans %#v", text, lines)
		}
	}
}

func TestLongLineAndInvalidUTF8(t *testing.T) {
	h := New()
	h.SetFile("main.go")

	long := "var s = \"" + strings.Repeat("a", 50000) + "\"\n"
	lines := h.Lines(long)
	checkSpans(t, long, lines)
	if countSpans(lines) == 0 {
		t.Fatal("long line produced no spans")
	}

	bad := "package main\nvar s = \"" + string([]byte{0xff, 0xfe, 'x'}) + "\"\n"
	lines = h.Lines(bad)
	checkSpans(t, bad, lines)
}

func TestANSIStyle(t *testing.T) {
	e := chroma.StyleEntry{
		Colour:     chroma.MustParseColour("#ff8000"),
		Background: chroma.MustParseColour("#101010"),
		Bold:       chroma.Yes,
		Italic:     chroma.Yes,
		Underline:  chroma.Yes,
	}
	got := ANSIStyle(e)
	for _, want := range []string{
		"\x1b[1m",
		"\x1b[3m",
		"\x1b[4m",
		"\x1b[38;2;255;128;0m",
		"\x1b[48;2;16;16;16m",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("ANSIStyle = %q, missing %q", got, want)
		}
	}
	if strings.Contains(got, "\x1b[0m") {
		t.Fatalf("ANSIStyle contains reset: %q", got)
	}
	if ANSIStyle(chroma.StyleEntry{}) != "" {
		t.Fatal("empty entry should produce empty SGR")
	}
}

func TestThemeSGR(t *testing.T) {
	src := "package main\n\nfunc main() { println(\"hi\") }\n"
	for _, theme := range []string{"everforest", "vesper", "nord"} {
		h := New()
		h.SetFile("main.go")
		h.SetTheme(theme)
		lines := h.Lines(src)
		checkSpans(t, src, lines)
		if countSpans(lines) == 0 {
			t.Fatalf("%s: no spans", theme)
		}
		for _, spans := range lines {
			for _, sp := range spans {
				if !strings.HasPrefix(sp.SGR, "\x1b[") || strings.Contains(sp.SGR, "\x1b[0m") {
					t.Fatalf("%s: bad SGR %q", theme, sp.SGR)
				}
			}
		}
	}
}
