package markdown

import (
	"math"
	"reflect"
	"testing"
)

func TestIsProse(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"todo.md", true},
		{"/tmp/README.MD", true},
		{"a/b/c.markdown", true},
		{"notes.TXT", true},
		{"doc.rst", true},
		{"plan.org", true},
		{"main.go", false},
		{"README", false},
		{"x.md.bak", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsProse(c.path); got != c.want {
			t.Errorf("IsProse(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestIsCheckbox(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"- [ ] задача", true},
		{"- [x] сделано", true},
		{"  - [x] вложенный", true},
		{"\t* [ ] таб", true},
		{"+ [X] плюс", true},
		{"[ ] голый", true},
		{"   [x] голый с отступом", true},
		{"- [ ] ", true},
		{"[x] ", true},
		{"- [x]без пробела", false},
		{"- [x]", false},
		{"[x]без пробела", false},
		{"[x]", false},
		{"- [] пусто", false},
		{"- [y] буква", false},
		{"- [  ] два пробела", false},
		{"- [ ]", false},
		{"- просто пункт", false},
		{"> - [ ] цитата", false},
		{"обычный текст", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsCheckbox(c.line); got != c.want {
			t.Errorf("IsCheckbox(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}

func TestToggleCheckboxCycle(t *testing.T) {
	cases := []struct{ in, once, twice string }{
		{"- [ ] задача", "- [x] задача", "- [ ] задача"},
		{"- [x] задача", "- [ ] задача", "- [x] задача"},
		{"  - [ ] вложенный", "  - [x] вложенный", "  - [ ] вложенный"},
		{"\t- [x] таб", "\t- [ ] таб", "\t- [x] таб"},
		{"    * [ ] звёздочка", "    * [x] звёздочка", "    * [ ] звёздочка"},
		{"  + [x] плюс", "  + [ ] плюс", "  + [x] плюс"},
		{"[ ] голый", "[x] голый", "[ ] голый"},
		{"  [x] голый с отступом", "  [ ] голый с отступом", "  [x] голый с отступом"},
		{"- [X] заглавная", "- [ ] заглавная", "- [x] заглавная"},
	}
	for _, c := range cases {
		once, ok := ToggleCheckbox(c.in)
		if !ok {
			t.Fatalf("ToggleCheckbox(%q) не нашёл чекбокс", c.in)
		}
		if once != c.once {
			t.Errorf("ToggleCheckbox(%q) = %q, want %q", c.in, once, c.once)
		}
		twice, ok := ToggleCheckbox(once)
		if !ok {
			t.Fatalf("ToggleCheckbox(%q) не нашёл чекбокс", once)
		}
		if twice != c.twice {
			t.Errorf("повтор ToggleCheckbox(%q) = %q, want %q", once, twice, c.twice)
		}
	}
}

func TestToggleCheckboxInserts(t *testing.T) {
	cases := []struct{ in, want string }{
		{"задача", "- [ ] задача"},
		{"  задача", "  - [ ] задача"},
		{"\tзадача", "\t- [ ] задача"},
		{"", "- [ ] "},
		{"   ", "   - [ ] "},
		{"> цитата", "- [ ] > цитата"},
		{"- просто пункт", "- [ ] - просто пункт"},
		{"- [x]без пробела", "- [ ] - [x]без пробела"},
	}
	for _, c := range cases {
		got, ok := ToggleCheckbox(c.in)
		if !ok {
			t.Fatalf("ToggleCheckbox(%q) = false", c.in)
		}
		if got != c.want {
			t.Errorf("ToggleCheckbox(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestToggleCheckboxAt(t *testing.T) {
	lines := runes(
		"- [ ] до",
		"```go",
		"- [ ] код",
		"```",
		"  - [x] после",
	)
	cases := []struct {
		line int
		want string
		ok   bool
	}{
		{0, "- [x] до", true},
		{1, "", false},
		{2, "", false},
		{3, "", false},
		{4, "  - [ ] после", true},
		{-1, "", false},
		{5, "", false},
	}
	for _, c := range cases {
		got, ok := ToggleCheckboxAt(lines, c.line)
		if got != c.want || ok != c.ok {
			t.Errorf("ToggleCheckboxAt(line %d) = (%q, %v), want (%q, %v)", c.line, got, ok, c.want, c.ok)
		}
	}

	// Незакрытый fence: всё после ограждения — код.
	if _, ok := ToggleCheckboxAt(runes("```", "- [ ] код"), 1); ok {
		t.Error("ToggleCheckboxAt залез в незакрытый fence")
	}
}

func TestListPrefix(t *testing.T) {
	cases := []struct {
		line string
		want string
		ok   bool
	}{
		{"- пункт", "- ", true},
		{"* пункт", "* ", true},
		{"+ пункт", "+ ", true},
		{"1. пункт", "1. ", true},
		{"12) пункт", "12) ", true},
		{"   3. вложенный", "3. ", true},
		{"\t- таб", "- ", true},
		{"-  два пробела", "- ", true},
		{"1) пункт", "1) ", true},
		{"-", "", false},
		{"1.", "", false},
		{"1 . пункт", "", false},
		{"текст", "", false},
		{"  ", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := ListPrefix(c.line)
		if got != c.want || ok != c.ok {
			t.Errorf("ListPrefix(%q) = (%q, %v), want (%q, %v)", c.line, got, ok, c.want, c.ok)
		}
	}
}

func TestContinueList(t *testing.T) {
	cases := []struct {
		line   string
		insert string
		clear  bool
	}{
		{"- пункт", "\n- ", false},
		{"* пункт", "\n* ", false},
		{"+ пункт", "\n+ ", false},
		{"  - вложенный", "\n  - ", false},
		{"\t* таб", "\n\t* ", false},
		{"9. пункт", "\n10. ", false},
		{"99. пункт", "\n100. ", false},
		{"12) пункт", "\n13) ", false},
		{"  12. вложенный", "\n  13. ", false},
		{"- ", "", true},
		{"  *   ", "", true},
		{"1. ", "", true},
		{"-", "", false},
		{"1.", "", false},
		{"", "", false},
		{"  ", "", false},
		{"обычный текст", "", false},
	}
	for _, c := range cases {
		insert, clear := ContinueList(c.line)
		if insert != c.insert || clear != c.clear {
			t.Errorf("ContinueList(%q) = (%q, %v), want (%q, %v)", c.line, insert, clear, c.insert, c.clear)
		}
	}
}

func TestCollapseSpaces(t *testing.T) {
	cases := []struct{ in, want string }{
		{"a  b", "a b"},
		{"  a   b  ", "  a b"},
		{"\t\ta\t\tb", "\t\ta b"},
		{"    a \t b", "    a b"},
		{"a\tb", "a b"},
		{"a  ", "a"},
		{"a\t", "a"},
		{"   ", "   "},
		{"", ""},
		{"без  изменений", "без изменений"},
		{"  ё  ё", "  ё ё"},
		{"a  b  c", "a b c"},
	}
	for _, c := range cases {
		if got := CollapseSpaces(c.in); got != c.want {
			t.Errorf("CollapseSpaces(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFenceState(t *testing.T) {
	lines := runes(
		"обычный",
		"```go",
		"fmt.Println(`x`)",
		"- [ ] не задача",
		"```",
		"снаружи",
		"  ~~~",
		"тильды",
		"  ~~~  ",
		"хвост",
	)
	want := []bool{false, false, true, true, false, false, false, true, false, false}
	if got := FenceState(lines); !reflect.DeepEqual(got, want) {
		t.Fatalf("FenceState = %v, want %v", got, want)
	}
}

func TestFenceStateNestedAndUnclosed(t *testing.T) {
	nested := FenceState(runes(
		"````",
		"```",
		"````",
		"```",
		"```",
	))
	if want := []bool{false, true, false, false, false}; !reflect.DeepEqual(nested, want) {
		t.Errorf("вложенный fence = %v, want %v", nested, want)
	}

	mixed := FenceState(runes(
		"```",
		"~~~",
		"```",
	))
	if want := []bool{false, true, false}; !reflect.DeepEqual(mixed, want) {
		t.Errorf("разные ограждения = %v, want %v", mixed, want)
	}

	unclosed := FenceState(runes(
		"```",
		"код",
		"",
	))
	if want := []bool{false, true, true}; !reflect.DeepEqual(unclosed, want) {
		t.Errorf("незакрытый fence = %v, want %v", unclosed, want)
	}

	if got := FenceState(nil); len(got) != 0 {
		t.Errorf("FenceState(nil) = %v, want пусто", got)
	}
}

func TestSentence(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"одно два три четыре", false},
		{"одно два три четыре пять", true},
		{"Это простое предложение из пяти слов", true},
		{"one two three four five", true},
		{"одно\nдва\nтри\nчетыре\nпять", true},
		{"", false},
		{"     ", false},
		{"12345 67890 11111 22222 33333", false},
		{"1 2 3 4 пять", true},
	}
	for _, c := range cases {
		if got := Sentence(c.text); got != c.want {
			t.Errorf("Sentence(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}

func TestCyrillicShare(t *testing.T) {
	cases := []struct {
		sample string
		want   float64
	}{
		{"Привет, мир!", 1},
		{"Ёлка", 1},
		{"ё", 1},
		{"hello", 0},
		{"", 0},
		{"123 !", 0},
		{"Привет hello", 6.0 / 11.0},
		{"abcПривет", 6.0 / 9.0},
		{"hello Ё", 1.0 / 6.0},
	}
	for _, c := range cases {
		if got := CyrillicShare(c.sample); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("CyrillicShare(%q) = %v, want %v", c.sample, got, c.want)
		}
	}
}

func runes(lines ...string) [][]rune {
	out := make([][]rune, len(lines))
	for i, line := range lines {
		out[i] = []rune(line)
	}
	return out
}
