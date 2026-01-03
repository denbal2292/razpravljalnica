package server

import (
	"errors"
	"testing"
	"time"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/denbal2292/razpravljalnica/pkg/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestNode_applyEvent_CreateUser(t *testing.T) {
	n := newTestNode()

	username := "testuser"
	event := &pb.Event{
		Op: pb.OpType_OP_CREATE_USER,
		CreateUser: &pb.CreateUserRequest{
			Name: username,
		},
		EventAt: timestamppb.New(time.Now()),
	}

	result := n.applyEvent(event)

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if result.user == nil {
		t.Fatal("expected user to be created")
	}
	if result.user.Name != username {
		t.Errorf("expected user name '%s', got '%s'", username, result.user.Name)
	}
	if result.user.Id != 1 {
		t.Errorf("expected user ID 1, got %d", result.user.Id)
	}
}

func TestNode_applyEvent_CreateTopic(t *testing.T) {
	n := newTestNode()

	topicName := "testtopic"
	event := &pb.Event{
		Op: pb.OpType_OP_CREATE_TOPIC,
		CreateTopic: &pb.CreateTopicRequest{
			Name: topicName,
		},
		EventAt: timestamppb.New(time.Now()),
	}

	result := n.applyEvent(event)

	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if result.topic == nil {
		t.Fatal("expected topic to be created")
	}
	if result.topic.Name != topicName {
		t.Errorf("expected topic name '%s', got '%s'", topicName, result.topic.Name)
	}
	if result.topic.Id != 1 {
		t.Errorf("expected topic ID 1, got %d", result.topic.Id)
	}
}

func TestNode_applyEvent_PostMessage(t *testing.T) {
	n := newTestNode()

	// Test valid post message event
	t.Run("ValidPost", func(t *testing.T) {
		// Create user and topic first
		user, _ := n.storage.CreateUser("testuser")
		topic, _ := n.storage.CreateTopic("testtopic")

		testMessage := "hello world"
		event := &pb.Event{
			Op: pb.OpType_OP_POST,
			PostMessage: &pb.PostMessageRequest{
				TopicId: topic.Id,
				UserId:  user.Id,
				Text:    testMessage,
			},
			EventAt: timestamppb.New(time.Now()),
		}

		result := n.applyEvent(event)

		if result.err != nil {
			t.Fatalf("unexpected error: %v", result.err)
		}
		if result.message == nil {
			t.Fatal("expected message to be created")
		}
		if result.message.Text != testMessage {
			t.Errorf("expected message text '%s', got '%s'", testMessage, result.message.Text)
		}
		if result.message.UserId != user.Id {
			t.Errorf("expected user ID %d, got %d", user.Id, result.message.UserId)
		}
	})

	// Test post message with non-existent topic
	t.Run("NonExistentTopic", func(t *testing.T) {
		// Create user but not topic
		user, _ := n.storage.CreateUser("testuser")

		event := &pb.Event{
			Op: pb.OpType_OP_POST,
			PostMessage: &pb.PostMessageRequest{
				TopicId: 999, // non-existent
				UserId:  user.Id,
				Text:    "hello",
			},
			EventAt: timestamppb.New(time.Now()),
		}

		result := n.applyEvent(event)

		if result.err != storage.ErrTopicNotFound {
			t.Fatal("expected error for non-existent topic")
		}
	})

	// Test post message with non-existent user
	t.Run("NonExistentUser", func(t *testing.T) {
		n := newTestNode()
		topic, _ := n.storage.CreateTopic("testtopic")

		event := &pb.Event{
			Op: pb.OpType_OP_POST,
			PostMessage: &pb.PostMessageRequest{
				TopicId: topic.Id,
				UserId:  999, // Non-existent user
				Text:    "hello",
			},
			EventAt: timestamppb.New(time.Now()),
		}

		result := n.applyEvent(event)

		if result.err != storage.ErrUserNotFound {
			t.Errorf("expected ErrUserNotFound, got %v", result.err)
		}
	})
}

func TestNode_applyEvent_UpdateMessage(t *testing.T) {
	n := newTestNode()

	// Create user, topic, and message
	user1, _ := n.storage.CreateUser("testuser1")
	user2, _ := n.storage.CreateUser("testuser2")

	topic, _ := n.storage.CreateTopic("testtopic")
	msg, _ := n.storage.PostMessage(topic.Id, user1.Id, "original text", timestamppb.New(time.Now()))

	// Test update by author
	t.Run("UpdateByAuthor", func(t *testing.T) {
		event := &pb.Event{
			Op: pb.OpType_OP_UPDATE,
			UpdateMessage: &pb.UpdateMessageRequest{
				TopicId:   topic.Id,
				UserId:    user1.Id,
				MessageId: msg.Id,
				Text:      "updated text",
			},
			EventAt: timestamppb.New(time.Now()),
		}

		result := n.applyEvent(event)

		if result.err != nil {
			t.Fatalf("unexpected error: %v", result.err)
		}
		if result.message == nil {
			t.Fatal("expected message to be returned")
		}
		if result.message.Text != "updated text" {
			t.Errorf("expected updated text 'updated text', got '%s'", result.message.Text)
		}
	})

	// Test update by non-author
	t.Run("UpdateByNonAuthor", func(t *testing.T) {
		event := &pb.Event{
			Op: pb.OpType_OP_UPDATE,
			UpdateMessage: &pb.UpdateMessageRequest{
				TopicId:   topic.Id,
				UserId:    user2.Id,
				MessageId: msg.Id,
				Text:      "updated text",
			},
			EventAt: timestamppb.New(time.Now()),
		}

		result := n.applyEvent(event)

		if result.err != storage.ErrUserNotAuthor {
			t.Errorf("Expected ErrUserNotAuthor, got: %v", result.err)
		}
	})
}

func TestNode_applyEvent_DeleteMessage(t *testing.T) {
	n := newTestNode()

	// Create user1, topic, and message
	user1, _ := n.storage.CreateUser("testuser1")
	user2, _ := n.storage.CreateUser("testuser2")
	topic, _ := n.storage.CreateTopic("testtopic")

	// Test delete by author
	t.Run("DeleteByAuthor", func(t *testing.T) {
		msg, _ := n.storage.PostMessage(topic.Id, user1.Id, "to be deleted", timestamppb.New(time.Now()))
		event := &pb.Event{
			Op: pb.OpType_OP_DELETE,
			DeleteMessage: &pb.DeleteMessageRequest{
				TopicId:   topic.Id,
				UserId:    user1.Id,
				MessageId: msg.Id,
			},
			EventAt: timestamppb.New(time.Now()),
		}

		result := n.applyEvent(event)

		if result.err != nil {
			t.Fatalf("unexpected error: %v", result.err)
		}
		if result.message == nil {
			t.Fatal("expected message to be returned")
		}
	})

	// Test delete by non-author
	t.Run("DeleteByNonAuthor", func(t *testing.T) {
		msg, _ := n.storage.PostMessage(topic.Id, user1.Id, "to be deleted", timestamppb.New(time.Now()))
		event := &pb.Event{
			Op: pb.OpType_OP_DELETE,
			DeleteMessage: &pb.DeleteMessageRequest{
				TopicId:   topic.Id,
				UserId:    user2.Id,
				MessageId: msg.Id,
			},
			EventAt: timestamppb.New(time.Now()),
		}

		result := n.applyEvent(event)

		if result.err != storage.ErrUserNotAuthor {
			t.Errorf("Expected ErrUserNotAuthor, got: %v", result.err)
		}
	})

	// Test delete by nonexistent message
	t.Run("DeleteNonexistent", func(t *testing.T) {
		event := &pb.Event{
			Op: pb.OpType_OP_DELETE,
			DeleteMessage: &pb.DeleteMessageRequest{
				TopicId:   topic.Id,
				UserId:    user2.Id,
				MessageId: 999,
			},
			EventAt: timestamppb.New(time.Now()),
		}

		result := n.applyEvent(event)

		if result.err != storage.ErrMsgNotFound {
			t.Errorf("Expected ErrMsgNotFound, got: %v", result.err)
		}
	})
}

func TestNode_applyEvent_LikeMessage(t *testing.T) {
	n := newTestNode()

	// Create users, topic, and message
	author, _ := n.storage.CreateUser("author")
	liker, _ := n.storage.CreateUser("liker")
	topic, _ := n.storage.CreateTopic("testtopic")
	msg, _ := n.storage.PostMessage(topic.Id, author.Id, "likeable post", timestamppb.New(time.Now()))

	// Test valid like event - first like
	t.Run("ValidLike", func(t *testing.T) {
		event := &pb.Event{
			Op: pb.OpType_OP_LIKE,
			LikeMessage: &pb.LikeMessageRequest{
				TopicId:   topic.Id,
				UserId:    liker.Id,
				MessageId: msg.Id,
			},
			EventAt: timestamppb.New(time.Now()),
		}

		result := n.applyEvent(event)

		if result.err != nil {
			t.Fatalf("unexpected error: %v", result.err)
		}
		if result.message == nil {
			t.Fatal("expected message to be returned")
		}
		if result.message.Likes != 1 {
			t.Errorf("expected 1 like, got %d", result.message.Likes)
		}
	})

	// Test duplicate like by same user
	t.Run("DuplicateLike", func(t *testing.T) {
		event := &pb.Event{
			Op: pb.OpType_OP_LIKE,
			LikeMessage: &pb.LikeMessageRequest{
				TopicId:   topic.Id,
				UserId:    liker.Id,
				MessageId: msg.Id,
			},
			EventAt: timestamppb.New(time.Now()),
		}

		result := n.applyEvent(event)

		if result.err != storage.ErrUserAlreadyLiked {
			t.Errorf("Expected ErrUserAlreadyLiked, got: %v", result.err)
		}
	})
}

func TestNode_applyEvent_UnknownOp(t *testing.T) {
	n := newTestNode()

	event := &pb.Event{
		Op:      pb.OpType(999), // invalid op
		EventAt: timestamppb.New(time.Now()),
	}

	defer func() {
		// We expect a panic for unknown operation -> we need to recover here
		if r := recover(); r == nil {
			t.Error("expected panic for unknown operation")
		}
	}()

	n.applyEvent(event)
}

func TestHandleStorageError(t *testing.T) {
	// Test not found errors
	t.Run("NotFoundErrors", func(t *testing.T) {
		tests := []struct {
			name string
			err  error
		}{
			{"TopicNotFound", storage.ErrTopicNotFound},
			{"UserNotFound", storage.ErrUserNotFound},
			{"MsgNotFound", storage.ErrMsgNotFound},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				grpcErr := handleStorageError(tt.err)
				st, ok := status.FromError(grpcErr)
				if !ok {
					t.Fatal("expected gRPC status error")
				}
				if st.Code() != codes.NotFound {
					t.Errorf("expected NotFound code, got %v", st.Code())
				}
			})
		}
	})

	// Test permission denied error
	t.Run("PermissionDenied", func(t *testing.T) {
		grpcErr := handleStorageError(storage.ErrUserNotAuthor)
		st, ok := status.FromError(grpcErr)
		if !ok {
			t.Fatal("expected gRPC status error")
		}
		if st.Code() != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied code, got %v", st.Code())
		}
	})

	// Test already exists error
	t.Run("AlreadyExists", func(t *testing.T) {
		grpcErr := handleStorageError(storage.ErrUserAlreadyLiked)
		st, ok := status.FromError(grpcErr)
		if !ok {
			t.Fatal("expected gRPC status error")
		}
		if st.Code() != codes.AlreadyExists {
			t.Errorf("expected AlreadyExists code, got %v", st.Code())
		}
	})

	// Test invalid argument error
	t.Run("InvalidArgument", func(t *testing.T) {
		grpcErr := handleStorageError(storage.ErrInvalidLimit)
		st, ok := status.FromError(grpcErr)
		if !ok {
			t.Fatal("expected gRPC status error")
		}
		if st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument code, got %v", st.Code())
		}
	})

	// Test internal error for unknown errors
	customErr := errors.New("unknown error")
	grpcErr := handleStorageError(customErr)
	st, ok := status.FromError(grpcErr)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.Internal {
		t.Errorf("expected Internal code, got %v", st.Code())
	}
}

func TestNode(t *testing.T) {
	// Test requireHead with valid head
	t.Run("RequireHead_IsHead", func(t *testing.T) {
		n := newTestNode()
		// Node with no predecessor is HEAD
		n.predecessor = nil

		err := n.requireHead()
		if err != nil {
			t.Errorf("expected no error for HEAD node, got %v", err)
		}
	})

	// Test requireHead with non-head
	t.Run("RequireHead_NotHead", func(t *testing.T) {
		n := newTestNode()
		// Node with predecessor is not HEAD
		n.predecessor = &NodeConnection{}

		err := n.requireHead()
		if err == nil {
			t.Fatal("expected error for non-HEAD node")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("expected gRPC status error")
		}
		if st.Code() != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition code, got %v", st.Code())
		}
	})

	// Test requireTail with valid tail
	t.Run("RequireTail_IsTail", func(t *testing.T) {
		n := newTestNode()
		// Node with no successor is TAIL
		n.successor = nil

		err := n.requireTail()
		if err != nil {
			t.Errorf("expected no error for TAIL node, got %v", err)
		}
	})

	// Test requireTail with non-tail
	t.Run("RequireTail_NotTail", func(t *testing.T) {
		n := newTestNode()
		// Node with successor is not TAIL
		n.successor = &NodeConnection{}

		err := n.requireTail()
		if err == nil {
			t.Fatal("expected error for non-TAIL node")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("expected gRPC status error")
		}
		if st.Code() != codes.FailedPrecondition {
			t.Errorf("expected FailedPrecondition code, got %v", st.Code())
		}
	})

	// Test IsHead
	t.Run("IsHead", func(t *testing.T) {
		n := newTestNode()

		// Initially no predecessor (HEAD)
		if !n.IsHead() {
			t.Error("expected node to be HEAD initially")
		}

		// Set predecessor (not HEAD anymore)
		n.predecessor = &NodeConnection{}
		if n.IsHead() {
			t.Error("expected node to not be HEAD with predecessor set")
		}
	})

	// Test IsTail
	t.Run("IsTail", func(t *testing.T) {
		n := newTestNode()

		// Initially no successor (TAIL)
		if !n.IsTail() {
			t.Error("expected node to be TAIL initially")
		}

		// Set successor (not TAIL anymore)
		n.successor = &NodeConnection{}
		if n.IsTail() {
			t.Error("expected node to not be TAIL with successor set")
		}
	})
}

// Helper function to create a test node with minimal setup
func newTestNode() *Node {
	return &Node{
		storage: storage.NewStorage(),
	}
}
