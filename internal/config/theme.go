package config

import "strings"

// Theme — цвета интерфейса и синтаксиса. Значения — hex (#RRGGBB) или номера
// ANSI, пригодные для lipgloss.
type Theme struct {
	Name              string
	BG                string
	Panel             string
	Border            string
	Text              string
	Muted             string
	Accent            string
	SelectionBG       string
	CurrentLine       string
	Gutter            string
	StatusBG          string
	StatusFG          string
	TabActive         string
	TabInactive       string
	Error             string
	Warning           string
	Info              string
	Ghost             string
	SearchBG          string
	SearchCurrent     string
	DiagnosticError   string
	DiagnosticWarning string
	DiagnosticInfo    string

	// Синтаксис.
	Keyword     string
	String      string
	Number      string
	Comment     string
	Function    string
	Type        string
	Operator    string
	Constant    string
	Builtin     string
	Punctuation string
}

// themeOrder — стабильный порядок: базовые, catppuccin, gruvbox, tokyonight,
// классические, ayu, kanagawa, rose-pine, github и прочие.
var themeOrder = []string{
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

var themes = map[string]Theme{
	"everforest": {
		Name: "everforest", BG: "#2d353b", Panel: "#343f44", Border: "#475258",
		Text: "#d3c6aa", Muted: "#859289", Accent: "#a7c080",
		SelectionBG: "#475258", CurrentLine: "#343f44", Gutter: "#5c6a72",
		StatusBG: "#232a2e", StatusFG: "#d3c6aa", TabActive: "#a7c080", TabInactive: "#859289",
		Error: "#e67e80", Warning: "#dbbc7f", Info: "#7fbbb3", Ghost: "#5c6a72",
		SearchBG: "#475258", SearchCurrent: "#dbbc7f",
		DiagnosticError: "#e67e80", DiagnosticWarning: "#dbbc7f", DiagnosticInfo: "#7fbbb3",
		Keyword: "#e67e80", String: "#a7c080", Number: "#d699b6", Comment: "#859289",
		Function: "#a7c080", Type: "#dbbc7f", Operator: "#e69875", Constant: "#d699b6",
		Builtin: "#e69875", Punctuation: "#859289",
	},
	"vesper": {
		Name: "vesper", BG: "#101010", Panel: "#161616", Border: "#2a2a2a",
		Text: "#ffffff", Muted: "#a0a0a0", Accent: "#ffc799",
		SelectionBG: "#2d2d2d", CurrentLine: "#1c1c1c", Gutter: "#5a5a5a",
		StatusBG: "#161616", StatusFG: "#ffffff", TabActive: "#ffc799", TabInactive: "#7a7a7a",
		Error: "#ff8080", Warning: "#ffc799", Info: "#99ffe4", Ghost: "#5a5a5a",
		SearchBG: "#2d2d2d", SearchCurrent: "#ffc799",
		DiagnosticError: "#ff8080", DiagnosticWarning: "#ffc799", DiagnosticInfo: "#99ffe4",
		Keyword: "#a0a0a0", String: "#ffc799", Number: "#99ffe4", Comment: "#7a7a7a",
		Function: "#ffc799", Type: "#99ffe4", Operator: "#a0a0a0", Constant: "#99ffe4",
		Builtin: "#ffc799", Punctuation: "#a0a0a0",
	},
	"nord": {
		Name: "nord", BG: "#2e3440", Panel: "#3b4252", Border: "#4c566a",
		Text: "#d8dee9", Muted: "#7b88a1", Accent: "#88c0d0",
		SelectionBG: "#434c5e", CurrentLine: "#3b4252", Gutter: "#4c566a",
		StatusBG: "#242933", StatusFG: "#eceff4", TabActive: "#88c0d0", TabInactive: "#7b88a1",
		Error: "#bf616a", Warning: "#ebcb8b", Info: "#81a1c1", Ghost: "#616e88",
		SearchBG: "#434c5e", SearchCurrent: "#ebcb8b",
		DiagnosticError: "#bf616a", DiagnosticWarning: "#ebcb8b", DiagnosticInfo: "#81a1c1",
		Keyword: "#81a1c1", String: "#a3be8c", Number: "#b48ead", Comment: "#616e88",
		Function: "#88c0d0", Type: "#8fbcbb", Operator: "#81a1c1", Constant: "#d08770",
		Builtin: "#5e81ac", Punctuation: "#d8dee9",
	},
	"plain": {
		Name: "plain", BG: "0", Panel: "0", Border: "8",
		Text: "15", Muted: "8", Accent: "6",
		SelectionBG: "8", CurrentLine: "0", Gutter: "8",
		StatusBG: "0", StatusFG: "15", TabActive: "15", TabInactive: "8",
		Error: "1", Warning: "3", Info: "6", Ghost: "8",
		SearchBG: "3", SearchCurrent: "2",
		DiagnosticError: "1", DiagnosticWarning: "3", DiagnosticInfo: "6",
		Keyword: "4", String: "2", Number: "5", Comment: "8",
		Function: "6", Type: "3", Operator: "7", Constant: "5",
		Builtin: "6", Punctuation: "8",
	},
	"catppuccin-mocha": {
		Name: "catppuccin-mocha", BG: "#1e1e2e", Panel: "#181825", Border: "#313244",
		Text: "#cdd6f4", Muted: "#a6adc8", Accent: "#cba6f7",
		SelectionBG: "#45475a", CurrentLine: "#313244", Gutter: "#6c7086",
		StatusBG: "#181825", StatusFG: "#cdd6f4", TabActive: "#cba6f7", TabInactive: "#6c7086",
		Error: "#f38ba8", Warning: "#f9e2af", Info: "#89b4fa", Ghost: "#6c7086",
		SearchBG: "#585b70", SearchCurrent: "#f9e2af",
		DiagnosticError: "#f38ba8", DiagnosticWarning: "#f9e2af", DiagnosticInfo: "#89b4fa",
		Keyword: "#cba6f7", String: "#a6e3a1", Number: "#fab387", Comment: "#6c7086",
		Function: "#89b4fa", Type: "#f9e2af", Operator: "#89dceb", Constant: "#fab387",
		Builtin: "#f38ba8", Punctuation: "#9399b2",
	},
	"catppuccin-macchiato": {
		Name: "catppuccin-macchiato", BG: "#24273a", Panel: "#1e2030", Border: "#363a4f",
		Text: "#cad3f5", Muted: "#a5adcb", Accent: "#c6a0f6",
		SelectionBG: "#494d64", CurrentLine: "#363a4f", Gutter: "#5b6078",
		StatusBG: "#1e2030", StatusFG: "#cad3f5", TabActive: "#c6a0f6", TabInactive: "#6e738d",
		Error: "#ed8796", Warning: "#eed49f", Info: "#8aadf4", Ghost: "#6e738d",
		SearchBG: "#5b6078", SearchCurrent: "#eed49f",
		DiagnosticError: "#ed8796", DiagnosticWarning: "#eed49f", DiagnosticInfo: "#8aadf4",
		Keyword: "#c6a0f6", String: "#a6da95", Number: "#f5a97f", Comment: "#6e738d",
		Function: "#8aadf4", Type: "#eed49f", Operator: "#91d7e3", Constant: "#f5a97f",
		Builtin: "#ed8796", Punctuation: "#939ab7",
	},
	"catppuccin-frappe": {
		Name: "catppuccin-frappe", BG: "#303446", Panel: "#292c3c", Border: "#414559",
		Text: "#c6d0f5", Muted: "#a5adce", Accent: "#ca9ee6",
		SelectionBG: "#51576d", CurrentLine: "#414559", Gutter: "#626880",
		StatusBG: "#292c3c", StatusFG: "#c6d0f5", TabActive: "#ca9ee6", TabInactive: "#737994",
		Error: "#e78284", Warning: "#e5c890", Info: "#8caaee", Ghost: "#737994",
		SearchBG: "#626880", SearchCurrent: "#e5c890",
		DiagnosticError: "#e78284", DiagnosticWarning: "#e5c890", DiagnosticInfo: "#8caaee",
		Keyword: "#ca9ee6", String: "#a6d189", Number: "#ef9f76", Comment: "#737994",
		Function: "#8caaee", Type: "#e5c890", Operator: "#99d1db", Constant: "#ef9f76",
		Builtin: "#e78284", Punctuation: "#949cbb",
	},
	"catppuccin-latte": {
		Name: "catppuccin-latte", BG: "#eff1f5", Panel: "#e6e9ef", Border: "#ccd0da",
		Text: "#4c4f69", Muted: "#6c6f85", Accent: "#8839ef",
		SelectionBG: "#bcc0cc", CurrentLine: "#e6e9ef", Gutter: "#9ca0b0",
		StatusBG: "#e6e9ef", StatusFG: "#4c4f69", TabActive: "#8839ef", TabInactive: "#9ca0b0",
		Error: "#d20f39", Warning: "#df8e1d", Info: "#1e66f5", Ghost: "#9ca0b0",
		SearchBG: "#bcc0cc", SearchCurrent: "#df8e1d",
		DiagnosticError: "#d20f39", DiagnosticWarning: "#df8e1d", DiagnosticInfo: "#1e66f5",
		Keyword: "#8839ef", String: "#40a02b", Number: "#fe640b", Comment: "#9ca0b0",
		Function: "#1e66f5", Type: "#df8e1d", Operator: "#04a5e5", Constant: "#fe640b",
		Builtin: "#d20f39", Punctuation: "#8c8fa1",
	},
	"gruvbox-dark": {
		Name: "gruvbox-dark", BG: "#282828", Panel: "#3c3836", Border: "#504945",
		Text: "#ebdbb2", Muted: "#928374", Accent: "#fabd2f",
		SelectionBG: "#504945", CurrentLine: "#32302f", Gutter: "#7c6f64",
		StatusBG: "#1d2021", StatusFG: "#ebdbb2", TabActive: "#fabd2f", TabInactive: "#928374",
		Error: "#fb4934", Warning: "#fabd2f", Info: "#83a598", Ghost: "#7c6f64",
		SearchBG: "#504945", SearchCurrent: "#fabd2f",
		DiagnosticError: "#fb4934", DiagnosticWarning: "#fabd2f", DiagnosticInfo: "#83a598",
		Keyword: "#fb4934", String: "#b8bb26", Number: "#d3869b", Comment: "#928374",
		Function: "#8ec07c", Type: "#fabd2f", Operator: "#fe8019", Constant: "#d3869b",
		Builtin: "#fabd2f", Punctuation: "#a89984",
	},
	"gruvbox-light": {
		Name: "gruvbox-light", BG: "#fbf1c7", Panel: "#ebdbb2", Border: "#d5c4a1",
		Text: "#3c3836", Muted: "#928374", Accent: "#d65d0e",
		SelectionBG: "#d5c4a1", CurrentLine: "#ebdbb2", Gutter: "#a89984",
		StatusBG: "#ebdbb2", StatusFG: "#3c3836", TabActive: "#d65d0e", TabInactive: "#928374",
		Error: "#9d0006", Warning: "#b57614", Info: "#076678", Ghost: "#a89984",
		SearchBG: "#d5c4a1", SearchCurrent: "#d79921",
		DiagnosticError: "#9d0006", DiagnosticWarning: "#b57614", DiagnosticInfo: "#076678",
		Keyword: "#9d0006", String: "#79740e", Number: "#8f3f71", Comment: "#928374",
		Function: "#427b58", Type: "#b57614", Operator: "#af3a03", Constant: "#8f3f71",
		Builtin: "#b57614", Punctuation: "#7c6f64",
	},
	"tokyonight-night": {
		Name: "tokyonight-night", BG: "#1a1b26", Panel: "#16161e", Border: "#292e42",
		Text: "#c0caf5", Muted: "#565f89", Accent: "#7aa2f7",
		SelectionBG: "#283457", CurrentLine: "#1e202e", Gutter: "#3b4261",
		StatusBG: "#16161e", StatusFG: "#c0caf5", TabActive: "#7aa2f7", TabInactive: "#565f89",
		Error: "#f7768e", Warning: "#e0af68", Info: "#7aa2f7", Ghost: "#565f89",
		SearchBG: "#283457", SearchCurrent: "#e0af68",
		DiagnosticError: "#f7768e", DiagnosticWarning: "#e0af68", DiagnosticInfo: "#7aa2f7",
		Keyword: "#bb9af7", String: "#9ece6a", Number: "#ff9e64", Comment: "#565f89",
		Function: "#7aa2f7", Type: "#2ac3de", Operator: "#89ddff", Constant: "#ff9e64",
		Builtin: "#f7768e", Punctuation: "#a9b1d6",
	},
	"tokyonight-storm": {
		Name: "tokyonight-storm", BG: "#24283b", Panel: "#1f2335", Border: "#292e42",
		Text: "#c0caf5", Muted: "#565f89", Accent: "#7aa2f7",
		SelectionBG: "#364a82", CurrentLine: "#292e42", Gutter: "#3b4261",
		StatusBG: "#1f2335", StatusFG: "#c0caf5", TabActive: "#7aa2f7", TabInactive: "#565f89",
		Error: "#f7768e", Warning: "#e0af68", Info: "#7aa2f7", Ghost: "#565f89",
		SearchBG: "#364a82", SearchCurrent: "#e0af68",
		DiagnosticError: "#f7768e", DiagnosticWarning: "#e0af68", DiagnosticInfo: "#7aa2f7",
		Keyword: "#bb9af7", String: "#9ece6a", Number: "#ff9e64", Comment: "#565f89",
		Function: "#7aa2f7", Type: "#2ac3de", Operator: "#89ddff", Constant: "#ff9e64",
		Builtin: "#f7768e", Punctuation: "#a9b1d6",
	},
	"tokyonight-moon": {
		Name: "tokyonight-moon", BG: "#222436", Panel: "#1e2030", Border: "#2f334d",
		Text: "#c8d3f5", Muted: "#636da6", Accent: "#82aaff",
		SelectionBG: "#2d3f76", CurrentLine: "#2f334d", Gutter: "#444a73",
		StatusBG: "#1e2030", StatusFG: "#c8d3f5", TabActive: "#82aaff", TabInactive: "#636da6",
		Error: "#ff757f", Warning: "#ffc777", Info: "#82aaff", Ghost: "#636da6",
		SearchBG: "#2d3f76", SearchCurrent: "#ffc777",
		DiagnosticError: "#ff757f", DiagnosticWarning: "#ffc777", DiagnosticInfo: "#82aaff",
		Keyword: "#c099ff", String: "#c3e88d", Number: "#ff966c", Comment: "#636da6",
		Function: "#82aaff", Type: "#ffc777", Operator: "#89ddff", Constant: "#ff966c",
		Builtin: "#ff757f", Punctuation: "#a9b8e8",
	},
	"tokyonight-day": {
		Name: "tokyonight-day", BG: "#e1e2e7", Panel: "#d0d5e3", Border: "#b4b9ca",
		Text: "#3760bf", Muted: "#848cb5", Accent: "#2e7de9",
		SelectionBG: "#b7c1e3", CurrentLine: "#d0d5e3", Gutter: "#a8aecb",
		StatusBG: "#d0d5e3", StatusFG: "#3760bf", TabActive: "#2e7de9", TabInactive: "#848cb5",
		Error: "#f52a65", Warning: "#8c6c3e", Info: "#2e7de9", Ghost: "#848cb5",
		SearchBG: "#b7c1e3", SearchCurrent: "#8c6c3e",
		DiagnosticError: "#f52a65", DiagnosticWarning: "#8c6c3e", DiagnosticInfo: "#2e7de9",
		Keyword: "#9854f1", String: "#587539", Number: "#b15c00", Comment: "#848cb5",
		Function: "#2e7de9", Type: "#007197", Operator: "#006a83", Constant: "#b15c00",
		Builtin: "#f52a65", Punctuation: "#6172b0",
	},
	"dracula": {
		Name: "dracula", BG: "#282a36", Panel: "#21222c", Border: "#44475a",
		Text: "#f8f8f2", Muted: "#6272a4", Accent: "#bd93f9",
		SelectionBG: "#44475a", CurrentLine: "#44475a", Gutter: "#6272a4",
		StatusBG: "#21222c", StatusFG: "#f8f8f2", TabActive: "#bd93f9", TabInactive: "#6272a4",
		Error: "#ff5555", Warning: "#f1fa8c", Info: "#8be9fd", Ghost: "#6272a4",
		SearchBG: "#44475a", SearchCurrent: "#f1fa8c",
		DiagnosticError: "#ff5555", DiagnosticWarning: "#f1fa8c", DiagnosticInfo: "#8be9fd",
		Keyword: "#ff79c6", String: "#50fa7b", Number: "#bd93f9", Comment: "#6272a4",
		Function: "#50fa7b", Type: "#8be9fd", Operator: "#ff79c6", Constant: "#bd93f9",
		Builtin: "#8be9fd", Punctuation: "#f8f8f2",
	},
	"solarized-dark": {
		Name: "solarized-dark", BG: "#002b36", Panel: "#073642", Border: "#586e75",
		Text: "#839496", Muted: "#586e75", Accent: "#268bd2",
		SelectionBG: "#073642", CurrentLine: "#073642", Gutter: "#586e75",
		StatusBG: "#073642", StatusFG: "#93a1a1", TabActive: "#268bd2", TabInactive: "#586e75",
		Error: "#dc322f", Warning: "#b58900", Info: "#268bd2", Ghost: "#586e75",
		SearchBG: "#073642", SearchCurrent: "#b58900",
		DiagnosticError: "#dc322f", DiagnosticWarning: "#b58900", DiagnosticInfo: "#268bd2",
		Keyword: "#859900", String: "#2aa198", Number: "#d33682", Comment: "#586e75",
		Function: "#268bd2", Type: "#b58900", Operator: "#859900", Constant: "#cb4b16",
		Builtin: "#b58900", Punctuation: "#839496",
	},
	"solarized-light": {
		Name: "solarized-light", BG: "#fdf6e3", Panel: "#eee8d5", Border: "#93a1a1",
		Text: "#657b83", Muted: "#93a1a1", Accent: "#268bd2",
		SelectionBG: "#eee8d5", CurrentLine: "#eee8d5", Gutter: "#93a1a1",
		StatusBG: "#eee8d5", StatusFG: "#586e75", TabActive: "#268bd2", TabInactive: "#93a1a1",
		Error: "#dc322f", Warning: "#b58900", Info: "#268bd2", Ghost: "#93a1a1",
		SearchBG: "#eee8d5", SearchCurrent: "#b58900",
		DiagnosticError: "#dc322f", DiagnosticWarning: "#b58900", DiagnosticInfo: "#268bd2",
		Keyword: "#859900", String: "#2aa198", Number: "#d33682", Comment: "#93a1a1",
		Function: "#268bd2", Type: "#b58900", Operator: "#859900", Constant: "#cb4b16",
		Builtin: "#b58900", Punctuation: "#657b83",
	},
	"one-dark": {
		Name: "one-dark", BG: "#282c34", Panel: "#21252b", Border: "#3e4451",
		Text: "#abb2bf", Muted: "#5c6370", Accent: "#61afef",
		SelectionBG: "#3e4451", CurrentLine: "#2c313a", Gutter: "#4b5263",
		StatusBG: "#21252b", StatusFG: "#abb2bf", TabActive: "#61afef", TabInactive: "#5c6370",
		Error: "#e06c75", Warning: "#e5c07b", Info: "#61afef", Ghost: "#5c6370",
		SearchBG: "#3e4451", SearchCurrent: "#e5c07b",
		DiagnosticError: "#e06c75", DiagnosticWarning: "#e5c07b", DiagnosticInfo: "#61afef",
		Keyword: "#c678dd", String: "#98c379", Number: "#d19a66", Comment: "#5c6370",
		Function: "#61afef", Type: "#e5c07b", Operator: "#56b6c2", Constant: "#d19a66",
		Builtin: "#e06c75", Punctuation: "#abb2bf",
	},
	"one-light": {
		Name: "one-light", BG: "#fafafa", Panel: "#f0f0f1", Border: "#d0d0d0",
		Text: "#383a42", Muted: "#a0a1a7", Accent: "#4078f2",
		SelectionBG: "#e5e5e6", CurrentLine: "#f0f0f1", Gutter: "#b0b0b0",
		StatusBG: "#f0f0f1", StatusFG: "#383a42", TabActive: "#4078f2", TabInactive: "#a0a1a7",
		Error: "#e45649", Warning: "#c18401", Info: "#4078f2", Ghost: "#a0a1a7",
		SearchBG: "#d0d0d0", SearchCurrent: "#c18401",
		DiagnosticError: "#e45649", DiagnosticWarning: "#c18401", DiagnosticInfo: "#4078f2",
		Keyword: "#a626a4", String: "#50a14f", Number: "#986801", Comment: "#a0a1a7",
		Function: "#4078f2", Type: "#c18401", Operator: "#0184bc", Constant: "#986801",
		Builtin: "#e45649", Punctuation: "#383a42",
	},
	"monokai": {
		Name: "monokai", BG: "#272822", Panel: "#1e1f1c", Border: "#3e3d32",
		Text: "#f8f8f2", Muted: "#75715e", Accent: "#a6e22e",
		SelectionBG: "#49483e", CurrentLine: "#3e3d32", Gutter: "#75715e",
		StatusBG: "#1e1f1c", StatusFG: "#f8f8f2", TabActive: "#a6e22e", TabInactive: "#75715e",
		Error: "#f92672", Warning: "#e6db74", Info: "#66d9ef", Ghost: "#75715e",
		SearchBG: "#49483e", SearchCurrent: "#e6db74",
		DiagnosticError: "#f92672", DiagnosticWarning: "#e6db74", DiagnosticInfo: "#66d9ef",
		Keyword: "#f92672", String: "#e6db74", Number: "#ae81ff", Comment: "#75715e",
		Function: "#a6e22e", Type: "#66d9ef", Operator: "#f92672", Constant: "#ae81ff",
		Builtin: "#66d9ef", Punctuation: "#f8f8f2",
	},
	"ayu-dark": {
		Name: "ayu-dark", BG: "#0a0e14", Panel: "#0d1017", Border: "#1f2430",
		Text: "#b3b1ad", Muted: "#565b66", Accent: "#e6b450",
		SelectionBG: "#1f2430", CurrentLine: "#131721", Gutter: "#3d424d",
		StatusBG: "#0d1017", StatusFG: "#b3b1ad", TabActive: "#e6b450", TabInactive: "#565b66",
		Error: "#f07178", Warning: "#e6b450", Info: "#59c2ff", Ghost: "#565b66",
		SearchBG: "#1f2430", SearchCurrent: "#e6b450",
		DiagnosticError: "#f07178", DiagnosticWarning: "#e6b450", DiagnosticInfo: "#59c2ff",
		Keyword: "#ff8f40", String: "#aad94c", Number: "#d2a6ff", Comment: "#626a73",
		Function: "#ffb454", Type: "#59c2ff", Operator: "#f29668", Constant: "#d2a6ff",
		Builtin: "#59c2ff", Punctuation: "#b3b1ad",
	},
	"ayu-mirage": {
		Name: "ayu-mirage", BG: "#1f2430", Panel: "#1a1f29", Border: "#2d3441",
		Text: "#cbccc6", Muted: "#707a8c", Accent: "#ffcc66",
		SelectionBG: "#2d3441", CurrentLine: "#262d3b", Gutter: "#3d4759",
		StatusBG: "#1a1f29", StatusFG: "#cbccc6", TabActive: "#ffcc66", TabInactive: "#707a8c",
		Error: "#f28779", Warning: "#ffcc66", Info: "#73d0ff", Ghost: "#707a8c",
		SearchBG: "#2d3441", SearchCurrent: "#ffcc66",
		DiagnosticError: "#f28779", DiagnosticWarning: "#ffcc66", DiagnosticInfo: "#73d0ff",
		Keyword: "#ffa759", String: "#bae67e", Number: "#d4bfff", Comment: "#5c6773",
		Function: "#ffd580", Type: "#73d0ff", Operator: "#f29e74", Constant: "#d4bfff",
		Builtin: "#73d0ff", Punctuation: "#cbccc6",
	},
	"ayu-light": {
		Name: "ayu-light", BG: "#fafafa", Panel: "#f0f0f0", Border: "#d8d8d8",
		Text: "#5c6166", Muted: "#8a9199", Accent: "#ff9940",
		SelectionBG: "#dbe3ef", CurrentLine: "#f0f0f0", Gutter: "#b8bcc2",
		StatusBG: "#f0f0f0", StatusFG: "#5c6166", TabActive: "#ff9940", TabInactive: "#8a9199",
		Error: "#f07171", Warning: "#f2ae49", Info: "#55b4d4", Ghost: "#8a9199",
		SearchBG: "#dbe3ef", SearchCurrent: "#f2ae49",
		DiagnosticError: "#f07171", DiagnosticWarning: "#f2ae49", DiagnosticInfo: "#55b4d4",
		Keyword: "#fa8d3e", String: "#86b300", Number: "#a37acc", Comment: "#8a9199",
		Function: "#f2ae49", Type: "#55b4d4", Operator: "#ed9366", Constant: "#a37acc",
		Builtin: "#55b4d4", Punctuation: "#5c6166",
	},
	"kanagawa-wave": {
		Name: "kanagawa-wave", BG: "#1f1f28", Panel: "#16161d", Border: "#2a2a37",
		Text: "#dcd7ba", Muted: "#727169", Accent: "#7e9cd8",
		SelectionBG: "#2d4f67", CurrentLine: "#2a2a37", Gutter: "#54546d",
		StatusBG: "#16161d", StatusFG: "#dcd7ba", TabActive: "#7e9cd8", TabInactive: "#727169",
		Error: "#e82424", Warning: "#e6c384", Info: "#7fb4ca", Ghost: "#54546d",
		SearchBG: "#2d4f67", SearchCurrent: "#e6c384",
		DiagnosticError: "#e82424", DiagnosticWarning: "#e6c384", DiagnosticInfo: "#7fb4ca",
		Keyword: "#957fb8", String: "#98bb6c", Number: "#d27e99", Comment: "#727169",
		Function: "#7e9cd8", Type: "#7aa89f", Operator: "#c0a36e", Constant: "#ffa066",
		Builtin: "#7fb4ca", Punctuation: "#9cabca",
	},
	"kanagawa-dragon": {
		Name: "kanagawa-dragon", BG: "#181616", Panel: "#12120f", Border: "#282727",
		Text: "#c5c9c5", Muted: "#737c73", Accent: "#8ba4b0",
		SelectionBG: "#223249", CurrentLine: "#1d1c19", Gutter: "#625e5a",
		StatusBG: "#12120f", StatusFG: "#c5c9c5", TabActive: "#8ba4b0", TabInactive: "#737c73",
		Error: "#c4746e", Warning: "#c4b28a", Info: "#8ba4b0", Ghost: "#625e5a",
		SearchBG: "#223249", SearchCurrent: "#c4b28a",
		DiagnosticError: "#c4746e", DiagnosticWarning: "#c4b28a", DiagnosticInfo: "#8ba4b0",
		Keyword: "#8992a7", String: "#87a987", Number: "#a292a3", Comment: "#737c73",
		Function: "#8ba4b0", Type: "#8ea4a2", Operator: "#c4746e", Constant: "#b98d7b",
		Builtin: "#a4b28e", Punctuation: "#9e9b93",
	},
	"kanagawa-lotus": {
		Name: "kanagawa-lotus", BG: "#f2ecbc", Panel: "#e7dba0", Border: "#d5cea3",
		Text: "#545464", Muted: "#8a8980", Accent: "#4d699b",
		SelectionBG: "#c9cbd1", CurrentLine: "#e7dba0", Gutter: "#b8b4a0",
		StatusBG: "#e7dba0", StatusFG: "#545464", TabActive: "#4d699b", TabInactive: "#8a8980",
		Error: "#c84053", Warning: "#de9800", Info: "#4d699b", Ghost: "#b8b4a0",
		SearchBG: "#c9cbd1", SearchCurrent: "#de9800",
		DiagnosticError: "#c84053", DiagnosticWarning: "#de9800", DiagnosticInfo: "#4d699b",
		Keyword: "#624c83", String: "#6f894e", Number: "#b35b79", Comment: "#8a8980",
		Function: "#4d699b", Type: "#597b75", Operator: "#cc6d00", Constant: "#b35b79",
		Builtin: "#4e8ca2", Punctuation: "#716e61",
	},
	"rose-pine": {
		Name: "rose-pine", BG: "#191724", Panel: "#1f1d2e", Border: "#26233a",
		Text: "#e0def4", Muted: "#6e6a86", Accent: "#c4a7e7",
		SelectionBG: "#26233a", CurrentLine: "#1f1d2e", Gutter: "#524f67",
		StatusBG: "#1f1d2e", StatusFG: "#e0def4", TabActive: "#c4a7e7", TabInactive: "#6e6a86",
		Error: "#eb6f92", Warning: "#f6c177", Info: "#9ccfd8", Ghost: "#524f67",
		SearchBG: "#403d52", SearchCurrent: "#f6c177",
		DiagnosticError: "#eb6f92", DiagnosticWarning: "#f6c177", DiagnosticInfo: "#9ccfd8",
		Keyword: "#c4a7e7", String: "#f6c177", Number: "#ebbcba", Comment: "#6e6a86",
		Function: "#ebbcba", Type: "#9ccfd8", Operator: "#908caa", Constant: "#f6c177",
		Builtin: "#31748f", Punctuation: "#908caa",
	},
	"rose-pine-moon": {
		Name: "rose-pine-moon", BG: "#232136", Panel: "#2a273f", Border: "#393552",
		Text: "#e0def4", Muted: "#6e6a86", Accent: "#c4a7e7",
		SelectionBG: "#44415a", CurrentLine: "#2a273f", Gutter: "#56526e",
		StatusBG: "#2a273f", StatusFG: "#e0def4", TabActive: "#c4a7e7", TabInactive: "#6e6a86",
		Error: "#eb6f92", Warning: "#f6c177", Info: "#9ccfd8", Ghost: "#56526e",
		SearchBG: "#44415a", SearchCurrent: "#f6c177",
		DiagnosticError: "#eb6f92", DiagnosticWarning: "#f6c177", DiagnosticInfo: "#9ccfd8",
		Keyword: "#c4a7e7", String: "#f6c177", Number: "#ea9a97", Comment: "#6e6a86",
		Function: "#ea9a97", Type: "#9ccfd8", Operator: "#908caa", Constant: "#f6c177",
		Builtin: "#3e8fb0", Punctuation: "#908caa",
	},
	"rose-pine-dawn": {
		Name: "rose-pine-dawn", BG: "#faf4ed", Panel: "#fffaf3", Border: "#dfdad9",
		Text: "#575279", Muted: "#9893a5", Accent: "#907aa9",
		SelectionBG: "#f2e9e1", CurrentLine: "#f4ede8", Gutter: "#cecacd",
		StatusBG: "#fffaf3", StatusFG: "#575279", TabActive: "#907aa9", TabInactive: "#9893a5",
		Error: "#b4637a", Warning: "#ea9d34", Info: "#56949f", Ghost: "#cecacd",
		SearchBG: "#f2e9e1", SearchCurrent: "#ea9d34",
		DiagnosticError: "#b4637a", DiagnosticWarning: "#ea9d34", DiagnosticInfo: "#56949f",
		Keyword: "#907aa9", String: "#ea9d34", Number: "#d7827e", Comment: "#9893a5",
		Function: "#d7827e", Type: "#56949f", Operator: "#797593", Constant: "#ea9d34",
		Builtin: "#286983", Punctuation: "#797593",
	},
	"github-dark": {
		Name: "github-dark", BG: "#0d1117", Panel: "#161b22", Border: "#30363d",
		Text: "#c9d1d9", Muted: "#8b949e", Accent: "#58a6ff",
		SelectionBG: "#264f78", CurrentLine: "#161b22", Gutter: "#484f58",
		StatusBG: "#161b22", StatusFG: "#c9d1d9", TabActive: "#58a6ff", TabInactive: "#8b949e",
		Error: "#f85149", Warning: "#d29922", Info: "#58a6ff", Ghost: "#6e7681",
		SearchBG: "#264f78", SearchCurrent: "#d29922",
		DiagnosticError: "#f85149", DiagnosticWarning: "#d29922", DiagnosticInfo: "#58a6ff",
		Keyword: "#ff7b72", String: "#a5d6ff", Number: "#79c0ff", Comment: "#8b949e",
		Function: "#d2a8ff", Type: "#ffa657", Operator: "#ff7b72", Constant: "#79c0ff",
		Builtin: "#ffa657", Punctuation: "#c9d1d9",
	},
	"github-light": {
		Name: "github-light", BG: "#ffffff", Panel: "#f6f8fa", Border: "#d0d7de",
		Text: "#24292f", Muted: "#57606a", Accent: "#0969da",
		SelectionBG: "#dbeafe", CurrentLine: "#f6f8fa", Gutter: "#8c959f",
		StatusBG: "#f6f8fa", StatusFG: "#24292f", TabActive: "#0969da", TabInactive: "#57606a",
		Error: "#cf222e", Warning: "#9a6700", Info: "#0969da", Ghost: "#8c959f",
		SearchBG: "#fff8c5", SearchCurrent: "#f2cc60",
		DiagnosticError: "#cf222e", DiagnosticWarning: "#9a6700", DiagnosticInfo: "#0969da",
		Keyword: "#cf222e", String: "#0a3069", Number: "#0550ae", Comment: "#6e7781",
		Function: "#8250df", Type: "#953800", Operator: "#cf222e", Constant: "#0550ae",
		Builtin: "#953800", Punctuation: "#24292f",
	},
	"zenburn": {
		Name: "zenburn", BG: "#3f3f3f", Panel: "#2b2b2b", Border: "#5f5f5f",
		Text: "#dcdccc", Muted: "#7f9f7f", Accent: "#f0dfaf",
		SelectionBG: "#5f5f5f", CurrentLine: "#4a4a4a", Gutter: "#6f6f6f",
		StatusBG: "#2b2b2b", StatusFG: "#dcdccc", TabActive: "#f0dfaf", TabInactive: "#7f9f7f",
		Error: "#cc9393", Warning: "#f0dfaf", Info: "#8cd0d3", Ghost: "#6f6f6f",
		SearchBG: "#5f5f5f", SearchCurrent: "#f0dfaf",
		DiagnosticError: "#cc9393", DiagnosticWarning: "#f0dfaf", DiagnosticInfo: "#8cd0d3",
		Keyword: "#f0dfaf", String: "#cc9393", Number: "#8cd0d3", Comment: "#7f9f7f",
		Function: "#efef8f", Type: "#dfdfbf", Operator: "#f0efd0", Constant: "#dca3a3",
		Builtin: "#8cd0d3", Punctuation: "#dcdccc",
	},
	"flexoki-dark": {
		Name: "flexoki-dark", BG: "#100f0f", Panel: "#1c1b1a", Border: "#282726",
		Text: "#cecdc3", Muted: "#878580", Accent: "#da702c",
		SelectionBG: "#282726", CurrentLine: "#1c1b1a", Gutter: "#575653",
		StatusBG: "#1c1b1a", StatusFG: "#cecdc3", TabActive: "#da702c", TabInactive: "#878580",
		Error: "#d14d41", Warning: "#d0a215", Info: "#3aa99f", Ghost: "#575653",
		SearchBG: "#403e3c", SearchCurrent: "#d0a215",
		DiagnosticError: "#d14d41", DiagnosticWarning: "#d0a215", DiagnosticInfo: "#3aa99f",
		Keyword: "#d14d41", String: "#879a39", Number: "#d0a215", Comment: "#6f6e69",
		Function: "#da702c", Type: "#d0a215", Operator: "#3aa99f", Constant: "#d0a215",
		Builtin: "#8b7ec8", Punctuation: "#878580",
	},
}

// ThemeByName возвращает тему по имени; неизвестное имя — plain.
func ThemeByName(name string) Theme {
	if t, ok := themes[strings.ToLower(strings.TrimSpace(name))]; ok {
		return t
	}
	return themes["plain"]
}

// ThemeNames — имена встроенных тем в стабильном порядке.
func ThemeNames() []string {
	return append([]string(nil), themeOrder...)
}

func knownTheme(name string) bool {
	_, ok := themes[name]
	return ok
}
