package gui

import (
	"context"
	"sort"
	"strings"

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

	gc.clientMu.RLock()
	userId := gc.userId
	gc.clientMu.RUnlock()

	if userId == -1 {
		gc.displayStatus("Za ustvarjanje teme se moraš prijaviti", "red")
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

		gc.app.QueueUpdateDraw(func() {
			// Clear the input field after processing
			gc.newTopicInput.SetText("")
			// Set focus back to topics list
			gc.app.SetFocus(gc.topicsList)
			gc.displayStatus("Tema uspešno ustvarjena", "green")
		})

		// Refresh the topics list to show the new topic - don't reload messages
		gc.refreshTopics(false)

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

		gc.clientMu.Lock()
		gc.topics = topicsMap
		gc.topicOrder = topicsIds
		gc.clientMu.Unlock()

		gc.app.QueueUpdateDraw(func() {
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
	gc.clientMu.Lock()
	gc.currentTopicId = topicId
	gc.clientMu.Unlock()

	// Load the messages from the selected topic
	gc.loadMessagesForCurrentTopic()
}
