package gui

import (
	"github.com/denbal2292/razpravljalnica/pkg/client/shared"
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

	// Keep reference to the connections
	clients *shared.ClientSet

	// Extra information about the client state
	userId int64

	// Current selected topic ID and list of topic IDs
	currentTopicId int64
	topicIds       []int64
}

func StartGUIClient(clients *shared.ClientSet) {
	app := tview.NewApplication()
	app.EnableMouse(true)

	// Set initial focus to the input field
	gc := newGuiClient(clients)
	gc.app.SetFocus(gc.topicsList)

	if err := gc.app.Run(); err != nil {
		panic(err)
	}
}

func newGuiClient(clients *shared.ClientSet) *guiClient {
	// Initialize GUI client structure
	gc := &guiClient{
		app:           tview.NewApplication(),
		topicsList:    tview.NewList(),
		newTopicInput: tview.NewInputField(),
		messageView:   tview.NewTextView(),
		messageInput:  tview.NewInputField(),
		statusBar:     tview.NewTextView(),

		// Hardcode for now
		userId:  1,
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
		if index >= 0 && index < len(gc.topicIds) {
			gc.handleSelectTopic(gc.topicIds[index])
		}
	})

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
