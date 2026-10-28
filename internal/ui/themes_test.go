package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/config"
)

func themeIndex(items []string, name string) int {
	for i, n := range items {
		if n == name {
			return i
		}
	}
	return -1
}

func TestThemeListPreviewAndEscReverts(t *testing.T) {
	m := testModel(t)
	orig := m.cfg.Theme
	m.openThemeList()
	if m.ov.kind != overlayThemes {
		t.Fatalf("оверлей %v", m.ov.kind)
	}
	items := m.themeItems()
	idx := themeIndex(items, "nord")
	if idx < 0 {
		t.Fatal("nord нет в списке")
	}
	m.ov.sel = idx
	m.previewTheme()
	if m.cfg.Theme != "nord" || m.theme.Name != "nord" {
		t.Fatalf("preview: cfg=%q theme=%q", m.cfg.Theme, m.theme.Name)
	}
	pressEsc(m)
	if m.ov.kind != overlayNone {
		t.Fatal("Esc не закрыл список")
	}
	if m.cfg.Theme != orig || m.theme.Name != orig {
		t.Fatalf("тема не восстановлена: cfg=%q theme=%q", m.cfg.Theme, m.theme.Name)
	}
}

func TestThemeListEnterSaves(t *testing.T) {
	m := testModel(t)
	m.openThemeList()
	items := m.themeItems()
	m.ov.sel = themeIndex(items, "nord")
	pressEnter(m)
	if m.ov.kind != overlayNone {
		t.Fatal("Enter не закрыл список")
	}
	if m.cfg.Theme != "nord" || m.toast.Kind != "success" {
		t.Fatalf("тема/тост: %q %+v", m.cfg.Theme, m.toast)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme != "nord" {
		t.Fatalf("в конфиге %q", cfg.Theme)
	}
}

func TestThemeListFilterAndPreview(t *testing.T) {
	m := testModel(t)
	m.openThemeList()
	typeText(m, "nord")
	items := m.themeItems()
	if len(items) == 0 || items[0] != "nord" {
		t.Fatalf("фильтр: %v", items)
	}
	if m.cfg.Theme != "nord" {
		t.Fatalf("фильтр не показал тему: %q", m.cfg.Theme)
	}
	// Подпоследовательность и регистр не важны.
	m.ov.input.Set("CNM")
	items = m.themeItems()
	if themeIndex(items, "catppuccin-mocha") < 0 {
		t.Fatalf("подпоследовательность: %v", items)
	}
}

func TestThemeListSaveErrorKeepsTheme(t *testing.T) {
	m := testModel(t)
	blocked := filepath.Join(t.TempDir(), "blocked")
	writeFile(t, blocked, "not a directory")
	t.Setenv("XDG_CONFIG_HOME", blocked)

	m.openThemeList()
	items := m.themeItems()
	m.ov.sel = themeIndex(items, "nord")
	pressEnter(m)
	if m.cfg.Theme != "nord" || m.theme.Name != "nord" {
		t.Fatalf("тема откатилась после ошибки: %q", m.cfg.Theme)
	}
	if m.toast.Kind != "error" {
		t.Fatalf("нет error-тоста: %+v", m.toast)
	}
}

func TestThemeListMouseHoverAndOutsideClick(t *testing.T) {
	m := testModel(t)
	orig := m.cfg.Theme
	m.openThemeList()
	listY, listH, _, _ := m.themeGeometry()
	if listH < 2 {
		t.Fatalf("listH=%d", listH)
	}
	m.themeHover(tea.Mouse{X: 0, Y: listY + 1})
	items := m.themeItems()
	if m.ov.sel != 1 || m.cfg.Theme != items[1] {
		t.Fatalf("hover: sel=%d theme=%q", m.ov.sel, m.cfg.Theme)
	}
	m.themeClick(tea.Mouse{X: 0, Y: 0, Button: tea.MouseLeft})
	if m.ov.kind != overlayNone {
		t.Fatal("клик вне не закрыл список")
	}
	if m.cfg.Theme != orig {
		t.Fatalf("клик вне не вернул тему: %q", m.cfg.Theme)
	}
}

func TestThemePaletteMappedToHighlighter(t *testing.T) {
	m := testModel(t)
	m.applyTheme("nord")
	p := themePalette(m.theme)
	if p.Keyword == "" || p.Comment == "" || p.Function == "" || p.Type == "" {
		t.Fatalf("палитра не заполнена: %+v", p)
	}
	if strings.Contains(p.Keyword, "\x1b") {
		t.Fatalf("палитра должна хранить hex, а не SGR: %q", p.Keyword)
	}
}

func TestThemeListViewFits(t *testing.T) {
	for _, sz := range []struct{ w, h int }{{40, 10}, {80, 24}, {160, 50}, {20, 5}, {8, 5}} {
		m := testModel(t)
		setSize(m, sz.w, sz.h)
		m.openThemeList()
		viewLines(t, m)
		if _, listH, _, _ := m.themeGeometry(); listH > 12 {
			t.Fatalf("%dx%d: строк %d", sz.w, sz.h, listH)
		}
	}
}
