package ed

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// clockEditor возвращает редактор с управляемыми часами.
func clockEditor(t *testing.T, text string) (*Editor, *time.Time) {
	t.Helper()
	now := time.Unix(1_700_000_000, 0)
	e := testEditor("f.txt", text)
	e.now = func() time.Time { return now }
	e.groupWindow = 300 * time.Millisecond
	return e, &now
}

func TestUndoGroupsByTime(t *testing.T) {
	e, now := clockEditor(t, "")
	e.Insert("а")
	*now = now.Add(100 * time.Millisecond)
	e.Insert("б")
	*now = now.Add(200 * time.Millisecond)
	e.Insert("в")
	if got := e.Buf.Text(); got != "абв" {
		t.Fatalf("Text = %q", got)
	}
	if !e.Undo() {
		t.Fatal("Undo = false")
	}
	if got := e.Buf.Text(); got != "" {
		t.Fatalf("группа 300 мс не откатилась целиком: %q", got)
	}
	if e.CanUndo() {
		t.Fatal("после отката группы остались ещё шаги")
	}
	if !e.Redo() || e.Buf.Text() != "абв" || e.Cursor != (Pos{0, 3}) {
		t.Fatalf("Redo = %q %v", e.Buf.Text(), e.Cursor)
	}
}

func TestUndoBreaksGroupAfterWindow(t *testing.T) {
	e, now := clockEditor(t, "")
	e.Insert("а")
	*now = now.Add(301 * time.Millisecond)
	e.Insert("б")
	if e.Undo(); e.Buf.Text() != "а" {
		t.Fatalf("откат после окна = %q", e.Buf.Text())
	}
	if e.Undo(); e.Buf.Text() != "" {
		t.Fatalf("второй откат = %q", e.Buf.Text())
	}
}

func TestUndoPasteStartsNewEntry(t *testing.T) {
	e, now := clockEditor(t, "")
	e.Insert("а")
	*now = now.Add(10 * time.Millisecond)
	e.Paste("бв")
	if e.Undo(); e.Buf.Text() != "а" {
		t.Fatalf("undo paste = %q", e.Buf.Text())
	}
	if e.Undo(); e.Buf.Text() != "" {
		t.Fatalf("undo insert = %q", e.Buf.Text())
	}
}

func TestMultiRuneInsertStartsNewEntry(t *testing.T) {
	e, now := clockEditor(t, "")
	e.Insert("а")
	*now = now.Add(10 * time.Millisecond)
	e.Insert("бв")
	if e.Undo(); e.Buf.Text() != "а" {
		t.Fatalf("undo word = %q", e.Buf.Text())
	}
	if e.Undo(); e.Buf.Text() != "" {
		t.Fatalf("undo char = %q", e.Buf.Text())
	}
}

func TestRedoClearedByNewEdit(t *testing.T) {
	e, now := clockEditor(t, "")
	e.Insert("а")
	*now = now.Add(time.Second)
	e.Insert("б")
	e.Undo()
	if !e.CanRedo() {
		t.Fatal("redo пуст после undo")
	}
	e.Insert("в")
	if e.CanRedo() || e.Redo() {
		t.Fatal("новая правка не очистила redo")
	}
	if got := e.Buf.Text(); got != "ав" {
		t.Fatalf("Text = %q", got)
	}
}

func TestExplicitAndNestedGroups(t *testing.T) {
	e, _ := clockEditor(t, "")
	e.BeginGroup()
	e.Insert("1")
	e.Insert("2")
	e.BeginGroup()
	e.Insert("3")
	e.EndGroup()
	e.Insert("4")
	e.EndGroup()
	if got := e.Buf.Text(); got != "1234" {
		t.Fatalf("Text = %q", got)
	}
	if !e.Undo() || e.Buf.Text() != "" {
		t.Fatalf("группа не одна транзакция: %q", e.Buf.Text())
	}
	if e.CanUndo() {
		t.Fatal("вложенная группа создала лишний шаг")
	}
	if !e.Redo() || e.Buf.Text() != "1234" {
		t.Fatalf("redo группы = %q", e.Buf.Text())
	}
}

func TestEmptyGroupKeepsRedo(t *testing.T) {
	e, _ := clockEditor(t, "")
	e.Insert("а")
	e.Undo()
	e.BeginGroup()
	e.EndGroup()
	if !e.CanRedo() {
		t.Fatal("пустая группа сбросила redo")
	}
}

func TestUndoRestoresCursorAndSelection(t *testing.T) {
	e, now := clockEditor(t, "один два три")
	e.SetSelection(Pos{0, 5}, Pos{0, 8})
	e.Insert("дважды")
	*now = now.Add(time.Second)
	e.SetCursor(Pos{0, 0})
	e.Insert("> ")
	if !e.Undo() {
		t.Fatal("Undo = false")
	}
	if got := e.Buf.Text(); got != "один дважды три" {
		t.Fatalf("после первого undo %q", got)
	}
	if e.Cursor != (Pos{0, 0}) {
		t.Fatalf("курсор до правки не восстановлен: %v", e.Cursor)
	}
	if !e.Undo() {
		t.Fatal("Undo 2 = false")
	}
	if e.Buf.Text() != "один два три" {
		t.Fatalf("после второго undo %q", e.Buf.Text())
	}
	if !e.HasSelection() || e.Sel.Anchor != (Pos{0, 5}) || e.Cursor != (Pos{0, 8}) {
		t.Fatalf("выделение: %+v cursor=%v", e.Sel, e.Cursor)
	}
}

func TestReplaceAllOneTransaction(t *testing.T) {
	e := testEditor("f.md", "micro и micro, micro")
	matches := e.FindAll("micro", false)
	if len(matches) != 3 {
		t.Fatalf("matches = %d", len(matches))
	}
	e.BeginGroup()
	for i := len(matches) - 1; i >= 0; i-- {
		e.ReplaceRange(matches[i], "editor")
	}
	e.EndGroup()
	if got := e.Buf.Text(); got != "editor и editor, editor" {
		t.Fatalf("Text = %q", got)
	}
	if !e.Undo() || e.Buf.Text() != "micro и micro, micro" {
		t.Fatalf("replace-all не одной транзакцией: %q", e.Buf.Text())
	}
	if e.CanUndo() {
		t.Fatal("replace-all создал лишние шаги")
	}
}

func TestUndoAcrossLines(t *testing.T) {
	e, now := clockEditor(t, "аб\nвг")
	e.SetCursor(Pos{0, 1})
	e.Insert("X")
	*now = now.Add(time.Second)
	e.SetCursor(Pos{0, 3})
	e.DeleteForward() // склеивает первую и вторую строки
	if got := e.Buf.Text(); got != "аXбвг" {
		t.Fatalf("после правок %q", got)
	}
	if !e.Undo() {
		t.Fatal("Undo = false")
	}
	if got := e.Buf.Text(); got != "аXб\nвг" {
		t.Fatalf("после undo %q", got)
	}
	if !e.Undo() {
		t.Fatal("Undo 2 = false")
	}
	if got := e.Buf.Text(); got != "аб\nвг" {
		t.Fatalf("после undo 2 %q", got)
	}
}

func TestUndoDirtyTracksSavePoint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	e := NewEditor(b)
	e.MoveDocEnd(false)
	e.Insert("!")
	e.Insert("?")
	if err := b.Save(); err != nil {
		t.Fatal(err)
	}
	if !e.Undo() {
		t.Fatal("Undo = false")
	}
	if !b.Dirty() {
		t.Fatal("undo ниже savepoint должен оставить dirty")
	}
	if !e.Redo() {
		t.Fatal("Redo = false")
	}
	if b.Dirty() {
		t.Fatalf("redo к сохранённому тексту оставил dirty: %q", b.Text())
	}
	if !e.Undo() || !b.Dirty() {
		t.Fatalf("повторный undo: dirty=%v text=%q", b.Dirty(), b.Text())
	}
}

func TestUndoGroupBreaksOnCursorMove(t *testing.T) {
	e, now := clockEditor(t, "one\ntwo")
	e.SetCursor(Pos{0, 3})
	e.Insert("X")
	*now = now.Add(50 * time.Millisecond)
	e.SetCursor(Pos{1, 3})
	e.Insert("Y")

	if !e.Undo() || e.Buf.Text() != "oneX\ntwo" {
		t.Fatalf("undo после прыжка курсора откатил лишнее: %q", e.Buf.Text())
	}
	if !e.Undo() || e.Buf.Text() != "one\ntwo" {
		t.Fatalf("второй undo: %q", e.Buf.Text())
	}
}

func TestUndoGroupBreaksOnSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	e, now := clockEditor(t, "")
	if err := e.Buf.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	e.Insert("a")
	if err := e.Buf.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(50 * time.Millisecond)
	e.Insert("b")
	if !e.Undo() || e.Buf.Text() != "a" {
		t.Fatalf("undo после save должен оставить сохранённый текст: %q", e.Buf.Text())
	}
	if e.Buf.Dirty() {
		t.Fatal("undo к сохранённому тексту пометил буфер грязным")
	}
	if !e.Undo() || e.Buf.Text() != "" || !e.Buf.Dirty() {
		t.Fatalf("undo ниже savepoint: %q dirty=%v", e.Buf.Text(), e.Buf.Dirty())
	}
}

func TestUndoDoesNotMergeTypingWithBackspace(t *testing.T) {
	e, now := clockEditor(t, "")
	e.Insert("a")
	*now = now.Add(50 * time.Millisecond)
	e.DeleteBackward()
	if got := e.Buf.Text(); got != "" {
		t.Fatalf("Text = %q", got)
	}
	if !e.Undo() || e.Buf.Text() != "a" {
		t.Fatalf("первый undo должен вернуть набранное: %q", e.Buf.Text())
	}
	if !e.Undo() || e.Buf.Text() != "" {
		t.Fatalf("второй undo: %q", e.Buf.Text())
	}
}
