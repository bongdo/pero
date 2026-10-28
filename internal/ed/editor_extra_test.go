package ed

import "testing"

func TestUndoReplacingSelectionThenTyping(t *testing.T) {
	b := New("")
	e := NewEditor(b)
	e.Insert("старое")
	e.SelectAll()
	e.Insert("X")
	e.Insert("Y")
	e.Insert("Z")
	if got := b.Text(); got != "XYZ" {
		t.Fatalf("text = %q", got)
	}
	if !e.Undo() {
		t.Fatal("undo не сработал")
	}
	if got := b.Text(); got != "старое" {
		t.Fatalf("после одного undo = %q, want старое", got)
	}
}
