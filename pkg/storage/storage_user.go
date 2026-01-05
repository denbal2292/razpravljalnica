package storage

import pb "github.com/denbal2292/razpravljalnica/pkg/pb"

// Create a user with a given name
func (s *Storage) CreateUser(name string) (*pb.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	userId := s.nextUserId
	user := &pb.User{
		Id:   userId,
		Name: name,
	}
	s.users[userId] = user
	s.nextUserId++

	return user, nil
}

// Get a user by ID
func (s *Storage) GetUser(userId int64) (*pb.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[userId]
	if !ok {
		return nil, ErrUserNotFound
	}

	return user, nil
}

// Check if a user exists
func (s *Storage) UserExists(userId int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.users[userId]
	return ok
}
