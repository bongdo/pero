package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/bongdo/pero/internal/config"
	"github.com/bongdo/pero/internal/ed"
	"github.com/bongdo/pero/internal/ui"
	"github.com/bongdo/pero/internal/workspace"
)

func main() {
	cfg, warning := config.LoadOrDefault()
	cwd, _ := os.Getwd()

	opts := ui.Options{Warning: warning}
	arg := ""
	if len(os.Args) > 1 {
		arg = os.Args[1]
	}

	switch {
	case arg == "":
		opts.Root = workspace.Root(cwd)
	case arg == "-":
		opts.Root = workspace.Root(cwd)
		opts.Empty = true
	default:
		path := expandHome(arg)
		if !filepath.IsAbs(path) {
			path = filepath.Join(cwd, path)
		}
		path = filepath.Clean(path)
		info, err := os.Stat(path)
		switch {
		case err == nil && info.IsDir():
			opts.Root = workspace.Root(path)
			opts.Empty = true
		case err == nil:
			buf, err := ed.Open(path)
			if err != nil {
				fmt.Fprintln(os.Stderr, "pero:", err)
				os.Exit(1)
			}
			opts.Initial = buf
			opts.Root = workspace.Root(filepath.Dir(path))
		case os.IsNotExist(err):
			opts.Initial = ed.New(path)
			opts.Root = workspace.Root(filepath.Dir(path))
		default:
			fmt.Fprintln(os.Stderr, "pero:", err)
			os.Exit(1)
		}
	}
	if opts.Root == "" {
		opts.Root = workspace.Root(cwd)
	}

	if _, err := tea.NewProgram(ui.New(cfg, opts)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "pero:", err)
		os.Exit(1)
	}
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
