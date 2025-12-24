package server

import (
	"fmt"
	"sync"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
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
func (eb *EventBuffer) CreateMessageEvent(op pb.OpType, message *pb.Message) *pb.Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	event := &pb.Event{
		SequenceNumber: eb.nextEventSeq,
		Op:             op,
		Message:        message,
	}

	eb.events = append(eb.events, event)
	eb.nextEventSeq++

	return event
}

// Create a new like event and add it to the buffer
func (eb *EventBuffer) CreateLikeEvent(message *pb.Message, like *pb.Like) *pb.Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	event := &pb.Event{
		SequenceNumber: eb.nextEventSeq,
		Op:             pb.OpType_OP_LIKE,
		Message:        message,
		Like:           like,
	}

	eb.events = append(eb.events, event)
	eb.nextEventSeq++

	return event
}

// Create a new user event and add it to the buffer
func (eb *EventBuffer) CreateUserEvent(user *pb.User) *pb.Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	event := &pb.Event{
		SequenceNumber: eb.nextEventSeq,
		Op:             pb.OpType_OP_CREATE_USER,
		User:           user,
	}

	eb.events = append(eb.events, event)
	eb.nextEventSeq++

	return event
}

// Create a new topic event and add it to the buffer
func (eb *EventBuffer) CreateTopicEvent(topic *pb.Topic) *pb.Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	event := &pb.Event{
		SequenceNumber: eb.nextEventSeq,
		Op:             pb.OpType_OP_CREATE_TOPIC,
		Topic:          topic,
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
		panic(fmt.Sprintf("out-of-order event addition: got %d, expected %d", event.SequenceNumber, eb.nextEventSeq))
	}

	eb.events = append(eb.events, event)
	eb.nextEventSeq++
}

// TODO: Think about a nicer way to handle this
// Acknowledge that an event has been confirmed by the tail
// Returns the acknowledged event
func (eb *EventBuffer) AcknowledgeEvent(sequenceNumber int64) *pb.Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if sequenceNumber == eb.lastConfirmed+1 {
		eb.lastConfirmed++
	} else {
		panic(fmt.Sprintf("out-of-order event acknowledgment: got %d, expected %d", sequenceNumber, eb.lastConfirmed+1))
	}

	return eb.events[sequenceNumber]
}

func (eb *EventBuffer) GetLastApplied() int64 {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	return eb.lastConfirmed
}
