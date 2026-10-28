package ed

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenPreservesEOLAndBOM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	raw := []byte("\xEF\xBB\xBFодин\r\nдва\r\n")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !b.BOM || b.EOL != "\r\n" {
		t.Fatalf("BOM=%v EOL=%q", b.BOM, b.EOL)
	}
	if got := b.Text(); got != "один\nдва\n" {
		t.Fatalf("Text() = %q", got)
	}
	if got := string(b.FileBytes()); got != string(raw) {
		t.Fatalf("FileBytes() = %q, want %q", got, raw)
	}
}

func TestInsertMultiline(t *testing.T) {
	b := New("")
	b.Insert(Pos{0, 0}, "привет мир")
	p := b.Insert(Pos{0, 6}, "\nбольшой")
	if p != (Pos{1, 7}) {
		t.Fatalf("pos = %+v", p)
	}
	if got := b.Text(); got != "привет\nбольшой мир" {
		t.Fatalf("Text() = %q", got)
	}
}

func TestBackspaceJoinsLines(t *testing.T) {
	b := New("")
	b.Insert(Pos{0, 0}, "один\nдва")
	p := b.DeleteBackward(Pos{1, 0})
	if p != (Pos{0, 4}) {
		t.Fatalf("pos = %+v", p)
	}
	if got := b.Text(); got != "одиндва" {
		t.Fatalf("Text() = %q", got)
	}
}

func TestDeleteForwardJoinsLines(t *testing.T) {
	b := New("")
	b.Insert(Pos{0, 0}, "один\nдва")
	p := b.DeleteForward(Pos{0, 4})
	if p != (Pos{0, 4}) {
		t.Fatalf("pos = %+v", p)
	}
	if got := b.Text(); got != "одиндва" {
		t.Fatalf("Text() = %q", got)
	}
}

func TestWordMovementCyrillic(t *testing.T) {
	b := New("")
	b.Insert(Pos{0, 0}, "привет, мир!")
	if got := b.WordRight(Pos{0, 0}); got != (Pos{0, 6}) {
		t.Fatalf("WordRight = %+v", got)
	}
	if got := b.WordRight(Pos{0, 6}); got != (Pos{0, 11}) {
		t.Fatalf("WordRight(6) = %+v", got)
	}
	if got := b.WordLeft(Pos{0, 11}); got != (Pos{0, 8}) {
		t.Fatalf("WordLeft = %+v", got)
	}
	if got := b.WordLeft(Pos{0, 8}); got != (Pos{0, 0}) {
		t.Fatalf("WordLeft(8) = %+v", got)
	}
}

func TestWordMovementAcrossLines(t *testing.T) {
	b := New("")
	b.Insert(Pos{0, 0}, "один\nдва")
	if got := b.WordRight(Pos{0, 4}); got != (Pos{1, 3}) {
		t.Fatalf("WordRight = %+v", got)
	}
	if got := b.WordLeft(Pos{1, 3}); got != (Pos{1, 0}) {
		t.Fatalf("WordLeft = %+v", got)
	}
}

func TestIndent(t *testing.T) {
	b := New("")
	b.Insert(Pos{0, 0}, "    - [ ] пункт")
	if got := b.Indent(Pos{0, 10}); got != "    " {
		t.Fatalf("Indent = %q", got)
	}
}

func TestSaveMarksClean(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("старое\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if b.Dirty() {
		t.Fatal("свежий буфер грязный")
	}
	b.Insert(Pos{0, 6}, "!")
	if !b.Dirty() {
		t.Fatal("после правки буфер не грязный")
	}
	if err := b.Save(); err != nil {
		t.Fatal(err)
	}
	if b.Dirty() {
		t.Fatal("после сохранения буфер грязный")
	}
	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Text(); got != "старое!\n" {
		t.Fatalf("на диске %q", got)
	}
}
