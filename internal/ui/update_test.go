package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/ed"
)

func TestKeyCtrlSSaves(t *testing.T) {
	m, path := testModelFile(t, "key.md", "abc")
	typeText(m, "!")
	pressCtrl(m, 's')
	if m.tab().ed.Buf.Dirty() {
		t.Fatal("Ctrl-S не сохранил")
	}
	if m.toast.Kind != "success" {
		t.Fatalf("тост %+v", m.toast)
	}
	_ = path
}

func TestKeyCtrlEOpensPalette(t *testing.T) {
	m := testModel(t)
	pressCtrl(m, 'e')
	if m.ov.kind != overlayCommands {
		t.Fatalf("оверлей %v", m.ov.kind)
	}
	typeText(m, "сохр")
	items := m.paletteItems()
	if len(items) == 0 || items[0].id != "file.save" {
		t.Fatalf("фильтр: %v", ids(items))
	}
	pressEnter(m)
	if m.ov.kind != overlaySaveAs {
		t.Fatalf("Enter не выполнил команду: %v", m.ov.kind)
	}
	pressEsc(m)
	if m.ov.kind != overlayNone {
		t.Fatal("Esc не закрыл")
	}
}

func TestKeyCtrlShiftSOpensSaveAs(t *testing.T) {
	m := testModel(t)
	m.Update(tea.KeyPressMsg(tea.Key{Code: 's', Mod: tea.ModCtrl | tea.ModShift, Text: "S"}))
	if m.ov.kind != overlaySaveAs {
		t.Fatalf("оверлей %v", m.ov.kind)
	}
	pressEsc(m)
}

func TestCtrlONewTabAndEsc(t *testing.T) {
	m := testModel(t)
	pressCtrl(m, 'o')
	if len(m.tabs) != 2 {
		t.Fatalf("вкладок %d", len(m.tabs))
	}
	if m.ov.kind != overlayOpen {
		t.Fatalf("палитра %v", m.ov.kind)
	}
	pressEsc(m)
	if m.ov.kind != overlayNone {
		t.Fatal("палитра не закрылась")
	}
	if m.tab().ed.Buf.Path != "" || len(m.tabs) != 2 {
		t.Fatal("пустая вкладка не осталась")
	}
	// Alt-O больше не входит в дефолтные биндинги.
	pressAlt(m, 'o')
	if len(m.tabs) != 2 || m.ov.kind != overlayNone {
		t.Fatal("Alt-O всё ещё создаёт вкладку")
	}
}

func TestCtrlTabSwitches(t *testing.T) {
	m := testModel(t)
	m.newEmptyTab()
	if m.active != 1 {
		t.Fatal("подготовка")
	}
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModCtrl}))
	if m.active != 0 {
		t.Fatalf("активная %d", m.active)
	}
	m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModCtrl | tea.ModShift}))
	if m.active != 1 {
		t.Fatalf("активная %d", m.active)
	}
}

func TestCtrlFAndEsc(t *testing.T) {
	m := testModel(t)
	typeText(m, "слово слово")
	pressCtrl(m, 'f')
	if m.ov.kind != overlayFind {
		t.Fatal("поиск не открылся")
	}
	typeText(m, "слово")
	if len(m.ov.findMatches) != 2 {
		t.Fatalf("совпадений %d", len(m.ov.findMatches))
	}
	pressEnter(m)
	if !m.tab().ed.HasSelection() {
		t.Fatal("Enter не выбрал")
	}
	pressEsc(m)
	if m.ov.kind != overlayNone {
		t.Fatal("поиск не закрылся")
	}
	if !m.tab().ed.HasSelection() {
		t.Fatal("закрытие поиска не должно снимать выделение (это следующий Esc)")
	}
	pressEsc(m)
	if m.tab().ed.HasSelection() {
		t.Fatal("второй Esc не снял выделение")
	}
}

func TestEscChainSelectionThenGhost(t *testing.T) {
	m := testModel(t)
	typeText(m, "abc")
	m.tab().ed.SelectAll()
	m.tab().ghost = ghostState{text: "ghost"}
	pressEsc(m)
	if m.tab().ed.HasSelection() {
		t.Fatal("Esc не снял выделение")
	}
	if m.tab().ghost.text == "" {
		t.Fatal("Esc снял ghost раньше выделения")
	}
	pressEsc(m)
	if m.tab().ghost.text != "" {
		t.Fatal("Esc не погасил ghost")
	}
}

func TestAltTogglesCheckbox(t *testing.T) {
	m := testModel(t)
	m.tab().ed = ed.NewEditor(ed.OpenBytes([]byte("- [ ] задача"), "todo.md"))
	m.tab().wrapCache.valid = false
	m.tab().ed.SetCursor(ed.Pos{Line: 0, Col: 7})
	pressAlt(m, 't')
	if got := text(m); got != "- [x] задача" {
		t.Fatalf("чекбокс: %q", got)
	}
	pressAlt(m, 't')
	if got := text(m); got != "- [ ] задача" {
		t.Fatalf("чекбокс обратно: %q", got)
	}
}

func TestAltTNotInCode(t *testing.T) {
	m := testModel(t)
	m.tab().ed = ed.NewEditor(ed.OpenBytes([]byte("- [ ] x"), "main.go"))
	m.tab().ed.SetCursor(ed.Pos{Line: 0, Col: 2})
	pressAlt(m, 't')
	if got := text(m); got != "- [ ] x" {
		t.Fatalf("в коде изменилось: %q", got)
	}
}

func TestEnterContinuesCheckboxList(t *testing.T) {
	m := testModel(t)
	m.tab().ed = ed.NewEditor(ed.OpenBytes([]byte("- [ ] задача"), "todo.md"))
	m.tab().ed.SetCursor(ed.Pos{Line: 0, Col: len([]rune("- [ ] задача"))})
	m.tab().wrapCache.valid = false
	pressEnter(m)
	if got := text(m); got != "- [ ] задача\n- [ ] " {
		t.Fatalf("список: %q", got)
	}
}

func TestEnterEmptyListItemClearsMarker(t *testing.T) {
	m := testModel(t)
	m.tab().ed = ed.NewEditor(ed.OpenBytes([]byte("- "), "todo.md"))
	m.tab().ed.SetCursor(ed.Pos{Line: 0, Col: 2})
	m.tab().wrapCache.valid = false
	pressEnter(m)
	if got := text(m); strings.TrimSpace(got) != "" {
		t.Fatalf("маркер не убран: %q", got)
	}
}

func TestTabIndentsWithSpaces(t *testing.T) {
	m := testModel(t)
	typeText(m, "x")
	m.tab().ed.SetCursor(ed.Pos{})
	m.Update(keyMsg('\t', "", 0))
	if got := text(m); got != "    x" {
		t.Fatalf("таб: %q", got)
	}
}

func TestCtrlDDuplicatesLine(t *testing.T) {
	m := testModel(t)
	typeText(m, "abc")
	m.tab().ed.SetCursor(ed.Pos{Line: 0, Col: 1})
	pressCtrl(m, 'd')
	if got := text(m); got != "abc\nabc" {
		t.Fatalf("дубль: %q", got)
	}
	if m.tab().ed.Cursor.Line != 1 || m.tab().ed.Cursor.Col != 1 {
		t.Fatalf("курсор %+v", m.tab().ed.Cursor)
	}
}

func TestCtrlKCutsLine(t *testing.T) {
	m := testModel(t)
	m.tab().ed = ed.NewEditor(ed.OpenBytes([]byte("abc\ndef"), ""))
	m.tab().ed.SetCursor(ed.Pos{})
	m.tab().wrapCache.valid = false
	pressCtrl(m, 'k')
	if got := text(m); got != "def" {
		t.Fatalf("Ctrl-K: %q", got)
	}
	if m.tab().ed.Cursor != (ed.Pos{}) {
		t.Fatalf("курсор %+v", m.tab().ed.Cursor)
	}
}

func TestCtrlZUndo(t *testing.T) {
	m := testModel(t)
	typeText(m, "abc")
	pressCtrl(m, 'z')
	if got := text(m); got != "" {
		t.Fatalf("undo: %q", got)
	}
	pressCtrl(m, 'y')
	if got := text(m); got != "abc" {
		t.Fatalf("redo: %q", got)
	}
}
