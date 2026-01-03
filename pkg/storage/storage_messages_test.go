package storage

import (
	"testing"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPostMessage(t *testing.T) {
	storage := NewStorage()

	// Create user and topic
	user, _ := storage.CreateUser("testuser")
	topic, _ := storage.CreateTopic("testtopic")

	// Test successful message posting
	t.Run("SuccessfulPost", func(t *testing.T) {
		message, err := storage.PostMessage(topic.Id, user.Id, "test message", timestamppb.Now())

		if err != nil {
			t.Fatalf("Expected no error posting message, got %v", err)
		}

		if message.Text != "test message" {
			t.Errorf("Expected text 'test message', got %s", message.Text)
		}

		if message.TopicId != topic.Id {
			t.Errorf("Expected topicId %d, got %d", topic.Id, message.TopicId)
		}

		if message.UserId != user.Id {
			t.Errorf("Expected userId %d, got %d", user.Id, message.UserId)
		}

		if message.Id != 1 {
			t.Errorf("Expected messageId 1 (first message), got %d", message.Id)
		}

		if message.Likes != 0 {
			t.Errorf("Expected 0 likes, got %d", message.Likes)
		}
	})

	// Test posting message in invalid topic
	t.Run("InvalidTopic", func(t *testing.T) {
		_, err := storage.PostMessage(999, user.Id, "test message", timestamppb.Now())

		if err != ErrTopicNotFound {
			t.Errorf("Expected ErrTopicNotFound, got %v", err)
		}
	})

	// Test posting message with invalid user
	t.Run("InvalidUser", func(t *testing.T) {
		_, err := storage.PostMessage(topic.Id, 999, "test message", timestamppb.Now())

		if err != ErrUserNotFound {
			t.Errorf("Expected ErrUserNotFound, got %v", err)
		}
	})
}

func TestUpdateMessage(t *testing.T) {
	storage := NewStorage()

	// Create user, topic, and message
	user, _ := storage.CreateUser("testuser")
	anotherUser, _ := storage.CreateUser("anotheruser")
	topic, _ := storage.CreateTopic("testtopic")
	message, _ := storage.PostMessage(topic.Id, user.Id, "original text", timestamppb.Now())

	updateText := "updated text"

	t.Run("SuccessfulUpdate", func(t *testing.T) {
		updatedMessage, err := storage.UpdateMessage(topic.Id, user.Id, message.Id, updateText)

		if err != nil {
			t.Fatalf("Expected no error updating message, got %v", err)
		}

		if updatedMessage.Text != updateText {
			t.Errorf("Expected text '%s', got %s", updateText, updatedMessage.Text)
		}
	})

	t.Run("InvalidTopic", func(t *testing.T) {
		_, err := storage.UpdateMessage(999, user.Id, message.Id, updateText)

		if err != ErrTopicNotFound {
			t.Errorf("Expected ErrTopicNotFound, got %v", err)
		}
	})

	t.Run("InvalidUser", func(t *testing.T) {
		_, err := storage.UpdateMessage(topic.Id, 999, message.Id, "updated text")

		if err != ErrUserNotFound {
			t.Errorf("Expected ErrUserNotFound, got %v", err)
		}
	})

	t.Run("MessageNotFound", func(t *testing.T) {
		_, err := storage.UpdateMessage(topic.Id, user.Id, 999, updateText)

		if err != ErrMsgNotFound {
			t.Errorf("Expected ErrMsgNotFound, got %v", err)
		}
	})

	t.Run("UserNotAuthor", func(t *testing.T) {
		_, err := storage.UpdateMessage(topic.Id, anotherUser.Id, message.Id, updateText)

		if err != ErrUserNotAuthor {
			t.Errorf("Expected ErrUserNotAuthor, got %v", err)
		}
	})
}

func TestDeleteMessage(t *testing.T) {
	storage := NewStorage()

	// Setup: Create user, topic, and message
	user, _ := storage.CreateUser("testuser")
	anotherUser, _ := storage.CreateUser("anotheruser")
	topic, _ := storage.CreateTopic("testtopic")

	messageText := "test message"

	// Test successful message deletion
	t.Run("SuccessfulDelete", func(t *testing.T) {
		message, _ := storage.PostMessage(topic.Id, user.Id, messageText, timestamppb.Now())

		deletedMessage, err := storage.DeleteMessage(topic.Id, user.Id, message.Id)

		if err != nil {
			t.Fatalf("Expected no error deleting message, got %v", err)
		}

		if deletedMessage.Id != message.Id {
			t.Errorf("Expected deleted message ID %d, got %d", message.Id, deletedMessage.Id)
		}

		// Verify message is actually deleted by checking ids
		messages, _ := storage.GetMessagesWithLimit(topic.Id, 1, 10)
		for _, msg := range messages {
			if msg.Id == message.Id {
				t.Error("Message still exists after deletion")
			}
		}
	})

	// Test deletion of message in invalid topic
	t.Run("InvalidTopic", func(t *testing.T) {
		message, _ := storage.PostMessage(topic.Id, user.Id, messageText, timestamppb.Now())
		_, err := storage.DeleteMessage(999, user.Id, message.Id)

		if err != ErrTopicNotFound {
			t.Errorf("Expected ErrTopicNotFound, got %v", err)
		}
	})

	// Test deletion with invalid user
	t.Run("InvalidUser", func(t *testing.T) {
		message, _ := storage.PostMessage(topic.Id, user.Id, messageText, timestamppb.Now())
		_, err := storage.DeleteMessage(topic.Id, 999, message.Id)

		if err != ErrUserNotFound {
			t.Errorf("Expected ErrUserNotFound, got %v", err)
		}
	})

	// Test deletion of non-existent message in the topic
	t.Run("MessageNotFound", func(t *testing.T) {
		_, err := storage.DeleteMessage(topic.Id, user.Id, 999)

		if err != ErrMsgNotFound {
			t.Errorf("Expected ErrMsgNotFound, got %v", err)
		}
	})

	// Test deletion by non-author user
	t.Run("UserNotAuthor", func(t *testing.T) {
		message, _ := storage.PostMessage(topic.Id, user.Id, messageText, timestamppb.Now())
		_, err := storage.DeleteMessage(topic.Id, anotherUser.Id, message.Id)

		if err != ErrUserNotAuthor {
			t.Errorf("Expected ErrUserNotAuthor, got %v", err)
		}
	})
}

func TestLikeMessage(t *testing.T) {
	storage := NewStorage()

	// Create users, topic, and message
	user, _ := storage.CreateUser("testuser")
	anotherUser, _ := storage.CreateUser("anotheruser")
	topic, _ := storage.CreateTopic("testtopic")

	messageText := "test message"
	message, _ := storage.PostMessage(topic.Id, user.Id, messageText, timestamppb.Now())

	// Test successful liking of a message
	t.Run("SuccessfulLike", func(t *testing.T) {
		likedMessage, err := storage.LikeMessage(topic.Id, anotherUser.Id, message.Id)

		if err != nil {
			t.Fatalf("Expected no error liking message, got %v", err)
		}

		if likedMessage.Likes != 1 {
			t.Errorf("Expected 1 like, got %d", likedMessage.Likes)
		}
	})

	// Test liking in invalid topic
	t.Run("InvalidTopic", func(t *testing.T) {
		_, err := storage.LikeMessage(999, user.Id, message.Id)

		if err != ErrTopicNotFound {
			t.Errorf("Expected ErrTopicNotFound, got %v", err)
		}
	})

	// Test liking with invalid user
	t.Run("InvalidUser", func(t *testing.T) {
		_, err := storage.LikeMessage(topic.Id, 999, message.Id)

		if err != ErrUserNotFound {
			t.Errorf("Expected ErrUserNotFound, got %v", err)
		}
	})

	// Test liking a non-existent message
	t.Run("MessageNotFound", func(t *testing.T) {
		_, err := storage.LikeMessage(topic.Id, user.Id, 999)

		if err != ErrMsgNotFound {
			t.Errorf("Expected ErrMsgNotFound, got %v", err)
		}
	})

	// Test liking a message already liked by the user
	t.Run("AlreadyLiked", func(t *testing.T) {
		// Like the message first time
		storage.LikeMessage(topic.Id, user.Id, message.Id)

		// Try to like again
		_, err := storage.LikeMessage(topic.Id, user.Id, message.Id)

		if err != ErrUserAlreadyLiked {
			t.Errorf("Expected ErrUserAlreadyLiked, got %v", err)
		}
	})

	// Test multiple likes from different users
	t.Run("MultipleLikes", func(t *testing.T) {
		// Create new message for this test
		newMsgText := "another test message"

		newMessage, _ := storage.PostMessage(topic.Id, user.Id, newMsgText, timestamppb.Now())
		user3, _ := storage.CreateUser("user3")
		user4, _ := storage.CreateUser("user4")

		// Test multiple likes from different users
		storage.LikeMessage(topic.Id, anotherUser.Id, newMessage.Id)
		storage.LikeMessage(topic.Id, user3.Id, newMessage.Id)
		likedMessage, _ := storage.LikeMessage(topic.Id, user4.Id, newMessage.Id)

		// Now it's liked by 3 users
		if likedMessage.Likes != 3 {
			t.Errorf("Expected 3 likes, got %d", likedMessage.Likes)
		}
	})

}

func TestGetMessagesWithLimit(t *testing.T) {
	storage := NewStorage()

	// Create user, topic, and messages
	user, _ := storage.CreateUser("testuser")
	topic, _ := storage.CreateTopic("testtopic")

	// Test invalid limits
	t.Run("InvalidLimit", func(t *testing.T) {
		tests := []struct {
			name  string
			limit int32
		}{
			{"ZeroLimit", 0},
			{"NegativeLimit", -1},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := storage.GetMessagesWithLimit(topic.Id, 1, tt.limit)

				if err != ErrInvalidLimit {
					t.Errorf("Expected ErrInvalidLimit, got %v", err)
				}
			})
		}
	})

	// Test getting messages from invalid topic
	t.Run("InvalidTopic", func(t *testing.T) {
		_, err := storage.GetMessagesWithLimit(999, 1, 10)

		if err != ErrTopicNotFound {
			t.Errorf("Expected ErrTopicNotFound, got %v", err)
		}
	})

	// Test getting messages from an empty topic
	t.Run("EmptyTopic", func(t *testing.T) {
		messages, err := storage.GetMessagesWithLimit(topic.Id, 1, 10)

		if err != nil {
			t.Fatalf("Expected no error getting messages, got %v", err)
		}

		if len(messages) != 0 {
			t.Errorf("Expected 0 messages, got %d", len(messages))
		}
	})

	// Create multiple messages
	testMessage := "test message"
	for range 5 {
		storage.PostMessage(topic.Id, user.Id, testMessage, timestamppb.Now())
	}

	// Test getting all messages from the topic
	t.Run("GetAllMessages", func(t *testing.T) {
		messages, err := storage.GetMessagesWithLimit(topic.Id, 1, 10)

		if err != nil {
			t.Fatalf("Expected no error getting messages, got %v", err)
		}

		if len(messages) != 5 {
			t.Errorf("Expected 5 messages, got %d", len(messages))
		}

		// Verify sequential IDs
		for i, msg := range messages {
			if msg.Id != int64(i)+1 {
				t.Errorf("Expected message ID %d, got %d", i+1, msg.Id)
			}
		}
	})

	// Test getting messages with limit less than total messages
	t.Run("GetMessagesWithLimitLessThanTotal", func(t *testing.T) {
		messages, err := storage.GetMessagesWithLimit(topic.Id, 1, 3)

		if err != nil {
			t.Fatalf("Expected no error getting messages, got %v", err)
		}

		if len(messages) != 3 {
			t.Errorf("Expected 3 messages, got %d", len(messages))
		}
	})

	// Test getting messages from a specific offset
	t.Run("GetMessagesFromOffset", func(t *testing.T) {
		messages, err := storage.GetMessagesWithLimit(topic.Id, 2, 2)

		if err != nil {
			t.Fatalf("Expected no error getting messages, got %v", err)
		}

		if len(messages) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(messages))
		}

		if messages[0].Id != 2 {
			t.Errorf("Expected first message ID 2, got %d", messages[0].Id)
		}

		if messages[1].Id != 3 {
			t.Errorf("Expected second message ID 3, got %d", messages[1].Id)
		}
	})

}

func TestGetMessagesFromTopics(t *testing.T) {
	storage := NewStorage()

	// Setup: Create user and topics
	user, _ := storage.CreateUser("testuser")
	topic1, _ := storage.CreateTopic("topic1")
	topic2, _ := storage.CreateTopic("topic2")
	topic3, _ := storage.CreateTopic("topic3")

	// Post messages to different topics
	storage.PostMessage(topic1.Id, user.Id, "message in topic1", timestamppb.Now())
	storage.PostMessage(topic1.Id, user.Id, "another message in topic1", timestamppb.Now())
	storage.PostMessage(topic2.Id, user.Id, "message in topic2", timestamppb.Now())

	// Test getting messages from multiple topics
	t.Run("GetMessagesFromMultipleTopics", func(t *testing.T) {
		messages, err := storage.GetMessagesFromTopics([]int64{topic1.Id, topic2.Id}, 1, nil)

		if err != nil {
			t.Fatalf("Expected no error getting messages, got %v", err)
		}

		if len(messages) != 3 {
			t.Errorf("Expected 3 messages, got %d", len(messages))
		}
	})

	// Test getting messages from a single topic
	t.Run("GetMessagesFromSingleTopic", func(t *testing.T) {
		messages, err := storage.GetMessagesFromTopics([]int64{topic1.Id}, 1, nil)

		if err != nil {
			t.Fatalf("Expected no error getting messages, got %v", err)
		}

		if len(messages) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(messages))
		}
	})

	// Test getting messages from topics that include an invalid topic
	t.Run("GetMessagesFromTopicsIncludingInvalidTopic", func(t *testing.T) {
		messages, err := storage.GetMessagesFromTopics([]int64{topic1.Id, 999}, 1, nil)

		if err != ErrTopicNotFound {
			t.Errorf("Expected ErrTopicNotFound, got %v", err)
		}

		if messages != nil {
			t.Errorf("Expected nil messages due to error, got %v", messages)
		}
	})

	// Test getting messages from a single topic
	t.Run("GetMessagesFromEmptyTopic", func(t *testing.T) {
		messages, err := storage.GetMessagesFromTopics([]int64{topic3.Id}, 1, nil)

		if err != nil {
			t.Fatalf("Expected no error getting messages, got %v", err)
		}

		if len(messages) != 0 {
			t.Errorf("Expected 0 messages, got %d", len(messages))
		}
	})

	// Test getting messages from offset
	t.Run("GetMessagesFromOffset", func(t *testing.T) {
		messages, err := storage.GetMessagesFromTopics([]int64{topic1.Id}, 1, nil)

		if err != nil {
			t.Fatalf("Expected no error getting messages, got %v", err)
		}

		if len(messages) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(messages))
		}

		if messages[0].Id != 1 {
			t.Errorf("Expected message ID 1, got %d", messages[0].Id)
		}
	})

	// Test callback execution
	t.Run("CallbackExecution", func(t *testing.T) {
		callbackExecuted := false

		// Callback sets callbackExecuted to true when executed
		callback := func() {
			// Capture variable by reference
			callbackExecuted = true
		}

		_, err := storage.GetMessagesFromTopics([]int64{topic1.Id}, 1, callback)

		if err != nil {
			t.Fatalf("Expected no error getting messages, got %v", err)
		}

		if !callbackExecuted {
			t.Error("Expected callback to be executed")
		}
	})
}
