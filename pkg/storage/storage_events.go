package storage

import (
	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
)

func (s *Storage) addTopicEvent(topic *pb.Topic) {
	event := &ReplicationEvent{
		sequenceNumber: s.eventCounter,
		eventType:      CreateTopicEvent,
		topic:          topic,
	}

	// Append the event
	s.events = append(s.events, event)

	// Increment the event counter
	s.eventCounter++
}

func (s *Storage) addUserEvent(user *pb.User) {
	event := &ReplicationEvent{
		sequenceNumber: s.eventCounter,
		eventType:      CreateUserEvent,
		user:           user,
	}

	// Append the event
	s.events = append(s.events, event)

	// Increment the event counter
	s.eventCounter++
}

// Add a new event to the event log
// Must be called with lock held
func (s *Storage) addMessageEvent(eventType ReplicationEventType, message *pb.Message) {
	if eventType == CreateTopicEvent || eventType == CreateUserEvent {
		panic("use specific functions for topic/user events")
	}

	event := &ReplicationEvent{
		sequenceNumber: s.eventCounter,
		message:        message,
		eventType:      eventType,
	}

	// Append the event
	s.events = append(s.events, event)

	// Increment the event counter
	s.eventCounter++
}

// Get the current event number for clients to use as a reference
func (s *Storage) GetCurrentSequenceNumber() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.eventCounter
}
