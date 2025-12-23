package storage

import (
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Create a new message in a given topic by a user
func (s *Storage) PostMessage(topicId int64, userId int64, text string) (*pb.Message, error) {
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

	// Initialize message map for the topic if not present
	if _, ok := s.messages[topicId]; !ok {
		s.messages[topicId] = make(map[int64]*pb.Message)
		s.nextMessageId[topicId] = 1
	}

	// Create new message
	messageId := s.nextMessageId[topicId]
	message := &pb.Message{
		Id:        messageId,
		TopicId:   topicId,
		UserId:    userId,
		CreatedAt: timestamppb.New(time.Now()),
		Text:      text,
		Likes:     0,
	}
	s.messages[topicId][messageId] = message
	s.nextMessageId[topicId]++

	s.addMessageEvent(PostMessageEvent, message)

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
	s.addMessageEvent(UpdateMessageEvent, message)

	return message, nil
}

// Delete an existing message
func (s *Storage) DeleteMessage(topicId int64, userId int64, messageId int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if topic exists
	if _, ok := s.topics[topicId]; !ok {
		return ErrTopicNotFound
	}

	// Check if user exists
	if _, ok := s.users[userId]; !ok {
		return ErrUserNotFound
	}

	// Check if message exists
	message, ok := s.messages[topicId][messageId]
	if !ok {
		return ErrMsgNotFound
	}

	// Check if the user is the author of the message
	if message.UserId != userId {
		return ErrUserNotAuthor
	}

	// Delete the message
	s.addMessageEvent(DeleteMessageEvent, message)
	delete(s.messages[topicId], messageId)

	return nil
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
	s.addMessageEvent(LikeMessageEvent, message)

	return message, nil
}

// Get messages for a topic
func (s *Storage) GetMessages(topicId int64, fromMessageId int64, limit int32) ([]*pb.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

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
