package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/ed"
)

func TestAutoPairsInsert(t *testing.T) {
	for _, tc := range []struct{ open, want string }{
		{"(", "()"}, {"[", "[]"}, {"{", "{}"}, {"\"", "\"\""}, {"'", "''"}, {"`", "``"},
	} {
		m := testModel(t)
		typeText(m, tc.open)
		if got := text(m); got != tc.want {
			t.Fatalf("%q: %q", tc.open, got)
		}
		if m.tab().ed.Cursor != (ed.Pos{Line: 0, Col: 1}) {
			t.Fatalf("%q: курсор %+v", tc.open, m.tab().ed.Cursor)
		}
	}
}

func TestAutoPairSkipOver(t *testing.T) {
	for _, tc := range []struct{ open, close string }{
		{"(", ")"}, {"[", "]"}, {"{", "}"}, {"\"", "\""}, {"'", "'"}, {"`", "`"},
	} {
		m := testModel(t)
		typeText(m, tc.open+tc.close)
		if got := text(m); got != tc.open+tc.close {
			t.Fatalf("%q+%q: %q", tc.open, tc.close, got)
		}
		if m.tab().ed.Cursor != (ed.Pos{Line: 0, Col: 2}) {
			t.Fatalf("%q+%q: курсор %+v", tc.open, tc.close, m.tab().ed.Cursor)
		}
	}
}

func TestAutoPairWrapSelection(t *testing.T) {
	for _, tc := range []struct{ open, close string }{
		{"(", ")"}, {"[", "]"}, {"{", "}"}, {"\"", "\""}, {"'", "'"}, {"`", "`"},
	} {
		m := testModel(t)
		typeText(m, "abc")
		m.tab().ed.SelectAll()
		typeText(m, tc.open)
		if got := text(m); got != tc.open+"abc"+tc.close {
			t.Fatalf("%q wrap: %q", tc.open, got)
		}
		if m.tab().ed.Cursor != (ed.Pos{Line: 0, Col: 5}) {
			t.Fatalf("%q wrap: курсор %+v", tc.open, m.tab().ed.Cursor)
		}
		if m.tab().ed.HasSelection() {
			t.Fatalf("%q wrap: выделение осталось", tc.open)
		}
	}
}

func TestAutoPairApostrophe(t *testing.T) {
	m := testModel(t)
	typeText(m, "don't")
	if got := text(m); got != "don't" {
		t.Fatalf("апостроф в слове: %q", got)
	}

	// Кавычка после буквы и перед буквой - обычный апостроф.
	m = testModel(t)
	m.tab().ed.Insert("abc")
	m.tab().ed.SetCursor(ed.Pos{})
	typeText(m, "'")
	if got := text(m); got != "'abc" {
		t.Fatalf("апостроф перед буквой: %q", got)
	}

	// Двойная кавычка парится всегда, даже после буквы.
	m = testModel(t)
	typeText(m, "a\"")
	if got := text(m); got != "a\"\"" {
		t.Fatalf("двойная после буквы: %q", got)
	}

	// Кавычка после пробела - начало пары.
	m = testModel(t)
	typeText(m, "a '")
	if got := text(m); got != "a ''" {
		t.Fatalf("кавычка после пробела: %q", got)
	}
}

func TestAutoPairDisabled(t *testing.T) {
	m := testModel(t)
	m.cfg.Editor.AutoPairs = false
	typeText(m, "(")
	if got := text(m); got != "(" {
		t.Fatalf("autopairs off: %q", got)
	}
}

func TestAutoPairReadOnly(t *testing.T) {
	m, _ := testModelFile(t, "x.txt", "abc")
	m.tab().ed.Buf.ReadOnly = true
	typeText(m, "(")
	if got := text(m); got != "abc" {
		t.Fatalf("read-only изменён: %q", got)
	}
	if m.toast.Kind != "error" || !strings.Contains(m.toast.Text, "только чтение") {
		t.Fatalf("нет RU-тоста: %+v", m.toast)
	}
	m.cfg.Language = "en"
	typeText(m, "[")
	if !strings.Contains(m.toast.Text, "read-only") {
		t.Fatalf("нет EN-тоста: %+v", m.toast)
	}
}

func TestSmartBackspacePairs(t *testing.T) {
	for _, open := range []string{"(", "[", "{", "\"", "'", "`"} {
		m := testModel(t)
		typeText(m, open)
		m.Update(keyMsg(tea.KeyBackspace, "", 0))
		if got := text(m); got != "" {
			t.Fatalf("%q: пустая пара не удалена: %q", open, got)
		}
	}
}

func TestSmartBackspaceNonEmptyPair(t *testing.T) {
	m := testModel(t)
	typeText(m, "(x")
	m.Update(keyMsg(tea.KeyBackspace, "", 0))
	if got := text(m); got != "()" {
		t.Fatalf("удалён не символ: %q", got)
	}
	m.Update(keyMsg(tea.KeyBackspace, "", 0))
	if got := text(m); got != "" {
		t.Fatalf("пустая пара не удалена: %q", got)
	}
}

func TestSmartBackspaceDisabled(t *testing.T) {
	m := testModel(t)
	m.cfg.Editor.AutoPairs = false
	m.tab().ed.Insert("()")
	m.tab().ed.SetCursor(ed.Pos{Line: 0, Col: 1})
	m.Update(keyMsg(tea.KeyBackspace, "", 0))
	if got := text(m); got != ")" {
		t.Fatalf("autopairs off: %q", got)
	}
}

func TestSmartEnterBreaksPair(t *testing.T) {
	for _, tc := range []struct{ open, want string }{
		{"{", "{\n    \n}"}, {"(", "(\n    \n)"}, {"[", "[\n    \n]"},
	} {
		m := testModel(t)
		typeText(m, tc.open)
		pressEnter(m)
		if got := text(m); got != tc.want {
			t.Fatalf("%q: %q", tc.open, got)
		}
		if m.tab().ed.Cursor != (ed.Pos{Line: 1, Col: 4}) {
			t.Fatalf("%q: курсор %+v", tc.open, m.tab().ed.Cursor)
		}
	}
}

func TestSmartEnterTabStep(t *testing.T) {
	m := testModel(t)
	m.cfg.Editor.Tabstospaces = false
	typeText(m, "{")
	pressEnter(m)
	if got := text(m); got != "{\n\t\n}" {
		t.Fatalf("таб-шаг: %q", got)
	}
}

func TestSmartEnterBreakPairUndo(t *testing.T) {
	m := testModel(t)
	typeText(m, "{")
	pressEnter(m)
	pressCtrl(m, 'z')
	if got := text(m); got != "{}" {
		t.Fatalf("undo: %q", got)
	}
	if m.tab().ed.Cursor != (ed.Pos{Line: 0, Col: 1}) {
		t.Fatalf("undo курсор %+v", m.tab().ed.Cursor)
	}
}

func TestEnterAutoIndentsAfterBrace(t *testing.T) {
	m := testModel(t)
	m.tab().ed = ed.NewEditor(ed.OpenBytes([]byte("    if x {"), "x.go"))
	m.tab().ed.SetCursor(ed.Pos{Line: 0, Col: 10})
	pressEnter(m)
	if got := text(m); got != "    if x {\n        " {
		t.Fatalf("отступ: %q", got)
	}
	if m.tab().ed.Cursor != (ed.Pos{Line: 1, Col: 8}) {
		t.Fatalf("курсор %+v", m.tab().ed.Cursor)
	}
}

func TestEnterNoExtraIndentInProse(t *testing.T) {
	m := testModel(t)
	m.tab().ed = ed.NewEditor(ed.OpenBytes([]byte("if x {"), "x.md"))
	m.tab().ed.SetCursor(ed.Pos{Line: 0, Col: 6})
	pressEnter(m)
	if got := text(m); got != "if x {\n" {
		t.Fatalf("prose-отступ: %q", got)
	}
}

func TestEnterReadOnly(t *testing.T) {
	m, _ := testModelFile(t, "x.txt", "abc")
	m.tab().ed.Buf.ReadOnly = true
	pressEnter(m)
	if got := text(m); got != "abc" {
		t.Fatalf("read-only изменён: %q", got)
	}
	if m.toast.Kind != "error" {
		t.Fatalf("нет тоста: %+v", m.toast)
	}
}

func TestSmartHome(t *testing.T) {
	m := testModel(t)
	m.tab().ed = ed.NewEditor(ed.OpenBytes([]byte("    foo"), ""))
	m.tab().ed.SetCursor(ed.Pos{Line: 0, Col: 2})
	m.runCommand("cursor.home")
	if got := m.tab().ed.Cursor; got != (ed.Pos{Line: 0, Col: 4}) {
		t.Fatalf("первый Home: %+v", got)
	}
	m.runCommand("cursor.home")
	if got := m.tab().ed.Cursor; got != (ed.Pos{Line: 0, Col: 0}) {
		t.Fatalf("второй Home: %+v", got)
	}
}

func TestSmartHomeExtendKeepsAnchor(t *testing.T) {
	m := testModel(t)
	m.tab().ed = ed.NewEditor(ed.OpenBytes([]byte("    foo"), ""))
	m.tab().ed.SetCursor(ed.Pos{Line: 0, Col: 6})
	m.runCommand("cursor.home.extend")
	e := m.tab().ed
	if e.Cursor != (ed.Pos{Line: 0, Col: 4}) || !e.HasSelection() || e.Sel.Anchor != (ed.Pos{Line: 0, Col: 6}) {
		t.Fatalf("extend: cursor=%+v sel=%+v", e.Cursor, e.Sel)
	}
	m.runCommand("cursor.home.extend")
	if e.Cursor != (ed.Pos{Line: 0, Col: 0}) || e.Sel.Anchor != (ed.Pos{Line: 0, Col: 6}) {
		t.Fatalf("повторный extend: cursor=%+v sel=%+v", e.Cursor, e.Sel)
	}
}

func TestBracketMatchView(t *testing.T) {
	m := testModel(t)
	m.tab().ed = ed.NewEditor(ed.OpenBytes([]byte("foo(bar)"), "x.go"))
	m.tab().ed.SetCursor(ed.Pos{Line: 0, Col: 3})
	v := m.View()
	if !m.tab().bracketOK || m.tab().bracketA != (ed.Pos{Line: 0, Col: 3}) || m.tab().bracketB != (ed.Pos{Line: 0, Col: 7}) {
		t.Fatalf("пара: %+v %+v ok=%v", m.tab().bracketA, m.tab().bracketB, m.tab().bracketOK)
	}
	if !strings.Contains(v.Content, m.th.bracketOn) {
		t.Fatal("стиль пары не попал в вывод")
	}

	// Курсор сразу после закрывающей - подсветка той же пары.
	m.tab().ed.SetCursor(ed.Pos{Line: 0, Col: 8})
	m.View()
	pair := map[ed.Pos]bool{m.tab().bracketA: true, m.tab().bracketB: true}
	if !m.tab().bracketOK || !pair[ed.Pos{Line: 0, Col: 3}] || !pair[ed.Pos{Line: 0, Col: 7}] {
		t.Fatalf("после close: %+v %+v", m.tab().bracketA, m.tab().bracketB)
	}

	// Вне скобок и при выделении подсветки нет.
	m.tab().ed.SetCursor(ed.Pos{Line: 0, Col: 2})
	m.View()
	if m.tab().bracketOK {
		t.Fatal("подсветка вне скобок")
	}
	m.tab().ed.SetCursor(ed.Pos{Line: 0, Col: 3})
	m.tab().ed.SetSelection(ed.Pos{Line: 0, Col: 0}, ed.Pos{Line: 0, Col: 2})
	m.View()
	if m.tab().bracketOK {
		t.Fatal("подсветка при выделении")
	}
}

func TestSaveTrimsTrailingWhitespaceInCode(t *testing.T) {
	m, path := testModelFile(t, "x.go", "a  \nb\t\nc  ")
	m.cfg.Editor.TrimTrailingWhitespace = true
	m.saveCurrent()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "a\nb\nc\n" {
		t.Fatalf("файл %q %v", data, err)
	}
}

func TestSaveKeepsProseTrailingSpaces(t *testing.T) {
	m, path := testModelFile(t, "x.md", "a  \nb")
	m.cfg.Editor.TrimTrailingWhitespace = true
	m.saveCurrent()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "a  \nb\n" {
		t.Fatalf("файл %q %v", data, err)
	}
}

func TestSaveFinalNewlineOff(t *testing.T) {
	m, path := testModelFile(t, "x.go", "abc")
	m.cfg.Editor.FinalNewline = false
	m.saveCurrent()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "abc" {
		t.Fatalf("файл %q %v", data, err)
	}
}

func TestSaveCleanDoesNotAddUndo(t *testing.T) {
	m, _ := testModelFile(t, "x.go", "abc\n")
	m.cfg.Editor.FinalNewline = true
	m.saveCurrent()
	ver := m.tab().ed.Buf.Version
	m.saveCurrent()
	if m.tab().ed.Buf.Version != ver {
		t.Fatal("повторное сохранение добавило правку")
	}
	if m.tab().ed.CanUndo() {
		t.Fatal("гигиена чистого файла попала в undo")
	}
}

func TestSaveAsHygiene(t *testing.T) {
	m := testModel(t)
	m.cfg.Editor.TrimTrailingWhitespace = true
	typeText(m, "a  ")
	m.openSaveAs()
	path := filepath.Join(t.TempDir(), "new.go")
	m.ov.input.Set(path)
	pressEnter(m)
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "a\n" {
		t.Fatalf("файл %q %v", data, err)
	}
}

func TestSaveAllHygiene(t *testing.T) {
	m, aPath := testModelFile(t, "a.go", "a  ")
	m.cfg.Editor.TrimTrailingWhitespace = true
	typeText(m, "x")
	bPath := filepath.Join(filepath.Dir(aPath), "b.go")
	writeFile(t, bPath, "b\t")
	m.openFile(bPath)
	typeText(m, "y")
	if !m.saveAll() {
		t.Fatal("saveAll = false")
	}
	for path, want := range map[string]string{aPath: "xa\n", bPath: "yb\n"} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != want {
			t.Fatalf("%s: %q %v", path, data, err)
		}
	}
}

func TestSelectionCountStatus(t *testing.T) {
	m := testModel(t)
	typeText(m, "abc\ndef")
	m.tab().ed.SetSelection(ed.Pos{Line: 0, Col: 0}, ed.Pos{Line: 1, Col: 3})
	if got := m.statusRow(); !strings.Contains(got, "7 символов") || !strings.Contains(got, "2 строки") {
		t.Fatalf("RU-счётчик: %q", got)
	}
	m.cfg.Language = "en"
	if got := m.statusRow(); !strings.Contains(got, "7 chars") || !strings.Contains(got, "2 lines") {
		t.Fatalf("EN-счётчик: %q", got)
	}
}

func TestReadOnlyReasonStatus(t *testing.T) {
	m, _ := testModelFile(t, "bin.dat", "\x00abc")
	if !m.tab().ed.Buf.ReadOnly {
		t.Fatal("бинарник не read-only")
	}
	if got := m.statusRow(); !strings.Contains(got, "бинарный") {
		t.Fatalf("RU-причина: %q", got)
	}
	m.cfg.Language = "en"
	if got := m.statusRow(); !strings.Contains(got, "binary") {
		t.Fatalf("EN-причина: %q", got)
	}

	m = testModel(t)
	m.tab().ed.Buf.ReadOnly = true
	m.tab().roReason = "large"
	if got := m.statusRow(); !strings.Contains(got, "МБ") {
		t.Fatalf("RU large: %q", got)
	}
	m.cfg.Language = "en"
	if got := m.statusRow(); !strings.Contains(got, "MB") {
		t.Fatalf("EN large: %q", got)
	}
}

func TestFocusChecksExternalChange(t *testing.T) {
	m, path := testModelFile(t, "x.md", "one")
	writeFile(t, path, "two")
	future := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(path, future, future)
	m.Update(tea.FocusMsg{})
	if m.ov.kind != overlayConfirm || m.ov.confirm.kind != confirmExternal {
		t.Fatalf("FocusMsg не проверил файл: %+v", m.ov.kind)
	}

	// Открытый диалог не перетираем.
	m2, path2 := testModelFile(t, "y.md", "one")
	m2.openConfirm(confirmQuit, -1, "Выйти?", "Есть правки")
	writeFile(t, path2, "two")
	_ = os.Chtimes(path2, future, future)
	m2.Update(tea.FocusMsg{})
	if m2.ov.confirm.kind != confirmQuit {
		t.Fatal("FocusMsg закрыл чужой диалог")
	}
}

func TestTabBarHiddenCount(t *testing.T) {
	m := testModel(t)
	setSize(m, 40, 24)
	for i := 0; i < 8; i++ {
		m.newEmptyTab()
	}
	m.active = 0
	st := m.tabBarState()
	if !st.right {
		t.Fatal("вкладки не переполнили полосу")
	}
	hidden := len(m.tabs) - st.end
	if hidden <= 0 {
		t.Fatalf("hidden=%d", hidden)
	}
	row := m.tabRow()
	if !strings.Contains(row, fmt.Sprintf("› %d", hidden)) {
		t.Fatalf("нет счётчика скрытых: %q", row)
	}
	if w := ansiWidth(row); w > m.w {
		t.Fatalf("полоса шире экрана: %d > %d (%q)", w, m.w, row)
	}
}
