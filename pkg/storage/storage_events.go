package storage

import pb "github.com/denbal2292/razpravljalnica/pkg/pb"

// TODO: Here we just assume the ids will match
// It would be nice to add a sanity check
func (s *Storage) ApplyMessageEvent(message *pb.Message, op pb.OpType) error {
	switch op {
	case pb.OpType_OP_POST:
		_, err := s.PostMessage(message.TopicId, message.UserId, message.Text, message.CreatedAt)
		return err
	case pb.OpType_OP_UPDATE:
		_, err := s.UpdateMessage(message.TopicId, message.UserId, message.Id, message.Text)
		return err
	case pb.OpType_OP_DELETE:
		return s.DeleteMessage(message.TopicId, message.UserId, message.Id)
	default:
		return nil
	}
}

func (s *Storage) ApplyLikeEvent(message *pb.Message, like *pb.Like) error {
	_, err := s.LikeMessage(message.TopicId, message.UserId, message.Id)
	return err
}

func (s *Storage) ApplyUserEvent(user *pb.User, op pb.OpType) error {
	// TODO: here we just assume the id will match
	_, err := s.CreateUser(user.Name)
	return err
}

func (s *Storage) ApplyTopicEvent(topic *pb.Topic, op pb.OpType) error {
	// TODO: here we just assume the id will match
	_, err := s.CreateTopic(topic.Name)
	return err
}
