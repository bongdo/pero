package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/bongdo/pero/internal/config"
	"github.com/bongdo/pero/internal/ed"
	"github.com/bongdo/pero/internal/workspace"
)

func testEnv(t *testing.T) {
	t.Helper()
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(base, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(base, "cache"))
}

func testModel(t *testing.T) *Model {
	t.Helper()
	testEnv(t)
	cfg := config.Default()
	cfg.Language = "ru"
	cfg.Editor.RestoreSession = false
	m := New(cfg, Options{Root: t.TempDir(), Empty: true})
	m.w, m.h = 80, 24
	m.lspAuto = false // тесты не запускают реальные серверы
	return m
}

func testModelFile(t *testing.T, name, content string) (*Model, string) {
	t.Helper()
	testEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	buf, err := ed.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Language = "ru"
	cfg.Editor.RestoreSession = false
	m := New(cfg, Options{Root: dir, Initial: buf})
	m.w, m.h = 80, 24
	m.lspAuto = false // тесты не запускают реальные серверы
	return m, path
}

func testModelLangEN(t *testing.T) *Model {
	t.Helper()
	m := testModel(t)
	m.cfg.Language = "en"
	return m
}

func setSize(m *Model, w, h int) { m.w, m.h = w, h }

func keyMsg(code rune, text string, mod tea.KeyMod) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code, Text: text, Mod: mod})
}

func pressCtrl(m *Model, r rune) {
	m.Update(keyMsg(r, string(r), tea.ModCtrl))
}

func pressAlt(m *Model, r rune) {
	m.Update(keyMsg(r, string(r), tea.ModAlt))
}

func typeText(m *Model, s string) {
	for _, r := range s {
		m.Update(keyMsg(r, string(r), 0))
	}
}

func pressEnter(m *Model) { m.Update(keyMsg(tea.KeyEnter, "", 0)) }

func pressEsc(m *Model) { m.Update(keyMsg(tea.KeyEscape, "", 0)) }

func viewLines(t *testing.T, m *Model) []string {
	t.Helper()
	v := m.View()
	lines := strings.Split(v.Content, "\n")
	if len(lines) != m.h {
		t.Fatalf("строк %d, ожидалось %d", len(lines), m.h)
	}
	for i, line := range lines {
		if w := ansi.StringWidth(line); w != m.w {
			t.Fatalf("строка %d шириной %d, ожидалось %d (%q)", i, w, m.w, line)
		}
	}
	return lines
}

func text(m *Model) string {
	if t := m.tab(); t != nil {
		return t.ed.Buf.Text()
	}
	return ""
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func listFiles(t *testing.T, dir string) []workspace.File {
	t.Helper()
	return workspace.List(context.Background(), dir, nil, 0)
}
