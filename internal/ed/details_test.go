package ed

import "testing"

func TestInsertPairWrapsSelection(t *testing.T) {
	e := testEditor("f.txt", "abc")
	e.SetSelection(Pos{0, 1}, Pos{0, 2})
	if !e.InsertPair("(", ")") {
		t.Fatal("InsertPair = false")
	}
	if got := e.Buf.Text(); got != "a(b)c" {
		t.Fatalf("Text = %q", got)
	}
	if e.Cursor != (Pos{0, 4}) || e.HasSelection() {
		t.Fatalf("cursor=%v sel=%+v", e.Cursor, e.Sel)
	}
	if !e.Undo() {
		t.Fatal("Undo = false")
	}
	if got := e.Buf.Text(); got != "abc" {
		t.Fatalf("после undo %q", got)
	}
	if !e.HasSelection() || e.SelectedText() != "b" || e.Cursor != (Pos{0, 2}) {
		t.Fatalf("выделение не восстановлено: %+v cursor=%v", e.Sel, e.Cursor)
	}
	if !e.Redo() {
		t.Fatal("Redo = false")
	}
	if got := e.Buf.Text(); got != "a(b)c" || e.Cursor != (Pos{0, 4}) {
		t.Fatalf("после redo %q cursor=%v", got, e.Cursor)
	}
}

func TestInsertPairInsertAndSkip(t *testing.T) {
	e := testEditor("f.txt", "ab")
	e.SetCursor(Pos{0, 1})
	if !e.InsertPair("(", ")") {
		t.Fatal("InsertPair = false")
	}
	if got := e.Buf.Text(); got != "a()b" || e.Cursor != (Pos{0, 2}) {
		t.Fatalf("вставка пары %q cursor=%v", got, e.Cursor)
	}
	if !e.Undo() || e.Buf.Text() != "ab" || e.Cursor != (Pos{0, 1}) {
		t.Fatalf("undo пары %q cursor=%v", e.Buf.Text(), e.Cursor)
	}

	e = testEditor("f.txt", "a)b")
	e.SetCursor(Pos{0, 1})
	if !e.InsertPair("(", ")") {
		t.Fatal("skip-over = false")
	}
	if got := e.Buf.Text(); got != "a)b" || e.Cursor != (Pos{0, 2}) {
		t.Fatalf("skip-over %q cursor=%v", got, e.Cursor)
	}
	if e.CanUndo() {
		t.Fatal("skip-over создал undo-запись")
	}

	e = testEditor("f.txt", `""`)
	e.SetCursor(Pos{0, 1})
	if !e.InsertPair(`"`, `"`) {
		t.Fatal("skip-over кавычки = false")
	}
	if got := e.Buf.Text(); got != `""` || e.Cursor != (Pos{0, 2}) {
		t.Fatalf("кавычка %q cursor=%v", got, e.Cursor)
	}

	if e.InsertPair("", ")") || e.InsertPair("(", "") {
		t.Fatal("пустые аргументы дали true")
	}
}

func TestDeleteEmptyPair(t *testing.T) {
	e := testEditor("f.txt", "a()b")
	e.SetCursor(Pos{0, 2})
	if !e.DeleteEmptyPair('(', ')') {
		t.Fatal("DeleteEmptyPair = false")
	}
	if got := e.Buf.Text(); got != "ab" || e.Cursor != (Pos{0, 1}) {
		t.Fatalf("после удаления %q cursor=%v", got, e.Cursor)
	}
	if !e.Undo() || e.Buf.Text() != "a()b" || e.Cursor != (Pos{0, 2}) {
		t.Fatalf("undo %q cursor=%v", e.Buf.Text(), e.Cursor)
	}

	e.SetCursor(Pos{0, 1})
	if e.DeleteEmptyPair('(', ')') {
		t.Fatal("не между парой = true")
	}
	e.SetSelection(Pos{0, 0}, Pos{0, 2})
	if e.DeleteEmptyPair('(', ')') {
		t.Fatal("с выделением = true")
	}
}

func TestMatchingBracket(t *testing.T) {
	e := testEditor("f.go", "a(b[c]d)e")
	cases := []struct {
		p, want Pos
	}{
		{Pos{0, 1}, Pos{0, 7}}, // на (
		{Pos{0, 3}, Pos{0, 5}}, // на [
		{Pos{0, 5}, Pos{0, 3}}, // на ]
		{Pos{0, 2}, Pos{0, 7}}, // сразу после (
		{Pos{0, 8}, Pos{0, 1}}, // сразу после )
	}
	for _, c := range cases {
		got, ok := e.Buf.MatchingBracket(c.p)
		if !ok || got != c.want {
			t.Fatalf("MatchingBracket(%v) = %v,%v want %v", c.p, got, ok, c.want)
		}
	}
	if _, ok := e.Buf.MatchingBracket(Pos{0, 0}); ok {
		t.Fatal("не скобка дала пару")
	}

	nested := testEditor("f.go", "((a))")
	if got, ok := nested.Buf.MatchingBracket(Pos{0, 0}); !ok || got != (Pos{0, 4}) {
		t.Fatalf("внешняя = %v,%v", got, ok)
	}
	if got, ok := nested.Buf.MatchingBracket(Pos{0, 1}); !ok || got != (Pos{0, 3}) {
		t.Fatalf("внутренняя = %v,%v", got, ok)
	}
}

func TestMatchingBracketMultilineUnbalancedEscaped(t *testing.T) {
	e := testEditor("f.go", "{\n  (x)\n}")
	ml := []struct {
		p, want Pos
	}{
		{Pos{0, 0}, Pos{2, 0}},
		{Pos{0, 1}, Pos{2, 0}}, // после {
		{Pos{2, 0}, Pos{0, 0}},
		{Pos{1, 2}, Pos{1, 4}},
		{Pos{1, 3}, Pos{1, 4}},
		{Pos{1, 4}, Pos{1, 2}},
	}
	for _, c := range ml {
		got, ok := e.Buf.MatchingBracket(c.p)
		if !ok || got != c.want {
			t.Fatalf("multiline MatchingBracket(%v) = %v,%v want %v", c.p, got, ok, c.want)
		}
	}

	for _, text := range []string{"(a", "a)", "((a)"} {
		u := testEditor("f.go", text)
		if got, ok := u.Buf.MatchingBracket(Pos{0, 0}); ok {
			t.Fatalf("%q: непарная скобка дала %v", text, got)
		}
	}

	esc := testEditor("f.go", `(\(a)`)
	if got, ok := esc.Buf.MatchingBracket(Pos{0, 0}); !ok || got != (Pos{0, 4}) {
		t.Fatalf("экранированная внутри: %v,%v", got, ok)
	}
	if got, ok := esc.Buf.MatchingBracket(Pos{0, 2}); ok {
		t.Fatalf("экранированная скобка дала пару %v", got)
	}
	if got, ok := esc.Buf.MatchingBracket(Pos{0, 3}); ok {
		t.Fatalf("после экранированной дала пару %v", got)
	}
}

func TestTrimTrailingWhitespace(t *testing.T) {
	src := "a  \n  b\t\n\t\n \n c \n"
	e := testEditor("f.txt", src)
	e.SetCursor(Pos{1, 3})
	if !e.TrimTrailingWhitespace() {
		t.Fatal("TrimTrailingWhitespace = false")
	}
	const want = "a\n  b\n\t\n \n c\n"
	if got := e.Buf.Text(); got != want {
		t.Fatalf("Text = %q want %q", got, want)
	}
	if e.Cursor != (Pos{1, 3}) {
		t.Fatalf("курсор сдвинут: %v", e.Cursor)
	}
	if e.TrimTrailingWhitespace() {
		t.Fatal("повторный trim = true")
	}
	if !e.Undo() {
		t.Fatal("Undo = false")
	}
	if got := e.Buf.Text(); got != src {
		t.Fatalf("undo = %q", got)
	}
	if e.CanUndo() {
		t.Fatal("trim создал больше одной undo-записи")
	}
}

func TestEnsureFinalNewline(t *testing.T) {
	e := testEditor("f.txt", "abc")
	e.SetCursor(Pos{0, 1})
	if !e.EnsureFinalNewline() {
		t.Fatal("EnsureFinalNewline = false")
	}
	if got := e.Buf.Text(); got != "abc\n" {
		t.Fatalf("Text = %q", got)
	}
	if e.Cursor != (Pos{0, 1}) {
		t.Fatalf("курсор потерян: %v", e.Cursor)
	}
	if e.EnsureFinalNewline() {
		t.Fatal("повторный EnsureFinalNewline = true")
	}
	if !e.Undo() || e.Buf.Text() != "abc" || e.Cursor != (Pos{0, 1}) {
		t.Fatalf("undo %q cursor=%v", e.Buf.Text(), e.Cursor)
	}

	if testEditor("f.txt", "").EnsureFinalNewline() {
		t.Fatal("пустой буфер = true")
	}
	if testEditor("f.txt", "abc\n").EnsureFinalNewline() {
		t.Fatal("уже с переводом = true")
	}
}

func TestBreakPair(t *testing.T) {
	e := testEditor("f.go", "  {}")
	e.SetCursor(Pos{0, 3})
	if !e.BreakPair('{', '}', "  ", "    ") {
		t.Fatal("BreakPair = false")
	}
	const want = "  {\n      \n  }"
	if got := e.Buf.Text(); got != want {
		t.Fatalf("Text = %q want %q", got, want)
	}
	if e.Cursor != (Pos{1, 6}) {
		t.Fatalf("курсор = %v", e.Cursor)
	}
	if !e.Undo() || e.Buf.Text() != "  {}" || e.Cursor != (Pos{0, 3}) {
		t.Fatalf("undo %q cursor=%v", e.Buf.Text(), e.Cursor)
	}
	if !e.Redo() || e.Buf.Text() != want || e.Cursor != (Pos{1, 6}) {
		t.Fatalf("redo %q cursor=%v", e.Buf.Text(), e.Cursor)
	}

	p := testEditor("f.go", "()")
	p.SetCursor(Pos{0, 1})
	if !p.BreakPair('(', ')', "", "\t") {
		t.Fatal("BreakPair () = false")
	}
	if got := p.Buf.Text(); got != "(\n\t\n)" || p.Cursor != (Pos{1, 1}) {
		t.Fatalf("() = %q cursor=%v", got, p.Cursor)
	}

	no := testEditor("f.go", "ab")
	no.SetCursor(Pos{0, 1})
	if no.BreakPair('{', '}', "", "    ") {
		t.Fatal("не между парой = true")
	}
	mis := testEditor("f.go", "{}")
	mis.SetCursor(Pos{0, 1})
	if mis.BreakPair('{', ')', "", "    ") {
		t.Fatal("разные скобки = true")
	}
}

func TestSmartHome(t *testing.T) {
	e := testEditor("f.txt", "    foo\n\tbar\n\n   ")
	e.SetCursor(Pos{0, 7})
	e.SmartHome()
	if e.Cursor != (Pos{0, 4}) {
		t.Fatalf("на первый непробел = %v", e.Cursor)
	}
	e.SmartHome()
	if e.Cursor != (Pos{0, 0}) {
		t.Fatalf("повторно = %v", e.Cursor)
	}
	e.SmartHome()
	if e.Cursor != (Pos{0, 4}) {
		t.Fatalf("снова = %v", e.Cursor)
	}

	e.SetCursor(Pos{1, 4})
	e.SmartHome()
	if e.Cursor != (Pos{1, 1}) {
		t.Fatalf("таб = %v", e.Cursor)
	}
	e.SmartHome()
	if e.Cursor != (Pos{1, 0}) {
		t.Fatalf("таб в 0 = %v", e.Cursor)
	}

	e.SetCursor(Pos{2, 0})
	e.SmartHome()
	if e.Cursor != (Pos{2, 0}) {
		t.Fatalf("пустая строка = %v", e.Cursor)
	}
	e.SetCursor(Pos{3, 0})
	e.SmartHome()
	if e.Cursor != (Pos{3, 3}) {
		t.Fatalf("строка из пробелов = %v", e.Cursor)
	}

	e.SetSelection(Pos{0, 0}, Pos{0, 6})
	e.SmartHome()
	if e.HasSelection() || e.Cursor != (Pos{0, 4}) {
		t.Fatalf("выделение не свернулось: %+v cursor=%v", e.Sel, e.Cursor)
	}
}

func TestDetailsReadOnly(t *testing.T) {
	b := OpenBytes([]byte("x\x00()"), "f.bin")
	if !b.ReadOnly {
		t.Fatal("бинарник не read-only")
	}
	e := NewEditor(b)
	before := b.Text()
	e.SetCursor(Pos{0, 3})
	if e.InsertPair("(", ")") || e.DeleteEmptyPair('(', ')') ||
		e.BreakPair('(', ')', "", "    ") || e.TrimTrailingWhitespace() ||
		e.EnsureFinalNewline() {
		t.Fatal("правка в read-only вернула true")
	}
	if e.CanUndo() {
		t.Fatal("read-only создал undo-запись")
	}
	if got := b.Text(); got != before {
		t.Fatalf("read-only изменился: %q", got)
	}
	if got, ok := b.MatchingBracket(Pos{0, 2}); !ok || got != (Pos{0, 3}) {
		t.Fatalf("MatchingBracket в read-only = %v,%v", got, ok)
	}
	e.SmartHome() // движение разрешено
	if e.Cursor != (Pos{0, 0}) {
		t.Fatalf("SmartHome в read-only: %v", e.Cursor)
	}
}
