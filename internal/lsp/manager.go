package lsp

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// NewManager creates a lazy language-server manager for one project root.
// servers maps language IDs, extensions or globs to command lines, e.g.
// {"go": "gopls", "*.py": "pyright-langserver --stdio"}.
func NewManager(root string, servers map[string]string) *Manager {
	copied := make(map[string]string, len(servers))
	for key, command := range servers {
		copied[key] = command
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		root:    root,
		servers: copied,
		entries: make(map[string]*managerEntry),
		ctx:     ctx,
		cancel:  cancel,
	}
}

type managerEntry struct {
	ready  chan struct{}
	client *Client
	err    error
}

type Manager struct {
	root    string
	servers map[string]string

	ctx    context.Context
	cancel context.CancelFunc

	mu       sync.Mutex
	closed   bool
	entries  map[string]*managerEntry
	settings map[string]any
}

// ClientFor returns (starting it on first use) the client for the given file.
// languageID may be empty; then it is inferred from the file extension.
func (m *Manager) ClientFor(filePath, languageID string) (*Client, error) {
	cfg, err := m.configFor(filePath, languageID)
	if err != nil {
		return nil, err
	}
	key := clientKey(cfg)

	for {
		m.mu.Lock()
		if m.closed {
			m.mu.Unlock()
			return nil, ErrClosed
		}
		if entry, ok := m.entries[key]; ok {
			m.mu.Unlock()
			<-entry.ready
			if entry.err != nil {
				return nil, entry.err
			}
			if entry.client != nil && entry.client.alive() {
				return entry.client, nil
			}
			// Drop a crashed/failed entry and try again from scratch.
			m.mu.Lock()
			if m.entries[key] == entry {
				delete(m.entries, key)
			}
			m.mu.Unlock()
			continue
		}
		entry := &managerEntry{ready: make(chan struct{})}
		m.entries[key] = entry
		m.mu.Unlock()

		client, startErr := m.start(cfg)

		m.mu.Lock()
		if m.closed {
			if client != nil {
				client.Close()
			}
			client = nil
			startErr = ErrClosed
		}
		entry.client = client
		entry.err = startErr
		if startErr != nil {
			delete(m.entries, key)
		}
		m.mu.Unlock()
		close(entry.ready)

		return client, startErr
	}
}

func (m *Manager) start(cfg ServerConfig) (*Client, error) {
	m.mu.Lock()
	settings := m.settings
	m.mu.Unlock()

	client := NewClient(cfg, m.root)
	client.SetConfiguration(settings)
	if err := client.Start(m.ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("lsp: start %q: %w", cfg.Command, err)
	}
	if err := client.Initialize(m.ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("lsp: initialize %q: %w", cfg.Command, err)
	}
	return client, nil
}

// SetConfiguration sets workspace/configuration values for clients started
// after this call. Call it before ClientFor; running clients keep their
// settings. Use sections like {"ltex": {"language": "ru-RU"}}.
func (m *Manager) SetConfiguration(config map[string]any) {
	copied := cloneSettings(config)
	m.mu.Lock()
	m.settings = copied
	m.mu.Unlock()
}

// Close stops every client started by the manager. ClientFor fails afterwards.
func (m *Manager) Close() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	entries := m.entries
	m.entries = make(map[string]*managerEntry)
	m.cancel()
	m.mu.Unlock()

	for _, entry := range entries {
		select {
		case <-entry.ready:
			if entry.client != nil {
				entry.client.Close()
			}
		default:
			// Still starting: ClientFor sees m.closed and closes it itself.
		}
	}
}

func clientKey(cfg ServerConfig) string {
	parts := append([]string{cfg.Command}, cfg.Args...)
	return cfg.LanguageID + "\x00" + strings.Join(parts, "\x00")
}

func (m *Manager) configFor(filePath, languageID string) (ServerConfig, error) {
	inferred := languageIDForPath(filePath)
	lang := strings.ToLower(strings.TrimSpace(languageID))
	if lang == "" {
		lang = inferred
	}
	pattern, command, ok := bestServer(m.servers, filePath, lang, inferred, m.root)
	if !ok {
		return ServerConfig{}, noServerError(filePath, lang, inferred)
	}
	cfg := ServerConfigFromCommand(lang, command)
	if cfg.LanguageID == "" {
		cfg.LanguageID = inferred
	}
	if cfg.LanguageID == "" {
		cfg.LanguageID = patternLanguage(pattern)
	}
	if cfg.LanguageID == "" {
		cfg.LanguageID = "plaintext"
	}
	if ext := strings.ToLower(filepath.Ext(filePath)); ext != "" {
		cfg.Extensions = []string{ext}
	}
	return cfg, nil
}

func noServerError(filePath, lang, inferred string) error {
	name := filepath.Base(filePath)
	if name == "" || name == "." {
		name = filePath
	}
	ext := strings.ToLower(filepath.Ext(filePath))
	detail := "no extension"
	if ext != "" {
		detail = ext
	}
	if inferred != "" {
		detail += ", language " + inferred
	}
	if lang != "" && lang != inferred {
		detail += ", requested " + lang
	}
	return fmt.Errorf("lsp: no server configured for %q (%s)", name, detail)
}

// ServerConfigFromCommand parses a command line with strings.Fields. Quoting is
// intentionally not supported; config values are simple "server --flag" lines.
func ServerConfigFromCommand(languageID, command string) ServerConfig {
	fields := strings.Fields(command)
	cfg := ServerConfig{LanguageID: languageID}
	if len(fields) == 0 {
		return cfg
	}
	cfg.Command = fields[0]
	cfg.Args = append([]string(nil), fields[1:]...)
	// harper-ls without --stdio listens on TCP, which an editor speaking over
	// pipes can never reach; stdio is the only useful mode here.
	if filepath.Base(cfg.Command) == "harper-ls" && !hasArg(cfg.Args, "--stdio") && !hasArg(cfg.Args, "-s") {
		cfg.Args = append(cfg.Args, "--stdio")
	}
	return cfg
}

func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func bestServer(servers map[string]string, filePath, lang, inferred, root string) (pattern, command string, ok bool) {
	base := filepath.Base(filePath)
	full := filepath.ToSlash(filePath)
	rel := ""
	if root != "" {
		if value, err := filepath.Rel(root, filePath); err == nil && value != "." &&
			!strings.HasPrefix(value, ".."+string(filepath.Separator)) && value != ".." {
			rel = filepath.ToSlash(value)
		}
	}
	ext := strings.ToLower(filepath.Ext(filePath))

	bestScore := -1
	for pat, cmd := range servers {
		pat = strings.TrimSpace(pat)
		if pat == "" || strings.TrimSpace(cmd) == "" {
			continue
		}
		score := matchScore(pat, base, full, rel, ext, lang, inferred)
		if score < 0 {
			continue
		}
		if score > bestScore || (score == bestScore && (len(pat) > len(pattern) || (len(pat) == len(pattern) && pat < pattern))) {
			bestScore = score
			pattern = pat
			command = cmd
			ok = true
		}
	}
	return pattern, command, ok
}

func matchScore(pattern, base, full, rel, ext, lang, inferred string) int {
	specificity := len(pattern)
	if lang != "" && strings.EqualFold(pattern, lang) {
		return 400 + specificity
	}
	if inferred != "" && strings.EqualFold(pattern, inferred) {
		return 380 + specificity
	}
	for _, alias := range languageAliases(inferred) {
		if strings.EqualFold(pattern, alias) {
			return 370 + specificity
		}
	}
	if !hasGlob(pattern) && strings.EqualFold(pattern, base) {
		return 360 + specificity
	}
	if strings.HasPrefix(pattern, ".") && !hasGlob(pattern) && strings.EqualFold(pattern, ext) {
		return 300 + specificity
	}
	if !hasGlob(pattern) && ext != "" && strings.EqualFold(pattern, strings.TrimPrefix(ext, ".")) {
		return 290 + specificity
	}
	if hasGlob(pattern) {
		if globMatch(pattern, base) {
			return 220 + specificity
		}
		if strings.ContainsAny(pattern, `/\`) {
			if rel != "" && globMatch(pattern, rel) {
				return 220 + specificity
			}
			if globMatch(pattern, full) {
				return 210 + specificity
			}
		}
	}
	return -1
}

func hasGlob(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}

// globMatch supports filepath.Match per segment and ** meaning any number of
// path segments (including none). A pattern without a slash is matched against
// a single file name by callers, but works here too.
func globMatch(pattern, target string) bool {
	return matchSegments(strings.Split(filepath.ToSlash(pattern), "/"),
		strings.Split(filepath.ToSlash(target), "/"))
}

func matchSegments(pattern, target []string) bool {
	if len(pattern) == 0 {
		return len(target) == 0
	}
	if pattern[0] == "**" {
		if matchSegments(pattern[1:], target) {
			return true
		}
		return len(target) > 0 && matchSegments(pattern, target[1:])
	}
	if len(target) == 0 {
		return false
	}
	matched, err := path.Match(pattern[0], target[0])
	if err != nil || !matched {
		return false
	}
	return matchSegments(pattern[1:], target[1:])
}

func patternLanguage(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || hasGlob(pattern) || strings.ContainsAny(pattern, `/\`) || strings.Contains(pattern, ".") {
		return ""
	}
	return strings.ToLower(pattern)
}

func languageIDForPath(filePath string) string {
	base := strings.ToLower(filepath.Base(filePath))
	switch base {
	case "makefile", "gnumakefile":
		return "makefile"
	case "dockerfile", "containerfile":
		return "dockerfile"
	case "cmakelists.txt":
		return "cmake"
	case "go.mod":
		return "gomod"
	case "go.work":
		return "gowork"
	}
	return extensionLanguages[strings.ToLower(filepath.Ext(filePath))]
}

func languageAliases(languageID string) []string {
	switch languageID {
	case "typescriptreact":
		return []string{"typescript", "javascript"}
	case "javascriptreact":
		return []string{"javascript", "typescript"}
	default:
		return nil
	}
}

var extensionLanguages = map[string]string{
	".go": "go", ".py": "python", ".pyi": "python", ".pyw": "python",
	".ts": "typescript", ".tsx": "typescriptreact", ".mts": "typescript", ".cts": "typescript",
	".js": "javascript", ".jsx": "javascriptreact", ".mjs": "javascript", ".cjs": "javascript",
	".c": "c", ".h": "c", ".cc": "cpp", ".cpp": "cpp", ".cxx": "cpp", ".hpp": "cpp", ".hh": "cpp", ".hxx": "cpp",
	".lua": "lua", ".nix": "nix", ".nim": "nim", ".nims": "nim",
	".md": "markdown", ".markdown": "markdown", ".mdown": "markdown", ".txt": "plaintext", ".rst": "restructuredtext", ".org": "org",
	".tex": "latex", ".json": "json", ".jsonc": "jsonc", ".toml": "toml", ".yaml": "yaml", ".yml": "yaml",
	".sh": "shellscript", ".bash": "shellscript", ".zsh": "shellscript",
	".rs": "rust", ".mojo": "mojo", ".html": "html", ".htm": "html", ".css": "css", ".scss": "scss",
	".sql": "sql", ".java": "java", ".kt": "kotlin", ".kts": "kotlin", ".rb": "ruby", ".php": "php",
	".cs": "csharp", ".swift": "swift", ".zig": "zig", ".hs": "haskell", ".ex": "elixir", ".exs": "elixir",
	".erl": "erlang", ".ml": "ocaml", ".mli": "ocaml", ".pl": "perl", ".r": "r", ".vue": "vue", ".svelte": "svelte",
}

// Preset returns a ready ServerConfig for a known language server. The name is
// usually the executable name ("gopls", "clangd", "ltex-ls"); common language
// aliases ("go", "python", "markdown" for harper) also work.
func Preset(name string) (ServerConfig, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "gopls", "go":
		return cloneConfig(ServerConfig{Command: "gopls", LanguageID: "go", Extensions: []string{".go"}}), true
	case "pyright", "pyright-langserver", "python":
		return cloneConfig(ServerConfig{Command: "pyright-langserver", Args: []string{"--stdio"}, LanguageID: "python", Extensions: []string{".py", ".pyi"}}), true
	case "typescript-language-server", "typescript", "tsserver":
		return cloneConfig(ServerConfig{Command: "typescript-language-server", Args: []string{"--stdio"}, LanguageID: "typescript", Extensions: []string{".ts", ".tsx", ".js", ".jsx"}}), true
	case "clangd", "c", "cpp":
		return cloneConfig(ServerConfig{Command: "clangd", LanguageID: "c", Extensions: []string{".c", ".h", ".cc", ".cpp", ".cxx", ".hpp"}}), true
	case "lua-language-server", "lua":
		return cloneConfig(ServerConfig{Command: "lua-language-server", LanguageID: "lua", Extensions: []string{".lua"}}), true
	case "nixd", "nix":
		return cloneConfig(ServerConfig{Command: "nixd", LanguageID: "nix", Extensions: []string{".nix"}}), true
	case "nimony-lsp", "nim":
		return cloneConfig(ServerConfig{Command: "nimony-lsp", LanguageID: "nim", Extensions: []string{".nim", ".nims"}}), true
	case "ltex-ls", "ltex":
		return cloneConfig(ServerConfig{Command: "ltex-ls", LanguageID: "markdown", Extensions: []string{".md", ".markdown", ".txt", ".tex"}}), true
	case "harper-ls", "harper":
		return cloneConfig(ServerConfig{Command: "harper-ls", Args: []string{"--stdio"}, LanguageID: "markdown", Extensions: []string{".md", ".markdown", ".txt"}}), true
	default:
		return ServerConfig{}, false
	}
}

// PresetNames lists canonical preset keys in sorted order.
func PresetNames() []string {
	names := []string{
		"clangd", "gopls", "harper-ls", "ltex-ls", "lua-language-server",
		"nimony-lsp", "nixd", "pyright-langserver", "typescript-language-server",
	}
	sort.Strings(names)
	return names
}

// DefaultServers is a ready map for NewManager. Markdown/txt default to harper;
// users who need ltex should put "*.md": "ltex-ls" in their config.
func DefaultServers() map[string]string {
	return map[string]string{
		"go":              "gopls",
		"python":          "pyright-langserver --stdio",
		"typescript":      "typescript-language-server --stdio",
		"typescriptreact": "typescript-language-server --stdio",
		"javascript":      "typescript-language-server --stdio",
		"javascriptreact": "typescript-language-server --stdio",
		"c":               "clangd",
		"cpp":             "clangd",
		"lua":             "lua-language-server",
		"nix":             "nixd",
		"nim":             "nimony-lsp",
		"markdown":        "harper-ls --stdio",
		"plaintext":       "harper-ls --stdio",
		"latex":           "ltex-ls",
		"harper":          "harper-ls --stdio",
		"ltex":            "ltex-ls",
	}
}

func cloneConfig(cfg ServerConfig) ServerConfig {
	cfg.Args = append([]string(nil), cfg.Args...)
	cfg.Extensions = append([]string(nil), cfg.Extensions...)
	if cfg.InitOptions != nil {
		options := make(map[string]any, len(cfg.InitOptions))
		for key, value := range cfg.InitOptions {
			options[key] = value
		}
		cfg.InitOptions = options
	}
	return cfg
}
