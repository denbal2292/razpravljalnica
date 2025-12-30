package server

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type SubscriptionManager struct {
	// Filled by control plane when informing nodes about new subscriptions
	availableSubscriptions map[string]*pb.SubscriptionNodeRequest // subscribeToken -> subscription request (which topics and which user)
	availMu                sync.RWMutex                           // protects availableSubscriptions

	activeSubscriptions map[string]*Subscription // subscribeToken -> subscription
	activeMu            sync.RWMutex             // protects activeSubscriptions

	subscriptionsByTopic map[int64][]*Subscription // topicId -> list of subscriptions
	topicMu              sync.RWMutex              // protects subscriptionsByTopic
}

type Subscription struct {
	userId   int64
	topicIds []int64
	channel  chan *pb.MessageEvent
}

func NewSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{
		availableSubscriptions: make(map[string]*pb.SubscriptionNodeRequest),
		activeSubscriptions:    make(map[string]*Subscription),
		subscriptionsByTopic:   make(map[int64][]*Subscription),
	}
}

func (sm *SubscriptionManager) CreateMessageEvent(message *pb.Message, seqNum int64, eventAt *timestamppb.Timestamp, opType pb.OpType) *pb.MessageEvent {
	return &pb.MessageEvent{
		SequenceNumber: seqNum,
		EventAt:        eventAt,
		Message:        message,
		Op:             opType,
	}
}

func (sm *SubscriptionManager) AddMessageEvent(event *pb.MessageEvent, topicId int64) {
	sm.topicMu.RLock()
	subscribers, exists := sm.subscriptionsByTopic[topicId]
	sm.topicMu.RUnlock()

	if !exists {
		// No subscribers for this topic
		return
	}

	for _, sub := range subscribers {
		// Non-blocking send to the subscription channel
		select {
		case sub.channel <- event:
		default:
			fmt.Printf("Dropping message for user %d on topic %d due to full channel\n", sub.userId, topicId)
		}
	}
}

// Control plane calls this RPC to inform the node about a new subscription request
func (sm *SubscriptionManager) AddSubscriptionRequest(ctx context.Context, req *pb.SubscriptionNodeRequest) (*pb.AddSubscriptionResponse, error) {
	sm.availMu.Lock()
	defer sm.availMu.Unlock()

	// Generate a subscription token (unique ID) for this subscription request
	subscribeToken, err := generateSubscriptionToken(16)
	if err != nil {
		return nil, err
	}

	// Store the subscription request in availableSubscriptions
	sm.availableSubscriptions[subscribeToken] = req

	return &pb.AddSubscriptionResponse{SubscribeToken: subscribeToken}, nil
}

// Helper function to generate a random subscription token of given length
func generateSubscriptionToken(n int) (string, error) {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b), nil
}

func (sm *SubscriptionManager) ValidateSubscriptionToken(subscribeToken string, userId int64) bool {
	sm.availMu.Lock()
	defer sm.availMu.Unlock()

	subReq, exists := sm.availableSubscriptions[subscribeToken]

	if !exists {
		return false
	}

	if subReq.UserId != userId {
		return false
	}

	delete(sm.availableSubscriptions, subscribeToken)

	return exists
}

// Open a subscription for the given request and return a channel to receive message events
func (sm *SubscriptionManager) OpenSubscriptionChannel(req *pb.SubscribeTopicRequest) <-chan *pb.MessageEvent {
	// 1. Create a channel for sending message events
	eventChan := make(chan *pb.MessageEvent, 100) // Buffered channel to avoid blocking

	// 2. Store the active subscription
	subscription := &Subscription{
		userId:   req.UserId,
		topicIds: req.TopicId,
		channel:  eventChan,
	}

	sm.activeMu.Lock()
	sm.activeSubscriptions[req.SubscribeToken] = subscription
	sm.activeMu.Unlock()

	// 3. Update subscriptionsByTopic mapping
	sm.topicMu.Lock()
	for _, topicId := range subscription.topicIds {
		sm.subscriptionsByTopic[topicId] = append(sm.subscriptionsByTopic[topicId], subscription)
	}
	sm.topicMu.Unlock()

	return eventChan
}

// Called to clear an active subscription when the client disconnects
func (sm *SubscriptionManager) ClearSubscription(subscribeToken string) {
	sm.activeMu.Lock()
	sm.topicMu.Lock()
	defer sm.activeMu.Unlock()
	defer sm.topicMu.Unlock()

	subDelete, exists := sm.activeSubscriptions[subscribeToken]

	if !exists {
		return // No active subscription to clear
	}

	close(subDelete.channel)
	delete(sm.activeSubscriptions, subscribeToken)

	for _, topicId := range subDelete.topicIds {
		subs:= sm.subscriptionsByTopic[topicId]

		for i, topicSub := range subs {
			if subDelete == topicSub {
				// Remove subscription from the slice
				sm.subscriptionsByTopic[topicId] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
	}
}
