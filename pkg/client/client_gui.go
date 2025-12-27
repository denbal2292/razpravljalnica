package client

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type guiClient struct {
	app           *tview.Application
	topicsList    *tview.List
	newTopicInput *tview.InputField
	messageView   *tview.TextView
	messageInput  *tview.InputField
	statusBar     *tview.TextView

	// Also keep reference to the connections
	clients *clientSet
}

func startGUIClient(clients *clientSet) {
	app := tview.NewApplication()
	app.EnableMouse(true)

	// Set initial focus to the input field
	gc := newGuiClient(clients)
	gc.app.SetFocus(gc.newTopicInput)

	if err := gc.app.Run(); err != nil {
		panic(err)
	}
}

func newGuiClient(clients *clientSet) *guiClient {
	// Initialize GUI client structure
	gc := &guiClient{
		app:           tview.NewApplication(),
		topicsList:    tview.NewList(),
		newTopicInput: tview.NewInputField(),
		messageView:   tview.NewTextView(),
		messageInput:  tview.NewInputField(),
		statusBar:     tview.NewTextView(),

		clients: clients,
	}

	gc.app.EnableMouse(true)
	gc.setupWidgets()
	gc.setupLayout()

	// Once it's setup, refresh the topics list
	gc.refreshTopics()

	return gc
}

// setupWidgets configures the individual widgets
func (gc *guiClient) setupWidgets() {
	// Configure topics list
	gc.topicsList.
		ShowSecondaryText(false).
		SetBorder(true).
		SetTitle("Teme")

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
		SetDynamicColors(false).
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
}

// setupLayout arranges the widgets into the main layout
func (gc *guiClient) setupLayout() {
	// Layout for topics column
	topicsColumn := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(gc.topicsList, 0, 1, true).
		AddItem(gc.newTopicInput, 1, 0, false)

	// Layout for messages column
	messageInputContainer := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(gc.messageInput, 1, 0, true).
		AddItem(nil, 0, 1, false)

	// Combine message view and input into messages column
	messagesColumn := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(gc.messageView, 0, 1, false).
		AddItem(messageInputContainer, 1, 0, true)

	// Combine topics and messages into the main screen
	messagesScreen := tview.NewFlex().
		AddItem(topicsColumn, 30, 1, false).
		AddItem(nil, 1, 0, false).
		AddItem(messagesColumn, 0, 3, true)

	// Set the main layout
	mainLayout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(messagesScreen, 0, 1, true).
		AddItem(gc.statusBar, 1, 0, false)

	gc.app.SetRoot(mainLayout, true)
}
