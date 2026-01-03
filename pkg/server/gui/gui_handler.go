package gui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/rivo/tview"
)

// Custom slog Handler
type GUIHandler struct {
	app    *tview.Application
	view   *tview.TextView
	attrs  []slog.Attr
	groups []string
}

// NewGUIHandler creates a new handler that writes to the provided TextView.
func NewGUIHandler(app *tview.Application, view *tview.TextView) *GUIHandler {
	return &GUIHandler{
		app:  app,
		view: view,
	}
}

// Enabled returns whether the handler is enabled for the given level.
func (h *GUIHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

// Handle formats and displays a log record in the TextView.
// It colors the log level and message, and includes any attributes.
func (h *GUIHandler) Handle(_ context.Context, r slog.Record) error {
	// Map log levels to colors
	levelColor := map[slog.Level]string{
		slog.LevelDebug: "gray",
		slog.LevelInfo:  "green",
		slog.LevelWarn:  "yellow",
		slog.LevelError: "red",
	}

	color, ok := levelColor[r.Level]
	if !ok {
		color = "white"
	}

	// Format timestamp
	timestamp := r.Time.Format("15:04:05")

	// Build the log line
	var line strings.Builder
	fmt.Fprintf(&line, "[darkgray]%s[-] [%s] %s [-] %s", timestamp, color, levelPadded(r.Level), r.Message)

	// Add attributes if present
	attrs := h.attrs
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)
		return true
	})

	if len(attrs) > 0 {
		line.WriteString(" [darkgray]|[-] ")
		for i, attr := range attrs {
			if i > 0 {
				line.WriteString(", ")
			}
			fmt.Fprintf(&line, "[cyan]%s[-]=%v", attr.Key, attr.Value)
		}
	}

	line.WriteString("\n")

	h.app.QueueUpdateDraw(func() {
		fmt.Fprint(h.view, line.String())
	})

	return nil
}

// WithAttrs returns a new handler with additional attributes.
func (h *GUIHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)

	return &GUIHandler{
		app:    h.app,
		view:   h.view,
		attrs:  newAttrs,
		groups: h.groups,
	}
}

// WithGroup returns a new handler with an additional group prefix.
func (h *GUIHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name

	return &GUIHandler{
		app:    h.app,
		view:   h.view,
		attrs:  h.attrs,
		groups: newGroups,
	}
}

// levelPadded returns a padded string representation of the log level.
func levelPadded(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return "DEBUG"
	case slog.LevelInfo:
		return "INFO"
	case slog.LevelWarn:
		return "WARN"
	case slog.LevelError:
		return "ERROR"
	default:
		return level.String()
	}
}
