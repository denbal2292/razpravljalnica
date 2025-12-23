package storage

import (
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Add a new event to the event log
// Must be called with lock held
func (s *Storage) addEvent(eventType pb.OpType, message *pb.Message) {
	event := &pb.MessageEvent{
		SequenceNumber: s.eventNumber,
		Message:        message,
		Op:             eventType,
		EventAt:        timestamppb.New(time.Now()),
	}

	// Append the event
	s.events = append(s.events, event)

	// Increment the event number
	s.eventNumber++
}

// Get events from a given sequence number for a given list of topics (or all topics if empty)
func (s *Storage) GetEvents(fromSequence int64, topicIds []int64) []*pb.MessageEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*pb.MessageEvent, 0)

	// Create a map for topic lookup
	topicFilter := make(map[int64]bool)
	for _, topicId := range topicIds {
		topicFilter[topicId] = true
	}

	// Loop from the given sequence number to the latest event
	for i := fromSequence; i < int64(len(s.events)); i++ {
		event := s.events[i]

		// Take only events for the specified topic
		if len(topicFilter) == 0 || topicFilter[event.Message.TopicId] {
			result = append(result, event)
		}
	}

	return result
}

// Get the current event number for clients to use as a reference
func (s *Storage) GetCurrentSequenceNumber() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.eventNumber
}
