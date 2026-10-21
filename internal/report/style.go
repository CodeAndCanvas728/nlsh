package report

import (
	"io"
	"os"

	"github.com/CodeAndCanvas728/nlsh/internal/guard"
)

// Style renders text with ANSI escapes, or without them when the output is
// not a terminal or NO_COLOR is set.
type Style struct {
	Enabled bool
}

func (s Style) wrap(code, text string) string {
	if !s.Enabled {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

// Dim renders dimmed text.
func (s Style) Dim(text string) string { return s.wrap("2", text) }

// Bold renders bold text.
func (s Style) Bold(text string) string { return s.wrap("1", text) }

// Cyan renders cyan text.
func (s Style) Cyan(text string) string { return s.wrap("36", text) }

// Green renders green text.
func (s Style) Green(text string) string { return s.wrap("32", text) }

// Yellow renders yellow text.
func (s Style) Yellow(text string) string { return s.wrap("33", text) }

// Red renders red text.
func (s Style) Red(text string) string { return s.wrap("31", text) }

// Badge renders the label shown next to a command.
func (s Style) Badge(risk guard.Risk) string {
	switch risk {
	case guard.RiskDestructive:
		return s.Red("[DESTRUCTIVE]")
	case guard.RiskCareful:
		return s.Yellow("[careful]")
	default:
		return s.Green("[safe]")
	}
}

// isTerminal reports whether a writer is a terminal.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
