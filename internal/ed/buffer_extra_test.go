package ed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenBinaryIsReadOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "image.bin")
	raw := []byte{0x89, 'P', 'N', 'G', 0, 0, 0, 0}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !b.ReadOnly {
		t.Fatal("бинарный файл не помечен ReadOnly")
	}
	if b.Insert(Pos{}, "x") != (Pos{}) {
		t.Fatal("Insert в read-only сдвинул позицию")
	}
	if !strings.Contains(b.Text(), "PNG") {
		t.Fatalf("Text() = %q", b.Text())
	}
	if err := b.Save(); err != ErrReadOnly {
		t.Fatalf("Save() = %v, want ErrReadOnly", err)
	}
}

func TestOpenNULAfterSampleIsEditable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "late-nul.txt")
	raw := append([]byte(strings.Repeat("a", binarySample+10)), 0)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if b.ReadOnly {
		t.Fatal("NUL после первых 8 КБ не должен делать файл read-only")
	}
}

func TestOpenLargeFileIsReadOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.txt")
	data := strings.Repeat("a", MaxFileSize+1)
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !b.ReadOnly {
		t.Fatal("файл больше 10 МБ не помечен ReadOnly")
	}
	if len(b.Text()) > MaxFileSize {
		t.Fatalf("прочитано %d байт, лимит %d", len(b.Text()), MaxFileSize)
	}
}

func TestOpenBytesEOLBOM(t *testing.T) {
	b := OpenBytes([]byte("\xEF\xBB\xBFодин\r\nдва\r\n"), "f.md")
	if !b.BOM || b.EOL != "\r\n" {
		t.Fatalf("BOM=%v EOL=%q", b.BOM, b.EOL)
	}
	if got := b.Text(); got != "один\nдва\n" {
		t.Fatalf("Text = %q", got)
	}
	if got := string(b.FileBytes()); got != "\xEF\xBB\xBFодин\r\nдва\r\n" {
		t.Fatalf("FileBytes = %q", got)
	}
	if b.ReadOnly {
		t.Fatal("обычный файл помечен read-only")
	}
}

func TestSaveAsAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	b := New(path)
	b.Insert(Pos{}, "один\nдва")
	if err := b.SaveAs(filepath.Join(dir, "b.txt")); err != nil {
		t.Fatal(err)
	}
	if b.Path != filepath.Join(dir, "b.txt") || b.Dirty() {
		t.Fatalf("Path=%q dirty=%v", b.Path, b.Dirty())
	}
	b.Insert(Pos{0, 0}, "!")
	if err := b.Reload(); err != nil {
		t.Fatal(err)
	}
	if got := b.Text(); got != "один\nдва" {
		t.Fatalf("после Reload %q", got)
	}
	if b.Dirty() {
		t.Fatal("Reload оставил dirty")
	}
}

func TestUTF16RoundTrip(t *testing.T) {
	b := New("")
	b.Insert(Pos{}, "а😀б")
	cases := []struct {
		pos  Pos
		want int
	}{
		{Pos{0, 0}, 0},
		{Pos{0, 1}, 1}, // а
		{Pos{0, 2}, 3}, // эмодзи — 2 единицы
		{Pos{0, 3}, 4}, // б
	}
	for _, c := range cases {
		if got := b.UTF16Offset(c.pos); got != c.want {
			t.Fatalf("UTF16Offset(%v) = %d, want %d", c.pos, got, c.want)
		}
		if got := b.PosFromUTF16(c.pos.Line, c.want); got != c.pos {
			t.Fatalf("PosFromUTF16(%d) = %v, want %v", c.want, got, c.pos)
		}
	}
	// Середина суррогатной пары не разрезает эмодзи.
	if got := b.PosFromUTF16(0, 2); got != (Pos{0, 1}) {
		t.Fatalf("PosFromUTF16(mid-surrogate) = %v", got)
	}
	if got := b.PosFromUTF16(0, 999); got != (Pos{0, 3}) {
		t.Fatalf("PosFromUTF16(past end) = %v", got)
	}

	// Character считается от начала своей строки.
	multi := OpenBytes([]byte("😀\nаб"), "f.txt")
	if got := multi.UTF16Offset(Pos{1, 2}); got != 2 {
		t.Fatalf("UTF16Offset line 1 = %d, want 2", got)
	}
	if got := multi.PosFromUTF16(1, 2); got != (Pos{1, 2}) {
		t.Fatalf("PosFromUTF16 line 1 = %v", got)
	}
}

func TestRuneOffsetRoundTrip(t *testing.T) {
	b := OpenBytes([]byte("аб\nвг\n\nд"), "f.txt")
	for line := 0; line <= b.LastLine(); line++ {
		for col := 0; col <= len(b.Lines[line]); col++ {
			p := Pos{line, col}
			if got := b.PosFromRuneOffset(b.RuneOffset(p)); got != p {
				t.Fatalf("roundtrip %v -> %d -> %v", p, b.RuneOffset(p), got)
			}
		}
	}
	if got := b.RuneOffset(Pos{1, 1}); got != 4 {
		t.Fatalf("RuneOffset = %d, want 4", got)
	}
	if got := b.PosFromRuneOffset(9999); got != (Pos{3, 1}) {
		t.Fatalf("PosFromRuneOffset(past) = %v", got)
	}
}

func TestLineAndRuneCount(t *testing.T) {
	b := OpenBytes([]byte("один\nдва"), "f.txt")
	if got := string(b.Line(1)); got != "два" {
		t.Fatalf("Line(1) = %q", got)
	}
	if b.Line(-1) != nil || b.Line(10) != nil {
		t.Fatal("Line за границей должен быть nil")
	}
	if got := b.RuneCount(Pos{0, 99}); got != 4 {
		t.Fatalf("RuneCount = %d, want 4", got)
	}
	if got := b.Prev(Pos{1, 0}); got != (Pos{0, 4}) {
		t.Fatalf("Prev = %v", got)
	}
}

func TestVersionIncrementsOnEdits(t *testing.T) {
	b := New("")
	start := b.Version
	b.Insert(Pos{}, "аб")
	if b.Version != start+1 {
		t.Fatalf("Version = %d, want %d", b.Version, start+1)
	}
	v := b.Version
	b.DeleteBackward(Pos{0, 2})
	if b.Version != v+1 {
		t.Fatalf("Version = %d, want %d", b.Version, v+1)
	}
	v = b.Version
	b.DeleteBackward(Pos{0, 0}) // нечего удалять
	if b.Version != v {
		t.Fatalf("no-op изменил Version: %d -> %d", v, b.Version)
	}
	b.Insert(Pos{}, "")
	if b.Version != v {
		t.Fatalf("пустая вставка изменила Version: %d -> %d", v, b.Version)
	}
}

func TestGraphemeDeleteAndMove(t *testing.T) {
	family := "👨‍👩‍👧‍👦"
	combining := "е\u0301"
	flag := "🇷🇺"
	b := New("")
	b.Insert(Pos{}, family+" "+combining+" "+flag)

	p := Pos{0, 0}
	p = b.Next(p) // Next остаётся по рунам
	if p != (Pos{0, 1}) {
		t.Fatalf("Next = %v", p)
	}

	// Курсорные шаги и удаление идут кластерами.
	n := len([]rune(family))
	famEnd, ok := b.nextPos(Pos{0, 0})
	if !ok || famEnd != (Pos{0, n}) {
		t.Fatalf("nextPos emoji = %v, %v (want col %d)", famEnd, ok, n)
	}
	if prev, ok := b.prevPos(famEnd); !ok || prev != (Pos{0, 0}) {
		t.Fatalf("prevPos emoji = %v", prev)
	}
	after := b.DeleteBackward(famEnd)
	if after != (Pos{0, 0}) {
		t.Fatalf("DeleteBackward emoji = %v", after)
	}
	if got := b.Text(); got != " "+combining+" "+flag {
		t.Fatalf("после удаления %q", got)
	}

	// Составная буква удаляется целиком.
	b2 := New("")
	b2.Insert(Pos{}, combining)
	if got := b2.DeleteBackward(Pos{0, len([]rune(combining))}); got != (Pos{0, 0}) || b2.Text() != "" {
		t.Fatalf("составная буква: %v %q", got, b2.Text())
	}

	// Флаг — один кластер.
	b3 := New("")
	b3.Insert(Pos{}, flag)
	if got := b3.DeleteBackward(Pos{0, 2}); got != (Pos{0, 0}) || b3.Text() != "" {
		t.Fatalf("флаг: %v %q", got, b3.Text())
	}
}

func TestBufferReadOnlyBlocksMutations(t *testing.T) {
	b := New("f.bin")
	b.Insert(Pos{}, "аб")
	b.dirty = false
	b.ReadOnly = true
	b.Insert(Pos{0, 2}, "x")
	p := b.DeleteBackward(Pos{0, 2})
	q := b.DeleteForward(Pos{0, 0})
	if got := b.Text(); got != "аб" || p != (Pos{0, 2}) || q != (Pos{0, 0}) || b.Dirty() {
		t.Fatalf("read-only изменился: %q %v %v %v", got, p, q, b.Dirty())
	}
}
