package gui

import (
	"log/slog"
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ServerGUI manages the terminal user interface for the server.
type ServerGUI struct {
	app       *tview.Application
	pages     *tview.Pages
	statsView *tview.TextView
	logView   *tview.TextView

	stats     *Stats
	collector *StatsCollector

	logger *slog.Logger
}

// NewServerGUI creates and initializes a new server GUI.
// It sets up the layout with stats at the top and logs at the bottom.
func NewServerGUI(nodeId, nodeAddr, cpAddr string) *ServerGUI {
	app := tview.NewApplication()
	app.EnableMouse(true)

	gui := &ServerGUI{
		app:       app,
		pages:     tview.NewPages(),
		statsView: tview.NewTextView(),
		logView:   tview.NewTextView(),
		stats:     NewStats(nodeId, nodeAddr, cpAddr),
	}

	gui.setupWidgets()
	gui.setupLayout()
	gui.setupInputCapture()

	// Create the custom logger
	handler := NewGUIHandler(app, gui.logView)
	gui.logger = slog.New(handler)

	// Create and start stats collector
	gui.collector = NewStatsCollector(app, gui.statsView, gui.stats)
	gui.collector.Start()

	return gui
}

// setupWidgets configures the individual UI widgets.
func (gui *ServerGUI) setupWidgets() {
	// Configure stats view (top pane)
	gui.statsView.
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetBorder(false)

	// Configure log view (bottom pane)
	gui.logView.
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(false).
		SetBorder(true).
		SetTitle(" Logs ")

	// Auto-scroll to bottom when new logs arrive
	gui.logView.SetChangedFunc(func() {
		gui.logView.ScrollToEnd()
		gui.app.Draw()
	})
}

// setupLayout creates the overall layout structure.
func (gui *ServerGUI) setupLayout() {
	// Main layout: stats at top (3 lines), logs fill remaining space
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(gui.statsView, 3, 0, false). // Fixed height for stats
		AddItem(gui.logView, 0, 1, true)     // Logs take remaining space

	gui.pages.AddPage("main", layout, true, true)
	gui.app.SetRoot(gui.pages, true)
}

// setupInputCapture sets up global keyboard shortcuts.
func (gui *ServerGUI) setupInputCapture() {
	gui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'q', 'Q':
			// Quit application
			gui.app.Stop()
			return nil
		case 'c', 'C':
			// Clear logs
			gui.logView.Clear()
			gui.logger.Info("Logs cleared")
			return nil
		}

		return event
	})
}

// GetLogger returns the slog.Logger that writes to the GUI.
func (gui *ServerGUI) GetLogger() *slog.Logger {
	return gui.logger
}

// GetStats returns the stats object for updating server metrics.
func (gui *ServerGUI) GetStats() *Stats {
	return gui.stats
}

// Run starts the GUI application.
// This blocks until the application is stopped.
func (gui *ServerGUI) Run() error {
	return gui.app.Run()
}

// Stop gracefully stops the GUI and cleans up resources.
func (gui *ServerGUI) Stop() {
	if gui.collector != nil {
		gui.collector.Stop()
	}
	gui.app.Stop()
}

// StartWithFallback starts the GUI and falls back to console logging on error.
// Returns the logger and stats to use, plus a cleanup function.
func StartWithFallback(nodeId, nodeAddr, cpAddr string, enableGUI bool) (*slog.Logger, *Stats, func()) {
	if !enableGUI {
		// Console mode
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
		stats := NewStats(nodeId, nodeAddr, cpAddr)
		return logger, stats, func() {}
	}

	// Try to start GUI
	gui := NewServerGUI(nodeId, nodeAddr, cpAddr)

	// Start GUI in goroutine
	go func() {
		if err := gui.Run(); err != nil {
			panic(err)
		}
	}()

	return gui.GetLogger(), gui.GetStats(), func() {
		gui.Stop()
	}
}
