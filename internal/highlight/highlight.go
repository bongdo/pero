// Package highlight turns source text into ANSI-styled spans using chroma.
//
// Span offsets are rune indexes into a logical line. SGR is a prefix such as
// "\x1b[38;2;211;198;170m"; the caller appends "\x1b[0m" after the span.
// Highlighter is not safe for concurrent use.
package highlight

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// Span is a half-open [Start, End) range of runes in one logical line.
type Span struct {
	Start, End int
	SGR        string
}

// Palette maps syntax elements to colours. Each colour is hex (#RRGGBB) or an
// ANSI number; an empty field leaves that element in the UI base colour.
type Palette struct {
	Keyword, String, Number, Comment, Function, Type, Operator, Constant, Builtin, Punctuation string
}

// Highlighter tokenises text with the lexer for the current file and renders
// chroma styles as ANSI SGR. It caches the last Lines call.
type Highlighter struct {
	path    string
	theme   string
	palette Palette
	lexer   chroma.Lexer
	style   *chroma.Style
	text    string
	lines   [][]Span
	cached  bool
}

// New returns a highlighter with the everforest theme and no file.
func New() *Highlighter {
	h := &Highlighter{}
	h.SetTheme("everforest")
	return h
}

// SetFile selects a lexer by file extension or well-known file name.
// Unknown files leave the lexer nil and produce empty spans.
func (h *Highlighter) SetFile(path string) {
	if path == h.path {
		return
	}
	h.path = path
	h.lexer = lexerFor(path)
	h.cached = false
}

// SetTheme selects everforest, vesper, nord, or plain. Unknown themes fall
// back to plain. It is used only while no palette is set.
func (h *Highlighter) SetTheme(name string) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == h.theme {
		return
	}
	h.theme = name
	h.style = themeStyle(name)
	h.cached = false
}

// SetPalette replaces the theme with explicit colours and takes priority over
// SetTheme. Setting an empty Palette restores the theme.
func (h *Highlighter) SetPalette(p Palette) {
	if p == h.palette {
		return
	}
	h.palette = p
	h.cached = false
}

// Lines splits text into logical lines and returns their styled spans.
// Consecutive calls with the same text return the cached result.
func (h *Highlighter) Lines(text string) [][]Span {
	text = normalizeNewlines(text)
	if h.cached && text == h.text {
		return h.lines
	}

	lines := strings.Split(text, "\n")
	out := make([][]Span, len(lines))
	if h.lexer != nil && (h.palette != Palette{} || h.style != nil) {
		h.tokenize(text, lines, out)
	}

	h.text = text
	h.lines = out
	h.cached = true
	return out
}

// ANSIStyle returns SGR prefixes for the entry's colour and attributes.
// It never includes a reset; the caller adds "\x1b[0m".
func ANSIStyle(e chroma.StyleEntry) string {
	var b strings.Builder
	if e.Bold == chroma.Yes {
		b.WriteString("\x1b[1m")
	}
	if e.Italic == chroma.Yes {
		b.WriteString("\x1b[3m")
	}
	if e.Underline == chroma.Yes {
		b.WriteString("\x1b[4m")
	}
	if e.Colour.IsSet() {
		fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm", e.Colour.Red(), e.Colour.Green(), e.Colour.Blue())
	}
	if e.Background.IsSet() {
		fmt.Fprintf(&b, "\x1b[48;2;%d;%d;%dm", e.Background.Red(), e.Background.Green(), e.Background.Blue())
	}
	return b.String()
}

// sgr returns the palette SGR for a token type; empty means the UI base colour.
func (p Palette) sgr(t chroma.TokenType) string {
	switch {
	case t == chroma.KeywordType, t == chroma.NameClass, t == chroma.NameNamespace:
		return colorSGR(p.Type)
	case t.InCategory(chroma.Keyword):
		return colorSGR(p.Keyword)
	case t.InSubCategory(chroma.LiteralString):
		return colorSGR(p.String)
	case t.InSubCategory(chroma.LiteralNumber):
		return colorSGR(p.Number)
	case t.InCategory(chroma.Comment):
		return colorSGR(p.Comment)
	case t.InSubCategory(chroma.NameFunction):
		return colorSGR(p.Function)
	case t.InSubCategory(chroma.NameBuiltin):
		return colorSGR(p.Builtin)
	case t == chroma.NameConstant, t == chroma.LiteralDate:
		return colorSGR(p.Constant)
	case t.InCategory(chroma.Operator):
		return colorSGR(p.Operator)
	case t.InCategory(chroma.Punctuation):
		return colorSGR(p.Punctuation)
	}
	return ""
}

// colorSGR turns hex (#RRGGBB) or an ANSI number into a foreground SGR prefix.
func colorSGR(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "#") {
		if len(v) != 7 {
			return ""
		}
		return ANSIStyle(chroma.StyleEntry{Colour: chroma.ParseColour(v)})
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return ""
	}
	switch {
	case n >= 0 && n < 8:
		return fmt.Sprintf("\x1b[%dm", 30+n)
	case n >= 8 && n < 16:
		return fmt.Sprintf("\x1b[%dm", 90+n-8)
	case n >= 16 && n < 256:
		return fmt.Sprintf("\x1b[38;5;%dm", n)
	}
	return ""
}

// tokenize fills out. Positions advance over every token value, including
// unstyled and meta tokens, so later spans stay aligned.
func (h *Highlighter) tokenize(text string, lines []string, out [][]Span) {
	lineLen := make([]int, len(lines))
	for i, line := range lines {
		lineLen[i] = utf8.RuneCountInString(line)
	}

	iter, err := h.lexer.Tokenise(nil, text)
	if err != nil {
		return
	}

	line, col := 0, 0
tokenLoop:
	for tok := iter(); tok != chroma.EOF; tok = iter() {
		sgr := ""
		if tok.Type >= 0 {
			if h.palette != (Palette{}) {
				sgr = h.palette.sgr(tok.Type)
			} else {
				entry := h.style.Get(tok.Type)
				entry.Background = 0 // the UI owns backgrounds (selection, current line)
				entry.Border = 0
				sgr = ANSIStyle(entry)
			}
		}

		parts := strings.Split(tok.Value, "\n")
		for i, part := range parts {
			if i > 0 {
				line++
				col = 0
				if line >= len(lines) {
					// Lexers with EnsureNL append a newline that is not in the input.
					break tokenLoop
				}
			}
			if sgr == "" {
				col += utf8.RuneCountInString(part)
				continue
			}
			out[line], col = appendStyled(out[line], col, lineLen[line], part, sgr)
		}
	}
}

// appendStyled emits one span per non-whitespace run and returns the next
// rune column. Whitespace is skipped so it never gets a colour or attribute.
func appendStyled(dst []Span, base, limit int, s string, sgr string) ([]Span, int) {
	runStart := -1
	n := 0
	for _, r := range s {
		if unicode.IsSpace(r) {
			if runStart >= 0 {
				dst = addSpan(dst, base+runStart, base+n, limit, sgr)
				runStart = -1
			}
		} else if runStart < 0 {
			runStart = n
		}
		n++
	}
	if runStart >= 0 {
		dst = addSpan(dst, base+runStart, base+n, limit, sgr)
	}
	return dst, base + n
}

func addSpan(dst []Span, start, end, limit int, sgr string) []Span {
	if end > limit {
		end = limit
	}
	if start < end {
		dst = append(dst, Span{Start: start, End: end, SGR: sgr})
	}
	return dst
}

func normalizeNewlines(s string) string {
	if !strings.ContainsRune(s, '\r') {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

func lexerFor(path string) chroma.Lexer {
	if path == "" {
		return nil
	}
	base := strings.ToLower(filepath.Base(path))

	var name string
	switch base {
	case ".bashrc", ".bash_profile", ".profile", ".zshrc":
		name = "bash"
	default:
		switch strings.ToLower(filepath.Ext(base)) {
		case ".md", ".markdown":
			name = "markdown"
		case ".go":
			name = "go"
		case ".lua":
			name = "lua"
		case ".nix":
			name = "nix"
		case ".json":
			name = "json"
		case ".toml":
			name = "toml"
		case ".yaml", ".yml":
			name = "yaml"
		case ".sh", ".bash", ".zsh":
			name = "bash"
		case ".py":
			name = "python"
		case ".js", ".mjs", ".cjs":
			name = "javascript"
		case ".ts":
			name = "typescript"
		case ".rs":
			name = "rust"
		case ".c", ".h":
			name = "c"
		case ".nim":
			name = "nim"
		}
	}
	if name == "" {
		return nil
	}
	return lexers.Get(name)
}

func themeStyle(name string) *chroma.Style {
	switch name {
	case "everforest":
		return chroma.MustNewStyle("everforest", chroma.StyleEntries{
			chroma.Text:                "#d3c6aa",
			chroma.Error:               "#e67e80",
			chroma.Keyword:             "#e67e80",
			chroma.KeywordConstant:     "#e67e80",
			chroma.KeywordDeclaration:  "#e67e80",
			chroma.KeywordNamespace:    "#e67e80",
			chroma.KeywordReserved:     "#e67e80",
			chroma.KeywordType:         "#dbbc7f",
			chroma.Name:                "#d3c6aa",
			chroma.NameAttribute:       "#a7c080",
			chroma.NameBuiltin:         "#e69875",
			chroma.NameClass:           "#dbbc7f",
			chroma.NameConstant:        "#d699b6",
			chroma.NameDecorator:       "#d699b6",
			chroma.NameFunction:        "#a7c080",
			chroma.NameTag:             "#e67e80",
			chroma.NameVariable:        "#d3c6aa",
			chroma.LiteralString:       "#a7c080",
			chroma.LiteralStringEscape: "#e69875",
			chroma.LiteralStringRegex:  "#e69875",
			chroma.LiteralStringSymbol: "#e69875",
			chroma.LiteralNumber:       "#d699b6",
			chroma.LiteralNumberFloat:  "#d699b6",
			chroma.Operator:            "#e69875",
			chroma.Punctuation:         "#859289",
			chroma.Comment:             "italic #859289",
			chroma.CommentPreproc:      "#e67e80",
			chroma.GenericHeading:      "bold #a7c080",
			chroma.GenericSubheading:   "bold #dbbc7f",
			chroma.GenericEmph:         "italic #83c092",
			chroma.GenericStrong:       "bold #d3c6aa",
			chroma.GenericDeleted:      "#e67e80",
			chroma.GenericInserted:     "#a7c080",
		})
	case "vesper":
		return chroma.MustNewStyle("vesper", chroma.StyleEntries{
			chroma.Text:                "#ffffff",
			chroma.Error:               "#ff8080",
			chroma.Keyword:             "#a0a0a0",
			chroma.KeywordConstant:     "#99ffe4",
			chroma.KeywordDeclaration:  "#a0a0a0",
			chroma.KeywordNamespace:    "#a0a0a0",
			chroma.KeywordReserved:     "#a0a0a0",
			chroma.KeywordType:         "#99ffe4",
			chroma.Name:                "#ffffff",
			chroma.NameAttribute:       "#ffc799",
			chroma.NameBuiltin:         "#ffc799",
			chroma.NameClass:           "#ffc799",
			chroma.NameConstant:        "#99ffe4",
			chroma.NameDecorator:       "#ffc799",
			chroma.NameFunction:        "#ffc799",
			chroma.NameTag:             "#ffc799",
			chroma.NameVariable:        "#ffffff",
			chroma.LiteralString:       "#ffc799",
			chroma.LiteralStringEscape: "#99ffe4",
			chroma.LiteralStringRegex:  "#99ffe4",
			chroma.LiteralStringSymbol: "#ffc799",
			chroma.LiteralNumber:       "#99ffe4",
			chroma.LiteralNumberFloat:  "#99ffe4",
			chroma.Operator:            "#a0a0a0",
			chroma.Punctuation:         "#a0a0a0",
			chroma.Comment:             "italic #7a7a7a",
			chroma.CommentPreproc:      "#a0a0a0",
			chroma.GenericHeading:      "bold #ffc799",
			chroma.GenericSubheading:   "bold #ffc799",
			chroma.GenericEmph:         "italic #a0a0a0",
			chroma.GenericStrong:       "bold #ffffff",
			chroma.GenericDeleted:      "#ff8080",
			chroma.GenericInserted:     "#99ffe4",
		})
	case "nord":
		return styles.Get("nord")
	default: // plain and unknown themes
		return nil
	}
}
