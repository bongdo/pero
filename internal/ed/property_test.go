package ed

import (
	"math/rand"
	"testing"
	"time"
)

func TestPropertyUndoRedoRoundTrip(t *testing.T) {
	tokens := []string{"a", "б", "🙂", "x y", "\n", "e\u0301", "- [ ] "}
	for seed := int64(0); seed < 25; seed++ {
		rng := rand.New(rand.NewSource(seed))
		e, now := clockEditor(t, "начало\nвторая строка\n")
		initial := e.Buf.Text()

		var firstCursor Pos
		for k := 0; k < 30; k++ {
			*now = now.Add(400 * time.Millisecond)
			line := rng.Intn(e.Buf.LastLine() + 1)
			col := rng.Intn(len(e.Buf.Lines[line]) + 1)
			e.SetCursor(Pos{line, col})
			if k == 0 {
				firstCursor = e.Cursor
				e.Insert(tokens[rng.Intn(len(tokens))])
				continue
			}
			switch rng.Intn(4) {
			case 0:
				e.Insert(tokens[rng.Intn(len(tokens))])
			case 1:
				e.DeleteBackward()
			case 2:
				e.DeleteForward()
			case 3:
				e.Paste(tokens[rng.Intn(len(tokens))])
			}
		}
		final := e.Buf.Text()

		for e.CanUndo() {
			if !e.Undo() {
				t.Fatalf("seed %d: Undo = false при CanUndo", seed)
			}
		}
		if got := e.Buf.Text(); got != initial {
			t.Fatalf("seed %d: после всех undo %q != %q", seed, got, initial)
		}
		if e.Cursor != firstCursor {
			t.Fatalf("seed %d: курсор после всех undo %v != %v", seed, e.Cursor, firstCursor)
		}
		if e.CanUndo() {
			t.Fatalf("seed %d: undo не опустошён", seed)
		}

		for e.CanRedo() {
			if !e.Redo() {
				t.Fatalf("seed %d: Redo = false при CanRedo", seed)
			}
		}
		if got := e.Buf.Text(); got != final {
			t.Fatalf("seed %d: после всех redo %q != %q", seed, got, final)
		}
	}
}

func TestPropertyMovementKeepsClusterBoundary(t *testing.T) {
	e := testEditor("f.txt", "е\u0301ж 👨‍👩‍👧‍👦 🇷🇺 foo-bar\nвторой\u0301 ряд\n\n😀😀")
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 500; i++ {
		switch rng.Intn(11) {
		case 0:
			e.MoveLeft(rng.Intn(2) == 0)
		case 1:
			e.MoveRight(rng.Intn(2) == 0)
		case 2:
			e.MoveWordLeft(rng.Intn(2) == 0)
		case 3:
			e.MoveWordRight(rng.Intn(2) == 0)
		case 4:
			e.MoveUp(rng.Intn(2) == 0)
		case 5:
			e.MoveDown(rng.Intn(2) == 0)
		case 6:
			e.MoveHome(rng.Intn(2) == 0)
		case 7:
			e.MoveEnd(rng.Intn(2) == 0)
		case 8:
			e.MovePage(rng.Intn(5)-2, rng.Intn(2) == 0)
		case 9:
			e.MoveDocStart(rng.Intn(2) == 0)
		case 10:
			e.MoveDocEnd(rng.Intn(2) == 0)
		}
		line := e.Buf.Lines[e.Cursor.Line]
		if e.Cursor.Col > 0 && e.Cursor.Col < len(line) && !graphemeBreak(line, e.Cursor.Col) {
			t.Fatalf("шаг %d: курсор %v внутри кластера %q", i, e.Cursor, string(line))
		}
	}
	_ = e.SelectedText()
}

func TestUndoRedoInterleaved(t *testing.T) {
	e, now := clockEditor(t, "")
	e.Insert("a")
	*now = now.Add(time.Second)
	e.Insert("b")
	steps := []struct {
		undo bool
		want string
	}{
		{true, "a"},
		{false, "ab"},
		{true, "a"},
		{true, ""},
		{false, "a"},
		{false, "ab"},
	}
	for i, s := range steps {
		if s.undo {
			if !e.Undo() {
				t.Fatalf("шаг %d: Undo = false", i)
			}
		} else if !e.Redo() {
			t.Fatalf("шаг %d: Redo = false", i)
		}
		if got := e.Buf.Text(); got != s.want {
			t.Fatalf("шаг %d: Text = %q, want %q", i, got, s.want)
		}
	}
}

func TestUndoSelectionAcrossEdits(t *testing.T) {
	e, now := clockEditor(t, "abcdef")
	e.SetSelection(Pos{0, 1}, Pos{0, 4})
	e.DeleteForward() // удаляет выделение
	*now = now.Add(time.Second)
	e.Insert("Z")
	if !e.Undo() {
		t.Fatal("Undo = false")
	}
	if got := e.Buf.Text(); got != "aef" {
		t.Fatalf("после undo вставки %q", got)
	}
	if e.Cursor != (Pos{0, 1}) {
		t.Fatalf("cursor = %v", e.Cursor)
	}
	if !e.Undo() {
		t.Fatal("Undo 2 = false")
	}
	if got := e.Buf.Text(); got != "abcdef" {
		t.Fatalf("после undo удаления %q", got)
	}
	if !e.HasSelection() || e.SelectedText() != "bcd" {
		t.Fatalf("выделение = %+v %q", e.Sel, e.SelectedText())
	}
}
