package workspace

import (
	"path/filepath"
	"slices"
	"strings"
)

// DisplayName — базовое имя файла для вкладки.
func DisplayName(path string) string {
	return filepath.Base(path)
}

// Disambiguate возвращает имя для каждого пути: базовое имя, а при совпадении
// базовых имён — минимально различающийся суффикс пути. Абсолютный путь
// попадает в результат только если относительных компонентов не хватило.
func Disambiguate(paths []string) map[string]string {
	names := make(map[string]string, len(paths))
	groups := make(map[string][]string, len(paths))
	for _, path := range paths {
		if _, seen := names[path]; seen {
			continue
		}
		base := DisplayName(path)
		names[path] = base
		groups[base] = append(groups[base], path)
	}

	for base, group := range groups {
		if len(group) < 2 {
			continue
		}
		for _, path := range group {
			dir := dirParts(filepath.Dir(path))
			assigned := false
			for depth := 1; depth <= max(1, len(dir)); depth++ {
				tail := dir
				if depth < len(dir) {
					tail = dir[len(dir)-depth:]
				}
				if uniqueTail(group, path, tail, depth) {
					parts := make([]string, 0, len(tail)+1)
					parts = append(parts, tail...)
					parts = append(parts, base)
					names[path] = filepath.Join(parts...)
					assigned = true
					break
				}
			}
			if !assigned {
				names[path] = filepath.Clean(path)
			}
		}
	}
	return names
}

// uniqueTail — хвост из depth компонентов каталога не встречается у других
// путей группы.
func uniqueTail(group []string, path string, tail []string, depth int) bool {
	for _, other := range group {
		if other == path {
			continue
		}
		otherDir := dirParts(filepath.Dir(other))
		if len(otherDir) < depth {
			continue
		}
		if slices.Equal(otherDir[len(otherDir)-depth:], tail) {
			return false
		}
	}
	return true
}

// dirParts разбивает каталог на компоненты; корень обозначается "/".
func dirParts(dir string) []string {
	clean := filepath.Clean(dir)
	if clean == "." || clean == string(filepath.Separator) {
		return nil
	}
	parts := strings.Split(clean, string(filepath.Separator))
	if parts[0] == "" {
		parts[0] = string(filepath.Separator)
	}
	return parts
}
