package storage

import (
	"errors"
	"sync"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
)

var (
	ErrTopicNotFound    = errors.New("topic not found")
	ErrUserNotFound     = errors.New("user not found")
	ErrMsgNotFound      = errors.New("message not found")
	ErrUserNotAuthor    = errors.New("user is not the author of the message")
	ErrUserAlreadyLiked = errors.New("user has already liked the message")
)

type Storage struct {
	mu            sync.RWMutex                    // protects all fields below
	users         map[int64]*pb.User              // userId -> User
	topics        map[int64]*pb.Topic             // topicId -> Topic
	messages      map[int64]map[int64]*pb.Message // topicId -> (messageId -> Message)
	likes         map[int64]map[int64]bool        // messageId -> (userId -> bool)
	nextMessageId map[int64]int64                 // topicId -> nextMessageId
	nextUserId    int64                           // next user ID to assign
	nextTopicId   int64                           // next topic ID to assign
}

func NewStorage() *Storage {
	return &Storage{
		mu:            sync.RWMutex{},
		users:         make(map[int64]*pb.User),
		topics:        make(map[int64]*pb.Topic),
		messages:      make(map[int64]map[int64]*pb.Message),
		likes:         make(map[int64]map[int64]bool),
		nextUserId:    1,
		nextTopicId:   1,
		nextMessageId: make(map[int64]int64),
	}
}
