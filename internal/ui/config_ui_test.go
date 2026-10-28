package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bongdo/pero/internal/config"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path, err := config.ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReloadConfigAppliesAll(t *testing.T) {
	m := testModelLangEN(t)
	m.tab().ed.TabWidth = 4
	writeConfig(t, `theme = "nord"
language = "ru"
[editor]
tabsize = 2
`)
	m.runCommand("config.reload")
	if m.cfg.Language != "ru" {
		t.Fatalf("язык %q", m.cfg.Language)
	}
	if m.cfg.Theme != "nord" || m.theme.Name != "nord" {
		t.Fatalf("тема %q", m.cfg.Theme)
	}
	if m.tab().ed.TabWidth != 2 {
		t.Fatalf("tabsize %d", m.tab().ed.TabWidth)
	}
	if m.toast.Kind != "success" {
		t.Fatalf("тост %+v", m.toast)
	}
	if got := m.statusRow(); !containsCyrillicText(got) {
		t.Fatalf("язык не применился к статусу: %q", got)
	}
}

func TestReloadConfigBrokenKeepsOld(t *testing.T) {
	m := testModelLangEN(t)
	writeConfig(t, "theme = \"nord\"\n[editor\n")
	m.runCommand("config.reload")
	if m.cfg.Theme != "everforest" || m.cfg.Language != "en" {
		t.Fatalf("битый конфиг применился: %q %q", m.cfg.Theme, m.cfg.Language)
	}
	if m.toast.Kind != "error" {
		t.Fatalf("нет error-тоста: %+v", m.toast)
	}
}
