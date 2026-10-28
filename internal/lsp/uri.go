package lsp

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// PathToURI converts a local path to a file:// URI. The path is made absolute
// and percent-encoded (spaces, Cyrillic and other non-ASCII characters).
func PathToURI(filePath string) string {
	if filePath == "" {
		return ""
	}
	abs, err := filepath.Abs(filePath)
	if err != nil {
		abs = filePath
	}
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}).String()
}

// URIToPath is the inverse of PathToURI. Only file:// URIs are accepted.
func URIToPath(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("lsp: invalid URI %q: %w", uri, err)
	}
	if !strings.EqualFold(u.Scheme, "file") {
		return "", fmt.Errorf("lsp: not a file URI: %q", uri)
	}
	if u.Path == "" {
		return "", fmt.Errorf("lsp: empty file URI: %q", uri)
	}
	// url.Parse already percent-decodes Path.
	filePath := u.Path
	if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
		filePath = "//" + u.Host + filePath
	}
	return filepath.FromSlash(filePath), nil
}
