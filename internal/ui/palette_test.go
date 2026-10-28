package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bongdo/pero/internal/config"
)

func TestPaletteFilterAndRecent(t *testing.T) {
	m := testModel(t)
	m.recentCmds = []string{"app.quit"}
	m.openCommandPalette()
	items := m.paletteItems()
	if len(items) == 0 || items[0].id != "app.quit" {
		t.Fatalf("история не сверху: %v", ids(items))
	}
	m.ov.input.Set("сохр")
	items = m.paletteItems()
	if len(items) == 0 || items[0].id != "file.save" {
		t.Fatalf("фильтр: %v", ids(items))
	}
	m.ov.input.Set("zzzz")
	if got := m.paletteItems(); len(got) != 0 {
		t.Fatalf("мусор что-то нашёл: %v", ids(got))
	}
}

func TestPaletteHidesUnavailable(t *testing.T) {
	m := testModel(t)
	aiVisible := false
	for _, c := range m.commandList() {
		if c.id == "lsp.hover" {
			t.Fatalf("заглушка %s попала в палитру", c.id)
		}
		if c.id == "ai.suggest" {
			aiVisible = true
		}
	}
	if !aiVisible {
		t.Fatal("ai.suggest должна быть видна в палитре всегда")
	}
}

func TestPaletteNavigationWraps(t *testing.T) {
	m := testModel(t)
	m.openCommandPalette()
	n := len(m.paletteItems())
	m.ov.sel = 0
	m.paletteMove(-1)
	if m.ov.sel != n-1 {
		t.Fatalf("sel=%d, want %d", m.ov.sel, n-1)
	}
	m.paletteMove(1)
	if m.ov.sel != 0 {
		t.Fatalf("sel=%d", m.ov.sel)
	}
}

func TestPaletteTabCompletes(t *testing.T) {
	m := testModel(t)
	m.openCommandPalette()
	m.ov.input.Set("сохр")
	// Tab дополняет выбранную команду.
	m.Update(keyMsg('\t', "", 0))
	if !strings.Contains(m.ov.input.Value(), "Сохранить") {
		t.Fatalf("не дополнилось: %q", m.ov.input.Value())
	}
}

func TestConfigKeysAddAndConflict(t *testing.T) {
	m := testModel(t)
	m.cfg.Keys = map[string]string{
		"F2":     "view.wrap",
		"Ctrl-S": "tab.close",
	}
	warns := m.buildBindings()
	if len(warns) != 1 || !strings.Contains(warns[0], "Ctrl-S") {
		t.Fatalf("нет предупреждения о конфликте: %v", warns)
	}
	if m.bindings["ctrl+s"] != "file.save" {
		t.Fatal("встроенный биндинг перетёрт")
	}
	if m.bindings["f2"] != "view.wrap" {
		t.Fatal("добавленный биндинг не работает")
	}
}

func TestNormalizeKey(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Ctrl-Shift-S", "ctrl+shift+s"},
		{"alt-t", "alt+t"},
		{"Ctrl-Tab", "ctrl+tab"},
		{"Shift-F3", "shift+f3"},
		{"Ctrl+Alt+Delete", "ctrl+alt+delete"},
	} {
		if got := normalizeKey(tc.in); got != tc.want {
			t.Fatalf("%q -> %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestOpenPaletteFilterAndChoose(t *testing.T) {
	m := testModel(t)
	dir := m.root
	writeFile(t, dir+"/alpha.md", "a")
	writeFile(t, dir+"/beta.md", "b")
	m.ov.kind = overlayOpen
	m.ov.files = listFiles(t, dir)
	m.ov.input.Set("bet")
	m.recomputeOpenMatches()
	if len(m.ov.matches) == 0 || !strings.Contains(m.ov.matches[0].File.Rel, "beta") {
		t.Fatalf("фильтр: %+v", m.ov.matches)
	}
	m.chooseOpen()
	if len(m.tabs) != 1 || !strings.HasSuffix(m.tab().ed.Buf.Path, "beta.md") {
		t.Fatalf("открылся %q", m.tab().ed.Buf.Path)
	}
}

func TestOpenPaletteAsyncWalk(t *testing.T) {
	m := testModel(t)
	writeFile(t, m.root+"/one.md", "1")
	writeFile(t, m.root+"/two.md", "2")
	writeFile(t, m.root+"/three.go", "3")
	cmd := m.openOpenPalette()
	for i := 0; i < 100 && cmd != nil; i++ {
		msg := cmd()
		_, next := m.Update(msg)
		cmd = next
		if !m.ov.loading {
			break
		}
	}
	if m.ov.loading {
		t.Fatal("обход не завершился")
	}
	if len(m.ov.files) != 3 {
		t.Fatalf("файлов %d: %+v", len(m.ov.files), m.ov.files)
	}
	m.ov.input.Set("two")
	m.recomputeOpenMatches()
	if len(m.ov.matches) == 0 || !strings.Contains(m.ov.matches[0].File.Rel, "two") {
		t.Fatalf("фильтр: %+v", m.ov.matches)
	}
	m.closeOverlays()
}

func TestOpenPaletteExistingFileAfterAltO(t *testing.T) {
	m := testModel(t)
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	writeFile(t, a, "a")
	m.openFile(a)
	if len(m.tabs) != 1 {
		t.Fatal("подготовка")
	}
	m.newEmptyTab()
	if len(m.tabs) != 2 || m.tab().ed.Buf.Path != "" {
		t.Fatal("нет пустой вкладки")
	}
	m.openFile(a)
	if len(m.tabs) != 1 || m.tab().ed.Buf.Path != a {
		t.Fatalf("пустая вкладка не убрана: %d %q", len(m.tabs), m.tab().ed.Buf.Path)
	}
}

func TestOpenPaletteCreateFile(t *testing.T) {
	m := testModel(t)
	m.ov.kind = overlayOpen
	m.ov.input.Set("new-file.txt")
	m.createFileFromQuery()
	if m.tab().ed.Buf.Path == "" || !strings.HasSuffix(m.tab().ed.Buf.Path, "new-file.txt") {
		t.Fatalf("путь %q", m.tab().ed.Buf.Path)
	}
	if m.ov.kind != overlayNone {
		t.Fatal("палитра не закрылась")
	}
}

func TestOpenPaletteDirMode(t *testing.T) {
	m := testModel(t)
	dir := t.TempDir()
	writeFile(t, dir+"/one.md", "1")
	writeFile(t, dir+"/two.md", "2")
	matches := m.dirMatches(dir + "/")
	if len(matches) != 2 {
		t.Fatalf("файлов %d", len(matches))
	}
}

func TestOpenViewNoPanic(t *testing.T) {
	m := testModel(t)
	m.ov.kind = overlayOpen
	for _, sz := range []struct{ w, h int }{{40, 10}, {80, 24}, {160, 50}, {20, 5}} {
		setSize(m, sz.w, sz.h)
		v := m.openView()
		if !v.shown {
			t.Fatalf("%dx%d: палитра не показана", sz.w, sz.h)
		}
		viewLines(t, m)
	}
}

func TestCommandsAvailableInPalette(t *testing.T) {
	m := testModel(t)
	have := map[string]bool{}
	for _, c := range m.commandList() {
		have[c.id] = true
	}
	for _, id := range []string{"file.save", "file.open", "file.saveAs", "tab.new", "tab.close", "edit.find", "edit.replace", "markdown.checkbox", "view.wrap", "theme.set", "help.keys", "app.quit"} {
		if !have[id] {
			t.Fatalf("нет команды %s", id)
		}
	}
}

func TestConfigDefault(t *testing.T) {
	m := testModel(t)
	if m.cfg.Theme != config.Default().Theme {
		t.Fatal("тема")
	}
}
