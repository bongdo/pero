package assist

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	opencodeTimeout = 60 * time.Second
	maxOutputBytes  = 8 << 10
)

// RunOpencode запускает `opencode run`, передаёт prompt в stdin и возвращает
// stdout. В буфер редактора ничего не пишет. Без deadline у ctx ставит 30 с.
func RunOpencode(ctx context.Context, prompt string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opencodeTimeout)
		defer cancel()
	}

	// Сообщение передаём аргументом: `opencode run` без сообщения уходит в
	// интерактивный режим и висит до таймаута. stdin оставляем как fallback
	// для очень длинных промптов (ARG_MAX).
	args := []string{"run"}
	if len(prompt) > 0 && len(prompt) <= 200_000 {
		args = append(args, prompt)
	}
	cmd := exec.CommandContext(ctx, "opencode", args...)
	cmd.Stdin = strings.NewReader(prompt)
	stdout := cappedBuffer{limit: maxOutputBytes}
	stderr := cappedBuffer{limit: maxOutputBytes}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		errText := strings.TrimSpace(stderr.String())
		switch {
		case errors.Is(err, exec.ErrNotFound):
			return "", fmt.Errorf("opencode не найден в PATH: %w", err)
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			return "", fmt.Errorf("opencode: превышено время ожидания: %w", err)
		case errors.Is(ctx.Err(), context.Canceled):
			return "", fmt.Errorf("opencode: отменено: %w", err)
		case errText != "":
			return "", fmt.Errorf("opencode: %w: %s", err, errText)
		default:
			return "", fmt.Errorf("opencode: %w", err)
		}
	}
	return stdout.String(), nil
}

// cappedBuffer хранит не больше limit байт, остальное молча отбрасывает,
// чтобы болтливый процесс не съел память редактора.
type cappedBuffer struct {
	buf   bytes.Buffer
	limit int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if room := b.limit - b.buf.Len(); room > 0 {
		if len(p) > room {
			p = p[:room]
		}
		b.buf.Write(p)
	}
	return n, nil
}

func (b *cappedBuffer) String() string {
	return truncateBytes(b.buf.String(), b.limit)
}

// truncateBytes режет строку до limit байт, отбрасывая недописанную руну.
func truncateBytes(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(s) > limit {
		s = s[:limit]
	}
	for len(s) > 0 {
		r, size := utf8.DecodeLastRuneInString(s)
		if r != utf8.RuneError || size > 1 {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}

// DetectLanguage выбирает язык спеллчека по доле кириллицы среди букв:
// >= 0.15 - "ru" (ltex), иначе "en" (harper).
func DetectLanguage(sample string) string {
	var letters, cyrillic int
	for _, r := range sample {
		if !unicode.IsLetter(r) {
			continue
		}
		letters++
		if unicode.Is(unicode.Cyrillic, r) {
			cyrillic++
		}
	}
	if letters > 0 && float64(cyrillic)/float64(letters) >= 0.15 {
		return "ru"
	}
	return "en"
}
