package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/ed"
	"github.com/bongdo/pero/internal/lsp"
)

func TestClickPlacesCursorExactly(t *testing.T) {
	m := testModel(t)
	typeText(m, "hello")
	setSize(m, 80, 24)
	m.View()
	lay := m.layout()
	x := lay.gutterW + 2
	y := lay.text.y
	m.mouseClick(tea.Mouse{X: x, Y: y, Button: tea.MouseLeft})
	if m.tab().ed.Cursor != (ed.Pos{Line: 0, Col: 2}) {
		t.Fatalf("курсор %+v", m.tab().ed.Cursor)
	}
	// Клик после последней руны — конец строки.
	x = lay.gutterW + 5
	m.mouseClick(tea.Mouse{X: x, Y: y, Button: tea.MouseLeft})
	if m.tab().ed.Cursor.Col != 5 {
		t.Fatalf("конец строки: %+v", m.tab().ed.Cursor)
	}
}

func TestDoubleClickSelectsWord(t *testing.T) {
	m := testModel(t)
	typeText(m, "привет мир")
	setSize(m, 80, 24)
	m.View()
	lay := m.layout()
	x := lay.gutterW + 2
	y := lay.text.y
	m.mouseClick(tea.Mouse{X: x, Y: y, Button: tea.MouseLeft})
	m.mouseClick(tea.Mouse{X: x, Y: y, Button: tea.MouseLeft})
	if got := m.tab().ed.SelectedText(); got != "привет" {
		t.Fatalf("выделено %q", got)
	}
}

func TestTripleClickSelectsLine(t *testing.T) {
	m := testModel(t)
	typeText(m, "abc\ndef")
	setSize(m, 80, 24)
	m.View()
	lay := m.layout()
	x := lay.gutterW + 1
	y := lay.text.y
	for i := 0; i < 3; i++ {
		m.mouseClick(tea.Mouse{X: x, Y: y, Button: tea.MouseLeft})
	}
	if got := m.tab().ed.SelectedText(); got != "abc\n" {
		t.Fatalf("выделено %q", got)
	}
}

func TestDragSelection(t *testing.T) {
	m := testModel(t)
	typeText(m, "abcdef")
	setSize(m, 80, 24)
	m.View()
	lay := m.layout()
	y := lay.text.y
	m.mouseClick(tea.Mouse{X: lay.gutterW + 1, Y: y, Button: tea.MouseLeft})
	m.mouseMotion(tea.Mouse{X: lay.gutterW + 4, Y: y, Button: tea.MouseLeft})
	if got := m.tab().ed.SelectedText(); got != "bcd" {
		t.Fatalf("выделено %q", got)
	}
	m.mouseRelease(tea.Mouse{Button: tea.MouseLeft})
	if m.drag.active {
		t.Fatal("drag не завершён")
	}
}

func TestWheelScrollsText(t *testing.T) {
	m := testModel(t)
	for i := 0; i < 100; i++ {
		typeText(m, "x")
		m.Update(keyMsg(tea.KeyEnter, "", 0))
	}
	setSize(m, 80, 24)
	m.View()
	m.tab().top = 0
	lay := m.layout()
	m.mouseWheel(tea.Mouse{X: lay.gutterW + 1, Y: lay.text.y + 1, Button: tea.MouseWheelDown})
	if m.tab().top != 3 {
		t.Fatalf("top=%d", m.tab().top)
	}
	m.View()
	if m.tab().top != 3 {
		t.Fatalf("View притянул курсор обратно: top=%d", m.tab().top)
	}
	// Любая клавиша возвращает курсор в видимую область.
	m.Update(keyMsg('x', "x", 0))
	m.View()
	rows := m.tab().screenRows(m.layout().text.w, m.cfg.Editor.Tabsize)
	cur := rowForPos(rows, m.tab().wrapCache.lineStart, m.tab().ed.Cursor)
	if cur < m.tab().top || cur >= m.tab().top+m.layout().text.h {
		t.Fatalf("курсор вне окна: cur=%d top=%d", cur, m.tab().top)
	}
}

func TestRightClickOpensMenu(t *testing.T) {
	m := testModel(t)
	typeText(m, "hello")
	m.tab().ed.SetSelection(ed.Pos{Line: 0, Col: 0}, ed.Pos{Line: 0, Col: 5})
	setSize(m, 80, 24)
	m.View()
	lay := m.layout()
	m.mouseClick(tea.Mouse{X: lay.gutterW + 2, Y: lay.text.y, Button: tea.MouseRight})
	if m.ov.kind != overlayMenu || !m.ov.menu.open {
		t.Fatal("меню не открылось")
	}
	if !m.tab().ed.HasSelection() {
		t.Fatal("ПКМ внутри выделения снял его")
	}
	pressEsc(m)
	if m.ov.kind != overlayNone {
		t.Fatal("меню не закрылось")
	}
}

func TestRightClickTabMenu(t *testing.T) {
	m := testModel(t)
	m.newEmptyTab()
	setSize(m, 80, 24)
	m.View()
	st := m.tabBarState()
	hit := st.hits[0]
	m.mouseClick(tea.Mouse{X: hit.x + 1, Y: 0, Button: tea.MouseRight})
	if m.ov.kind != overlayMenu {
		t.Fatal("меню вкладки не открылось")
	}
	m.mouseWheel(tea.Mouse{Button: tea.MouseWheelDown})
	if m.ov.kind != overlayNone {
		t.Fatal("скролл не закрыл меню")
	}
}

func TestStatusErrorClick(t *testing.T) {
	m := testModel(t)
	typeText(m, "abc")
	m.tab().diags = []lsp.Diagnostic{{
		Range:    lsp.Range{Start: lsp.Position{Line: 0, Character: 0}, End: lsp.Position{Line: 0, Character: 3}},
		Severity: 1,
		Message:  "ошибка",
	}}
	m.tab().diagVersion = m.tab().ed.Buf.Version
	m.tab().ed.SetCursor(ed.Pos{Line: 0, Col: 3})
	setSize(m, 80, 24)
	m.View()
	_, r := m.statusLayout()
	if r.w == 0 {
		t.Fatal("нет зоны счётчика")
	}
	m.mouseClick(tea.Mouse{X: r.x, Y: r.y, Button: tea.MouseLeft})
	if m.tab().ed.Cursor.Col != 0 {
		t.Fatalf("клик по счётчику: %+v", m.tab().ed.Cursor)
	}
}
