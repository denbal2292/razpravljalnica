package gui

import (
	"log/slog"
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/lmittmann/tint"
	"github.com/rivo/tview"
)

// ControlGUI manages the terminal user interface for the control plane.
type ControlGUI struct {
	app         *tview.Application
	pages       *tview.Pages
	statsView   *tview.TextView
	controlLogs *tview.TextView
	raftLogs    *tview.TextView

	stats     *Stats
	collector *StatsCollector

	logger     *slog.Logger
	raftLogger *slog.Logger
}

// NewControlGUI creates and initializes a new control plane GUI.
// It sets up the layout with stats at the top and logs at the bottom.
func NewControlGUI() *ControlGUI {
	app := tview.NewApplication()
	app.EnableMouse(true)

	gui := &ControlGUI{
		app:         app,
		pages:       tview.NewPages(),
		statsView:   tview.NewTextView(),
		controlLogs: tview.NewTextView(),
		raftLogs:    tview.NewTextView(),
		stats:       NewStats(),
	}

	gui.setupWidgets()
	gui.setupLayout()
	gui.setupInputCapture()

	// Create the custom loggers
	controlHandler := NewGUIHandler(app, gui.controlLogs)
	gui.logger = slog.New(controlHandler)

	raftHandler := NewGUIHandler(app, gui.raftLogs)
	gui.raftLogger = slog.New(raftHandler)

	// Create and start stats collector
	gui.collector = NewStatsCollector(app, gui.statsView, gui.stats)
	gui.collector.Start()

	return gui
}

func (gui *ControlGUI) setupWidgets() {
	// Configure stats view (top pane) - 4 lines for control plane stats
	gui.statsView.
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetBorder(false)

	// Configure control plane log view (left pane)
	gui.controlLogs.
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(false).
		SetBorder(true).
		SetTitle(" Control Plane Logs ")

	// Configure Raft log view (right pane)
	gui.raftLogs.
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(false).
		SetBorder(true).
		SetTitle(" Raft Logs ")

	// Auto-scroll to bottom when new logs arrive
	gui.controlLogs.SetChangedFunc(func() {
		gui.controlLogs.ScrollToEnd()
	})
	gui.raftLogs.SetChangedFunc(func() {
		gui.raftLogs.ScrollToEnd()
	})
}

func (gui *ControlGUI) setupLayout() {
	// Create horizontal layout for log panes (side by side)
	logLayout := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(gui.controlLogs, 0, 1, true). // Control logs take half
		AddItem(gui.raftLogs, 0, 1, false)    // Raft logs take half

	// Main layout: stats at top (4 lines), logs fill remaining space
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(gui.statsView, 8, 0, false). // Fixed height for stats
		AddItem(logLayout, 0, 1, true)       // Logs take remaining space

	gui.pages.AddPage("main", layout, true, true)
	gui.app.SetRoot(gui.pages, true)
}

func (gui *ControlGUI) setupInputCapture() {
	gui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'q', 'Q':
			// Quit application
			gui.app.Stop()
			return nil
		case 'c', 'C':
			// Clear both log panes
			gui.controlLogs.Clear()
			gui.raftLogs.Clear()
			go gui.logger.Info("Logs cleared by the user")
			return nil
		}

		return event
	})
}

// Run starts the GUI application.
func (gui *ControlGUI) Run() error {
	return gui.app.Run()
}

// Stop stops the GUI and cleans up resources.
func (gui *ControlGUI) Stop() {
	if gui.collector != nil {
		gui.collector.Stop()
	}
	// Stop both logger handlers to flush remaining logs
	if handler, ok := gui.logger.Handler().(*GUIHandler); ok {
		handler.Stop()
	}
	if handler, ok := gui.raftLogger.Handler().(*GUIHandler); ok {
		handler.Stop()
	}
	gui.app.Stop()
}

// StartWithFallback starts the GUI and falls back to console logging on error.
// Returns the control plane logger, Raft logger, stats, and a cleanup function.
func StartWithFallback(enableGUI bool) (*slog.Logger, *slog.Logger, *Stats, func()) {
	if !enableGUI {
		// Console mode
		// Use tint for nicer output
		logger := slog.New(tint.NewHandler(
			os.Stdout,
			&tint.Options{
				Level: slog.LevelDebug,
				// GO's default reference time
				TimeFormat: "02-01-2006 15:04:05",
			},
		))
		stats := NewStats()
		// Return same logger for both control plane and Raft in console mode
		return logger, logger, stats, func() {}
	}

	// Try to start GUI
	gui := NewControlGUI()

	// Start GUI in goroutine
	go func() {
		err := gui.Run()
		if err != nil {
			panic(err)
		}
		os.Exit(0)
	}()

	// Return separate loggers: control plane and Raft
	return gui.logger, gui.raftLogger, gui.stats, func() {
		gui.Stop()
	}
}

// GetRaftLogger returns the Raft-specific logger.
func (gui *ControlGUI) GetRaftLogger() *slog.Logger {
	return gui.raftLogger
}
