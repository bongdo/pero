// Package config читает пользовательский config.toml, хранит состояние сессии
// и отдаёт встроенные темы.
package config

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"
)

// DefaultTOML - дефолтный конфиг с комментариями; им же заполняется
// отсутствующий config.toml при первом запуске.
//
//go:embed default.toml
var DefaultTOML string

const (
	defaultTheme     = "everforest"
	defaultTabsize   = 4
	defaultGhostMS   = 250
	defaultMargin    = 3
	spellCyrillicMin = 0.15
)

// Config - пользовательский конфиг.
type Config struct {
	Theme    string            `toml:"theme"`
	Language string            `toml:"language"` // ru | en, пусто/неизвестно → en
	Editor   Editor            `toml:"editor"`
	Markdown Markdown          `toml:"markdown"`
	LSP      LSP               `toml:"lsp"`
	Ignore   Ignore            `toml:"ignore"`
	Keys     map[string]string `toml:"keys"`
	Files    map[string]string `toml:"files"`
}

// Editor - поведение набора и прокрутки.
type Editor struct {
	Tabsize                int    `toml:"tabsize"`
	Tabstospaces           bool   `toml:"tabstospaces"`
	Softwrap               string `toml:"softwrap"` // auto | on | off
	Ghost                  bool   `toml:"ghost"`
	GhostDelayMS           int    `toml:"ghost_delay_ms"`
	ScrollMargin           int    `toml:"scrollmargin"`
	RestoreSession         bool   `toml:"restore_session"`
	AutoPairs              bool   `toml:"autopairs"`
	TrimTrailingWhitespace bool   `toml:"trim_trailing_whitespace"`
	FinalNewline           bool   `toml:"final_newline"`
}

// Markdown - поведение для прозы и markdown.
type Markdown struct {
	Checkbox     bool   `toml:"checkbox"`
	ContinueList bool   `toml:"continue_list"`
	Spell        string `toml:"spell"` // auto | ltex | harper | off
}

// LSP - команды серверов: filetype/расширение/glob → командная строка.
type LSP struct {
	Servers map[string]string `toml:"servers"`
}

// Ignore - каталоги, которые обходит поиск файлов.
type Ignore struct {
	Dirs []string `toml:"dirs"`
}

// Default возвращает встроенные настройки.
func Default() Config {
	return Config{
		Theme:    defaultTheme,
		Language: "en",
		Editor: Editor{
			Tabsize:                defaultTabsize,
			Tabstospaces:           true,
			Softwrap:               "auto",
			Ghost:                  true,
			GhostDelayMS:           defaultGhostMS,
			ScrollMargin:           defaultMargin,
			RestoreSession:         true,
			AutoPairs:              true,
			TrimTrailingWhitespace: false,
			FinalNewline:           true,
		},
		Markdown: Markdown{Checkbox: true, ContinueList: true, Spell: "auto"},
		LSP: LSP{Servers: map[string]string{
			"go":       "gopls",
			"markdown": "ltex-ls",
			"python":   "pyright-langserver --stdio",
		}},
		Ignore: Ignore{Dirs: []string{
			".git", ".jj", "node_modules", "target", ".direnv",
			"result*", "dist", "build", ".cache",
		}},
		Keys: map[string]string{
			"Alt-t": "markdown.checkbox",
			"Alt-.": "lsp.codeaction",
		},
	}
}

// Normalize приводит значения к допустимым. Биндинги не трогает.
func (c *Config) Normalize() {
	t := strings.ToLower(strings.TrimSpace(c.Theme))
	switch {
	case t == "":
		c.Theme = defaultTheme
	case knownTheme(t):
		c.Theme = t
	default:
		c.Theme = "plain"
	}

	e := &c.Editor
	if e.Tabsize < 1 || e.Tabsize > 16 {
		e.Tabsize = defaultTabsize
	}
	if e.GhostDelayMS < 50 {
		e.GhostDelayMS = defaultGhostMS
	}
	if e.ScrollMargin < 0 {
		e.ScrollMargin = defaultMargin
	}
	e.Softwrap = choice(e.Softwrap, "auto", "auto", "on", "off")
	c.Markdown.Spell = choice(c.Markdown.Spell, "auto", "auto", "ltex", "harper", "off")
	c.Language = choice(c.Language, "en", "en", "ru")
}

func choice(v, fallback string, allowed ...string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	for _, a := range allowed {
		if v == a {
			return v
		}
	}
	return fallback
}

// ConfigPath - $XDG_CONFIG_HOME/pero/config.toml или ~/.config/pero/config.toml.
func ConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pero", "config.toml"), nil
}

// StatePath - $XDG_STATE_HOME/pero/state.toml или
// ~/.local/state/pero/state.toml.
func StatePath() (string, error) {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "pero", "state.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "pero", "state.toml"), nil
}

// Load читает конфиг. Отсутствующий файл создаётся из DefaultTOML; битый файл
// даёт ошибку и дефолты.
func Load() (Config, error) {
	cfg, _, err := load()
	return cfg, err
}

// LoadOrDefault читает конфиг и не падает: при ошибке возвращает дефолты и
// текст предупреждения. Неизвестные ключи и тема тоже попадают в предупреждение.
func LoadOrDefault() (Config, string) {
	cfg, warns, err := load()
	if err != nil {
		return cfg, err.Error()
	}
	return cfg, strings.Join(warns, "; ")
}

func load() (Config, []string, error) {
	cfg := Default()
	path, err := ConfigPath()
	if err != nil {
		return cfg, nil, fmt.Errorf("config: %w", err)
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return cfg, nil, fmt.Errorf("config: %w", err)
		}
		if err := os.WriteFile(path, []byte(DefaultTOML), 0o644); err != nil {
			return cfg, nil, fmt.Errorf("config: %w", err)
		}
		return cfg, nil, nil
	}
	if err != nil {
		return cfg, nil, fmt.Errorf("config: %s: %w", path, err)
	}

	// Карты из файла заменяют встроенные, а не сливаются с ними.
	cfg.LSP.Servers = nil
	cfg.Keys = nil
	cfg.Files = nil
	md, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return Default(), nil, fmt.Errorf("config: %s: %w", path, err)
	}

	var warns []string
	for _, k := range md.Undecoded() {
		warns = append(warns, "config: неизвестный ключ "+k.String())
	}

	rawTheme := strings.ToLower(strings.TrimSpace(cfg.Theme))
	cfg.Normalize()
	if rawTheme != "" && !knownTheme(rawTheme) {
		warns = append(warns, fmt.Sprintf("config: неизвестная тема %q, используется %q", rawTheme, cfg.Theme))
	}

	def := Default()
	if cfg.LSP.Servers == nil {
		cfg.LSP.Servers = def.LSP.Servers
	}
	if cfg.Keys == nil {
		cfg.Keys = def.Keys
	}
	return cfg, warns, nil
}

// SoftwrapFor - включён ли перенос для файла. on/off сильнее авторежима.
func (c Config) SoftwrapFor(path string) bool {
	switch strings.ToLower(strings.TrimSpace(c.Editor.Softwrap)) {
	case "on":
		return true
	case "off":
		return false
	}
	return c.fileMode(path) == "prose"
}

// SpellFor - какой сервер проверки орфографии поднимать. Для кода - off.
func (c Config) SpellFor(path, sample string) string {
	if c.fileMode(path) != "prose" {
		return "off"
	}
	switch strings.ToLower(strings.TrimSpace(c.Markdown.Spell)) {
	case "off":
		return "off"
	case "ltex":
		return "ltex"
	case "harper":
		return "harper"
	}
	if cyrillicShare(sample) >= spellCyrillicMin {
		return "ltex"
	}
	return "harper"
}

// ServerFor - команда LSP для файла или пустая строка.
func (c Config) ServerFor(path string) string {
	if v, ok := lookupByPath(c.LSP.Servers, path); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// fileMode - режим файла: prose или code, с учётом [files].
func (c Config) fileMode(path string) string {
	if v, ok := lookupByPath(c.Files, path); ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "prose":
			return "prose"
		case "code":
			return "code"
		}
	}
	if isProsePath(path) {
		return "prose"
	}
	return "code"
}

var proseExt = map[string]bool{
	".md": true, ".markdown": true, ".txt": true, ".rst": true, ".org": true,
}

// filetypeExts - filetype-ключи из [lsp.servers] и их расширения. Точное имя
// файла и явное расширение (.py) всё равно точнее filetype.
var filetypeExts = map[string][]string{
	"markdown":   {".md", ".markdown", ".mdown", ".mkd", ".mdx"},
	"python":     {".py"},
	"javascript": {".js", ".mjs", ".cjs", ".jsx"},
	"typescript": {".ts", ".tsx"},
	"rust":       {".rs"},
	"lua":        {".lua"},
	"nix":        {".nix"},
	"nim":        {".nim"},
	"c":          {".c", ".h"},
	"cpp":        {".cc", ".cpp", ".cxx", ".hpp", ".hh"},
	"bash":       {".sh", ".bash", ".zsh"},
	"shell":      {".sh", ".bash", ".zsh"},
	"json":       {".json"},
	"toml":       {".toml"},
	"yaml":       {".yaml", ".yml"},
	"html":       {".html", ".htm"},
	"css":        {".css"},
}

func isProsePath(path string) bool {
	if path == "" {
		return false
	}
	return proseExt[strings.ToLower(filepath.Ext(path))]
}

// lookupByPath ищет самое точное совпадение ключа: имя файла, расширение,
// filetype, затем glob. При равной точности ключи сравниваются, чтобы результат
// не зависел от порядка обхода карты.
func lookupByPath(m map[string]string, path string) (string, bool) {
	best, bestScore := "", 0
	for k := range m {
		s := matchScore(k, path)
		if s > bestScore || (s == bestScore && s > 0 && k < best) {
			best, bestScore = k, s
		}
	}
	if bestScore == 0 {
		return "", false
	}
	return m[best], true
}

func matchScore(pattern, path string) int {
	p := strings.ToLower(strings.TrimSpace(pattern))
	if p == "" || path == "" {
		return 0
	}
	base := strings.ToLower(filepath.Base(path))
	full := strings.ToLower(filepath.ToSlash(path))
	ext := strings.ToLower(filepath.Ext(base))

	if p == base {
		return 4
	}
	if strings.HasPrefix(p, ".") {
		if strings.HasSuffix(base, p) {
			return 3
		}
		return 0
	}
	if strings.ContainsAny(p, "*?[") {
		if ok, _ := filepath.Match(p, base); ok {
			return 1
		}
		if ok, _ := filepath.Match(p, full); ok {
			return 1
		}
		return 0
	}
	if p == strings.TrimPrefix(ext, ".") {
		return 2
	}
	for _, fileExt := range filetypeExts[p] {
		if fileExt == ext {
			return 2
		}
	}
	return 0
}

// cyrillicShare - доля кириллических букв среди букв (0..1).
func cyrillicShare(s string) float64 {
	letters, cyr := 0, 0
	for _, r := range s {
		if !unicode.IsLetter(r) {
			continue
		}
		letters++
		if isCyrillic(r) {
			cyr++
		}
	}
	if letters == 0 {
		return 0
	}
	return float64(cyr) / float64(letters)
}

func isCyrillic(r rune) bool {
	return (r >= 0x0400 && r <= 0x052F) ||
		(r >= 0x1C80 && r <= 0x1C8F) ||
		(r >= 0x2DE0 && r <= 0x2DFF) ||
		(r >= 0xA640 && r <= 0xA69F)
}
