package ed

import (
	"fmt"
	"strings"
	"testing"
)

func largeBuffer(lines int) *Editor {
	var sb strings.Builder
	for i := 0; i < lines; i++ {
		fmt.Fprintf(&sb, "line %d has some words and a needle inside\n", i)
	}
	return NewEditor(OpenBytes([]byte(sb.String()), "big.txt"))
}

func BenchmarkEditor100kInsert(b *testing.B) {
	e := largeBuffer(100_000)
	line := e.Buf.LastLine()
	e.SetCursor(Pos{line, 7})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Insert("x")
		e.DeleteBackward()
		e.undo, e.redo = nil, nil // не копим историю между итерациями
	}
}

func BenchmarkEditor100kUndo(b *testing.B) {
	e := largeBuffer(100_000)
	line := e.Buf.LastLine()
	e.SetCursor(Pos{line, 7})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		e.Insert("x")
		b.StartTimer()
		e.Undo()
		e.redo = e.redo[:0]
	}
}

func BenchmarkEditor100kFindAll(b *testing.B) {
	e := largeBuffer(100_000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if got := e.FindAll("needle", false); len(got) != 100_000 {
			b.Fatalf("matches = %d", len(got))
		}
	}
}
