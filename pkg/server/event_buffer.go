package server

import (
	"fmt"
	"sync"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type EventBuffer struct {
	mu            sync.RWMutex
	events        []*pb.Event // sequence of events (idx = sequence_number)
	lastConfirmed int64       // last confirmed event sequence number (last applied in storage)
	nextEventSeq  int64       // next event sequence number to be added
}

func NewEventBuffer() *EventBuffer {
	return &EventBuffer{
		events:        make([]*pb.Event, 0),
		lastConfirmed: -1, // no events confirmed yet
		nextEventSeq:  0,  // start from sequence number 0
	}
}

// Create a new message event and add it to the buffer
func (eb *EventBuffer) CreateMessageEvent(message *pb.PostMessageRequest) *pb.Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	event := &pb.Event{
		SequenceNumber: eb.nextEventSeq,
		Op:             pb.OpType_OP_POST,
		EventAt:        timestamppb.Now(),
		PostMessage:    message,
	}

	eb.events = append(eb.events, event)
	eb.nextEventSeq++

	return event
}

// Create a new update message event and add it to the buffer
func (eb *EventBuffer) UpdateMessageEvent(message *pb.UpdateMessageRequest) *pb.Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	event := &pb.Event{
		SequenceNumber: eb.nextEventSeq,
		Op:             pb.OpType_OP_UPDATE,
		EventAt:        timestamppb.Now(),
		UpdateMessage:  message,
	}

	eb.events = append(eb.events, event)
	eb.nextEventSeq++

	return event
}

func (eb *EventBuffer) DeleteMessageEvent(message *pb.DeleteMessageRequest) *pb.Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	event := &pb.Event{
		SequenceNumber: eb.nextEventSeq,
		Op:             pb.OpType_OP_DELETE,
		EventAt:        timestamppb.Now(),
		DeleteMessage:  message,
	}

	eb.events = append(eb.events, event)
	eb.nextEventSeq++

	return event
}

// Create a new like event and add it to the buffer
func (eb *EventBuffer) LikeMessageEvent(like *pb.LikeMessageRequest) *pb.Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	event := &pb.Event{
		SequenceNumber: eb.nextEventSeq,
		Op:             pb.OpType_OP_LIKE,
		EventAt:        timestamppb.Now(),
		LikeMessage:    like,
	}

	eb.events = append(eb.events, event)
	eb.nextEventSeq++

	return event
}

// Create a new user event and add it to the buffer
func (eb *EventBuffer) CreateUserEvent(user *pb.CreateUserRequest) *pb.Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	event := &pb.Event{
		SequenceNumber: eb.nextEventSeq,
		Op:             pb.OpType_OP_CREATE_USER,
		EventAt:        timestamppb.Now(),
		CreateUser:     user,
	}

	eb.events = append(eb.events, event)
	eb.nextEventSeq++

	return event
}

// Create a new topic event and add it to the buffer
func (eb *EventBuffer) CreateTopicEvent(topic *pb.CreateTopicRequest) *pb.Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	event := &pb.Event{
		SequenceNumber: eb.nextEventSeq,
		Op:             pb.OpType_OP_CREATE_TOPIC,
		EventAt:        timestamppb.Now(),
		CreateTopic:    topic,
	}

	eb.events = append(eb.events, event)
	eb.nextEventSeq++

	return event
}

func (eb *EventBuffer) AddEvent(event *pb.Event) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	// Ensure events are added in order
	if event.SequenceNumber != eb.nextEventSeq {
		panic(fmt.Errorf("out-of-order event addition: got %d, expected %d", event.SequenceNumber, eb.nextEventSeq))
	}

	eb.events = append(eb.events, event)
	eb.nextEventSeq++
}

// Acknowledge that an event has been confirmed by the tail
// Returns the acknowledged event
func (eb *EventBuffer) AcknowledgeEvent(sequenceNumber int64) *pb.Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if sequenceNumber <= eb.lastConfirmed {
		// Event already acknowledged - return nil
		return nil
	}

	if sequenceNumber == eb.lastConfirmed+1 && sequenceNumber < int64(len(eb.events)) {
		eb.lastConfirmed++
	} else {
		panic(fmt.Sprintf("out-of-order event acknowledgment: got %d, expected %d", sequenceNumber, eb.lastConfirmed+1))
	}

	return eb.events[sequenceNumber]
}

func (eb *EventBuffer) GetEvent(sequenceNumber int64) *pb.Event {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	if sequenceNumber < 0 || sequenceNumber >= int64(len(eb.events)) {
		return nil
	}

	return eb.events[sequenceNumber]
}

func (eb *EventBuffer) GetLastApplied() int64 {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	return eb.lastConfirmed
}

func (eb *EventBuffer) GetLastReceived() int64 {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	return eb.nextEventSeq - 1
}

func (eb *EventBuffer) GetNextEventSeq() int64 {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	return eb.nextEventSeq
}
