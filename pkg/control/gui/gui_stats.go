package gui

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
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
	app                *tview.Application
	statsView          *tview.TextView
	chainView          *tview.Flex
	chainViewContainer *tview.Frame
	stats              *Stats
	ticker             *time.Ticker
	stopChan           chan struct{}
}

// NewStatsCollector creates a new stats collector that updates the display.
func NewStatsCollector(app *tview.Application, view *tview.TextView, chainView *tview.Flex, chainViewContainer *tview.Frame, stats *Stats) *StatsCollector {
	return &StatsCollector{
		app:                app,
		statsView:          view,
		stats:              stats,
		chainView:          chainView,
		chainViewContainer: chainViewContainer,
		stopChan:           make(chan struct{}),
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

	// Node ID, addresses, and Raft state
	fmt.Fprintf(&display, "[white]Control Plane:[-] [cyan]%-10s[-] [darkgray](gRPC: %-21s Raft: %-21s)[-]",
		snapshot.NodeID, snapshot.GRPCAddr, snapshot.RaftAddr)

	// Raft state and metrics
	leaderDisplay := formatLeader(snapshot.RaftLeader)
	fmt.Fprintf(&display, "\n[white]Raft:[-] [%s]%-10s[-] | [white]Leader:[-] %-10s | [white]Uptime:[-] [green]%-10s[-] | [white]Goroutines:[-] [cyan]%-4d[-]",
		stateColor, snapshot.RaftState, leaderDisplay, uptime, goroutines)

	// fmt.Fprintf(&display, "\n[white]=== Server Chain (%d nodes) ===[-]", snapshot.TotalNodes)

	// Chain visualization with bigger, boxed nodes
	fmt.Fprintf(&display, "\n")

	// display.WriteString(formatChain(snapshot.ChainNodes))
	sc.app.QueueUpdateDraw(func() {
		sc.statsView.SetText(display.String())
	})

	sc.updateChainNodesView(snapshot.ChainNodes)
}

// formatLeader formats the leader display.
func formatLeader(leader string) string {
	if leader == "" {
		return "[gray]none[-]"
	}
	return leader
}

func (sc *StatsCollector) updateChainNodesView(nodes []ChainNodeInfo) {
	sc.chainViewContainer.SetTitle(fmt.Sprintf("Server chain ([yellow]%d[-] nodes)", len(nodes)))
	// Clear existing items
	sc.chainView.Clear()

	sc.chainView.AddItem(nil, 0, 1, false)
	for i, node := range nodes {
		var color tcell.Color
		var roleLabel string
		switch node.Role {
		case "HEAD":
			color = tcell.ColorGreen
			roleLabel = "HEAD"
		case "TAIL":
			color = tcell.ColorBlue
			roleLabel = "TAIL"
		case "MIDDLE":
			color = tcell.ColorYellow
			roleLabel = "MIDDLE"
		case "SINGLE":
			color = tcell.ColorOrange
			roleLabel = "SINGLE"
		default:
			color = tcell.ColorDarkGray
			roleLabel = "UNKNOWN"
		}

		view := tview.NewTextView().
			SetTextAlign(tview.AlignCenter).
			SetDynamicColors(true)

		view.SetBorder(true).SetBorderColor(color)
		view.SetText(fmt.Sprintf("%s\n\n[gray]%s[-]", roleLabel, node.Address))

		sc.chainView.AddItem(view, 12, 1, false)
		if i < len(nodes)-1 {
			sc.chainView.AddItem(createArrow(), 5, 1, false)
		}
	}
	sc.chainView.AddItem(nil, 0, 1, false)
}

func createArrow() *tview.TextView {
	return tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetText("\n\n ==>")
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
