package gui

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/denbal2292/razpravljalnica/pkg/client/shared"
	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (gc *guiClient) loadMessagesForCurrentTopic() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
		defer cancel()

		gc.clientMu.RLock()
		currentTopicId := gc.currentTopicId
		gc.clientMu.RUnlock()

		messages, err := shared.RetryFetch(ctx, gc.clients, func(ctx context.Context) (*pb.GetMessagesResponse, error) {
			return gc.clients.Reads.GetMessages(ctx, &pb.GetMessagesRequest{
				TopicId:       currentTopicId,
				FromMessageId: 0,
				Limit:         50,
			})
		})

		if err != nil {
			gc.displayStatus("Napaka pri pridobivanju sporočil", "red")
			return
		}

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
	// At the beggining, display the instructions - if no topic is selected
	if topicId <= 0 {
		gc.renderCenteredMessage(titleText)
		return
	}

	gc.clientMu.RLock()
	topic, topicOk := gc.topics[topicId]
	msgCache, msgOk := gc.messageCache[topicId]
	if !topicOk || !msgOk || msgCache == nil {
		gc.clientMu.RUnlock()
		gc.displayStatus("Napaka pri prikazovanju sporočil", "red")
		return
	}

	topicName := topic.Name
	// Copy messages and order to avoid race condition during iteration
	msgs := make(map[int64]*pb.Message)
	maps.Copy(msgs, msgCache.messages)
	order := make([]int64, len(msgCache.order))
	copy(order, msgCache.order)
	gc.clientMu.RUnlock()

	// Fetch missing users and cache them
	for _, msg := range msgs {
		userId := msg.UserId
		gc.clientMu.RLock()
		_, exists := gc.users[userId]
		gc.clientMu.RUnlock()

		if !exists {
			userName, err := gc.getUserName(userId)
			gc.clientMu.Lock()
			if err != nil {
				// This really should not happen
				gc.users[userId] = &pb.User{Id: userId, Name: "Neznan uporabnik"}
			} else {
				gc.users[userId] = &pb.User{Id: userId, Name: userName}
			}
			gc.clientMu.Unlock()
		}
	}

	gc.app.QueueUpdateDraw(func() {
		gc.messageView.SetTitle(fmt.Sprintf("Sporočila v [yellow]%s[-]", topicName))
		// Clear the screen before displaying messages
		gc.messageView.Clear()

		if len(order) > 0 {
			// Make sure to align left when there are messages
			gc.messageView.SetTextAlign(tview.AlignLeft)
			var messageText strings.Builder
			for _, msgId := range order {
				msg, ok := msgs[msgId]
				if !ok || msg == nil {
					// there is no message corresponding to the ID in the order
					// slice - this can happen if the message was deleted
					continue
				}

				gc.clientMu.RLock()
				user, ok := gc.users[msg.UserId]
				gc.clientMu.RUnlock()
				if !ok || user == nil {
					// User doesn't exist, skip message - for robustness
					continue
				}

				timestamp := msg.CreatedAt.AsTime().Local().Format("02-01-2006 15:04")
				regionId := fmt.Sprintf("msg-%d", msg.Id)

				messageLine := fmt.Sprintf(`["%s"][yellow]%s[-]: %s ([green]Všečki: %d[-]) [%s][""]`+"\n", regionId, user.Name, msg.Text, msg.Likes, timestamp)
				messageText.WriteString(messageLine)
			}
			// Set all text at once
			gc.messageView.SetText(messageText.String())
			gc.messageView.ScrollToEnd()
		} else {
			// No messages in this topic yet - write a centered message
			gc.renderCenteredMessage(fmt.Sprintf("[yellow]Znotraj %s še ni nobenih sporočil[-]", topicName))
		}
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

		message, err := shared.RetryFetch[*pb.Message](ctx, gc.clients, func(ctx context.Context) (*pb.Message, error) {
			return gc.clients.Writes.PostMessage(ctx, &pb.PostMessageRequest{
				UserId:  userId,
				TopicId: currentTopicId,
				Text:    message,
			})
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
			if _, exists := entry.messages[message.Id]; !exists {
				entry.order = append(entry.order, message.Id)
			}
			entry.messages[message.Id] = message
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

		_, err := shared.RetryFetch(ctx, gc.clients, func(ctx context.Context) (*emptypb.Empty, error) {
			return gc.clients.Writes.DeleteMessage(ctx, &pb.DeleteMessageRequest{
				UserId:    userId,
				TopicId:   topicId,
				MessageId: messageId,
			})
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

		// After deleting, select another message
		gc.app.QueueUpdateDraw(func() {
			gc.selectNearestMessage(topicId, messageId)
		})
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
		message, err := shared.RetryFetch(ctx, gc.clients, func(ctx context.Context) (*pb.Message, error) {
			return gc.clients.Writes.LikeMessage(ctx, &pb.LikeMessageRequest{
				UserId:    userId,
				TopicId:   topicId,
				MessageId: messageId,
			})
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

		updatedMessage, err := shared.RetryFetch(ctx, gc.clients, func(ctx context.Context) (*pb.Message, error) {
			return gc.clients.Writes.UpdateMessage(ctx, &pb.UpdateMessageRequest{
				TopicId:   topicId,
				UserId:    userId,
				MessageId: messageId,
				Text:      message,
			})
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

	// Get original input capture to restore later - so the modal focus
	// isn't messed up
	originalCapture := gc.app.GetInputCapture()
	closeModal := func() {
		gc.pages.RemovePage("modal")
		gc.app.SetFocus(gc.messageView)
		gc.app.SetInputCapture(originalCapture)
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
			closeModal()
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
		closeModal()
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
			// gc.pages.RemovePage("modal")
			closeModal()
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
		closeModal()
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

	// Set modal-specific input capture
	gc.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab, tcell.KeyDown:
			// Move to next focusable
			for i, focusable := range focusables {
				if focusable.HasFocus() {
					next := (i + 1) % len(focusables)
					gc.app.SetFocus(focusables[next])
					return nil
				}
			}
			gc.app.SetFocus(focusables[0])
			return nil
		case tcell.KeyBacktab, tcell.KeyUp:
			// Move to previous focusable
			for i, focusable := range focusables {
				if focusable.HasFocus() {
					prev := (i - 1 + len(focusables)) % len(focusables)
					gc.app.SetFocus(focusables[prev])
					return nil
				}
			}
			gc.app.SetFocus(focusables[len(focusables)-1])
			return nil
		case tcell.KeyEscape:
			// Close modal and restore original capture
			gc.pages.RemovePage("modal")
			gc.app.SetFocus(gc.messageView)
			gc.app.SetInputCapture(originalCapture)
			return nil
		}
		return event
	})

	// Use closeModal in your buttons
	likeButton.SetSelectedFunc(func() {
		gc.handleLikeMessage(messageId)
		closeModal()
	})

	gc.pages.AddPage("modal", flex, true, true)
	gc.app.SetFocus(focusables[0])
}

func (gc *guiClient) handleSubscriptionStream(topicId int64, msgEventStream grpc.ServerStreamingClient[pb.MessageEvent]) {
	// Wrapping this in a goroutine is not wanted since that makes connection closing harder.
	for {
		// TODO: this seems to block forever on new requests
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
		if msg == nil {
			continue
		}
		msgTopicId := msg.TopicId

		gc.clientMu.Lock()
		entry, ok := gc.messageCache[msgTopicId]
		if !ok {
			entry = &messageCacheEntry{
				messages: make(map[int64]*pb.Message),
				order:    make([]int64, 0),
			}
			gc.messageCache[msgTopicId] = entry
		}

		var deletedMsgId int64
		var wasCurrentlySelected bool

		switch msgEvent.Op {
		case pb.OpType_OP_POST:
			if _, exists := entry.messages[msg.Id]; !exists {
				entry.order = append(entry.order, msg.Id)
			}
			entry.messages[msg.Id] = msg
		case pb.OpType_OP_DELETE:
			deletedMsgId = msg.Id
			// Check if the deleted message was currently selected
			wasCurrentlySelected = (gc.selectedMessageId == msg.Id)
			delete(entry.messages, msg.Id)
			// We don't remove from the order slice - the ID just doesn't
			// exist - that is checked in updateMessageView when iterting
			// over the order slice
		case pb.OpType_OP_UPDATE, pb.OpType_OP_LIKE:
			entry.messages[msg.Id] = msg
		}
		gc.clientMu.Unlock()
		// Special care is taken on the other GUI update handlers to not redraw
		// twice
		gc.clientMu.RLock()
		currentTopicId := gc.currentTopicId
		gc.clientMu.RUnlock()

		if msgTopicId == currentTopicId {
			gc.updateMessageView(msgTopicId)

			// If a message was deleted and it was currently selected, select another
			if msgEvent.Op == pb.OpType_OP_DELETE && wasCurrentlySelected && deletedMsgId > 0 {
				gc.app.QueueUpdateDraw(func() {
					gc.selectNearestMessage(msgTopicId, deletedMsgId)
				})
			}
		} else {
			// Add a small notification that there are new messages in another topic
			gc.clientMu.RLock()
			// Se if it's already marked as having unread messages
			status := gc.unreadTopic[msgTopicId]
			gc.clientMu.RUnlock()

			if !status {
				gc.clientMu.Lock()
				gc.unreadTopic[msgTopicId] = true
				gc.clientMu.Unlock()
				gc.displayTopics()
			}
		}
	}
}

// selectNearestMessage selects the nearest valid message after a deletion
func (gc *guiClient) selectNearestMessage(topicId int64, deletedMessageId int64) {
	gc.clientMu.RLock()
	cache, ok := gc.messageCache[topicId]
	gc.clientMu.RUnlock()

	if !ok || cache == nil || len(cache.order) == 0 {
		return
	}

	// Find the position of the deleted message
	deletedPos := -1
	for i, id := range cache.order {
		if id == deletedMessageId {
			deletedPos = i
			break
		}
	}

	if deletedPos == -1 {
		// Message not found in order, just select first valid message
		gc.navigateMessages(0)
		return
	}

	// Set navigating flag to prevent modal from opening
	gc.clientMu.Lock()
	gc.isNavigating = true
	gc.clientMu.Unlock()

	defer func() {
		gc.clientMu.Lock()
		gc.isNavigating = false
		gc.clientMu.Unlock()
	}()

	// Try to find the next valid message starting from the deleted position
	for i := deletedPos; i < len(cache.order); i++ {
		msgId := cache.order[i]
		if _, exists := cache.messages[msgId]; exists {
			regionId := fmt.Sprintf("msg-%d", msgId)
			gc.messageView.Highlight(regionId)
			gc.messageView.ScrollToHighlight()
			return
		}
	}

	// If no next message, try to find previous valid message
	for i := deletedPos - 1; i >= 0; i-- {
		msgId := cache.order[i]
		if _, exists := cache.messages[msgId]; exists {
			regionId := fmt.Sprintf("msg-%d", msgId)
			gc.messageView.Highlight(regionId)
			gc.messageView.ScrollToHighlight()
			return
		}
	}

	// No valid messages left, clear highlight
	gc.messageView.Highlight()
}

func (gc *guiClient) navigateMessages(delta int) {
	gc.clientMu.RLock()
	topicId := gc.currentTopicId
	cache, ok := gc.messageCache[topicId]
	lastId := gc.lastSelectedMessageId
	gc.clientMu.RUnlock()

	if !ok || cache == nil || len(cache.order) == 0 {
		return
	}

	// Build list of valid (non-deleted) message indices
	validIndices := []int{}
	for i, msgId := range cache.order {
		if _, exists := cache.messages[msgId]; exists {
			validIndices = append(validIndices, i)
		}
	}

	if len(validIndices) == 0 {
		// No valid messages to select
		return
	}

	// Find current position in the valid indices
	highlights := gc.messageView.GetHighlights()
	var currentValidPos int = -1

	if len(highlights) > 0 {
		var currentMsgId int64
		fmt.Sscanf(highlights[0], "msg-%d", &currentMsgId)

		// Find this message's position in validIndices
		for i, id := range cache.order {
			if id == currentMsgId {
				// Find this index in validIndices
				for vp, vi := range validIndices {
					if vi == i {
						currentValidPos = vp
						break
					}
				}
				break
			}
		}
	} else if lastId > 0 {
		// Try to restore last selected message
		for i, id := range cache.order {
			if id == lastId {
				if _, exists := cache.messages[id]; exists {
					for vp, vi := range validIndices {
						if vi == i {
							currentValidPos = vp
							break
						}
					}
				}
				break
			}
		}
	}

	// Calculate next position
	var nextValidPos int
	if currentValidPos == -1 {
		// No current selection - choose first or last based on delta
		if delta >= 0 {
			nextValidPos = 0
		} else {
			nextValidPos = len(validIndices) - 1
		}
	} else {
		// Move from current position
		nextValidPos = currentValidPos + delta
		// Clamp to valid range
		if nextValidPos < 0 {
			nextValidPos = 0
		} else if nextValidPos >= len(validIndices) {
			nextValidPos = len(validIndices) - 1
		}
	}

	// Select the message at nextValidPos
	nextIndex := validIndices[nextValidPos]
	msgId := cache.order[nextIndex]
	regionId := fmt.Sprintf("msg-%d", msgId)

	gc.clientMu.Lock()
	gc.isNavigating = true
	gc.clientMu.Unlock()

	gc.messageView.Highlight(regionId)
	gc.messageView.ScrollToHighlight()

	gc.clientMu.Lock()
	gc.isNavigating = false
	gc.clientMu.Unlock()
}
