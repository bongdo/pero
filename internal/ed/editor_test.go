package ed

import (
	"testing"
)

func testEditor(path, text string) *Editor {
	b := OpenBytes([]byte(text), path)
	return NewEditor(b)
}

func TestInsertReplacesSelectionAndUndoRestoresIt(t *testing.T) {
	e := testEditor("f.txt", "один два")
	e.SetSelection(Pos{0, 0}, Pos{0, 4})
	e.Insert("три")
	if got := e.Buf.Text(); got != "три два" {
		t.Fatalf("Text = %q", got)
	}
	if e.Cursor != (Pos{0, 3}) || e.HasSelection() {
		t.Fatalf("cursor=%v sel=%+v", e.Cursor, e.Sel)
	}
	if !e.Buf.Dirty() {
		t.Fatal("буфер не грязный")
	}
	if !e.Undo() {
		t.Fatal("Undo = false")
	}
	if got := e.Buf.Text(); got != "один два" {
		t.Fatalf("после undo %q", got)
	}
	if !e.HasSelection() || e.Sel.Anchor != (Pos{0, 0}) || e.Cursor != (Pos{0, 4}) {
		t.Fatalf("выделение не восстановлено: %+v cursor=%v", e.Sel, e.Cursor)
	}
	if got := e.SelectedText(); got != "один" {
		t.Fatalf("SelectedText = %q", got)
	}
	if !e.Redo() {
		t.Fatal("Redo = false")
	}
	if got := e.Buf.Text(); got != "три два" || e.Cursor != (Pos{0, 3}) || e.HasSelection() {
		t.Fatalf("после redo %q cursor=%v sel=%+v", got, e.Cursor, e.Sel)
	}
}

func TestInsertMultilineCursor(t *testing.T) {
	e := testEditor("f.txt", "ab")
	e.SetCursor(Pos{0, 1})
	e.Insert("X\nY")
	if got := e.Buf.Text(); got != "aX\nYb" {
		t.Fatalf("Text = %q", got)
	}
	if e.Cursor != (Pos{1, 1}) {
		t.Fatalf("cursor = %v", e.Cursor)
	}
}

func TestNewlineNormalization(t *testing.T) {
	e := testEditor("f.txt", "ab")
	e.SetCursor(Pos{0, 1})
	e.Insert("\r\nX\rY")
	if got := e.Buf.Text(); got != "a\nX\nYb" {
		t.Fatalf("Text = %q", got)
	}
	if e.Cursor != (Pos{2, 1}) {
		t.Fatalf("cursor = %v", e.Cursor)
	}
}

func TestDeleteBackwardAndForward(t *testing.T) {
	e := testEditor("f.txt", "абв\nгде")
	e.SetCursor(Pos{1, 0})
	if !e.DeleteBackward() {
		t.Fatal("DeleteBackward = false")
	}
	if got := e.Buf.Text(); got != "абвгде" {
		t.Fatalf("Text = %q", got)
	}
	if e.Cursor != (Pos{0, 3}) {
		t.Fatalf("cursor = %v", e.Cursor)
	}
	if !e.DeleteForward() {
		t.Fatal("DeleteForward = false")
	}
	if got := e.Buf.Text(); got != "абвде" {
		t.Fatalf("Text = %q", got)
	}
	if e.Cursor != (Pos{0, 3}) {
		t.Fatalf("cursor = %v", e.Cursor)
	}
	e.SetCursor(Pos{0, 0})
	if e.DeleteBackward() {
		t.Fatal("DeleteBackward в начале = true")
	}
	e.MoveDocEnd(false)
	if e.DeleteForward() {
		t.Fatal("DeleteForward в конце = true")
	}
}

func TestDeleteSelection(t *testing.T) {
	e := testEditor("f.txt", "один два")
	e.SetSelection(Pos{0, 5}, Pos{0, 8})
	if !e.DeleteBackward() {
		t.Fatal("DeleteBackward = false")
	}
	if got := e.Buf.Text(); got != "один " || e.Cursor != (Pos{0, 5}) {
		t.Fatalf("%q %v", got, e.Cursor)
	}
}

func TestMovementGoalsAndCollapse(t *testing.T) {
	e := testEditor("f.txt", "абв\nгде\nёж")
	e.MoveEnd(false)
	if e.Cursor != (Pos{0, 3}) {
		t.Fatalf("End = %v", e.Cursor)
	}
	e.MoveDown(false)
	if e.Cursor != (Pos{1, 3}) {
		t.Fatalf("Down = %v", e.Cursor)
	}
	e.MoveDown(false)
	if e.Cursor != (Pos{2, 2}) {
		t.Fatalf("Down short line = %v", e.Cursor)
	}
	e.MoveUp(false)
	if e.Cursor != (Pos{1, 3}) {
		t.Fatalf("Up goal = %v", e.Cursor)
	}
	e.MoveHome(false)
	if e.Cursor != (Pos{1, 0}) {
		t.Fatalf("Home = %v", e.Cursor)
	}
	e.MoveDocStart(false)
	if e.Cursor != (Pos{0, 0}) {
		t.Fatalf("DocStart = %v", e.Cursor)
	}
	e.MoveDocEnd(false)
	if e.Cursor != (Pos{2, 2}) {
		t.Fatalf("DocEnd = %v", e.Cursor)
	}
	e.MovePage(-2, false)
	if e.Cursor != (Pos{0, 2}) {
		t.Fatalf("Page = %v", e.Cursor)
	}
}

func TestSelectionExtendAndCollapse(t *testing.T) {
	e := testEditor("f.txt", "абвгд\nеё")
	e.MoveRight(false)
	e.MoveRight(true)
	e.MoveRight(true)
	if !e.HasSelection() || e.Sel.Anchor != (Pos{0, 1}) || e.Cursor != (Pos{0, 3}) {
		t.Fatalf("extend: %+v cursor=%v", e.Sel, e.Cursor)
	}
	e.MoveRight(false)
	if e.HasSelection() || e.Cursor != (Pos{0, 3}) {
		t.Fatalf("collapse right: %+v %v", e.Sel, e.Cursor)
	}

	e.SetSelection(Pos{0, 1}, Pos{0, 3})
	e.MoveLeft(false)
	if e.HasSelection() || e.Cursor != (Pos{0, 1}) {
		t.Fatalf("collapse left: %+v %v", e.Sel, e.Cursor)
	}

	e.MoveDown(true)
	if !e.HasSelection() || e.Sel.Anchor != (Pos{0, 1}) || e.Cursor != (Pos{1, 1}) {
		t.Fatalf("vertical extend: %+v %v", e.Sel, e.Cursor)
	}
}

func TestWordMovementAndSelectWord(t *testing.T) {
	e := testEditor("f.txt", "foo-bar_baz, мир\nещё")
	e.MoveWordRight(false)
	if e.Cursor != (Pos{0, 3}) {
		t.Fatalf("WordRight = %v", e.Cursor)
	}
	e.MoveWordRight(false)
	if e.Cursor != (Pos{0, 11}) {
		t.Fatalf("WordRight 2 = %v", e.Cursor)
	}
	e.MoveWordRight(false)
	if e.Cursor != (Pos{0, 16}) {
		t.Fatalf("WordRight 3 = %v", e.Cursor)
	}
	e.MoveWordLeft(false)
	if e.Cursor != (Pos{0, 13}) {
		t.Fatalf("WordLeft = %v", e.Cursor)
	}

	e.SetCursor(Pos{0, 5})
	e.SelectWord()
	if got := e.SelectedText(); got != "foo-bar_baz" {
		t.Fatalf("SelectWord = %q", got)
	}
	e.SetCursor(Pos{0, 20})
	e.SelectWord()
	if got := e.SelectedText(); got != "мир" {
		t.Fatalf("SelectWord at end = %q", got)
	}

	e.SetCursor(Pos{0, 2})
	e.SelectLine()
	if got := e.SelectedText(); got != "foo-bar_baz, мир\n" {
		t.Fatalf("SelectLine = %q", got)
	}
}

func TestSelectAllAndCopyCutPaste(t *testing.T) {
	e := testEditor("f.txt", "один\nдва")
	e.SelectAll()
	if got := e.Copy(); got != "один\nдва" {
		t.Fatalf("Copy = %q", got)
	}
	if got := e.Cut(); got != "один\nдва" {
		t.Fatalf("Cut = %q", got)
	}
	if e.Buf.Text() != "" || e.Cursor != (Pos{0, 0}) {
		t.Fatalf("после Cut %q %v", e.Buf.Text(), e.Cursor)
	}
	e.Paste("два")
	if e.Buf.Text() != "два" || e.Cursor != (Pos{0, 3}) {
		t.Fatalf("после Paste %q %v", e.Buf.Text(), e.Cursor)
	}
	e.SetSelection(Pos{0, 0}, Pos{0, 1})
	e.Paste("Д")
	if e.Buf.Text() != "Два" {
		t.Fatalf("Paste поверх выделения %q", e.Buf.Text())
	}
}

func TestReplaceRangeMapsCursor(t *testing.T) {
	e := testEditor("f.txt", "abcdef")
	e.SetCursor(Pos{0, 4})
	e.ReplaceRange(Range{Start: Pos{0, 1}, End: Pos{0, 3}}, "X")
	if got := e.Buf.Text(); got != "aXdef" {
		t.Fatalf("Text = %q", got)
	}
	if e.Cursor != (Pos{0, 3}) {
		t.Fatalf("cursor = %v", e.Cursor)
	}

	e.SetCursor(Pos{0, 1})
	e.ReplaceRange(Range{Start: Pos{0, 1}, End: Pos{0, 2}}, "YYY")
	if e.Cursor != (Pos{0, 4}) {
		t.Fatalf("cursor после попадания в диапазон = %v", e.Cursor)
	}
}

func TestReplaceRangeReversedAndClamped(t *testing.T) {
	e := testEditor("f.txt", "abcdef")
	e.ReplaceRange(Range{Start: Pos{0, 4}, End: Pos{0, 1}}, "X")
	if got := e.Buf.Text(); got != "aXef" {
		t.Fatalf("обратный диапазон = %q", got)
	}
	e.ReplaceRange(Range{Start: Pos{-5, -5}, End: Pos{99, 99}}, "")
	if got := e.Buf.Text(); got != "" {
		t.Fatalf("clamp диапазона = %q", got)
	}
}

func TestEditorReadOnly(t *testing.T) {
	b := OpenBytes([]byte("text\x00"), "f.txt")
	if !b.ReadOnly {
		t.Fatal("бинарник не read-only")
	}
	e := NewEditor(b)
	before := e.Buf.Text()
	e.Insert("x")
	e.Paste("y")
	e.ReplaceRange(Range{Start: Pos{}, End: Pos{0, 1}}, "z")
	if e.DeleteBackward() || e.DeleteForward() {
		t.Fatal("удаление в read-only вернуло true")
	}
	if e.Undo() || e.Redo() {
		t.Fatal("undo/redo в read-only вернули true")
	}
	if e.ToggleCheckbox() || e.CollapseSpaces() || e.IndentSelection() || e.OutdentSelection() {
		t.Fatal("markdown-операции в read-only вернули true")
	}
	e.SelectAll()
	if e.Cut() != "" {
		t.Fatal("Cut в read-only вернул текст")
	}
	if got := e.Buf.Text(); got != before {
		t.Fatalf("read-only изменился: %q", got)
	}
}

func TestEditorUnicodeCursors(t *testing.T) {
	family := "👨‍👩‍👧‍👦"
	e := testEditor("f.txt", family+"а")
	e.MoveRight(false)
	if e.Cursor != (Pos{0, 7}) {
		t.Fatalf("MoveRight по ZWJ-эмодзи = %v", e.Cursor)
	}
	e.MoveLeft(false)
	if e.Cursor != (Pos{0, 0}) {
		t.Fatalf("MoveLeft по ZWJ-эмодзи = %v", e.Cursor)
	}
	e.MoveRight(false)
	if !e.DeleteBackward() {
		t.Fatal("DeleteBackward = false")
	}
	if got := e.Buf.Text(); got != "а" {
		t.Fatalf("эмодзи разорван: %q", got)
	}

	e2 := testEditor("f.txt", "е\u0301ж")
	e2.MoveRight(false)
	if e2.Cursor != (Pos{0, 2}) {
		t.Fatalf("MoveRight по составной = %v", e2.Cursor)
	}
	e2.MoveLeft(false)
	if e2.Cursor != (Pos{0, 0}) {
		t.Fatalf("MoveLeft по составной = %v", e2.Cursor)
	}
}

func TestSetSelectionEqualIsInactive(t *testing.T) {
	e := testEditor("f.txt", "abc")
	e.SetSelection(Pos{0, 1}, Pos{0, 1})
	if e.HasSelection() || e.Sel.Active {
		t.Fatalf("равное выделение активно: %+v", e.Sel)
	}
	if e.Cursor != (Pos{0, 1}) {
		t.Fatalf("cursor = %v", e.Cursor)
	}
	e.SetSelection(Pos{0, 0}, Pos{0, 2})
	e.ClearSelection()
	if e.HasSelection() || e.Sel.Active {
		t.Fatalf("ClearSelection не сработал: %+v", e.Sel)
	}
	if e.Cursor != (Pos{0, 2}) {
		t.Fatalf("ClearSelection сдвинул курсор: %v", e.Cursor)
	}
}
