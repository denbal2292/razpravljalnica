package shared

import (
	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"google.golang.org/grpc"
)

// ServerAddresses holds the addresses for HEAD and TAIL servers - this
// might be useful later when we add more complex functionality (subscriptions)
// ClientSet holds gRPC clients for different services
type ClientSet struct {
	// Connections
	ControlConn *grpc.ClientConn
	HeadConn    *grpc.ClientConn
	TailConn    *grpc.ClientConn

	// Clients
	Reads         pb.MessageBoardReadsClient
	Writes        pb.MessageBoardWritesClient
	Subscriptions pb.MessageBoardSubscriptionsClient
}

func (c *ClientSet) Close() {
	if c.ControlConn != nil {
		c.ControlConn.Close()
	}
	if c.HeadConn != nil {
		c.HeadConn.Close()
	}
	if c.TailConn != nil {
		c.TailConn.Close()
	}
}
