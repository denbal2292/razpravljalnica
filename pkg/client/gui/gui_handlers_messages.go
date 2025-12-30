package gui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/denbal2292/razpravljalnica/pkg/client/shared"
	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
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
