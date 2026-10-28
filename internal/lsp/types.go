package lsp

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// Position is a UTF-16 based LSP position.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Diagnostic struct {
	Range    Range           `json:"range"`
	Severity int             `json:"severity,omitempty"`
	Message  string          `json:"message"`
	Source   string          `json:"source,omitempty"`
	Code     json.RawMessage `json:"code,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
}

type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// CompletionItem is a normalized completion item. Kind is always human readable
// ("func", "type", "method", ...), even if the server sent a numeric kind.
type CompletionItem struct {
	Label         string    `json:"label"`
	Detail        string    `json:"detail,omitempty"`
	Kind          string    `json:"kind,omitempty"`
	InsertText    string    `json:"insertText,omitempty"`
	TextEdit      *TextEdit `json:"textEdit,omitempty"`
	Documentation string    `json:"documentation,omitempty"`
}

func (c *CompletionItem) UnmarshalJSON(data []byte) error {
	var wire struct {
		Label         string          `json:"label"`
		Detail        string          `json:"detail"`
		Kind          json.RawMessage `json:"kind"`
		InsertText    string          `json:"insertText"`
		TextEdit      json.RawMessage `json:"textEdit"`
		Documentation json.RawMessage `json:"documentation"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	c.Label = wire.Label
	c.Detail = wire.Detail
	c.Kind = normalizeCompletionKind(wire.Kind)
	c.InsertText = wire.InsertText
	c.TextEdit = decodeTextEdit(wire.TextEdit)
	c.Documentation = decodeMarkup(wire.Documentation)
	if c.InsertText == "" {
		if c.TextEdit != nil {
			c.InsertText = c.TextEdit.NewText
		} else {
			c.InsertText = c.Label
		}
	}
	return nil
}

type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

func (l *Location) UnmarshalJSON(data []byte) error {
	var wire struct {
		URI                  string `json:"uri"`
		Range                Range  `json:"range"`
		TargetURI            string `json:"targetUri"`
		TargetRange          Range  `json:"targetRange"`
		TargetSelectionRange Range  `json:"targetSelectionRange"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	l.URI = wire.URI
	l.Range = wire.Range
	if l.URI == "" {
		l.URI = wire.TargetURI
	}
	if l.Range == (Range{}) {
		if wire.TargetSelectionRange != (Range{}) {
			l.Range = wire.TargetSelectionRange
		} else {
			l.Range = wire.TargetRange
		}
	}
	return nil
}

type Hover struct {
	Contents string `json:"contents"`
	Range    *Range `json:"range,omitempty"`
}

func (h *Hover) UnmarshalJSON(data []byte) error {
	var wire struct {
		Contents json.RawMessage `json:"contents"`
		Range    *Range          `json:"range"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	h.Contents = decodeHoverContents(wire.Contents)
	h.Range = wire.Range
	return nil
}

type CodeAction struct {
	Title       string          `json:"title"`
	Kind        string          `json:"kind,omitempty"`
	Diagnostics []Diagnostic    `json:"diagnostics,omitempty"`
	Edit        *WorkspaceEdit  `json:"edit,omitempty"`
	Command     *Command        `json:"command,omitempty"`
	Data        json.RawMessage `json:"data,omitempty"`
}

type Command struct {
	Title     string          `json:"title"`
	Command   string          `json:"command"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// WorkspaceEdit keeps the simple map form. documentChanges returned by servers
// is merged into Changes during decoding so callers only deal with one shape.
type WorkspaceEdit struct {
	Changes map[string][]TextEdit `json:"changes,omitempty"`
}

func (w *WorkspaceEdit) UnmarshalJSON(data []byte) error {
	var wire struct {
		Changes         map[string][]TextEdit `json:"changes"`
		DocumentChanges []json.RawMessage     `json:"documentChanges"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	w.Changes = wire.Changes
	if len(wire.DocumentChanges) == 0 {
		return nil
	}
	if w.Changes == nil {
		w.Changes = make(map[string][]TextEdit)
	}
	for _, raw := range wire.DocumentChanges {
		var change struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Edits []TextEdit `json:"edits"`
		}
		if err := json.Unmarshal(raw, &change); err != nil {
			continue
		}
		if change.TextDocument.URI == "" || len(change.Edits) == 0 {
			continue
		}
		w.Changes[change.TextDocument.URI] = append(w.Changes[change.TextDocument.URI], change.Edits...)
	}
	return nil
}

// ServerConfig describes how to launch one language server. A config is built
// from a command line with ServerConfigFromCommand or taken from Preset.
type ServerConfig struct {
	Command     string
	Args        []string
	LanguageID  string
	Extensions  []string
	InitOptions map[string]any
}

// ServerCapabilities is the subset of LSP server capabilities pero uses.
// A false field means the server did not declare that capability, including
// servers that send no capabilities at all. Supports follows the same rule:
// only an explicitly advertised capability is true, so the UI hides a feature
// instead of sending a request the server is likely to reject.
type ServerCapabilities struct {
	HoverProvider              bool
	CompletionProvider         bool
	DefinitionProvider         bool
	ReferencesProvider         bool
	DocumentFormattingProvider bool
	CodeActionProvider         bool
}

// capability decodes an LSP capability that may be a boolean or an options
// object. The object form means supported; null or an absent field means not
// supported.
type capability bool

func (c *capability) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*c = false
		return nil
	}
	if data[0] == '{' {
		*c = true
		return nil
	}
	var value bool
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*c = capability(value)
	return nil
}

// decodeServerCapabilities reads the capabilities of an initialize result.
// An absent or malformed capabilities object yields the zero value, which
// advertises nothing; Initialize stays successful because capabilities are
// advisory.
func decodeServerCapabilities(raw json.RawMessage) ServerCapabilities {
	var result struct {
		Capabilities json.RawMessage `json:"capabilities"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return ServerCapabilities{}
	}
	var wire struct {
		HoverProvider              capability `json:"hoverProvider"`
		CompletionProvider         capability `json:"completionProvider"`
		DefinitionProvider         capability `json:"definitionProvider"`
		ReferencesProvider         capability `json:"referencesProvider"`
		DocumentFormattingProvider capability `json:"documentFormattingProvider"`
		CodeActionProvider         capability `json:"codeActionProvider"`
	}
	if err := json.Unmarshal(result.Capabilities, &wire); err != nil {
		return ServerCapabilities{}
	}
	return ServerCapabilities{
		HoverProvider:              bool(wire.HoverProvider),
		CompletionProvider:         bool(wire.CompletionProvider),
		DefinitionProvider:         bool(wire.DefinitionProvider),
		ReferencesProvider:         bool(wire.ReferencesProvider),
		DocumentFormattingProvider: bool(wire.DocumentFormattingProvider),
		CodeActionProvider:         bool(wire.CodeActionProvider),
	}
}

var completionKinds = map[int]string{
	1: "text", 2: "method", 3: "func", 4: "func", 5: "field",
	6: "var", 7: "class", 8: "interface", 9: "module", 10: "property",
	11: "unit", 12: "value", 13: "enum", 14: "keyword", 15: "snippet",
	16: "color", 17: "file", 18: "reference", 19: "folder", 20: "enum",
	21: "const", 22: "type", 23: "event", 24: "operator", 25: "type",
}

var completionKindNames = map[string]string{
	"text": "text", "method": "method", "function": "func", "func": "func",
	"constructor": "func", "field": "field", "variable": "var", "class": "class",
	"interface": "interface", "module": "module", "property": "property",
	"unit": "unit", "value": "value", "enum": "enum", "keyword": "keyword",
	"snippet": "snippet", "color": "color", "file": "file", "reference": "reference",
	"folder": "folder", "enummember": "enum", "constant": "const", "struct": "type",
	"event": "event", "operator": "operator", "typeparameter": "type", "type": "type",
}

func normalizeCompletionKind(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return ""
		}
		s = strings.TrimSpace(s)
		if n, err := strconv.Atoi(s); err == nil {
			return completionKinds[n]
		}
		if short, ok := completionKindNames[strings.ToLower(s)]; ok {
			return short
		}
		return strings.ToLower(s)
	}
	var n int
	if err := json.Unmarshal(raw, &n); err != nil {
		return ""
	}
	return completionKinds[n]
}

// decodeMarkup accepts string, MarkupContent or MarkedString and returns the
// text value. Documentation only uses the value, not the markup kind.
func decodeMarkup(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	switch raw[0] {
	case '"':
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return s
		}
	case '{':
		var mc struct {
			Value    string `json:"value"`
			Language string `json:"language"`
		}
		if json.Unmarshal(raw, &mc) == nil {
			if mc.Language != "" && mc.Value != "" {
				return "```" + mc.Language + "\n" + mc.Value + "\n```"
			}
			return mc.Value
		}
	case '[':
		var parts []json.RawMessage
		if json.Unmarshal(raw, &parts) == nil {
			var b strings.Builder
			for i, p := range parts {
				if s := decodeMarkup(p); s != "" {
					if i > 0 && b.Len() > 0 {
						b.WriteString("\n\n")
					}
					b.WriteString(s)
				}
			}
			return b.String()
		}
	}
	return ""
}

func decodeHoverContents(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	switch raw[0] {
	case '"':
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return s
		}
	case '{':
		var mc struct {
			Kind     string `json:"kind"`
			Language string `json:"language"`
			Value    string `json:"value"`
		}
		if json.Unmarshal(raw, &mc) == nil {
			if mc.Language != "" && mc.Value != "" {
				return "```" + mc.Language + "\n" + mc.Value + "\n```"
			}
			return mc.Value
		}
	case '[':
		var parts []json.RawMessage
		if json.Unmarshal(raw, &parts) == nil {
			var b strings.Builder
			for _, p := range parts {
				s := decodeMarkup(p)
				if s == "" {
					continue
				}
				if b.Len() > 0 {
					b.WriteString("\n\n")
				}
				b.WriteString(s)
			}
			return b.String()
		}
	}
	return ""
}

func decodeTextEdit(raw json.RawMessage) *TextEdit {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil
	}
	var wire struct {
		Range   *Range `json:"range"`
		NewText string `json:"newText"`
		Insert  *Range `json:"insert"`
		Replace *Range `json:"replace"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil
	}
	r := wire.Range
	if r == nil {
		r = wire.Replace
	}
	if r == nil {
		r = wire.Insert
	}
	if r == nil {
		return nil
	}
	return &TextEdit{Range: *r, NewText: wire.NewText}
}
