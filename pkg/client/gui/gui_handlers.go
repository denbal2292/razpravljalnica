package gui

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/denbal2292/razpravljalnica/pkg/client/shared"
	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/emptypb"
)

// handleCreateTopic processes the creation of a new topic
func (gc *guiClient) handleCreateTopic() {
	// Extract the topic name from the input field
	topicName := gc.newTopicInput.GetText()

	if topicName == "" {
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
		} else {
			gc.displayStatus("Tema uspešno ustvarjena", "green")
		}

		// Clear the input field after processing
		gc.newTopicInput.SetText("")
		// Set focus back to topics list
		gc.app.SetFocus(gc.topicsList)

		// Refresh the topics list to show the new topic
		gc.refreshTopics()

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

func (gc *guiClient) refreshTopics() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
		defer cancel()

		topics, err := gc.clients.Reads.ListTopics(ctx, &emptypb.Empty{})
		if err != nil {
			go gc.displayStatus("Napaka pri pridobivanju tem", "red")
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
}
