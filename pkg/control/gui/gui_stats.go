package gui

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/hashicorp/raft"
	"github.com/rivo/tview"
)

// ControlStatsProvider defines the interface for getting control plane statistics.
type ControlStatsProvider interface {
	GetStats() ControlStatsSnapshot
}

// ControlStatsSnapshot represents a point-in-time snapshot of control plane stats.
type ControlStatsSnapshot struct {
	NodeID        string
	GRPCAddr      string
	RaftAddr      string
	RaftState     string
	RaftLeader    string
	ChainNodes    []ChainNodeInfo
	RegisteredCPs int
	TotalNodes    int
}

// ChainNodeInfo represents information about a node in the chain.
type ChainNodeInfo struct {
	NodeID  string
	Address string
	Role    string // HEAD, MIDDLE, TAIL, SINGLE
}

// Stats holds the statistics configuration and provider.
type Stats struct {
	startTime time.Time
	provider  ControlStatsProvider
}

// NewStats creates a new Stats instance.
func NewStats() *Stats {
	return &Stats{
		startTime: time.Now(),
	}
}

// SetProvider sets the stats provider (called after control plane is created).
func (s *Stats) SetProvider(provider ControlStatsProvider) {
	s.provider = provider
}

// GetUptime returns the elapsed time since the control plane started.
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
	sc.updateDisplay() // Initial display

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
	goroutines := runtime.NumGoroutine()

	// Format the Raft state with color
	var stateColor string
	switch snapshot.RaftState {
	case raft.Leader.String():
		stateColor = "green"
	case raft.Follower.String():
		stateColor = "blue"
	case raft.Candidate.String():
		stateColor = "yellow"
	default:
		stateColor = "gray"
	}

	// Build the stats display with fixed-width formatting to prevent jitter
	var display strings.Builder

	// Node ID, addresses, and Raft state (line 1)
	display.WriteString(fmt.Sprintf("[white]Control Plane:[-] [cyan]%-10s[-] [darkgray](gRPC: %-21s Raft: %-21s)[-]",
		snapshot.NodeID, snapshot.GRPCAddr, snapshot.RaftAddr))

	// Raft state and metrics (line 2)
	leaderDisplay := formatLeader(snapshot.RaftLeader)
	display.WriteString(fmt.Sprintf("\n[white]Raft:[-] [%s]%-10s[-] | [white]Leader:[-] %-10s | [white]Uptime:[-] [green]%-10s[-] | [white]Goroutines:[-] [cyan]%-4d[-]",
		stateColor, snapshot.RaftState, leaderDisplay, uptime, goroutines))

	// Chain header (line 3)
	display.WriteString(fmt.Sprintf("\n[white]=== Server Chain (%d nodes) ===[-]", snapshot.TotalNodes))

	// Chain visualization with bigger, boxed nodes (line 4)
	display.WriteString("\n")
	display.WriteString(formatChain(snapshot.ChainNodes))

	sc.app.QueueUpdateDraw(func() {
		sc.statsView.SetText(display.String())
	})
}

// formatLeader formats the leader display.
func formatLeader(leader string) string {
	if leader == "" {
		return "[gray]none[-]"
	}
	return leader
}

// formatChain creates a visual representation of the chain with bigger, boxed nodes.
func formatChain(nodes []ChainNodeInfo) string {
	if len(nodes) == 0 {
		return "[gray]empty[-]"
	}

	var chain strings.Builder

	for i, node := range nodes {
		// Determine color and label based on role
		var color string
		// var roleLabel string
		switch node.Role {
		case "HEAD":
			color = "green"
			// roleLabel = "HEAD"
		case "TAIL":
			color = "blue"
			// roleLabel = "TAIL"
		case "MIDDLE":
			color = "yellow"
			// roleLabel = "MIDDLE"
		case "SINGLE":
			color = "orange"
			// roleLabel = "SINGLE"
		default:
			color = "gray"
			// roleLabel = "UNKNOWN"
		}

		// Format node with box drawing and fixed width
		// Truncate node ID if too long
		nodeDisplay := node.NodeID
		if len(nodeDisplay) > 15 {
			nodeDisplay = nodeDisplay[:12] + "..."
		}
		// chain.WriteString(fmt.Sprintf("[%s]╔═══════════════╗[-]", color))
		// chain.WriteString(fmt.Sprintf(" [%s]║ %-6s      ║[-]", color, roleLabel))
		// chain.WriteString(fmt.Sprintf(" [%s]║ %-13s ║[-]", color, nodeDisplay))
		fmt.Fprintf(&chain, "[%s]%s[-]", color, node.Address)

		// Add arrow between nodes
		if i < len(nodes)-1 {
			chain.WriteString(" [white]=>[-] ")
		}
	}

	return chain.String()
}

// displayInitializing shows the initializing state.
func (sc *StatsCollector) displayInitializing() {
	display := "[white]Control Plane:[-] [gray]INITIALIZING[-]\n"
	display += fmt.Sprintf("[white]Uptime:[-] [green]%s[-]\n", sc.stats.GetUptime())
	display += "[white]Chain:[-] [gray]waiting...[-]"

	sc.app.QueueUpdateDraw(func() {
		sc.statsView.SetText(display)
	})
}
