package gui

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/rivo/tview"
)

// Stats holds the current statistics for the server node.
type Stats struct {
	mu sync.RWMutex

	startTime       time.Time
	eventsProcessed int
	messagesReplied int

	// General node info
	nodeId   string
	nodeAddr string
	role     string // HEAD, MIDDLE, TAIL, or SINGLE

	// Chain information
	predAddr string
	succAddr string
	cpAddr   string // Control Plane address

	// Connection status
	connected bool
}

// NewStats creates a new Stats instance with the given initial values.
func NewStats(nodeId, nodeAddr, cpAddr string) *Stats {
	return &Stats{
		startTime: time.Now(),
		nodeId:    nodeId,
		nodeAddr:  nodeAddr,
		cpAddr:    cpAddr,
		role:      "INITIALIZING",
		connected: false,
	}
}

// IncrementEvents increments the events processed counter.
func (s *Stats) IncrementEvents() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.eventsProcessed++
}

// IncrementMessages increments the messages counter.
func (s *Stats) IncrementMessages() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messagesReplied++
}

// DecrementMessages decrements the messages counter.
func (s *Stats) DecrementMessages() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messagesReplied--
}

// SetRole updates the current role of the node.
func (s *Stats) SetRole(role string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.role = role
}

// SetPredecessor updates the predecessor address.
func (s *Stats) SetPredecessor(addr string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.predAddr = addr
}

// SetSuccessor updates the successor address.
func (s *Stats) SetSuccessor(addr string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.succAddr = addr
}

// SetConnected updates the connection status.
func (s *Stats) SetConnected(connected bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.connected = connected
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

// run is the main loop that updates stats every second.
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
	sc.stats.mu.RLock()
	role := sc.stats.role
	nodeId := sc.stats.nodeId
	nodeAddr := sc.stats.nodeAddr
	predAddr := sc.stats.predAddr
	succAddr := sc.stats.succAddr
	cpAddr := sc.stats.cpAddr
	connected := sc.stats.connected
	numEvents := sc.stats.eventsProcessed
	numMessages := sc.stats.messagesReplied
	sc.stats.mu.RUnlock()

	uptime := sc.stats.GetUptime()
	events := numEvents
	messages := numMessages
	// Get number of goroutines - interesting to look at
	goroutines := runtime.NumGoroutine()

	// Format the role with color
	var roleColor string
	switch role {
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
		nodeId, nodeAddr, roleColor, role)

	// Uptime and metrics
	display += fmt.Sprintf("\n[white]Uptime:[-] [green]%s[-] | [white]Events:[-] [yellow]%d[-] | [white]Messages:[-] [yellow]%d[-] | [white]Goroutines:[-] [cyan]%d[-]",
		uptime, events, messages, goroutines)

	// Other chain info
	predDisplay := predAddr
	if predDisplay == "" {
		predDisplay = "[gray]none[-]"
	} else {
		predDisplay = "[green]" + predDisplay + "[-]"
	}

	succDisplay := succAddr
	if succDisplay == "" {
		succDisplay = "[gray]none[-]"
	} else {
		succDisplay = "[green]" + succDisplay + "[-]"
	}

	connStatus := "[red]disconnected[-]"
	if connected {
		connStatus = "[green]connected[-]"
	}

	display += fmt.Sprintf("\n[white]Pred:[-] %s | [white]Succ:[-] %s | [white]CP:[-] [blue]%s[-] %s",
		predDisplay, succDisplay, cpAddr, connStatus)

	sc.app.QueueUpdateDraw(func() {
		sc.statsView.SetText(display)
	})
}
