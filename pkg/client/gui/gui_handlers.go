package gui

import (
	"context"
	"fmt"
	"sort"
	"strconv"
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

	if gc.userId == -1 {
		gc.displayStatus("Za ustvarjanje teme se moraš prijaviti", "red")
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

		// Build the map and ordered slice of IDs
		topicsMap := make(map[int64]*pb.Topic, len(topics.Topics))
		topicsIds := make([]int64, 0, len(topics.Topics))
		for _, topic := range topics.Topics {
			topicsMap[topic.Id] = topic
			topicsIds = append(topicsIds, topic.Id)
		}

		gc.app.QueueUpdateDraw(func() {
			gc.topics = topicsMap
			gc.topicOrder = topicsIds
			gc.topicsList.Clear()
			for _, topicId := range topicsIds {
				gc.topicsList.AddItem(gc.topics[topicId].Name, "", 0, nil)
			}
			if len(gc.topicOrder) > 0 {
				// Set the current topic to the first one if available
				gc.currentTopicId = gc.topicOrder[0]
			}
		})

		if reloadMessages && len(gc.topicOrder) > 0 {
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

func (gc *guiClient) getUserName(id int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
	defer cancel()

	user, err := gc.clients.Reads.GetUser(ctx, &pb.GetUserRequest{
		UserId: id,
	})

	if err != nil {
		return "", err
	}

	return user.Name, nil
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
			gc.messageView.SetTitle(fmt.Sprintf("Sporočila v [yellow]%s[-]", gc.topics[gc.currentTopicId].Name))

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
				messageLine := fmt.Sprintf("[yellow]%s[-]: %s ([green]Všečki: %d[-]) [%s]\n", user.Name, message.Text, message.Likes, timestamp)
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

func (gc *guiClient) handleCreateUser() {
	// Extract the user name from the input field
	userName := strings.TrimSpace(gc.newUserInput.GetText())

	if userName == "" {
		gc.displayStatus("Ime uporabnika ne sme biti prazno", "red")
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
		defer cancel()

		user, err := gc.clients.Writes.CreateUser(ctx, &pb.CreateUserRequest{
			Name: userName,
		})

		if err != nil {
			gc.displayStatus("Napaka pri ustvarjanju uporabnika", "red")
			return
		} else {
			gc.displayStatus("Uporabnik uspešno ustvarjen", "green")
		}

		// Set the user ID for future operations
		gc.userId = user.Id

		// Clear the input field after processing
		gc.app.QueueUpdateDraw(func() {
			gc.newUserInput.SetText("")
			gc.loggedInUserView.SetText(fmt.Sprintf("[green]Prijavljen kot[-]: [yellow]%s[-]", user.Name))
		})
	}()
}

func (gc *guiClient) handleLogInUser() {
	// Extract the user name from the input field
	userId := strings.TrimSpace(gc.logInUserInput.GetText())

	if userId == "" {
		gc.displayStatus("ID uporabnika ni bil podan", "red")
		return
	}

	go func() {
		// Set the user ID for future operations
		id, err := strconv.ParseInt(userId, 10, 64)

		// Probably caused by an overflow
		if err != nil {
			gc.displayStatus("Neveljaven ID uporabnika", "red")
			return
		}

		userName, err := gc.getUserName(id)
		if err != nil {
			gc.displayStatus("Napaka pri prijavi uporabnika", "red")
			return
		}

		// We can safely set the user ID now
		gc.userId = id
		gc.app.QueueUpdateDraw(func() {
			gc.logInUserInput.SetText("")
			gc.loggedInUserView.SetText(fmt.Sprintf("[green]Prijavljen kot[-]: [yellow]%s[-]", userName))
			gc.displayStatus("Uspešna prijava uporabnika", "green")
		})
	}()
}
