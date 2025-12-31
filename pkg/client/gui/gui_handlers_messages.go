package gui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/denbal2292/razpravljalnica/pkg/client/shared"
	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (gc *guiClient) loadMessagesForCurrentTopic() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
		defer cancel()

		gc.clientMu.RLock()
		currentTopicId := gc.currentTopicId
		gc.clientMu.RUnlock()

		// This should be more dynamic
		messages, err := gc.clients.Reads.GetMessages(ctx, &pb.GetMessagesRequest{
			TopicId:       currentTopicId,
			FromMessageId: 0,
			Limit:         50,
		})

		if err != nil {
			gc.displayStatus("Napaka pri pridobivanju sporočil", "red")
			return
		}

		// Sort messages by createdAt ascending
		sort.Slice(messages.Messages, func(i, j int) bool {
			return messages.Messages[i].CreatedAt.AsTime().Before(messages.Messages[j].CreatedAt.AsTime())
		})

		// Fetch all unique users before updating the GUI
		users := make(map[int64]*pb.User)

		for _, message := range messages.Messages {
			userId := message.UserId

			// Fetch user only if not already fetched - for performance reasons
			if _, exists := users[userId]; !exists {
				username, err := gc.getUserName(userId)

				if err != nil {
					// Skip this message
					users[userId] = &pb.User{Name: "Neznan uporabnik"}
				} else {
					users[userId] = &pb.User{Name: username}
				}
			}
		}

		// Update the message view in the GUI
		gc.app.QueueUpdateDraw(func() {
			// Change the title
			gc.clientMu.RLock()
			currentTopicId := gc.topics[gc.currentTopicId].Name
			gc.clientMu.RUnlock()

			gc.messageView.SetTitle(fmt.Sprintf("Sporočila v [yellow]%s[-]", currentTopicId))

			// Clear the screen before displaying messages
			gc.messageView.Clear()

			// Prepare the message display
			for _, message := range messages.Messages {
				user, ok := users[message.UserId]

				// If user not found, skip the message
				if !ok {
					continue
				}

				// TODO: Make this nicer?
				timestamp := message.CreatedAt.AsTime().Local().Format("02-01-2006 15:04")
				regionId := fmt.Sprintf("msg-%d", message.Id)

				messageLine := fmt.Sprintf(`["%s"][yellow]%s[-]: %s ([green]Všečki: %d[-]) [%s][""]`+"\n", regionId, user.Name, message.Text, message.Likes, timestamp)
				gc.messageView.Write([]byte(messageLine))
			}
			gc.messageView.ScrollToEnd()
		})
	}()
}

func (gc *guiClient) handlePostMessage() {
	message := strings.TrimSpace(gc.messageInput.GetText())

	if message == "" {
		gc.displayStatus("Sporočilo ne sme biti prazno", "red")
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
		defer cancel()

		gc.clientMu.RLock()
		userId := gc.userId
		currentTopicId := gc.currentTopicId
		gc.clientMu.RUnlock()

		_, err := gc.clients.Writes.PostMessage(ctx, &pb.PostMessageRequest{
			UserId:  userId,
			TopicId: currentTopicId,
			Text:    message,
		})

		if err != nil {
			gc.displayStatus("Napaka pri pošiljanju sporočila", "red")
			return
		} else {
			gc.displayStatus("Sporočilo uspešno ustvarjeno", "green")
		}

		// Clear the input field after processing
		gc.app.QueueUpdateDraw(func() {
			gc.messageInput.SetText("")
			gc.loadMessagesForCurrentTopic()
		})
	}()
}

func (gc *guiClient) showMessageActionsModal(messageId int64) {
	likeButton := createButton(
		"Všeč mi je",
		tcell.ColorDarkGray,
		tcell.ColorGreen,
		tcell.ColorBlack,
		tcell.ColorWhite,
	)
	likeButton.SetSelectedFunc(func() {
		gc.pages.RemovePage("modal")
		// Unighlight all text to remove visual selection
		gc.messageView.Highlight()
	})

	deleteButton := createButton(
		"Izbriši",
		tcell.ColorDarkGray,
		tcell.ColorRed,
		tcell.ColorBlack,
		tcell.ColorWhite,
	)
	deleteButton.SetSelectedFunc(func() {
		gc.pages.RemovePage("modal")
		// Unighlight all text to remove visual selection
		gc.messageView.Highlight()
	})

	closeButton := createButton(
		"Zapri",
		tcell.ColorDarkGray,
		tcell.ColorGray,
		tcell.ColorBlack,
		tcell.ColorWhite,
	)
	closeButton.SetSelectedFunc(func() {
		gc.pages.RemovePage("modal")
		// Unighlight all text to remove visual selection
		gc.messageView.Highlight()
	})

	// Create button layout
	buttons := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(likeButton, 1, 0, true).
		AddItem(tview.NewBox(), 1, 0, false).
		AddItem(deleteButton, 1, 0, false).
		AddItem(tview.NewBox(), 1, 0, false).
		AddItem(closeButton, 1, 0, false)

	buttons.SetBorder(true).SetTitle("Akcije").SetBorderPadding(1, 1, 1, 1)

	// Center the modal
	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(buttons, 9, 1, true).
			AddItem(nil, 0, 1, false), 50, 1, true).
		AddItem(nil, 0, 1, false)

	// Define the order of focus
	btns := []*tview.Button{likeButton, deleteButton, closeButton}

	// We need this if we want different styles for different buttons
	buttons.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab, tcell.KeyDown:
			// Move to next button
			for i, btn := range btns {
				if btn.HasFocus() {
					next := (i + 1) % len(btns)
					gc.app.SetFocus(btns[next])
					return nil
				}
			}
		case tcell.KeyBacktab, tcell.KeyUp:
			// Move to previous button
			for i, btn := range btns {
				if btn.HasFocus() {
					prev := (i - 1 + len(btns)) % len(btns)
					gc.app.SetFocus(btns[prev])
					return nil
				}
			}
		}
		return event
	})
	gc.pages.AddPage("modal", flex, true, true)
	gc.app.SetFocus(likeButton)
}
