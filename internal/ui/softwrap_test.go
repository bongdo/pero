package ui

import (
	"strings"
	"testing"

	"github.com/bongdo/pero/internal/ed"
)

func wrapTab(t *testing.T, content string, width int) (*Tab, []screenRow) {
	t.Helper()
	m := testModel(t)
	tb := m.tab()
	tb.ed = ed.NewEditor(ed.OpenBytes([]byte(content), "test.md"))
	tb.ed.TabWidth = 4
	tb.wrap = true
	rows := tb.screenRows(width, 4)
	return tb, rows
}

func TestWrapCoversLineAndFitsWidth(t *testing.T) {
	tb, rows := wrapTab(t, "    - пункт списка который точно не влезет в узкий терминал и будет перенесён\nкороткая", 20)
	if len(rows) < 2 {
		t.Fatalf("ожидался перенос, строк: %d", len(rows))
	}
	for _, r := range rows {
		line := tb.ed.Buf.Line(r.logical)
		cols := displayCols(line, 4)
		used := r.indent + cols[r.runeEnd] - cols[r.runeStart]
		if used > 20 {
			t.Fatalf("экранная строка шире 20: %d (%+v)", used, r)
		}
		if r.runeStart > r.runeEnd || r.runeEnd > len(line) {
			t.Fatalf("плохой диапазон: %+v (len=%d)", r, len(line))
		}
	}
	var got []string
	for _, r := range rows {
		if r.logical == 0 {
			line := tb.ed.Buf.Line(0)
			got = append(got, string(line[r.runeStart:r.runeEnd]))
		}
	}
	joined := strings.Join(got, "")
	if joined != string(tb.ed.Buf.Line(0)) {
		t.Fatalf("перенос потерял текст: %q != %q", joined, string(tb.ed.Buf.Line(0)))
	}
	if rows[1].indent == 0 {
		t.Fatalf("продолжение потеряло отступ: %+v", rows[1])
	}
}

func TestWrapLongWordByRunes(t *testing.T) {
	tb, rows := wrapTab(t, strings.Repeat("я", 55), 10)
	if len(rows) < 5 {
		t.Fatalf("длинное слово не порезано: %d", len(rows))
	}
	for _, r := range rows {
		if r.runeEnd-r.runeStart > 10 {
			t.Fatalf("кусок слова шире экрана: %+v", r)
		}
	}
	_ = tb
}

func TestWrapCursorScreenRoundTrip(t *testing.T) {
	tb, rows := wrapTab(t, "первая строка с довольно длинным текстом для переноса\nвторая", 16)
	line := tb.ed.Buf.Line(0)
	for col := 0; col <= len(line); col++ {
		p := ed.Pos{Line: 0, Col: col}
		row := rowForPos(rows, tb.wrapCache.lineStart, p)
		x := rowScreenCol(tb, rows, row, col, 4)
		back := posAtRow(tb, rows, row, x, 0, 4, true)
		if back != p {
			t.Fatalf("col %d: row %d x %d -> %+v", col, row, x, back)
		}
	}
	if len(rows) < 2 {
		t.Fatal("нет переноса")
	}
	b := tb.ed.Buf.Line(1)
	for col := 0; col <= len(b); col++ {
		p := ed.Pos{Line: 1, Col: col}
		row := rowForPos(rows, tb.wrapCache.lineStart, p)
		x := rowScreenCol(tb, rows, row, col, 4)
		if back := posAtRow(tb, rows, row, x, 0, 4, true); back != p {
			t.Fatalf("line1 col %d -> %+v", col, back)
		}
	}
}

func TestWrapClickPastRowEndGoesToRuneEnd(t *testing.T) {
	tb, rows := wrapTab(t, "abc def ghi jkl", 10)
	if len(rows) < 2 {
		t.Fatal("нет переноса")
	}
	r := rows[0]
	cols := displayCols(tb.ed.Buf.Line(0), 4)
	used := r.indent + cols[r.runeEnd] - cols[r.runeStart]
	if used >= 10 {
		t.Fatalf("первая строка заняла всю ширину: %+v", r)
	}
	p := posAtRow(tb, rows, 0, used, 0, 4, true)
	if p.Col != r.runeEnd {
		t.Fatalf("клик за концом строки: %+v, runeEnd=%d", p, r.runeEnd)
	}
}

func TestPosAfterLastRune(t *testing.T) {
	tb, rows := wrapTab(t, "abc", 20)
	p := ed.Pos{Line: 0, Col: 3}
	if r := rowForPos(rows, tb.wrapCache.lineStart, p); r != 0 {
		t.Fatalf("строка курсора %d", r)
	}
	if x := rowScreenCol(tb, rows, 0, 3, 4); x != 3 {
		t.Fatalf("x=%d", x)
	}
}

func TestEmojiClusters(t *testing.T) {
	const family = "👨‍👩‍👧‍👦"
	line := family + " привет " + family
	tb, rows := wrapTab(t, line, 12)
	// Курсор после кластера и внутри кластера не разрывает эмодзи на экране.
	after := ed.Pos{Line: 0, Col: len([]rune(family))}
	row := rowForPos(rows, tb.wrapCache.lineStart, after)
	x := rowScreenCol(tb, rows, row, after.Col, 4)
	if back := posAtRow(tb, rows, row, x, 0, 4, true); back != after {
		t.Fatalf("после кластера: %+v -> x=%d -> %+v", after, x, back)
	}
	inside := ed.Pos{Line: 0, Col: 2}
	row = rowForPos(rows, tb.wrapCache.lineStart, inside)
	x = rowScreenCol(tb, rows, row, inside.Col, 4)
	if back := posAtRow(tb, rows, row, x, 0, 4, true); back.Col != 0 {
		t.Fatalf("клик внутри кластера вернул %+v", back)
	}
	// Рендер не паникует и не рвёт ширину.
	m := testModel(t)
	m.tab().ed = tb.ed
	m.tab().wrap = true
	m.tab().wrapCache.valid = false
	setSize(m, 12, 10)
	viewLines(t, m)
}
