package gui

import (
	"fmt"
	"sync"

	"github.com/denbal2292/razpravljalnica/pkg/client/shared"
	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// messageCacheEntry holds cached messages for a topic
type messageCacheEntry struct {
	messages map[int64]*pb.Message
	order    []int64
}

type guiClient struct {
	clientMu sync.RWMutex

	app              *tview.Application
	pages            *tview.Pages
	topicsList       *tview.List
	newUserInput     *tview.InputField
	loggedInUserView *tview.TextView
	logInUserInput   *tview.InputField
	newTopicInput    *tview.InputField
	messageView      *tview.TextView
	messageInput     *tview.InputField
	statusBar        *tview.TextView
	modal            *tview.Modal

	// Keep reference to the connections
	clients *shared.ClientSet

	// Extra information about the client state
	userId int64
	users  map[int64]*pb.User

	// Current selected topic ID and list of topic IDs
	currentTopicId int64
	topics         map[int64]*pb.Topic
	topicOrder     []int64
	// Topics we are subscribed to
	subscribedTopics map[int64]bool

	// Messages
	selectedMessageId int64
	messageCache      map[int64]*messageCacheEntry // topicId -> messages

	isNavigating bool
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
		pages:            tview.NewPages(),
		newUserInput:     tview.NewInputField(),
		loggedInUserView: tview.NewTextView(),
		logInUserInput:   tview.NewInputField(),
		topicsList:       tview.NewList(),
		newTopicInput:    tview.NewInputField(),
		messageView:      tview.NewTextView(),
		messageInput:     tview.NewInputField(),
		statusBar:        tview.NewTextView(),
		modal:            tview.NewModal(),

		subscribedTopics: make(map[int64]bool),

		clients: clients,

		// Messages
		messageCache: make(map[int64]*messageCacheEntry),
	}
	gc.app.EnableMouse(true)

	// Setup widgets and layout
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

	gc.topicsList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		if index >= 0 && index < len(gc.topics) {
			gc.clientMu.RLock()
			topicId := gc.topicOrder[index]
			gc.clientMu.RUnlock()

			gc.handleSelectTopic(topicId)
		}
	})

	gc.topicsList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'r' || event.Rune() == 'R' {
			// Refresh topics on 'r' key
			gc.refreshTopics()
			return nil
		} else if event.Rune() == 's' || event.Rune() == 'S' {
			gc.handleTopicSubscription()
			return nil
		}

		return event
	})

	// Configure new user input
	gc.newUserInput.
		SetLabel("Nov uporabnik > ").
		SetLabelColor(tcell.ColorGreen).
		SetFieldBackgroundColor(tcell.ColorDarkGrey).
		SetFieldTextColor(tcell.ColorBlack).
		SetFieldWidth(14)

	gc.newUserInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			gc.handleCreateUser()
		}
	})

	gc.logInUserInput.
		SetLabel("Prijava v uporabnika(ID) > ").
		SetLabelColor(tcell.ColorGreen).
		SetFieldBackgroundColor(tcell.ColorDarkGrey).
		SetFieldTextColor(tcell.ColorBlack).
		SetFieldWidth(14)

	gc.logInUserInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			gc.handleLogInUser()
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
		SetTextAlign(tview.AlignLeft).
		SetLabel("[white]Status:[-] ").
		SetText("[green]Povezan")

	// Configure messages view
	gc.messageView.
		SetDynamicColors(true).
		SetRegions(true).
		SetScrollable(true).
		SetWordWrap(true).
		SetBorder(true).
		SetTitle("Sporočila")

	// This is called when the highlighted region changes
	// added holds the the region ids od the highlighted regions,
	// removed holds the region ids that were unhighlighted,
	// remaining holds the region ids that are still highlighted,
	// This will be useful for setting up click events on messages
	gc.messageView.SetHighlightedFunc(func(added, removed, remaining []string) {
		if len(added) > 0 {
			regionId := added[0]
			var messageId int64
			// Extract message ID from region ID
			fmt.Sscanf(regionId, "msg-%d", &messageId)
			// Store the selected message ID
			gc.clientMu.Lock()
			gc.selectedMessageId = messageId
			navigating := gc.isNavigating
			gc.clientMu.Unlock()

			// Handle message click
			if !navigating {
				gc.showMessageActionsModal(messageId)
			}
		} else {
			gc.clientMu.Lock()
			gc.selectedMessageId = 0
			gc.clientMu.Unlock()
		}
	})

	gc.messageView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyUp {
			gc.navigateMessages(-1)
			return nil
		} else if event.Key() == tcell.KeyDown {
			gc.navigateMessages(1)
			return nil
		} else if event.Key() == tcell.KeyEnter {
			gc.clientMu.RLock()
			msgId := gc.selectedMessageId
			gc.clientMu.RUnlock()
			if msgId > 0 {
				gc.showMessageActionsModal(msgId)
			}
			return nil
		}

		// Refresh messages on 'r' key
		if event.Rune() == 'r' || event.Rune() == 'R' {
			gc.loadMessagesForCurrentTopic()
			return nil
		}

		return event
	})

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
	grid := tview.NewGrid().
		SetRows(1, 0, 1).     // Header, Main, Status
		SetColumns(30, 1, 0). // Topics, Spacer, Messages (0 = flexible)
		SetBorders(false)

	userCredentialsColumn := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(gc.newUserInput, 30, 0, false).
		AddItem(tview.NewBox(), 2, 0, false).
		AddItem(gc.logInUserInput, 40, 0, false).
		AddItem(gc.loggedInUserView, 0, 1, false)

	// Topics Column
	topicsColumn := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(gc.topicsList, 0, 1, true).
		AddItem(gc.newTopicInput, 1, 0, false)

	// Messages Column
	messagesColumn := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(gc.messageView, 0, 1, false).
		AddItem(gc.messageInput, 1, 0, true)

	// User credentials header
	grid.AddItem(userCredentialsColumn, 0, 2, 1, 1, 0, 0, false)
	grid.AddItem(
		tview.NewTextView().SetDynamicColors(true).SetText("[blue]RAZPRAVLJALNICA[-]"),
		0, 0, 1, 1, 0, 0, false,
	)

	grid.AddItem(topicsColumn, 1, 0, 1, 1, 0, 0, true)
	grid.AddItem(messagesColumn, 1, 2, 1, 1, 0, 0, true)
	grid.AddItem(gc.statusBar, 2, 0, 1, 3, 0, 0, false)

	// Add main layout as a page
	gc.pages.AddPage("main", grid, true, true)

	gc.app.SetRoot(gc.pages, true)
}
