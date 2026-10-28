package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/config"
	"github.com/bongdo/pero/internal/ed"
)

func TestCloseDirtyTabAsks(t *testing.T) {
	m, _ := testModelFile(t, "a.md", "text")
	typeText(m, "!")
	if !m.tab().ed.Buf.Dirty() {
		t.Fatal("буфер не грязный")
	}
	m.runCommand("tab.close")
	if m.ov.kind != overlayConfirm || m.ov.confirm.kind != confirmCloseTab {
		t.Fatalf("нет подтверждения: %+v", m.ov.kind)
	}
	if len(m.tabs) != 1 {
		t.Fatal("вкладка закрылась без ответа")
	}
	// n = не сохранять
	m.Update(keyMsg('n', "n", 0))
	if len(m.tabs) != 1 {
		t.Fatalf("вкладок %d", len(m.tabs))
	}
	if m.tab().ed.Buf.Path != "" {
		t.Fatal("вместо пустой вкладки осталась закрытая")
	}
}

func TestCloseDirtySaveThenClose(t *testing.T) {
	m, path := testModelFile(t, "saveclose.md", "old")
	typeText(m, "!")
	m.runCommand("tab.close")
	if m.ov.kind != overlayConfirm {
		t.Fatal("нет подтверждения")
	}
	m.Update(keyMsg('y', "y", 0))
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "!old\n" {
		t.Fatalf("файл %q %v", data, err)
	}
	if len(m.tabs) != 1 || m.tab().ed.Buf.Path != "" {
		t.Fatalf("вкладка не закрылась: %d %q", len(m.tabs), m.tab().ed.Buf.Path)
	}
}

func TestCloseLastTabCreatesEmpty(t *testing.T) {
	m := testModel(t)
	m.runCommand("tab.close")
	if len(m.tabs) != 1 || m.tab().ed.Buf.Path != "" {
		t.Fatal("последняя вкладка должна замениться пустой")
	}
}

func TestSaveAs(t *testing.T) {
	m := testModel(t)
	typeText(m, "содержимое")
	m.openSaveAs()
	path := filepath.Join(t.TempDir(), "new.txt")
	m.ov.input.Set(path)
	pressEnter(m)
	if m.ov.kind != overlayNone {
		t.Fatalf("оверлей не закрылся: %v", m.ov.kind)
	}
	if m.tab().ed.Buf.Path != path {
		t.Fatalf("путь %q", m.tab().ed.Buf.Path)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "содержимое\n" {
		t.Fatalf("файл: %q %v", data, err)
	}
}

func TestSaveCurrent(t *testing.T) {
	m, path := testModelFile(t, "save.md", "old")
	typeText(m, "new")
	m.runCommand("file.save")
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "newold\n" && string(data) != "oldnew\n" {
		t.Fatalf("файл: %q %v", data, err)
	}
	if m.tab().ed.Buf.Dirty() {
		t.Fatal("после сохранения dirty")
	}
}

func TestExternalChangeCleanAsks(t *testing.T) {
	m, path := testModelFile(t, "ext.md", "one")
	if err := os.WriteFile(path, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(path, future, future)
	m.checkExternal(m.tab(), 0, false)
	if m.ov.kind != overlayConfirm || m.ov.confirm.kind != confirmExternal {
		t.Fatal("нет вопроса про внешнее изменение")
	}
	// y = перезагрузить
	m.Update(keyMsg('y', "y", 0))
	if text(m) != "two" {
		t.Fatalf("не перезагрузилось: %q", text(m))
	}
}

func TestExternalChangeDirtyAsks(t *testing.T) {
	m, path := testModelFile(t, "ext.md", "one")
	typeText(m, "!")
	if err := os.WriteFile(path, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(path, future, future)
	m.checkExternal(m.tab(), 0, false)
	if m.ov.kind != overlayConfirm || m.ov.confirm.kind != confirmExternal {
		t.Fatal("нет вопроса про внешнее изменение")
	}
	if text(m) != "!one" {
		t.Fatal("буфер затёрт до ответа")
	}
	// y = перезагрузить
	m.Update(keyMsg('y', "y", 0))
	if text(m) != "two" {
		t.Fatalf("не перезагрузилось: %q", text(m))
	}
}

func TestExternalChangeSaveKeepMine(t *testing.T) {
	m, path := testModelFile(t, "ext.md", "one")
	typeText(m, "!")
	if err := os.WriteFile(path, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(path, future, future)
	m.saveCurrent()
	if m.ov.kind != overlayConfirm || m.ov.confirm.kind != confirmExternal || !m.ov.confirm.pendingSave {
		t.Fatal("нет вопроса при сохранении")
	}
	// n = оставить моё и сохранить
	m.Update(keyMsg('n', "n", 0))
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "!one\n" {
		t.Fatalf("файл %q %v", data, err)
	}
	if m.tab().ed.Buf.Dirty() {
		t.Fatal("буфер остался грязным")
	}
}

func TestAlreadyOpenSwitches(t *testing.T) {
	m := testModel(t)
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	os.WriteFile(a, []byte("a"), 0o644)
	os.WriteFile(b, []byte("b"), 0o644)
	m.openFile(a)
	m.openFile(b)
	if len(m.tabs) != 2 {
		t.Fatalf("вкладок %d", len(m.tabs))
	}
	m.openFile(a)
	if len(m.tabs) != 2 || m.tab().ed.Buf.Path != a {
		t.Fatalf("дубль или не та вкладка: %d %q", len(m.tabs), m.tab().ed.Buf.Path)
	}
}

func TestEmptyTabReused(t *testing.T) {
	m := testModel(t)
	path := filepath.Join(t.TempDir(), "x.txt")
	os.WriteFile(path, []byte("x"), 0o644)
	m.openFile(path)
	if len(m.tabs) != 1 {
		t.Fatalf("пустая вкладка не переиспользована: %d", len(m.tabs))
	}
}

func TestStateRoundTrip(t *testing.T) {
	m, path := testModelFile(t, "state.md", "abc")
	m.cfg.Editor.RestoreSession = true
	m.tab().ed.SetCursor(ed.Pos{Line: 0, Col: 2})
	if err := m.saveState(); err != nil {
		t.Fatal(err)
	}
	cfg := m.cfg
	cfg.Editor.RestoreSession = true
	nm := New(cfg, Options{Root: m.root})
	if len(nm.tabs) != 1 || nm.tab().ed.Buf.Path != path {
		t.Fatalf("сессия не восстановлена: %+v", nm.tab().ed.Buf.Path)
	}
	if nm.tab().ed.Cursor.Col != 2 {
		t.Fatalf("курсор %+v", nm.tab().ed.Cursor)
	}
}

func TestRestoreSkipsMissing(t *testing.T) {
	testEnv(t)
	dir := t.TempDir()
	missing := filepath.Join(dir, "gone.md")
	st := config.State{Tabs: []config.TabState{{Path: missing}}}
	if err := config.SaveState(st); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Editor.RestoreSession = true
	m := New(cfg, Options{Root: dir})
	if len(m.tabs) != 1 || m.tab().ed.Buf.Path != "" {
		t.Fatal("пропавший файл не пропущен")
	}
}

func TestQuitConfirmOnDirty(t *testing.T) {
	m := testModel(t)
	typeText(m, "x")
	cmd := m.runCommand("app.quit")
	if cmd != nil {
		t.Fatal("выход без вопроса")
	}
	if m.ov.kind != overlayConfirm || m.ov.confirm.kind != confirmQuit {
		t.Fatal("нет подтверждения выхода")
	}
	// n = выйти без сохранения
	m.Update(keyMsg('n', "n", 0))
	if m.ov.kind != overlayNone {
		t.Fatal("подтверждение не закрылось")
	}
}

func TestTabBarClick(t *testing.T) {
	m := testModel(t)
	m.newEmptyTab()
	setSize(m, 80, 24)
	st := m.tabBarState()
	if len(st.hits) < 2 {
		t.Fatal("нет двух вкладок в полосе")
	}
	hit := st.hits[0]
	m.mouseClick(tea.Mouse{X: hit.x + 1, Y: 0, Button: tea.MouseLeft})
	if m.active != hit.idx {
		t.Fatalf("активная %d, клик по %d", m.active, hit.idx)
	}
}

func TestMiddleClickClosesTab(t *testing.T) {
	m := testModel(t)
	m.newEmptyTab()
	setSize(m, 80, 24)
	st := m.tabBarState()
	hit := st.hits[1]
	m.mouseClick(tea.Mouse{X: hit.x + 1, Y: 0, Button: tea.MouseMiddle})
	if len(m.tabs) != 1 {
		t.Fatalf("вкладок %d", len(m.tabs))
	}
}

func TestCloseOthersKeepsOne(t *testing.T) {
	m := testModel(t)
	m.newEmptyTab()
	m.newEmptyTab()
	m.active = 1
	m.runCommand("tab.closeOthers")
	if len(m.tabs) != 1 {
		t.Fatalf("вкладок %d", len(m.tabs))
	}
	if m.active != 0 {
		t.Fatalf("активная %d", m.active)
	}
}

func TestCloseOthersDirtyAsks(t *testing.T) {
	m := testModel(t)
	typeText(m, "x")
	m.newEmptyTab()
	m.active = 1
	m.runCommand("tab.closeOthers")
	if m.ov.kind != overlayConfirm || m.ov.confirm.kind != confirmCloseOthers {
		t.Fatal("нет подтверждения")
	}
	m.Update(keyMsg('n', "n", 0))
	if len(m.tabs) != 1 || m.tab().ed.Buf.Path != "" {
		t.Fatalf("осталось %d вкладок", len(m.tabs))
	}
	if m.tab().ed.Buf.Dirty() {
		t.Fatal("осталась чужая грязная вкладка")
	}
}
