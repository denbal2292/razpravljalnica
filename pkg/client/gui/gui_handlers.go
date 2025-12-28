package gui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/denbal2292/razpravljalnica/pkg/client/shared"
	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/emptypb"
)

// handleCreateTopic processes the creation of a new topic
func (gc *guiClient) handleCreateTopic() {
	// Extract the topic name from the input field
	topicName := strings.TrimSpace(gc.newTopicInput.GetText())

	if topicName == "" {
		gc.displayStatus("Ime teme ne sme biti prazno", "red")
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
		defer cancel()

		// We ignore the response - and refetch the topic in order for
		// the state to be consistent
		_, err := gc.clients.Writes.CreateTopic(ctx, &pb.CreateTopicRequest{
			Name: topicName,
		})

		if err != nil {
			gc.displayStatus("Napaka pri ustvarjanju teme", "red")
			return
		} else {
			gc.displayStatus("Tema uspešno ustvarjena", "green")
		}

		// Clear the input field after processing
		gc.newTopicInput.SetText("")
		// Set focus back to topics list
		gc.app.SetFocus(gc.topicsList)

		// Refresh the topics list to show the new topic - don't reload messages
		gc.refreshTopics(false)

	}()
}

// displayStatus updates the status bar with a message and color for 3s
func (gc *guiClient) displayStatus(message string, color string) {
	go func() {
		formattedMessage := fmt.Sprintf("[%s]%s[-]", color, message)

		// Refresh screen on status update
		gc.app.QueueUpdateDraw(func() {
			gc.statusBar.SetText(formattedMessage)
		})
		time.Sleep(3 * time.Second)
		gc.app.QueueUpdateDraw(func() {
			gc.statusBar.SetText("[green]Povezan[-]")
		})
	}()
}

// refreshTopics fetches the list of topics from the server and updates the GUI
func (gc *guiClient) refreshTopics(reloadMessages bool) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
		defer cancel()

		topics, err := gc.clients.Reads.ListTopics(ctx, &emptypb.Empty{})
		if err != nil {
			gc.displayStatus("Napaka pri pridobivanju tem", "red")
			return
		}

		// Sort the topics by id
		sort.Slice(topics.Topics, func(i, j int) bool {
			return topics.Topics[i].Id > topics.Topics[j].Id
		})

		// Update the topic IDs list - this probably isn't necessary but
		// it might increase robustness and clarity of code
		gc.topicIds = make([]int64, 0, len(topics.Topics))
		gc.app.QueueUpdateDraw(func() {
			gc.topicsList.Clear()
			for _, topic := range topics.Topics {
				gc.topicsList.AddItem(topic.Name, "", 0, nil)
				gc.topicIds = append(gc.topicIds, topic.Id)
			}
			if len(gc.topicIds) > 0 {
				// Set the current topic to the first one if available
				gc.currentTopicId = gc.topicIds[0]
			}
		})

		if reloadMessages && len(gc.topicIds) > 0 {
			gc.loadMessagesForCurrentTopic()
		}
	}()
}

// handleSelectTopic processes topic selection from the list and updates the GUI
func (gc *guiClient) handleSelectTopic(topicId int64) {
	// Set the current topic ID
	gc.currentTopicId = topicId

	// Load the messages from the selected topic
	gc.loadMessagesForCurrentTopic()
}

func (gc *guiClient) loadMessagesForCurrentTopic() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
		defer cancel()

		// This should be more dynamic
		messages, err := gc.clients.Reads.GetMessages(ctx, &pb.GetMessagesRequest{
			TopicId:       gc.currentTopicId,
			FromMessageId: 0,
			Limit:         50,
		})

		if err != nil {
			gc.displayStatus("Napaka pri pridobivanju sporočil", "red")
			return
		}

		// Sort messages by createdAt ascending - NOTE not yet supported
		// sort.Slice(messages.Messages, func(i, j int) bool {
		// 	// return messages.Messages[i].CreatedAt.AsTime().Before(messages.Messages[j].CreatedAt.AsTime())
		// })
		// Sort by ID for now
		sort.Slice(messages.Messages, func(i, j int) bool {
			return messages.Messages[i].Id < messages.Messages[j].Id
		})

		// Fetch all unique users before updating the GUI
		users := make(map[int64]*pb.User)

		for _, message := range messages.Messages {
			userId := message.UserId

			// Fetch user only if not already fetched - for performance reasons
			if _, exists := users[userId]; !exists {
				user, err := gc.clients.Reads.GetUser(ctx, &pb.GetUserRequest{
					UserId: userId,
				})

				if err != nil {
					// Skip this message
					users[userId] = &pb.User{Name: "Neznan uporabnik"}
				} else {
					users[userId] = user
				}
			}
		}

		// Update the message view in the GUI
		gc.app.QueueUpdateDraw(func() {
			// Clear the screen before displaying messages
			gc.messageView.Clear()

			// Prepare the message display
			for _, message := range messages.Messages {
				user, ok := users[message.UserId]

				// If user not found, skip the message
				if !ok {
					continue
				}

				// Currently timestamp isn't yet supported
				// timestamp := message.CreatedAt.AsTime().Format("02-01-2006 15:04")
				messageLine := fmt.Sprintf("[yellow]%s[-]: %s ([green]Všečki: %d[-])\n", user.Name, message.Text, message.Likes)
				gc.messageView.Write([]byte(messageLine))
			}
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

		_, err := gc.clients.Writes.PostMessage(ctx, &pb.PostMessageRequest{
			UserId:  gc.userId,
			TopicId: gc.currentTopicId,
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
