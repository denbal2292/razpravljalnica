package storage

import (
	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Create a new message in a given topic by a user
func (s *Storage) PostMessage(topicId int64, userId int64, text string, createdAt *timestamppb.Timestamp) (*pb.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if topic exists
	if _, ok := s.topics[topicId]; !ok {
		return nil, ErrTopicNotFound
	}

	// Check if user exists
	if _, ok := s.users[userId]; !ok {
		return nil, ErrUserNotFound
	}

	// Create new message
	messageId := s.nextMessageId[topicId]
	message := &pb.Message{
		Id:        messageId,
		TopicId:   topicId,
		UserId:    userId,
		CreatedAt: createdAt,
		Text:      text,
		Likes:     0,
	}
	s.messages[topicId][messageId] = message
	s.nextMessageId[topicId]++

	return message, nil
}

// Update an existing message's text
func (s *Storage) UpdateMessage(topicId int64, userId int64, messageId int64, text string) (*pb.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if topic exists
	if _, ok := s.topics[topicId]; !ok {
		return nil, ErrTopicNotFound
	}

	// Check if user exists
	if _, ok := s.users[userId]; !ok {
		return nil, ErrUserNotFound
	}

	// Check if message exists
	message, ok := s.messages[topicId][messageId]
	if !ok {
		return nil, ErrMsgNotFound
	}

	// Check if the user is the author of the message
	if message.UserId != userId {
		return nil, ErrUserNotAuthor
	}

	// Update the message text
	message.Text = text

	return message, nil
}

// Delete an existing message
func (s *Storage) DeleteMessage(topicId int64, userId int64, messageId int64) (*pb.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if topic exists
	if _, ok := s.topics[topicId]; !ok {
		return nil, ErrTopicNotFound
	}

	// Check if user exists
	if _, ok := s.users[userId]; !ok {
		return nil, ErrUserNotFound
	}

	// Check if message exists
	message, ok := s.messages[topicId][messageId]
	if !ok {
		return nil, ErrMsgNotFound
	}

	// Check if the user is the author of the message
	if message.UserId != userId {
		return nil, ErrUserNotAuthor
	}

	// Delete the message
	delete(s.messages[topicId], messageId)

	return message, nil
}

// Like a message
func (s *Storage) LikeMessage(topicId int64, userId int64, messageId int64) (*pb.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if topic exists
	if _, ok := s.topics[topicId]; !ok {
		return nil, ErrTopicNotFound
	}

	// Check if user exists
	if _, ok := s.users[userId]; !ok {
		return nil, ErrUserNotFound
	}

	// Check if message exists
	message, ok := s.messages[topicId][messageId]
	if !ok {
		return nil, ErrMsgNotFound
	}

	// Initialize likes map for the message if not present
	if _, ok := s.likes[messageId]; !ok {
		s.likes[messageId] = make(map[int64]bool)
	}

	// Check if user already liked the message
	if s.likes[messageId][userId] {
		return message, ErrUserAlreadyLiked // Already liked, do nothing
	}

	// Like the message
	s.likes[messageId][userId] = true
	message.Likes++

	return message, nil
}

// Get messages for a topic with a limit
func (s *Storage) GetMessagesWithLimit(topicId int64, fromMessageId int64, limit int32) ([]*pb.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		return nil, ErrInvalidLimit
	}

	// Check if topic exists
	if _, ok := s.topics[topicId]; !ok {
		return nil, ErrTopicNotFound
	}

	// Check if there are not yet any messages in the topic
	messagesMap, ok := s.messages[topicId]
	if !ok {
		return []*pb.Message{}, nil // No messages yet
	}

	// Retrieve messages for the topic
	messages := make([]*pb.Message, 0)

	for id := fromMessageId; id < fromMessageId+int64(limit) && id < s.nextMessageId[topicId]; id++ {
		if msg, exists := messagesMap[id]; exists {
			messages = append(messages, msg)
		}
	}

	return messages, nil
}

// Get messages for a topic without a limit and a callback after locking (to make sure no messages are duplicated)
func (s *Storage) GetMessagesFromTopics(topicIds []int64, fromMessageId int64, callback func()) ([]*pb.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	messages := make([]*pb.Message, 0)

	for _, topicId := range topicIds {
		// Check if topic exists
		if _, ok := s.topics[topicId]; !ok {
			return nil, ErrTopicNotFound
		}

		// Check if there are not yet any messages in the topic
		messagesMap, ok := s.messages[topicId]
		if !ok {
			continue // No messages yet
		}

		// Retrieve messages for the topic
		for id := fromMessageId; id < s.nextMessageId[topicId]; id++ {
			if msg, exists := messagesMap[id]; exists {
				messages = append(messages, msg)
			}
		}
	}

	// This is a callback to be executed while the storage lock is held
	// Useful to open subscription channels without missing/duplicating messages
	if callback != nil {
		callback()
	}

	return messages, nil
}
