package storage

import pb "github.com/denbal2292/razpravljalnica/pkg/pb"

// Create a topic with a given name
func (s *Storage) CreateTopic(name string) (*pb.Topic, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	topicId := s.nextTopicId
	topic := &pb.Topic{
		Id:   topicId,
		Name: name,
	}
	s.topics[topicId] = topic
	s.nextTopicId++

	s.addTopicEvent(topic)

	return topic, nil
}

// List all topics
func (s *Storage) ListTopics() ([]*pb.Topic, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	topics := make([]*pb.Topic, 0, len(s.topics))
	for _, topic := range s.topics {
		topics = append(topics, topic)
	}

	return topics, nil
}
