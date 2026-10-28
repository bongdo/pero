package lsp

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathToURIRoundTrip(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "привет мир", "файл #1%?.go")
	uri := PathToURI(filePath)

	if !strings.HasPrefix(uri, "file:///") {
		t.Fatalf("uri = %q", uri)
	}
	if strings.ContainsAny(uri, " #?") {
		t.Fatalf("uri is not escaped: %q", uri)
	}
	if !strings.Contains(uri, "%") {
		t.Fatalf("uri is not percent-encoded: %q", uri)
	}

	got, err := URIToPath(uri)
	if err != nil {
		t.Fatalf("URIToPath: %v", err)
	}
	abs, _ := filepath.Abs(filePath)
	if got != abs {
		t.Fatalf("round trip = %q, want %q", got, abs)
	}
}

func TestURIToPathRejectsNonFile(t *testing.T) {
	for _, uri := range []string{
		"https://example.com/a.go",
		"untitled:Untitled-1",
		"file:///tmp/%zz",
		"file://",
	} {
		if _, err := URIToPath(uri); err == nil {
			t.Errorf("URIToPath(%q) returned nil error", uri)
		}
	}
}

func TestClientUsesEscapedCyrillicURI(t *testing.T) {
	client, server := newTestClient(t)
	filePath := filepath.Join(client.root, "заметки", "файл с пробелом.md")
	uri := PathToURI(filePath)
	if _, err := url.Parse(uri); err != nil {
		t.Fatalf("invalid uri: %v", err)
	}

	client.DidOpen(uri, "markdown", 1, "текст")
	msg := server.next(t)
	if msg.Method != "textDocument/didOpen" {
		t.Fatalf("method = %q", msg.Method)
	}
	var params struct {
		TextDocument struct {
			URI  string `json:"uri"`
			Text string `json:"text"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		t.Fatal(err)
	}
	if params.TextDocument.URI != uri {
		t.Fatalf("uri on wire = %q, want %q", params.TextDocument.URI, uri)
	}
	if params.TextDocument.Text != "текст" {
		t.Fatalf("text = %q", params.TextDocument.Text)
	}
}
