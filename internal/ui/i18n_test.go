package ui

import (
	"strings"
	"testing"
)

func joinedHelp(m *Model) string {
	return strings.Join(m.helpLines(100), "\n")
}

func containsCyrillicText(s string) bool {
	for _, r := range s {
		if r >= 0x0400 && r <= 0x052F {
			return true
		}
	}
	return false
}

func joinedPalette(m *Model) string {
	m.openCommandPalette()
	return strings.Join(m.commandPaletteRows(), "\n")
}

func TestEnglishUIIsDefault(t *testing.T) {
	m := testModelLangEN(t)
	if containsCyrillicText(m.statusRow()) {
		t.Fatalf("статус по-русски: %q", m.statusRow())
	}
	if got := joinedHelp(m); containsCyrillicText(got) {
		t.Fatalf("help по-русски:\n%s", got)
	}
	if got := joinedPalette(m); containsCyrillicText(got) {
		t.Fatalf("палитра по-русски:\n%s", got)
	}
	m.closeOverlays()
	m.openThemeList()
	rows := m.drawThemeOverlay(make([]string, m.h))
	for _, line := range rows {
		if containsCyrillicText(line) {
			t.Fatalf("список тем по-русски: %q", line)
		}
	}
}

func TestRussianUIWhenConfigured(t *testing.T) {
	m := testModel(t)
	if !containsCyrillicText(m.statusRow()) {
		t.Fatalf("статус не по-русски: %q", m.statusRow())
	}
	if got := joinedHelp(m); !strings.Contains(got, "Файлы") {
		t.Fatalf("help без русских секций:\n%s", got)
	}
	if got := joinedPalette(m); !strings.Contains(got, "Сохранить файл") {
		t.Fatalf("палитра без русских команд:\n%s", got)
	}
}

func TestCommandFilterFindsBothLanguages(t *testing.T) {
	m := testModelLangEN(t)
	all := m.commandList()
	for _, tc := range []struct{ query, want string }{
		{"сохр", "file.save"},
		{"save", "file.save"},
		{"тему", "theme.set"},
		{"theme", "theme.set"},
		{"конфиг", "config.reload"},
		{"reload", "config.reload"},
	} {
		got := filterCommands(all, tc.query)
		if len(got) == 0 || got[0].id != tc.want {
			t.Fatalf("%q: %v, want %q", tc.query, ids(got), tc.want)
		}
	}
}
