package ed

import "testing"

func TestToggleCheckboxCycle(t *testing.T) {
	e := testEditor("todo.md", "  - [ ] задача")
	if !e.ToggleCheckbox() {
		t.Fatal("ToggleCheckbox = false")
	}
	if got := e.Buf.Text(); got != "  - [x] задача" {
		t.Fatalf("Text = %q", got)
	}
	if !e.ToggleCheckbox() {
		t.Fatal("ToggleCheckbox 2 = false")
	}
	if got := e.Buf.Text(); got != "  - [ ] задача" {
		t.Fatalf("Text = %q", got)
	}
	if !e.Undo() || e.Buf.Text() != "  - [x] задача" {
		t.Fatalf("undo чекбокса = %q", e.Buf.Text())
	}
	if !e.Undo() || e.Buf.Text() != "  - [ ] задача" {
		t.Fatalf("undo 2 чекбокса = %q", e.Buf.Text())
	}
}

func TestToggleCheckboxBareAndNew(t *testing.T) {
	e := testEditor("todo.md", "[ ] задача")
	e.ToggleCheckbox()
	if got := e.Buf.Text(); got != "[x] задача" {
		t.Fatalf("голый чекбокс = %q", got)
	}

	e = testEditor("todo.md", "задача")
	e.ToggleCheckbox()
	if got := e.Buf.Text(); got != "- [ ] задача" {
		t.Fatalf("новый чекбокс = %q", got)
	}
	if e.Cursor != (Pos{0, 6}) {
		t.Fatalf("курсор после вставки = %v", e.Cursor)
	}

	e = testEditor("todo.md", "  - задача")
	e.ToggleCheckbox()
	if got := e.Buf.Text(); got != "  - [ ] задача" {
		t.Fatalf("список без чекбокса = %q", got)
	}

	e = testEditor("todo.md", "")
	e.ToggleCheckbox()
	if got := e.Buf.Text(); got != "- [ ] " {
		t.Fatalf("пустая строка = %q", got)
	}
}

func TestToggleCheckboxSkipsCodeAndOtherFiles(t *testing.T) {
	src := "```\n- [ ] код\n```\n- [ ] задача"
	e := testEditor("todo.md", src)
	e.SetCursor(Pos{1, 3})
	if e.ToggleCheckbox() {
		t.Fatal("чекбокс внутри кода изменён")
	}
	if got := e.Buf.Text(); got != src {
		t.Fatalf("код изменился: %q", got)
	}
	e.SetCursor(Pos{3, 3})
	if !e.ToggleCheckbox() {
		t.Fatal("чекбокс после кода не переключился")
	}
	if got := e.Buf.Text(); got != "```\n- [ ] код\n```\n- [x] задача" {
		t.Fatalf("Text = %q", got)
	}

	e = testEditor("main.go", "// задача")
	if e.ToggleCheckbox() {
		t.Fatal("ToggleCheckbox в .go = true")
	}
	if got := e.Buf.Text(); got != "// задача" {
		t.Fatalf(".go изменился: %q", got)
	}
}

func TestToggleCheckboxTildeFence(t *testing.T) {
	e := testEditor("todo.md", "~~~\n- [ ] код\n~~~\n- [ ] задача")
	e.SetCursor(Pos{1, 0})
	if e.ToggleCheckbox() {
		t.Fatal("~~~-блок не распознан")
	}
	e.SetCursor(Pos{3, 0})
	if !e.ToggleCheckbox() {
		t.Fatal("строка после ~~~ не переключилась")
	}
}

func TestMarkdownFourBacktickFence(t *testing.T) {
	src := "````\n```\n- [ ] код\n```\n````\n- [ ] задача"
	e := testEditor("todo.md", src)
	e.SetCursor(Pos{2, 3})
	if e.ToggleCheckbox() {
		t.Fatalf("короткое ограждение закрыло длинный fence: %q", e.Buf.Text())
	}
	e.SetCursor(Pos{5, 3})
	if !e.ToggleCheckbox() || e.Buf.Text() == src {
		t.Fatalf("строка после fence не переключилась: %q", e.Buf.Text())
	}

	e = testEditor("todo.md", "````\n```\n- пункт\n```\n````")
	e.SetCursor(Pos{2, 6})
	e.ContinueList()
	if got := e.Buf.Text(); got != "````\n```\n- пункт\n```\n````" {
		t.Fatalf("ContinueList залез в fence: %q", got)
	}
}

func TestToggleCheckboxTabIndentAndStar(t *testing.T) {
	e := testEditor("todo.md", "\t* [ ] задача")
	if !e.ToggleCheckbox() {
		t.Fatal("ToggleCheckbox = false")
	}
	if got := e.Buf.Text(); got != "\t* [x] задача" {
		t.Fatalf("Text = %q", got)
	}
	if got := e.Buf.Indent(Pos{0, 0}); got != "\t" {
		t.Fatalf("отступ = %q", got)
	}
}

func TestContinueList(t *testing.T) {
	e := testEditor("todo.md", "- пункт")
	e.MoveEnd(false)
	e.ContinueList()
	if got := e.Buf.Text(); got != "- пункт\n- " {
		t.Fatalf("Text = %q", got)
	}
	if e.Cursor != (Pos{1, 2}) {
		t.Fatalf("cursor = %v", e.Cursor)
	}

	e = testEditor("todo.md", "1. пункт")
	e.MoveEnd(false)
	e.ContinueList()
	if got := e.Buf.Text(); got != "1. пункт\n2. " {
		t.Fatalf("нумерованный список = %q", got)
	}

	e = testEditor("todo.md", "- [x] сделано")
	e.MoveEnd(false)
	e.ContinueList()
	if got := e.Buf.Text(); got != "- [x] сделано\n- [ ] " {
		t.Fatalf("чекбокс-список = %q", got)
	}

	e = testEditor("todo.md", "   - вложенный")
	e.MoveEnd(false)
	e.ContinueList()
	if got := e.Buf.Text(); got != "   - вложенный\n   - " {
		t.Fatalf("вложенный список = %q", got)
	}

	e = testEditor("todo.md", "текст")
	e.MoveEnd(false)
	e.ContinueList()
	if got := e.Buf.Text(); got != "текст" {
		t.Fatalf("не список изменился: %q", got)
	}
}

func TestContinueListEmptyItem(t *testing.T) {
	e := testEditor("todo.md", "- ")
	e.MoveEnd(false)
	e.ContinueList()
	if got := e.Buf.Text(); got != "" {
		t.Fatalf("пустой пункт = %q", got)
	}
	if e.Cursor != (Pos{0, 0}) {
		t.Fatalf("cursor = %v", e.Cursor)
	}

	e = testEditor("todo.md", "  - ")
	e.MoveEnd(false)
	e.ContinueList()
	if got := e.Buf.Text(); got != "  " {
		t.Fatalf("вложенный пустой пункт = %q", got)
	}

	e = testEditor("todo.md", "- [ ] ")
	e.MoveEnd(false)
	e.ContinueList()
	if got := e.Buf.Text(); got != "" {
		t.Fatalf("пустой чекбокс = %q", got)
	}
}

func TestContinueListSkipsCodeAndOtherFiles(t *testing.T) {
	e := testEditor("todo.md", "```\n- код\n```")
	e.SetCursor(Pos{1, 6})
	e.ContinueList()
	if got := e.Buf.Text(); got != "```\n- код\n```" {
		t.Fatalf("код изменился: %q", got)
	}
	e = testEditor("main.go", "- не список")
	e.MoveEnd(false)
	e.ContinueList()
	if got := e.Buf.Text(); got != "- не список" {
		t.Fatalf(".go изменился: %q", got)
	}
}

func TestContinueListMidLine(t *testing.T) {
	e := testEditor("todo.md", "- abc")
	e.SetCursor(Pos{0, 4})
	e.ContinueList()
	if got := e.Buf.Text(); got != "- ab\n- c" {
		t.Fatalf("Text = %q", got)
	}
	if e.Cursor != (Pos{1, 2}) {
		t.Fatalf("cursor = %v", e.Cursor)
	}
	if !e.Undo() || e.Buf.Text() != "- abc" {
		t.Fatalf("undo продолжения = %q", e.Buf.Text())
	}
}

func TestCollapseSpaces(t *testing.T) {
	e := testEditor("f.txt", "  a   b\t c  d")
	if !e.CollapseSpaces() {
		t.Fatal("CollapseSpaces = false")
	}
	if got := e.Buf.Text(); got != "  a b\t c d" {
		t.Fatalf("Text = %q", got)
	}
	if e.CollapseSpaces() {
		t.Fatal("повторный вызов = true")
	}
}

func TestCollapseSpacesSelection(t *testing.T) {
	e := testEditor("f.txt", "a  b\nc   d")
	e.SelectAll()
	if !e.CollapseSpaces() {
		t.Fatal("CollapseSpaces = false")
	}
	if got := e.Buf.Text(); got != "a b\nc d" {
		t.Fatalf("Text = %q", got)
	}
	if !e.Undo() || e.Buf.Text() != "a  b\nc   d" {
		t.Fatalf("undo схлопывания = %q", e.Buf.Text())
	}
}

func TestIndentOutdentSelection(t *testing.T) {
	e := testEditor("f.go", "a\nb\nc")
	e.SelectAll()
	if !e.IndentSelection() {
		t.Fatal("IndentSelection = false")
	}
	if got := e.Buf.Text(); got != "    a\n    b\n    c" {
		t.Fatalf("indent = %q", got)
	}
	if !e.HasSelection() {
		t.Fatal("indent снял выделение")
	}
	if !e.Undo() || e.Buf.Text() != "a\nb\nc" {
		t.Fatalf("undo indent = %q", e.Buf.Text())
	}

	e.SetCursor(Pos{1, 0})
	if !e.IndentSelection() || e.Buf.Text() != "a\n    b\nc" {
		t.Fatalf("indent текущей строки = %q", e.Buf.Text())
	}
	if !e.OutdentSelection() || e.Buf.Text() != "a\nb\nc" {
		t.Fatalf("outdent текущей строки = %q", e.Buf.Text())
	}
}

func TestIndentKeepsTabs(t *testing.T) {
	e := testEditor("f.go", "\tx\n\ty")
	e.SelectAll()
	e.IndentSelection()
	if got := e.Buf.Text(); got != "\t\tx\n\t\ty" {
		t.Fatalf("indent с табами = %q", got)
	}
	e.OutdentSelection()
	if got := e.Buf.Text(); got != "\tx\n\ty" {
		t.Fatalf("outdent табов = %q", got)
	}
	e.OutdentSelection()
	if got := e.Buf.Text(); got != "x\ny" {
		t.Fatalf("outdent табов 2 = %q", got)
	}
}

func TestOutdentRemovesPartialSpaces(t *testing.T) {
	e := testEditor("f.go", "  a\n      b")
	e.SelectAll()
	if !e.OutdentSelection() {
		t.Fatal("OutdentSelection = false")
	}
	if got := e.Buf.Text(); got != "a\n  b" {
		t.Fatalf("Text = %q", got)
	}
	e = testEditor("f.go", "a")
	if e.OutdentSelection() {
		t.Fatal("outdent без отступа = true")
	}
}

func TestIndentSelectionExcludesTrailingEmptyLine(t *testing.T) {
	e := testEditor("f.go", "a\nb\nc")
	e.SetSelection(Pos{0, 0}, Pos{2, 0})
	e.IndentSelection()
	if got := e.Buf.Text(); got != "    a\n    b\nc" {
		t.Fatalf("Text = %q", got)
	}
}

func TestOutdentReversedSelection(t *testing.T) {
	e := testEditor("f.go", "    a\n    b")
	e.SetSelection(Pos{1, 5}, Pos{0, 0}) // обратное выделение
	if !e.OutdentSelection() {
		t.Fatal("OutdentSelection = false")
	}
	if got := e.Buf.Text(); got != "a\nb" {
		t.Fatalf("Text = %q", got)
	}
	if !e.HasSelection() || e.SelectedText() != "a\nb" {
		t.Fatalf("выделение после outdent: %+v %q", e.Sel, e.SelectedText())
	}
}

func TestOutdentKeepsCursorAfterLine(t *testing.T) {
	e := testEditor("f.go", "    x")
	e.SetCursor(Pos{0, 5})
	e.OutdentSelection()
	if e.Cursor != (Pos{0, 1}) {
		t.Fatalf("cursor = %v, want {0 1}", e.Cursor)
	}
}

func TestCollapseSpacesKeepsCursor(t *testing.T) {
	e := testEditor("f.txt", "a   b")
	e.SetCursor(Pos{0, 5})
	e.CollapseSpaces()
	if e.Cursor != (Pos{0, 3}) {
		t.Fatalf("cursor = %v, want {0 3}", e.Cursor)
	}
}

func TestCutUndoRestoresTextAndSelection(t *testing.T) {
	e := testEditor("f.txt", "один два")
	e.SetSelection(Pos{0, 5}, Pos{0, 8})
	if got := e.Cut(); got != "два" {
		t.Fatalf("Cut = %q", got)
	}
	if e.Buf.Text() != "один " {
		t.Fatalf("Text = %q", e.Buf.Text())
	}
	if !e.Undo() || e.Buf.Text() != "один два" {
		t.Fatalf("undo cut = %q", e.Buf.Text())
	}
	if !e.HasSelection() || e.SelectedText() != "два" {
		t.Fatalf("выделение cut: %+v %q", e.Sel, e.SelectedText())
	}
}
