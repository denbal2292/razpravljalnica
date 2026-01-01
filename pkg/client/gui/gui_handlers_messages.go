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

func (gc *guiClient) handleDeleteMessage(messageId int64) {
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
		gc.selectedMessageId = 0
		gc.clientMu.Unlock()

		gc.app.QueueUpdateDraw(func() {
			gc.messageView.Highlight()
		})

		gc.updateMessageView(topicId)
	}()
}

func (gc *guiClient) handleLikeMessage(messageId int64) {
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

func (gc *guiClient) handleUpdateMessage(messageId int64, editInput *tview.InputField) {
	message := strings.TrimSpace(editInput.GetText())

	if message == "" {
		gc.displayStatus("Sporočilo ne sme biti prazno", "red")
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
		defer cancel()

		gc.clientMu.RLock()
		userId := gc.userId
		topicId := gc.currentTopicId
		gc.clientMu.RUnlock()

		updatedMessage, err := gc.clients.Writes.UpdateMessage(ctx, &pb.UpdateMessageRequest{
			TopicId:   topicId,
			UserId:    userId,
			MessageId: messageId,
			Text:      message,
		})

		if err != nil {
			gc.displayStatus("Napaka pri urejanju sporočila", "red")
			return
		}

		gc.displayStatus("Sporočilo uspešno urejeno", "green")

		gc.clientMu.Lock()

		subscribedToCurrentTopic, exists := gc.subscribedTopics[topicId]

		if exists && subscribedToCurrentTopic {
			gc.clientMu.Unlock()
			return
		}

		if entry, ok := gc.messageCache[topicId]; ok {
			entry.messages[updatedMessage.Id] = updatedMessage
		}
		gc.clientMu.Unlock()

		gc.updateMessageView(topicId)
	}()
}

func (gc *guiClient) showMessageActionsModal(messageId int64) {
	gc.clientMu.RLock()
	userId := gc.userId
	var messageAuthorId int64
	entry, ok := gc.messageCache[gc.currentTopicId]
	if ok && entry != nil {
		if entry.messages != nil {
			if msg, exists := entry.messages[messageId]; exists {
				messageAuthorId = msg.UserId
			} else {
				gc.clientMu.RUnlock()
				gc.displayStatus("Sporočilo ne obstaja", "red")
				return
			}
		}
	}
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

	// Check if the current user is the author of the message
	isAuthor := (userId == messageAuthorId)

	// Build items conditionally
	items := tview.NewFlex().
		SetDirection(tview.FlexRow)

	focusables := []tview.Primitive{}
	// Edit option for author
	var editInput *tview.InputField
	if isAuthor {
		editInput = tview.NewInputField().
			SetLabel("Uredi > ").
			SetLabelColor(tcell.ColorGreen).
			SetFieldTextColor(tcell.ColorBlack).
			SetFieldWidth(0).
			SetFieldBackgroundColor(tcell.ColorDarkGray)

		// Handle edit on Enter key
		editInput.SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter {
				gc.handleUpdateMessage(messageId, editInput)
			}
			gc.pages.RemovePage("modal")
		})
		items.AddItem(tview.NewBox(), 1, 0, false)
		items.AddItem(editInput, 1, 0, true)
		focusables = append(focusables, editInput)
	}

	// Like option for everyone
	likeButton := createButton(
		"Všeč mi je",
		tcell.ColorDarkGray,
		tcell.ColorGreen,
		tcell.ColorBlack,
		tcell.ColorWhite,
	)
	likeButton.SetSelectedFunc(func() {
		gc.handleLikeMessage(messageId)
		// Close the modal
		gc.pages.RemovePage("modal")
	})
	items.AddItem(tview.NewBox(), 1, 0, false)
	items.AddItem(likeButton, 1, 0, false)
	focusables = append(focusables, likeButton)

	// Delete option for author
	if isAuthor {
		deleteButton := createButton(
			"Izbriši",
			tcell.ColorDarkGray,
			tcell.ColorRed,
			tcell.ColorBlack,
			tcell.ColorWhite,
		)
		deleteButton.SetSelectedFunc(func() {
			gc.handleDeleteMessage(messageId)
			// Close the modal
			gc.pages.RemovePage("modal")
		})
		items.AddItem(tview.NewBox(), 1, 0, false)
		items.AddItem(deleteButton, 1, 0, false)
		focusables = append(focusables, deleteButton)
	}

	// Close button for everyone
	closeButton := createButton(
		"Zapri",
		tcell.ColorDarkGray,
		tcell.ColorGray,
		tcell.ColorBlack,
		tcell.ColorWhite,
	)
	closeButton.SetSelectedFunc(func() {
		gc.pages.RemovePage("modal")
	})
	items.AddItem(tview.NewBox(), 1, 0, false)
	items.AddItem(closeButton, 1, 0, false)
	items.AddItem(tview.NewBox(), 1, 0, false)
	focusables = append(focusables, closeButton)

	paddedItems := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(tview.NewBox(), 1, 0, false).
		AddItem(items, 0, 1, true).
		AddItem(tview.NewBox(), 1, 0, false)

	paddedItems.SetBorder(true).SetTitle("Akcije")

	// Center the modal
	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(paddedItems, len(focusables)*2+3, 1, true).
			AddItem(nil, 0, 1, false), 50, 1, false).
		AddItem(nil, 0, 1, false)

	// We need this if we want different styles for different buttons
	items.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			// Close the modal with Escape key
			gc.pages.RemovePage("modal")
			return nil

		case tcell.KeyDown:
			// Move to next button
			for i, focusable := range focusables {
				if focusable.HasFocus() {
					next := (i + 1) % len(focusables)
					gc.app.SetFocus(focusables[next])
					return nil
				}
			}
		case tcell.KeyUp:
			// Move to previous button
			for i, focusable := range focusables {
				if focusable.HasFocus() {
					prev := (i - 1 + len(focusables)) % len(focusables)
					gc.app.SetFocus(focusables[prev])
					return nil
				}
			}
		}
		return event
	})

	gc.pages.AddPage("modal", flex, true, true)
	gc.app.SetFocus(focusables[0])
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
		case pb.OpType_OP_UPDATE, pb.OpType_OP_LIKE:
			if msg != nil {
				entry.messages[msg.Id] = msg
			}
		}
		gc.clientMu.Unlock()
		// Special care is taken on the other GUI update handlers to not redraw
		// twice
		gc.clientMu.RLock()
		currentTopicId := gc.currentTopicId
		gc.clientMu.RUnlock()

		if topicId == currentTopicId {
			gc.updateMessageView(topicId)
		}
	}
}

func (gc *guiClient) navigateMessages(delta int) {
	gc.clientMu.RLock()
	topicId := gc.currentTopicId
	cache, ok := gc.messageCache[topicId]
	gc.clientMu.RUnlock()

	if !ok || len(cache.order) == 0 {
		return
	}

	highlights := gc.messageView.GetHighlights()

	// Find the index (position) of the currently highlighted message
	var currentIndex int = -1

	if len(highlights) > 0 {
		var currentMsgId int64
		fmt.Sscanf(highlights[0], "msg-%d", &currentMsgId)

		for i, id := range cache.order {
			if id == currentMsgId {
				currentIndex = i
				break
			}
		}
	}

	var nextIndex int
	if currentIndex == -1 {
		if delta > 0 {
			nextIndex = 0
		} else {
			nextIndex = len(cache.order) - 1
		}
	} else {
		nextIndex = currentIndex + delta
		if nextIndex < 0 {
			nextIndex = 0
		} else if nextIndex >= len(cache.order) {
			nextIndex = len(cache.order) - 1
		}
	}

	if nextIndex != currentIndex || (currentIndex == -1 && len(cache.order) > 0) {
		// Find the next valid (non-deleted) message
		for {
			if nextIndex < 0 || nextIndex >= len(cache.order) {
				break
			}

			msgId := cache.order[nextIndex]
			// Check if message still exists (not deleted)
			if _, exists := cache.messages[msgId]; exists {
				regionId := fmt.Sprintf("msg-%d", msgId)

				gc.clientMu.Lock()
				gc.isNavigating = true
				gc.clientMu.Unlock()

				gc.messageView.Highlight(regionId)
				gc.messageView.ScrollToHighlight()

				gc.clientMu.Lock()
				gc.isNavigating = false
				gc.clientMu.Unlock()
				return
			}
			// Message was deleted, try next/previous
			if delta > 0 {
				nextIndex++
			} else {
				nextIndex--
			}
		}
	}
}
