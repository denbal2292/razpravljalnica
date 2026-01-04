package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/denbal2292/razpravljalnica/pkg/server"
	"github.com/denbal2292/razpravljalnica/pkg/server/gui"
	"google.golang.org/grpc"
)

func registerServices(node *server.Node, gRPCServer *grpc.Server) {
	pb.RegisterMessageBoardWritesServer(gRPCServer, node)
	pb.RegisterMessageBoardReadsServer(gRPCServer, node)
	pb.RegisterChainReplicationServer(gRPCServer, node)
	pb.RegisterMessageBoardSubscriptionsServer(gRPCServer, node)
}

func main() {
	// Port 0 means to pick a random available port
	port := flag.Int("port", 0, "Port to listen on")
	controlPlanePort := flag.Int("control-port", 50051, "Control plane port")
	interfaceType := flag.String("type", "terminal", "Interface type: terminal or gui")

	flag.Parse()

	// Build list of all control plane server addresses
	// In a Raft cluster, we have multiple control plane servers
	controlPlaneAddrs := []string{
		fmt.Sprintf("localhost:%d", *controlPlanePort),
		fmt.Sprintf("localhost:%d", *controlPlanePort+1),
		fmt.Sprintf("localhost:%d", *controlPlanePort+2),
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(err)
	}

	addr := lis.Addr().String()

	// Initialize GUI or console mode
	logger, stats, cleanup := gui.StartWithFallback(
		controlPlaneAddrs[0],
		*interfaceType == "gui",
	)
	// Stop the app on exit
	defer cleanup()

	// Set default logger
	slog.SetDefault(logger)

	// Create node with custom logger
	node := server.NewServer("server-"+addr, addr, controlPlaneAddrs)

	// Set the node as the stats provider
	stats.SetProvider(node)

	// Initialize and register gRPC server
	gRPCServer := grpc.NewServer()
	pb.RegisterNodeUpdateServer(gRPCServer, node)
	registerServices(node, gRPCServer)

	logger.Info(
		"Node started",
		"address", addr,
		"control_plane", controlPlaneAddrs,
		"interface_type", *interfaceType,
	)

	// Start serving
	if err := gRPCServer.Serve(lis); err != nil {
		logger.Info("Server failed", "error", err)
		panic(err)
	}
}
