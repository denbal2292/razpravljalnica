package gui

import (
	"github.com/denbal2292/razpravljalnica/pkg/client/shared"
	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type guiClient struct {
	app              *tview.Application
	topicsList       *tview.List
	newUserInput     *tview.InputField
	loggedInUserView *tview.TextView
	newTopicInput    *tview.InputField
	messageView      *tview.TextView
	messageInput     *tview.InputField
	statusBar        *tview.TextView

	// Keep reference to the connections
	clients *shared.ClientSet

	// Extra information about the client state
	userId int64

	// Current selected topic ID and list of topic IDs
	currentTopicId int64
	topics         map[int64]*pb.Topic
	topicOrder     []int64
}

func StartGUIClient(clients *shared.ClientSet) {
	// Set initial focus to the input field
	gc := newGuiClient(clients)

	if err := gc.app.Run(); err != nil {
		panic(err)
	}
}

func newGuiClient(clients *shared.ClientSet) *guiClient {
	// Initialize GUI client structure
	gc := &guiClient{
		app:              tview.NewApplication(),
		newUserInput:     tview.NewInputField(),
		loggedInUserView: tview.NewTextView(),
		topicsList:       tview.NewList(),
		newTopicInput:    tview.NewInputField(),
		messageView:      tview.NewTextView(),
		messageInput:     tview.NewInputField(),
		statusBar:        tview.NewTextView(),

		clients: clients,
	}
	gc.app.EnableMouse(true)

	gc.setupWidgets()
	gc.setupLayout()

	// Once it's setup, refresh the topics list
	gc.refreshTopics(true)

	return gc
}

// setupWidgets configures the individual widgets
func (gc *guiClient) setupWidgets() {
	// Configure topics list
	gc.topicsList.
		ShowSecondaryText(false).
		SetBorder(true).
		SetTitle("Teme")

	gc.topicsList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(gc.topics) {
			topicId := gc.topicOrder[index]
			gc.handleSelectTopic(topicId)
		}
	})

	// Configure new user input
	gc.newUserInput.
		SetLabel("Nov uporabnik > ").
		SetLabelColor(tcell.ColorGreen).
		SetFieldBackgroundColor(tcell.ColorDarkGrey).
		SetFieldTextColor(tcell.ColorBlack).
		SetFieldWidth(0)

	gc.newUserInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			gc.handleCreateUser()
		}
	})

	gc.loggedInUserView.
		SetDynamicColors(true).
		SetTextAlign(tview.AlignRight).
		SetText("[blue]Nisi prijavljen[-]")

	// Configure new topic input
	gc.newTopicInput.
		SetLabel("Nova tema > ").
		SetLabelColor(tcell.ColorGreen).
		SetFieldBackgroundColor(tcell.ColorDarkGrey).
		SetFieldTextColor(tcell.ColorBlack).
		SetFieldWidth(0)

	// Handle new topic creation on Enter key
	gc.newTopicInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			gc.handleCreateTopic()
		}
	})

	// Configure status bar
	gc.statusBar.
		SetDynamicColors(true). // Allow inline color changes
		SetLabel("[white]Status:[-] ").
		SetText("[green]Povezan").
		SetTextAlign(tview.AlignLeft)

	// Configure messages view
	gc.messageView.
		SetDynamicColors(true).
		SetWordWrap(true).
		SetBorder(true).
		SetTitle("Sporočila")

	// Configure message input
	gc.messageInput.
		SetLabel("Vnesi sporočilo > ").
		SetLabelColor(tcell.ColorGreen).
		SetFieldBackgroundColor(tcell.ColorDarkGrey).
		SetFieldTextColor(tcell.ColorBlack).
		SetFieldWidth(0)

	// Handle message posting on Enter key
	gc.messageInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			gc.handlePostMessage()
		}
	})
}

func (gc *guiClient) setupLayout() {
	// Create a grid layout
	grid := tview.NewGrid().
		SetRows(1, 0, 1).     // Header, main area, status bar
		SetColumns(30, 1, 0). // Topics, spacer, messages
		SetBorders(false)     // No borders between grid cells

	topicsColumn := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(gc.topicsList, 0, 1, true).
		AddItem(gc.newTopicInput, 1, 0, false)

	messagesColumn := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(gc.messageView, 0, 1, false).
		AddItem(gc.messageInput, 1, 0, true)

	grid.AddItem(gc.newUserInput, 0, 0, 1, 1, 0, 0, false)
	grid.AddItem(gc.loggedInUserView, 0, 2, 1, 1, 0, 0, false)

	grid.AddItem(topicsColumn, 1, 0, 1, 1, 0, 0, true)
	grid.AddItem(tview.NewBox(), 1, 1, 1, 1, 0, 0, false)
	grid.AddItem(messagesColumn, 1, 2, 1, 1, 0, 0, true)

	// Bottom Row: Status
	grid.AddItem(gc.statusBar, 2, 0, 1, 3, 0, 0, false)

	// Set the Grid as root
	gc.app.SetRoot(grid, true)

	gc.app.SetFocus(gc.topicsList)
}
