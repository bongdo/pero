package config

import (
	"strconv"
	"strings"
	"testing"
)

func TestThemeNamesStable(t *testing.T) {
	want := []string{
		"everforest", "vesper", "nord", "plain",
		"catppuccin-mocha", "catppuccin-macchiato", "catppuccin-frappe", "catppuccin-latte",
		"gruvbox-dark", "gruvbox-light",
		"tokyonight-night", "tokyonight-storm", "tokyonight-moon", "tokyonight-day",
		"dracula", "solarized-dark", "solarized-light", "one-dark", "one-light", "monokai",
		"ayu-dark", "ayu-mirage", "ayu-light",
		"kanagawa-wave", "kanagawa-dragon", "kanagawa-lotus",
		"rose-pine", "rose-pine-moon", "rose-pine-dawn",
		"github-dark", "github-light", "zenburn", "flexoki-dark",
	}
	got := ThemeNames()
	if len(got) != len(want) || len(got) != len(themes) {
		t.Fatalf("ThemeNames: %d имён, в списке %d, в map %d", len(got), len(want), len(themes))
	}
	if len(got) < 24 {
		t.Fatalf("тем %d, нужно минимум 24", len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ThemeNames[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	seen := make(map[string]bool, len(got))
	for _, name := range got {
		if name == "" || seen[name] {
			t.Fatalf("пустое или повторяющееся имя темы: %q", name)
		}
		seen[name] = true
	}

	got[0] = "changed"
	if ThemeNames()[0] != "everforest" {
		t.Fatal("ThemeNames должен возвращать копию")
	}
}

func TestThemeColors(t *testing.T) {
	for _, name := range ThemeNames() {
		th := ThemeByName(name)
		if th.Name != name {
			t.Errorf("ThemeByName(%q).Name = %q", name, th.Name)
		}
		fields := map[string]string{
			"BG": th.BG, "Panel": th.Panel, "Border": th.Border, "Text": th.Text,
			"Muted": th.Muted, "Accent": th.Accent, "SelectionBG": th.SelectionBG,
			"CurrentLine": th.CurrentLine, "Gutter": th.Gutter, "StatusBG": th.StatusBG,
			"StatusFG": th.StatusFG, "TabActive": th.TabActive, "TabInactive": th.TabInactive,
			"Error": th.Error, "Warning": th.Warning, "Info": th.Info, "Ghost": th.Ghost,
			"SearchBG": th.SearchBG, "SearchCurrent": th.SearchCurrent,
			"DiagnosticError": th.DiagnosticError, "DiagnosticWarning": th.DiagnosticWarning,
			"DiagnosticInfo": th.DiagnosticInfo,
			"Keyword":        th.Keyword, "String": th.String, "Number": th.Number,
			"Comment": th.Comment, "Function": th.Function, "Type": th.Type,
			"Operator": th.Operator, "Constant": th.Constant, "Builtin": th.Builtin,
			"Punctuation": th.Punctuation,
		}
		for field, color := range fields {
			if !validColor(color) {
				t.Errorf("%s.%s = %q: не hex и не ANSI", name, field, color)
			}
		}
		// Обязательный минимум, чтобы тема не выглядела пустой.
		for field, color := range map[string]string{
			"BG": th.BG, "Text": th.Text, "Accent": th.Accent,
			"Keyword": th.Keyword, "String": th.String, "Comment": th.Comment,
			"Function": th.Function,
		} {
			if strings.TrimSpace(color) == "" {
				t.Errorf("%s.%s пуст", name, field)
			}
		}
		if name == "plain" {
			for field, color := range fields {
				if !plainColor(color) {
					t.Errorf("plain.%s = %q: нужен ANSI 0..15 без hex", field, color)
				}
			}
		}
	}

	if th := ThemeByName("  NoRd \n"); th.Name != "nord" {
		t.Errorf("имя темы не нормализовалось: %+v", th)
	}
	if th := ThemeByName("dracula"); th.Name != "dracula" {
		t.Errorf("dracula должна быть встроенной, got %q", th.Name)
	}
	if th := ThemeByName("no-such-theme"); th.Name != "plain" {
		t.Errorf("неизвестная тема должна дать plain, got %q", th.Name)
	}
	if th := ThemeByName(""); th.Name != "plain" {
		t.Errorf("пустая тема должна дать plain, got %q", th.Name)
	}
}

func validColor(s string) bool {
	if strings.HasPrefix(s, "#") {
		if len(s) != 7 {
			return false
		}
		for _, r := range s[1:] {
			if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
				return false
			}
		}
		return true
	}
	n, err := strconv.Atoi(s)
	return err == nil && n >= 0 && n <= 255
}

func plainColor(s string) bool {
	if strings.HasPrefix(s, "#") {
		return false
	}
	n, err := strconv.Atoi(s)
	return err == nil && n >= 0 && n <= 15
}
