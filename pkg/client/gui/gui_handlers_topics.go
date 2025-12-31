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
		topic, err := gc.clients.Writes.CreateTopic(ctx, &pb.CreateTopicRequest{
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
		})

		// Add the new topic to the local cache
		gc.clientMu.Lock()
		gc.topics[topic.Id] = topic
		// In order to keep the correct ordering (newest first), we
		// prepend the new topic ID to the order slice
		gc.topicOrder = append([]int64{topic.Id}, gc.topicOrder...)
		gc.clientMu.Unlock()

		// Refresh the topics list to show the new topic and all the others
		// gc.refreshTopics()
		gc.displayTopics()
	}()
}

// refreshTopics fetches the list of topics from the server and updates the GUI
func (gc *guiClient) refreshTopics() {
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

		gc.displayTopics()
	}()
}

func (gc *guiClient) displayTopics() {
	gc.clientMu.RLock()
	topics := gc.topics
	topicOrder := gc.topicOrder
	selectedTopicId := gc.currentTopicId
	gc.clientMu.RUnlock()

	gc.app.QueueUpdateDraw(func() {
		gc.topicsList.Clear()
		selectedIndex := -1
		for i, topicId := range topicOrder {
			gc.topicsList.AddItem(topics[topicId].Name, "", 0, nil)
			if topicId == selectedTopicId {
				selectedIndex = i
			}
		}
		if len(gc.topicOrder) > 0 && selectedIndex >= 0 {
			// Set the current topic to the previously selected one if available
			gc.clientMu.Lock()
			gc.currentTopicId = gc.topicOrder[selectedIndex]
			gc.clientMu.Unlock()
			gc.topicsList.SetCurrentItem(selectedIndex)
		} else {
			gc.topicsList.SetCurrentItem(-1)
			gc.clientMu.Lock()
			gc.currentTopicId = 0
			gc.clientMu.Unlock()
		}
	})
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
