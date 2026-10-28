package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// testEnv изолирует XDG-пути и возвращает ожидаемые config.toml/state.toml.
func testEnv(t *testing.T) (configPath, statePath string) {
	t.Helper()
	cfgDir := t.TempDir()
	stateDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
	t.Setenv("XDG_STATE_HOME", stateDir)
	return filepath.Join(cfgDir, "pero", "config.toml"), filepath.Join(stateDir, "pero", "state.toml")
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadCreatesDefault(t *testing.T) {
	cfgPath, _ := testEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Fatalf("Load вернул не дефолт:\n got %+v\nwant %+v", cfg, Default())
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("конфиг не создан: %v", err)
	}
	if !strings.Contains(string(data), `theme = "everforest"`) || !strings.Contains(string(data), "# Настройки pero.") {
		t.Fatalf("в созданном конфиге нет шапки/темы:\n%s", data)
	}
	for _, want := range []string{`language = "en"`, "autopairs = true", "trim_trailing_whitespace = false", "final_newline = true"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("в созданном конфиге нет %q:\n%s", want, data)
		}
	}
	assertMode(t, cfgPath, 0o644)
	// Каталог создаётся с 0755; umask может урезать права, поэтому проверяем владельца.
	if info, err := os.Stat(filepath.Dir(cfgPath)); err != nil || info.Mode().Perm()&0o700 != 0o700 {
		t.Fatalf("каталог конфига: %v, %v", info, err)
	}

	again, warn := LoadOrDefault()
	if warn != "" {
		t.Fatalf("дефолтный конфиг дал предупреждение: %q", warn)
	}
	if !reflect.DeepEqual(again, Default()) {
		t.Fatalf("повторная загрузка разошлась с дефолтом: %+v", again)
	}
}

func TestLoadBrokenTOML(t *testing.T) {
	cfgPath, _ := testEnv(t)
	writeFile(t, cfgPath, "theme = [\n")

	cfg, err := Load()
	if err == nil {
		t.Fatal("битый TOML должен вернуть ошибку")
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Fatalf("битый TOML должен дать дефолты, got %+v", cfg)
	}

	cfg, warn := LoadOrDefault()
	if warn == "" || !strings.Contains(warn, "config") {
		t.Fatalf("ожидалось предупреждение, got %q", warn)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Fatalf("LoadOrDefault должен дать дефолты, got %+v", cfg)
	}

	// state.toml читается независимо от сломанного конфига.
	if err := SaveState(State{LastFile: "note.md"}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	if st, ok := LoadState(); !ok || st.LastFile != "note.md" {
		t.Fatalf("state не прочитался при битом конфиге: %+v, %v", st, ok)
	}
}

func TestLoadUnknownKeyWarns(t *testing.T) {
	cfgPath, _ := testEnv(t)
	writeFile(t, cfgPath, "theme = \"nord\"\n\n[editor]\ntabsize = 8\nbogus = 1\n")

	cfg, warn := LoadOrDefault()
	if !strings.Contains(warn, "неизвестный ключ") || !strings.Contains(warn, "editor.bogus") {
		t.Fatalf("нет предупреждения о неизвестном ключе: %q", warn)
	}
	if cfg.Theme != "nord" || cfg.Editor.Tabsize != 8 {
		t.Fatalf("известные ключи не применились: %+v", cfg)
	}
	if !cfg.Editor.Ghost || cfg.Markdown.Spell != "auto" {
		t.Fatalf("остальные значения должны быть дефолтными: %+v", cfg)
	}
}

func TestLoadUnknownTheme(t *testing.T) {
	cfgPath, _ := testEnv(t)
	writeFile(t, cfgPath, "theme = \"no-such-theme\"\n")

	cfg, warn := LoadOrDefault()
	if !strings.Contains(warn, "тема") || !strings.Contains(warn, "no-such-theme") {
		t.Fatalf("нет предупреждения о теме: %q", warn)
	}
	if cfg.Theme != "plain" {
		t.Fatalf("неизвестная тема должна стать plain, got %q", cfg.Theme)
	}
	if ThemeByName(cfg.Theme).Name != "plain" {
		t.Fatalf("ThemeByName(plain) не plain")
	}
}

func TestDefaultIgnoreDirs(t *testing.T) {
	want := []string{".git", ".jj", "node_modules", "target", ".direnv", "result*", "dist", "build", ".cache"}
	if got := Default().Ignore.Dirs; !reflect.DeepEqual(got, want) {
		t.Fatalf("Ignore.Dirs = %v, want %v", got, want)
	}
}

func TestEditorCareDefaults(t *testing.T) {
	e := Default().Editor
	if !e.AutoPairs {
		t.Error("autopairs должен быть включён по умолчанию")
	}
	if e.TrimTrailingWhitespace {
		t.Error("trim_trailing_whitespace должен быть выключен по умолчанию")
	}
	if !e.FinalNewline {
		t.Error("final_newline должен быть включён по умолчанию")
	}
}

func TestLanguage(t *testing.T) {
	if got := Default().Language; got != "en" {
		t.Fatalf("Default().Language = %q, want en", got)
	}

	cases := []struct{ in, want string }{
		{"", "en"},
		{"en", "en"},
		{"ru", "ru"},
		{" RU ", "ru"},
		{"de", "en"},
		{"russian", "en"},
	}
	for _, tc := range cases {
		c := Config{Language: tc.in}
		c.Normalize()
		if c.Language != tc.want {
			t.Errorf("Normalize(%q).Language = %q, want %q", tc.in, c.Language, tc.want)
		}
	}

	cfgPath, _ := testEnv(t)
	writeFile(t, cfgPath, "language = \"ru\"\n")
	cfg, warn := LoadOrDefault()
	if cfg.Language != "ru" {
		t.Errorf("Load language = %q, want ru", cfg.Language)
	}
	if strings.Contains(warn, "language") {
		t.Errorf("ru не должен давать предупреждение: %q", warn)
	}
}

func TestNormalize(t *testing.T) {
	c := Config{
		Theme:    " NORD ",
		Editor:   Editor{Tabsize: 0, GhostDelayMS: 10, ScrollMargin: -1, Softwrap: " sometimes "},
		Markdown: Markdown{Spell: "bogus"},
		Keys:     map[string]string{"Alt-t": "markdown.checkbox"},
	}
	c.Normalize()

	if c.Theme != "nord" {
		t.Errorf("тема: got %q", c.Theme)
	}
	if c.Editor.Tabsize != 4 {
		t.Errorf("tabsize: got %d, want 4", c.Editor.Tabsize)
	}
	if c.Editor.GhostDelayMS != 250 {
		t.Errorf("ghost_delay_ms: got %d, want 250", c.Editor.GhostDelayMS)
	}
	if c.Editor.ScrollMargin != 3 {
		t.Errorf("scrollmargin: got %d, want 3", c.Editor.ScrollMargin)
	}
	if c.Editor.Softwrap != "auto" || c.Markdown.Spell != "auto" {
		t.Errorf("choices: softwrap=%q spell=%q", c.Editor.Softwrap, c.Markdown.Spell)
	}
	if !reflect.DeepEqual(c.Keys, map[string]string{"Alt-t": "markdown.checkbox"}) {
		t.Errorf("Keys изменились: %v", c.Keys)
	}

	cases := []struct {
		tabsIn, delayIn, marginIn       int
		tabsWant, delayWant, marginWant int
	}{
		{1, 50, 0, 1, 50, 0},
		{16, 50, 100, 16, 50, 100},
		{17, 50, 0, 4, 50, 0},
		{-3, 49, -3, 4, 250, 3},
	}
	for _, tc := range cases {
		c := Config{Editor: Editor{Tabsize: tc.tabsIn, GhostDelayMS: tc.delayIn, ScrollMargin: tc.marginIn}}
		c.Normalize()
		if c.Editor.Tabsize != tc.tabsWant || c.Editor.GhostDelayMS != tc.delayWant || c.Editor.ScrollMargin != tc.marginWant {
			t.Errorf("in %+v: got tabsize=%d delay=%d margin=%d", tc, c.Editor.Tabsize, c.Editor.GhostDelayMS, c.Editor.ScrollMargin)
		}
	}
}

func TestSoftwrapFor(t *testing.T) {
	c := Default()
	for _, p := range []string{"notes.md", "NOTES.MD", "a.markdown", "a.txt", "a.rst", "a.org"} {
		if !c.SoftwrapFor(p) {
			t.Errorf("%s: проза должна переноситься", p)
		}
	}
	for _, p := range []string{"main.go", "conf.json", "Cargo.toml", "README", "x.yaml"} {
		if c.SoftwrapFor(p) {
			t.Errorf("%s: код/конфиг не должен переноситься", p)
		}
	}

	c.Editor.Softwrap = "on"
	if !c.SoftwrapFor("main.go") {
		t.Error("on должен включать перенос для кода")
	}
	c.Editor.Softwrap = "off"
	if c.SoftwrapFor("notes.md") {
		t.Error("off должен выключать перенос для прозы")
	}

	c.Editor.Softwrap = "auto"
	c.Files = map[string]string{"*.md": "code"}
	if c.SoftwrapFor("notes.md") {
		t.Error("[files] *.md = code не применился")
	}
	c.Files = map[string]string{"*.go": "prose"}
	if !c.SoftwrapFor("main.go") {
		t.Error("[files] *.go = prose не применился")
	}

	c.Files = map[string]string{"*.md": "code"}
	c.Editor.Softwrap = "on"
	if !c.SoftwrapFor("notes.md") {
		t.Error("on должен быть сильнее [files]")
	}
}

func TestSpellFor(t *testing.T) {
	c := Default()
	const ru = "Это довольно длинный русский текст для проверки орфографии."
	const en = "This is an English sentence used for spell checking."

	if got := c.SpellFor("notes.md", ru); got != "ltex" {
		t.Errorf("русский: got %q, want ltex", got)
	}
	if got := c.SpellFor("notes.md", en); got != "harper" {
		t.Errorf("английский: got %q, want harper", got)
	}
	if got := c.SpellFor("notes.md", ""); got != "harper" {
		t.Errorf("пустой sample: got %q, want harper", got)
	}
	if got := c.SpellFor("main.go", ru); got != "off" {
		t.Errorf("код: got %q, want off", got)
	}
	if got := c.SpellFor("conf.json", ru); got != "off" {
		t.Errorf("конфиг: got %q, want off", got)
	}

	c.Markdown.Spell = "off"
	if got := c.SpellFor("notes.md", ru); got != "off" {
		t.Errorf("явный off: got %q", got)
	}
	c.Markdown.Spell = "harper"
	if got := c.SpellFor("notes.md", ru); got != "harper" {
		t.Errorf("явный harper: got %q", got)
	}
	c.Markdown.Spell = "ltex"
	if got := c.SpellFor("notes.md", en); got != "ltex" {
		t.Errorf("явный ltex: got %q", got)
	}

	c.Markdown.Spell = "auto"
	c.Files = map[string]string{"*.md": "code"}
	if got := c.SpellFor("notes.md", ru); got != "off" {
		t.Errorf("[files] code должен отключать спелл: got %q", got)
	}
	c.Files = map[string]string{"*.go": "prose"}
	if got := c.SpellFor("main.go", ru); got != "ltex" {
		t.Errorf("[files] prose должен включать спелл: got %q", got)
	}
}

func TestServerFor(t *testing.T) {
	c := Default()
	if got := c.ServerFor("main.py"); got != "pyright-langserver --stdio" {
		t.Errorf("дефолтный python по filetype: %q", got)
	}
	if got := c.ServerFor("notes.md"); got != "ltex-ls" {
		t.Errorf("дефолтный markdown по filetype: %q", got)
	}
	if got := c.ServerFor("main.go"); got != "gopls" {
		t.Errorf("дефолтный go: %q", got)
	}

	c.LSP.Servers = map[string]string{
		"go":   "gopls -remote",
		".py":  "pyright-langserver --stdio",
		"*.rs": "rust-analyzer",
	}
	cases := []struct {
		path, want string
	}{
		{"main.go", "gopls -remote"},
		{"tools/x.py", "pyright-langserver --stdio"},
		{"src/lib.rs", "rust-analyzer"},
		{"app.ts", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := c.ServerFor(tc.path); got != tc.want {
			t.Errorf("ServerFor(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}

	c.LSP.Servers = map[string]string{"go": "filetype", "*.go": "glob"}
	if got := c.ServerFor("main.go"); got != "filetype" {
		t.Errorf("filetype должен быть точнее glob: %q", got)
	}

	// Filetype по имени языка: python → .py, javascript → .mjs/ts.
	c.LSP.Servers = map[string]string{
		"python":     "pyright",
		"javascript": "tsserver",
		"typescript": "tsserver",
	}
	for path, want := range map[string]string{
		"a.py":  "pyright",
		"a.js":  "tsserver",
		"a.mjs": "tsserver",
		"a.tsx": "tsserver",
		"a.rs":  "",
	} {
		if got := c.ServerFor(path); got != want {
			t.Errorf("ServerFor(%q) = %q, want %q", path, got, want)
		}
	}

	// Явное расширение точнее filetype.
	c.LSP.Servers = map[string]string{"python": "by-filetype", ".py": "by-ext"}
	if got := c.ServerFor("a.py"); got != "by-ext" {
		t.Errorf(".py должен быть точнее python: %q", got)
	}
}

func TestPaths(t *testing.T) {
	cfgDir := t.TempDir()
	stateDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
	t.Setenv("XDG_STATE_HOME", stateDir)

	cp, err := ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(cfgDir, "pero", "config.toml"); cp != want {
		t.Errorf("ConfigPath = %q, want %q", cp, want)
	}
	sp, err := StatePath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(stateDir, "pero", "state.toml"); sp != want {
		t.Errorf("StatePath = %q, want %q", sp, want)
	}

	t.Setenv("XDG_STATE_HOME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	sp, err = StatePath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".local", "state", "pero", "state.toml"); sp != want {
		t.Errorf("StatePath без XDG_STATE_HOME = %q, want %q", sp, want)
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s: права %o, want %o", path, got, want)
	}
}
