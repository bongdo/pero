package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// TabState — открытая вкладка и позиция курсора.
type TabState struct {
	Path       string `toml:"path"`
	CursorLine int    `toml:"cursor_line"`
	CursorCol  int    `toml:"cursor_col"`
}

// State — состояние между запусками.
type State struct {
	Tabs           []TabState `toml:"tabs"`
	Recent         []string   `toml:"recent"`
	RecentCommands []string   `toml:"recent_commands"`
	LastFile       string     `toml:"last_file"`
}

// LoadState читает state.toml. Отсутствующий или битый файл — не ошибка.
func LoadState() (State, bool) {
	path, err := StatePath()
	if err != nil {
		return State{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}, false
	}
	var st State
	if err := toml.Unmarshal(data, &st); err != nil {
		return State{}, false
	}
	return st, true
}

// SaveState атомарно пишет state.toml (временный файл + rename).
func SaveState(st State) error {
	path, err := StatePath()
	if err != nil {
		return err
	}
	data, err := toml.Marshal(st)
	if err != nil {
		return err
	}
	body := append([]byte("# сгенерировано pero; можно удалить\n"), data...)
	if err := writeAtomic(path, body, 0o644); err != nil {
		return fmt.Errorf("state: %w", err)
	}
	return nil
}

func writeAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Chmod(perm); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
