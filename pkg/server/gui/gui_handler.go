package gui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/rivo/tview"
)

// Custom slog Handler
type GUIHandler struct {
	app          *tview.Application
	view         *tview.TextView
	attrs        []slog.Attr
	groups       []string
	buffer       strings.Builder
	bufferMutex  sync.Mutex
	ticker       *time.Ticker
	stopFlush    chan struct{}
	maxBufferLen int
}

// NewGUIHandler creates a new handler that writes to the provided TextView.
func NewGUIHandler(app *tview.Application, view *tview.TextView) *GUIHandler {
	h := &GUIHandler{
		app:          app,
		view:         view,
		ticker:       time.NewTicker(100 * time.Millisecond),
		stopFlush:    make(chan struct{}),
		maxBufferLen: 8192, // Flush when buffer exceeds 8KB
	}

	// Start background flusher
	go h.periodicFlush()

	return h
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

	// Write to buffer instead of directly to view
	h.bufferMutex.Lock()
	h.buffer.WriteString(line.String())
	shouldFlush := h.buffer.Len() >= h.maxBufferLen
	h.bufferMutex.Unlock()

	// Flush immediately if buffer is too large
	if shouldFlush {
		h.flush()
	}

	return nil
}

// flush writes the buffered content to the TextView.
func (h *GUIHandler) flush() {
	h.bufferMutex.Lock()
	if h.buffer.Len() == 0 {
		h.bufferMutex.Unlock()
		return
	}
	content := h.buffer.String()
	h.buffer.Reset()
	h.bufferMutex.Unlock()

	h.app.QueueUpdateDraw(func() {
		fmt.Fprint(h.view, content)
	})
}

// periodicFlush flushes the buffer periodically.
func (h *GUIHandler) periodicFlush() {
	for {
		select {
		case <-h.ticker.C:
			h.flush()
		case <-h.stopFlush:
			h.ticker.Stop()
			h.flush() // Final flush
			return
		}
	}
}

// Stop stops the periodic flusher and flushes remaining content.
func (h *GUIHandler) Stop() {
	close(h.stopFlush)
}

// WithAttrs returns a new handler with additional attributes.
func (h *GUIHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)

	return &GUIHandler{
		app:          h.app,
		view:         h.view,
		attrs:        newAttrs,
		groups:       h.groups,
		ticker:       h.ticker,
		stopFlush:    h.stopFlush,
		maxBufferLen: h.maxBufferLen,
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
		app:          h.app,
		view:         h.view,
		attrs:        h.attrs,
		groups:       newGroups,
		ticker:       h.ticker,
		stopFlush:    h.stopFlush,
		maxBufferLen: h.maxBufferLen,
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
