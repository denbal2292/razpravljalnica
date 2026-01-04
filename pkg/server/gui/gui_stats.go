package gui

import (
	"fmt"
	"runtime"
	"time"

	"github.com/rivo/tview"
)

// ServerStatsProvider defines the interface for getting server statistics.
type ServerStatsProvider interface {
	GetStats() ServerStatsSnapshot
}

// ServerStatsSnapshot represents a point-in-time snapshot of server stats.
type ServerStatsSnapshot struct {
	NodeID          string
	NodeAddr        string
	Role            string
	PredecessorAddr string
	SuccessorAddr   string
	Connected       bool
	EventsProcessed int64
	EventsApplied   int64
	MessagesStored  int
	TopicsCount     int
	UsersCount      int
}

// Stats holds the statistics configuration and provider.
type Stats struct {
	startTime time.Time
	cpAddr    string // Control Plane address
	provider  ServerStatsProvider
}

// NewStats creates a new Stats instance.
func NewStats(cpAddr string) *Stats {
	return &Stats{
		startTime: time.Now(),
		cpAddr:    cpAddr,
	}
}

// SetProvider sets the stats provider (called after node is created).
func (s *Stats) SetProvider(provider ServerStatsProvider) {
	s.provider = provider
}

// GetUptime returns the elapsed time since the server started.
func (s *Stats) GetUptime() time.Duration {
	return time.Since(s.startTime).Truncate(time.Second)
}

// StatsCollector periodically updates the stats display in the GUI.
type StatsCollector struct {
	app       *tview.Application
	statsView *tview.TextView
	stats     *Stats
	ticker    *time.Ticker
	stopChan  chan struct{}
}

// NewStatsCollector creates a new stats collector that updates the display.
func NewStatsCollector(app *tview.Application, view *tview.TextView, stats *Stats) *StatsCollector {
	return &StatsCollector{
		app:       app,
		statsView: view,
		stats:     stats,
		stopChan:  make(chan struct{}),
	}
}

// Start begins the periodic stats update.
func (sc *StatsCollector) Start() {
	sc.ticker = time.NewTicker(time.Second)
	go sc.run()
}

// Stop terminates the stats collector.
func (sc *StatsCollector) Stop() {
	if sc.ticker != nil {
		sc.ticker.Stop()
	}
	close(sc.stopChan)
}

// Updates the stats header every second.
func (sc *StatsCollector) run() {
	for {
		select {
		case <-sc.ticker.C:
			sc.updateDisplay()
		case <-sc.stopChan:
			return
		}
	}
}

// updateDisplay formats and displays the current stats.
func (sc *StatsCollector) updateDisplay() {
	// If no provider is set yet, show initializing state
	if sc.stats.provider == nil {
		sc.displayInitializing()
		return
	}

	// Poll the provider for fresh stats
	snapshot := sc.stats.provider.GetStats()
	uptime := sc.stats.GetUptime()
	cpAddr := sc.stats.cpAddr
	goroutines := runtime.NumGoroutine()

	// Format the role with color
	var roleColor string
	switch snapshot.Role {
	case "HEAD":
		roleColor = "green"
	case "TAIL":
		roleColor = "blue"
	case "MIDDLE":
		roleColor = "yellow"
	case "SINGLE":
		roleColor = "orange"
	default:
		roleColor = "gray"
	}

	// Build the stats display
	var display string

	// Node ID, address, and role
	display += fmt.Sprintf("[white]Node:[-] [cyan]%s[-] [darkgray](%s)[-] | [white]Role:[-] [%s]%s[-]",
		snapshot.NodeID, snapshot.NodeAddr, roleColor, snapshot.Role)

	// Uptime and metrics
	display += fmt.Sprintf("\n[white]Uptime:[-] [green]%s[-] | [white]Events:[-] [yellow]%d/%d[-] | [white]Messages:[-] [yellow]%d[-] | [white]Goroutines:[-] [cyan]%d[-]",
		uptime, snapshot.EventsProcessed, snapshot.EventsApplied, snapshot.MessagesStored, goroutines)

	// Chain info
	predDisplay := snapshot.PredecessorAddr
	if predDisplay == "" {
		predDisplay = "[gray]none[-]"
	} else {
		predDisplay = "[green]" + predDisplay + "[-]"
	}

	succDisplay := snapshot.SuccessorAddr
	if succDisplay == "" {
		succDisplay = "[gray]none[-]"
	} else {
		succDisplay = "[green]" + succDisplay + "[-]"
	}

	connStatus := "[red]disconnected[-]"
	if snapshot.Connected {
		connStatus = "[green]connected[-]"
	}

	display += fmt.Sprintf("\n[white]Pred:[-] %s | [white]Succ:[-] %s | [white]CP:[-] [blue]%s[-] %s",
		predDisplay, succDisplay, cpAddr, connStatus)

	sc.app.QueueUpdateDraw(func() {
		sc.statsView.SetText(display)
	})
}

// displayInitializing shows the initializing state.
func (sc *StatsCollector) displayInitializing() {
	display := "[white]Node:[-] [gray]INITIALIZING[-]\n"
	display += fmt.Sprintf("[white]Uptime:[-] [green]%s[-]\n", sc.stats.GetUptime())
	display += fmt.Sprintf("[white]CP:[-] [blue]%s[-]", sc.stats.cpAddr)

	sc.app.QueueUpdateDraw(func() {
		sc.statsView.SetText(display)
	})
}
