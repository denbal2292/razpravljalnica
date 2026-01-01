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
	"google.golang.org/grpc"
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

		gc.clientMu.Lock()
		if entry, ok := gc.messageCache[currentTopicId]; !ok {
			// Create new cache entry
			entry = &messageCacheEntry{
				messages: make(map[int64]*pb.Message),
				order:    make([]int64, 0),
			}
			gc.messageCache[currentTopicId] = entry
		}

		// gc.messageCache[currentTopicId].order = make([]int64, 0)
		order := make([]int64, 0)
		messagesMap := make(map[int64]*pb.Message)

		for _, message := range messages.Messages {
			order = append(order, message.Id)
			messagesMap[message.Id] = message
		}
		gc.messageCache[currentTopicId].messages = messagesMap
		gc.messageCache[currentTopicId].order = order

		gc.clientMu.Unlock()

		// Update the message view
		gc.updateMessageView(currentTopicId)
	}()
}

func (gc *guiClient) updateMessageView(topicId int64) {
	gc.clientMu.RLock()
	topic, topicOk := gc.topics[topicId]
	msgCache, msgOk := gc.messageCache[topicId]
	gc.clientMu.RUnlock()

	// The last condition should never happen
	if !topicOk || !msgOk || msgCache == nil {
		gc.displayStatus("Napaka pri prikazovanju sporočil", "red")
		return
	}

	topicName := topic.Name
	msgs := msgCache.messages
	order := msgCache.order

	gc.messageView.SetTitle(fmt.Sprintf("Sporočila v [yellow]%s[-]", topicName))

	// Clear the screen before displaying messages
	gc.messageView.Clear()

	users := make(map[int64]*pb.User)

	for _, msg := range msgs {
		userId := msg.UserId
		if _, exists := users[userId]; !exists {
			userName, err := gc.getUserName(userId)
			if err != nil {
				// This really should not happen
				users[userId] = &pb.User{Id: userId, Name: "Neznan uporabnik"}
			} else {
				users[userId] = &pb.User{Id: userId, Name: userName}
			}
		}
	}

	// Set the fetched users
	gc.clientMu.Lock()
	gc.users = users
	gc.clientMu.Unlock()

	gc.app.QueueUpdateDraw(func() {
		for _, msgId := range order {
			msg, ok := msgs[msgId]
			if !ok || msg == nil {
				// there is no message corresponding to the ID in the order
				// slice - this can happen if the message was deleted
				continue
			}

			user, ok := users[msg.UserId]
			if !ok || user == nil {
				// User doesn't exist, skip message - for robustness
				continue
			}

			timestamp := msg.CreatedAt.AsTime().Local().Format("02-01-2006 15:04")
			regionId := fmt.Sprintf("msg-%d", msg.Id)

			messageLine := fmt.Sprintf(`["%s"][yellow]%s[-]: %s ([green]Všečki: %d[-]) [%s][""]`+"\n", regionId, user.Name, msg.Text, msg.Likes, timestamp)
			gc.messageView.Write([]byte(messageLine))
		}
		gc.messageView.ScrollToEnd()
	})
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

		message, err := gc.clients.Writes.PostMessage(ctx, &pb.PostMessageRequest{
			UserId:  userId,
			TopicId: currentTopicId,
			Text:    message,
		})

		if err != nil {
			gc.displayStatus("Napaka pri pošiljanju sporočila", "red")
			return
		}

		gc.displayStatus("Sporočilo uspešno ustvarjeno", "green")

		// Clear the input field
		gc.app.QueueUpdateDraw(func() {
			gc.messageInput.SetText("")
		})

		// We now append the message to the cache
		gc.clientMu.Lock()

		// If we are subscribed to the current topic,
		// the message will arrive through the subscription stream
		// so we don't need to add it to the cache here
		subscribedToCurrentTopic, exists := gc.subscribedTopics[currentTopicId]

		if exists && subscribedToCurrentTopic {
			gc.clientMu.Unlock()
			return
		}

		if entry, ok := gc.messageCache[currentTopicId]; ok {
			entry.messages[message.Id] = message
			// This arrived later since we posted it
			entry.order = append(entry.order, message.Id)
		}
		gc.clientMu.Unlock()

		gc.updateMessageView(currentTopicId)
	}()
}

func (gc *guiClient) deleteMessage(messageId int64) {
	gc.clientMu.RLock()
	userId := gc.userId
	topicId := gc.currentTopicId
	gc.clientMu.RUnlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
		defer cancel()

		_, err := gc.clients.Writes.DeleteMessage(ctx, &pb.DeleteMessageRequest{
			UserId:    userId,
			TopicId:   topicId,
			MessageId: messageId,
		})

		if err != nil {
			gc.displayStatus("Napaka pri brisanju sporočila", "red")
			return
		}

		gc.displayStatus("Sporočilo uspešno izbrisano", "green")

		// Remove the message from the cache
		gc.clientMu.Lock()

		// If we are subscribed to the current topic,
		// the deletion will arrive through the subscription stream
		// so we don't need to update the cache here
		subscribedToCurrentTopic, exists := gc.subscribedTopics[topicId]

		if exists && subscribedToCurrentTopic {
			gc.clientMu.Unlock()
			return
		}

		if entry, ok := gc.messageCache[topicId]; ok {
			delete(entry.messages, messageId)
			// We don't remove from the order slice - the ID just doesn't
			// exist - that is checked in updateMessageView when iterting
			// over the order slice
		}
		gc.clientMu.Unlock()

		gc.updateMessageView(topicId)
	}()
}

func (gc *guiClient) likeMessage(messageId int64) {
	gc.clientMu.RLock()
	userId := gc.userId
	topicId := gc.currentTopicId
	gc.clientMu.RUnlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
		defer cancel()

		// We just replace the message in the cache with the updated one
		message, err := gc.clients.Writes.LikeMessage(ctx, &pb.LikeMessageRequest{
			UserId:    userId,
			TopicId:   topicId,
			MessageId: messageId,
		})

		if err != nil {
			gc.displayStatus("Napaka pri všečkanju sporočila", "red")
			return
		}

		gc.displayStatus("Sporočilo uspešno všečkano", "green")

		// Update the message in the cache
		gc.clientMu.Lock()

		// If we are subscribed to the current topic,
		// the like update will arrive through the subscription stream
		// so we don't need to update the cache here
		subscribedToCurrentTopic, exists := gc.subscribedTopics[topicId]

		if exists && subscribedToCurrentTopic {
			gc.clientMu.Unlock()
			return
		}

		if entry, ok := gc.messageCache[topicId]; ok {
			entry.messages[message.Id] = message
		}
		gc.clientMu.Unlock()

		gc.updateMessageView(topicId)
	}()
}

func (gc *guiClient) showMessageActionsModal(messageId int64) {
	gc.clientMu.RLock()
	userId := gc.userId
	gc.clientMu.RUnlock()

	if userId <= 0 {
		gc.displayStatus("Za interakcijo s sporočilom se moraš prijaviti", "red")
		return
	}

	// This should not happen
	if messageId <= 0 {
		gc.displayStatus("Neveljavno sporočilo", "red")
		return
	}

	likeButton := createButton(
		"Všeč mi je",
		tcell.ColorDarkGray,
		tcell.ColorGreen,
		tcell.ColorBlack,
		tcell.ColorWhite,
	)
	likeButton.SetSelectedFunc(func() {
		gc.likeMessage(messageId)
		// Close the modal
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
		gc.deleteMessage(messageId)
		// Close the modal
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
	buttons.
		SetBorder(true).
		SetTitle("Akcije").
		SetBorderColor(tcell.ColorWhite).
		SetBorderPadding(1, 1, 1, 1)

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

func (gc *guiClient) handleSubscriptionStream(topicId int64, msgEventStream grpc.ServerStreamingClient[pb.MessageEvent]) {
	// Wrapping this in a goroutine is not wanted since that makes connection closing harder.
	for {
		msgEvent, err := msgEventStream.Recv()

		if err != nil {
			gc.displayStatus("Prekinjena povezava za naročanje na temo", "red")

			// Mark the topic as unsubscribed
			gc.clientMu.Lock()
			gc.subscribedTopics[topicId] = false
			gc.clientMu.Unlock()

			// Update the topics display to show subscription status
			gc.displayTopics()
			return
		}

		msg := msgEvent.Message
		topicId := msg.TopicId

		gc.clientMu.Lock()
		entry, ok := gc.messageCache[topicId]
		if !ok {
			entry = &messageCacheEntry{
				messages: make(map[int64]*pb.Message),
				order:    make([]int64, 0),
			}
			gc.messageCache[topicId] = entry
		}
		switch msgEvent.Op {
		case pb.OpType_OP_POST:
			if msg != nil {
				entry.messages[msg.Id] = msg
				entry.order = append(entry.order, msg.Id)
			}
		case pb.OpType_OP_DELETE:
			if msg != nil {
				delete(entry.messages, msg.Id)
				// We don't remove from the order slice - the ID just doesn't
				// exist - that is checked in updateMessageView when iterting
				// over the order slice
			}
		case pb.OpType_OP_LIKE:
			if msg != nil {
				entry.messages[msg.Id] = msg
			}
		}
		gc.clientMu.Unlock()
		gc.updateMessageView(topicId)
	}
}
