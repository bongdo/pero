package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// LoadRecent читает recent из state-файла. Нет файла — пустой список.
// Остальные ключи state-файла игнорируются.
func LoadRecent(path string) ([]string, error) {
	var doc struct {
		Recent []string `toml:"recent"`
	}
	if _, err := toml.DecodeFile(path, &doc); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if doc.Recent == nil {
		return []string{}, nil
	}
	return doc.Recent, nil
}

// SaveRecent атомарно пишет recent, сохраняя остальные ключи state-файла.
func SaveRecent(path string, recent []string) error {
	doc := make(map[string]any)
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := toml.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	case errors.Is(err, fs.ErrNotExist):
	default:
		return err
	}

	if recent == nil {
		recent = []string{}
	}
	doc["recent"] = recent
	body, err := toml.Marshal(doc)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// Уникальный временный файл: параллельные сохранения не затирают друг друга.
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Chmod(0o644); err != nil {
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
