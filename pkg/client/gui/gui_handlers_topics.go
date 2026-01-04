package gui

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/denbal2292/razpravljalnica/pkg/client/shared"
	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
		topic, err := shared.RetryFetch(ctx, gc.clients, func(ctx context.Context) (*pb.Topic, error) {
			return gc.clients.Writes.CreateTopic(ctx, &pb.CreateTopicRequest{
				Name: topicName,
			})
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

		topics, err := shared.RetryFetch(ctx, gc.clients, func(ctx context.Context) (*pb.ListTopicsResponse, error) {
			return gc.clients.Reads.ListTopics(ctx, &emptypb.Empty{})
		})
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
	go func() {
		gc.clientMu.RLock()
		topics := gc.topics
		selectedTopicId := gc.currentTopicId
		subscribedTopics := make(map[int64]bool, len(gc.subscribedTopics))
		topicOrder := make([]int64, len(gc.topicOrder))

		// Copy subscribed topics
		maps.Copy(subscribedTopics, gc.subscribedTopics)
		copy(topicOrder, gc.topicOrder)

		gc.clientMu.RUnlock()

		gc.app.QueueUpdateDraw(func() {
			gc.topicsList.Clear()
			selectedIndex := -1
			for i, topicId := range topicOrder {
				topicName := topics[topicId].Name
				if subscribed, ok := subscribedTopics[topicId]; ok && subscribed {
					// Mark subscribed topics with an asterisk
					topicName = topicName + " [yellow]*[-]"
				}
				if unread, ok := gc.unreadTopic[topicId]; ok && unread {
					// Mark topics with unread messages with a plus sign
					topicName = topicName + " [green]N[-]"
				}

				gc.topicsList.AddItem(topicName, "", 0, nil)
				if topicId == selectedTopicId {
					selectedIndex = i
				}
			}
			if len(topicOrder) > 0 && selectedIndex >= 0 {
				gc.topicsList.SetCurrentItem(selectedIndex)
			} else {
				gc.topicsList.SetCurrentItem(-1)
			}
		})
	}()
}

// handleSelectTopic processes topic selection from the list and updates the GUI
func (gc *guiClient) handleSelectTopic(topicId int64) {
	// Set the current topic ID
	gc.clientMu.Lock()
	gc.currentTopicId = topicId
	// Remove the unread marker for this topic - it was selected
	gc.unreadTopic[topicId] = false
	gc.clientMu.Unlock()

	// Display the topics
	gc.displayTopics()

	// Load the messages from the selected topic
	gc.loadMessagesForCurrentTopic()
}

func (gc *guiClient) handleTopicSubscription() {
	index := gc.topicsList.GetCurrentItem()

	if index < 0 || index >= len(gc.topicOrder) {
		gc.displayStatus("Izbrana tema ni na voljo", "red")
		return
	}

	// Fetch the topic ID based on the selected topic index
	gc.clientMu.RLock()
	topicId := gc.topicOrder[index]
	gc.clientMu.RUnlock()

	gc.subscribeToTopic(topicId)
}

func (gc *guiClient) subscribeToTopic(topicId int64) {
	go func() {
		gc.clientMu.RLock()
		subscribedToCurrentTopic, exists := gc.subscribedTopics[topicId]
		gc.clientMu.RUnlock()

		if exists && subscribedToCurrentTopic {
			gc.displayStatus("Že naročen na izbrano temo", "yellow")
			return
		}

		gc.clientMu.RLock()
		userId := gc.userId
		gc.clientMu.RUnlock()

		// Get subscription node using the retry mechanism
		var subResponse *pb.SubscriptionNodeResponse
		err := gc.clients.TryControlPlaneRequest(func(client pb.ClientDiscoveryClient) error {
			ctx, cancel := context.WithTimeout(context.Background(), shared.Timeout)
			defer cancel()

			var err error
			subResponse, err = client.GetSubscriptionNode(ctx, &pb.SubscriptionNodeRequest{
				UserId: userId,
			})
			return err
		})

		if err != nil {
			gc.displayStatus("Napaka pri pridobivanju vozlišča za naročanje", "red")
			return
		}

		// Connect to the subscription node
		conn, err := grpc.NewClient(
			subResponse.Node.Address,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			gc.displayStatus("Napaka pri povezovanju z vozliščem za naročanje", "red")
			return
		}
		defer conn.Close()

		// Create a subscription client
		subClient := pb.NewMessageBoardSubscriptionsClient(conn)

		// Get the last message ID we already have for the topic
		var fromMessageId int64 = 1
		gc.clientMu.RLock()
		if entry, ok := gc.messageCache[topicId]; ok && len(entry.order) > 0 {
			fromMessageId = entry.order[len(entry.order)-1] + 1
		}
		gc.displayStatus(fmt.Sprintf("Zadnje sporočilo ID: %d", fromMessageId), "blue")
		gc.clientMu.RUnlock()

		// Send subscription request
		subscriptionStream, err := subClient.SubscribeTopic(context.Background(), &pb.SubscribeTopicRequest{
			UserId: userId,
			// Only subscribe to the selected topic
			TopicId:        []int64{topicId},
			FromMessageId:  fromMessageId,
			SubscribeToken: subResponse.SubscribeToken,
		})

		if err != nil {
			gc.displayStatus("Overitev neuspešna", "red")
			return
		}

		// Mark the topic as subscribed
		gc.clientMu.Lock()
		gc.subscribedTopics[topicId] = true
		gc.clientMu.Unlock()

		// Update the topics display to show subscription status
		gc.displayTopics()

		gc.displayStatus("Uspešna naročnina na temo", "green")
		gc.handleSubscriptionStream(topicId, subscriptionStream)
	}()
}
