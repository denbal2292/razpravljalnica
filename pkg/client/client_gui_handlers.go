package client

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"
)

// handleCreateTopic processes the creation of a new topic
func (gc *guiClient) handleCreateTopic() {
	// Extract the topic name from the input field
	topicName := gc.newTopicInput.GetText()

	if topicName == "" {
		return
	}

	// Due to compatibility with the CLI, we accept a slice of strings
	err := createTopic(gc.clients.writes, []string{topicName})

	if err != nil {
		gc.displayStatus("Napaka pri ustvarjanju teme", "red")
	}

	// Clear the input field after processing
	gc.newTopicInput.SetText("")

	// Refresh the topics list to show the new topic
	gc.refreshTopics()
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
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		topics, err := gc.clients.reads.ListTopics(ctx, &emptypb.Empty{})

		if err != nil {
			go gc.displayStatus("Napaka pri pridobivanju tem", "red")
		}

		gc.app.QueueUpdateDraw(func() {
			gc.topicsList.Clear()
			for _, topic := range topics.Topics {
				gc.topicsList.AddItem(topic.Name, "", 0, nil)
			}
		})
	}()
}
