package lsp

import (
	"strings"
	"testing"
)

func TestPresets(t *testing.T) {
	names := PresetNames()
	if len(names) != 9 {
		t.Fatalf("preset count = %d, want 9: %v", len(names), names)
	}
	want := map[string][]string{
		"gopls":                      {"gopls"},
		"pyright-langserver":         {"pyright-langserver", "--stdio"},
		"typescript-language-server": {"typescript-language-server", "--stdio"},
		"clangd":                     {"clangd"},
		"lua-language-server":        {"lua-language-server"},
		"nixd":                       {"nixd"},
		"nimony-lsp":                 {"nimony-lsp"},
		"ltex-ls":                    {"ltex-ls"},
		"harper-ls":                  {"harper-ls", "--stdio"},
	}
	for name, command := range want {
		cfg, ok := Preset(name)
		if !ok {
			t.Fatalf("Preset(%q) not found", name)
		}
		got := append([]string{cfg.Command}, cfg.Args...)
		if strings.Join(got, " ") != strings.Join(command, " ") {
			t.Errorf("Preset(%q) command = %v, want %v", name, got, command)
		}
		if cfg.LanguageID == "" {
			t.Errorf("Preset(%q) has no language ID", name)
		}
	}
	if _, ok := Preset("nope"); ok {
		t.Error("unknown preset accepted")
	}
	if cfg, _ := Preset("go"); cfg.Command != "gopls" {
		t.Errorf("alias go = %#v", cfg)
	}

	servers := DefaultServers()
	for _, name := range []string{"go", "python", "typescript", "c", "lua", "nix", "nim", "markdown", "latex"} {
		if servers[name] == "" {
			t.Errorf("DefaultServers has no %q", name)
		}
	}
	if servers["python"] != "pyright-langserver --stdio" ||
		servers["typescript"] != "typescript-language-server --stdio" {
		t.Errorf("default commands = %#v", servers)
	}
}

func TestServerConfigFromCommand(t *testing.T) {
	cfg := ServerConfigFromCommand("typescript", "typescript-language-server --stdio --log-level 4")
	if cfg.Command != "typescript-language-server" ||
		len(cfg.Args) != 3 || cfg.Args[0] != "--stdio" || cfg.Args[2] != "4" {
		t.Fatalf("cfg = %#v", cfg)
	}
	if empty := ServerConfigFromCommand("go", "   "); empty.Command != "" {
		t.Fatalf("empty command = %#v", empty)
	}
	// harper-ls needs stdio; without it the editor connects to nothing.
	harper := ServerConfigFromCommand("markdown", "harper-ls")
	if harper.Command != "harper-ls" || len(harper.Args) != 1 || harper.Args[0] != "--stdio" {
		t.Fatalf("harper cfg = %#v", harper)
	}
	if again := ServerConfigFromCommand("markdown", "harper-ls --stdio"); len(again.Args) != 1 {
		t.Fatalf("duplicate --stdio: %#v", again.Args)
	}
	if servers := DefaultServers(); servers["markdown"] != "harper-ls --stdio" || servers["plaintext"] != "harper-ls --stdio" {
		t.Fatalf("defaults = %#v", servers)
	}
}

func TestManagerSelectionByLanguageExtensionAndGlob(t *testing.T) {
	m := NewManager("/project", map[string]string{
		"go":       "gopls",
		"python":   "pyright-langserver --stdio",
		"*.py":     "wrong-python-server",
		"*.ts":     "typescript-language-server --stdio",
		"**/*.tsx": "tsx-server --stdio",
		"lua":      "lua-language-server",
		".rb":      "ruby-ls",
		"*.md":     "harper-ls",
		"note.txt": "plain-note-server",
		"*.custom": "glob-server",
	})

	cases := []struct {
		path string
		lang string
		want string
		lgid string
	}{
		{"/project/main.go", "", "gopls", "go"},
		{"/project/app.py", "", "pyright-langserver", "python"},
		{"/project/app.ts", "", "typescript-language-server", "typescript"},
		{"/project/src/ui/app.tsx", "", "tsx-server", "typescriptreact"},
		{"/project/init.lua", "", "lua-language-server", "lua"},
		{"/project/script.rb", "", "ruby-ls", "ruby"},
		{"/project/README.md", "", "harper-ls", "markdown"},
		{"/project/note.txt", "", "plain-note-server", "plaintext"},
		{"/project/data.custom", "", "glob-server", "plaintext"},
		{"/project/README.md", "lua", "lua-language-server", "lua"},
	}
	for _, tc := range cases {
		cfg, err := m.configFor(tc.path, tc.lang)
		if err != nil {
			t.Errorf("configFor(%q, %q): %v", tc.path, tc.lang, err)
			continue
		}
		if cfg.Command != tc.want {
			t.Errorf("configFor(%q, %q).Command = %q, want %q", tc.path, tc.lang, cfg.Command, tc.want)
		}
		if tc.lgid != "" && cfg.LanguageID != tc.lgid {
			t.Errorf("configFor(%q, %q).LanguageID = %q, want %q", tc.path, tc.lang, cfg.LanguageID, tc.lgid)
		}
	}
}

func TestManagerReactExtensionFallsBackToTypescript(t *testing.T) {
	m := NewManager("/project", map[string]string{"typescript": "tsserver --stdio"})
	cfg, err := m.configFor("/project/App.tsx", "")
	if err != nil {
		t.Fatalf("configFor: %v", err)
	}
	if cfg.Command != "tsserver" || cfg.LanguageID != "typescriptreact" {
		t.Fatalf("cfg = %#v", cfg)
	}
	if _, err := NewManager("/project", DefaultServers()).configFor("/project/App.tsx", ""); err != nil {
		t.Fatalf("default servers do not cover .tsx: %v", err)
	}
}

func TestManagerNoServerError(t *testing.T) {
	m := NewManager("/project", map[string]string{"go": "gopls"})
	_, err := m.ClientFor("/project/data.zzz", "")
	if err == nil {
		t.Fatal("ClientFor without a server returned nil error")
	}
	if !strings.Contains(err.Error(), "no server configured") || !strings.Contains(err.Error(), "data.zzz") {
		t.Fatalf("error = %q", err)
	}
}

func TestManagerRelativeGlob(t *testing.T) {
	m := NewManager("/project", map[string]string{"src/**/*.go": "src-go-server"})
	cfg, err := m.configFor("/project/src/a/b/main.go", "go")
	if err != nil {
		t.Fatalf("configFor: %v", err)
	}
	if cfg.Command != "src-go-server" {
		t.Fatalf("Command = %q", cfg.Command)
	}
	if _, err := m.configFor("/project/other/main.go", "go"); err == nil {
		t.Fatal("glob matched outside src/")
	}
}

func TestGlobMatch(t *testing.T) {
	cases := []struct {
		pattern string
		target  string
		want    bool
	}{
		{"*.go", "main.go", true},
		{"*.go", "main.py", false},
		{"**/*.go", "main.go", true},
		{"src/*.go", "src/main.go", true},
		{"src/*.go", "other/main.go", false},
		{"src/**/*.go", "src/a/b/main.go", true},
		{"src/**", "src/a/b/main.go", true},
		{"[ab].go", "a.go", true},
	}
	for _, tc := range cases {
		if got := globMatch(tc.pattern, tc.target); got != tc.want {
			t.Errorf("globMatch(%q, %q) = %v, want %v", tc.pattern, tc.target, got, tc.want)
		}
	}
}

func TestLanguageIDForPath(t *testing.T) {
	cases := map[string]string{
		"a.go":        "go",
		"a.py":        "python",
		"a.tsx":       "typescriptreact",
		"a.md":        "markdown",
		"a.txt":       "plaintext",
		"Makefile":    "makefile",
		"go.mod":      "gomod",
		"unknown.zzz": "",
	}
	for path, want := range cases {
		if got := languageIDForPath(path); got != want {
			t.Errorf("languageIDForPath(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestManagerCloseIdempotent(t *testing.T) {
	m := NewManager("/project", nil)
	m.Close()
	m.Close()
	if _, err := m.ClientFor("/project/main.go", "go"); err == nil {
		t.Fatal("ClientFor after Close returned nil error")
	}
}
