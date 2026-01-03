package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	razpravljalnica "github.com/denbal2292/razpravljalnica/pkg/pb"
	"github.com/denbal2292/razpravljalnica/pkg/server"
	"github.com/denbal2292/razpravljalnica/pkg/server/gui"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TODO: Move this so we check the node role inside the node
func registerServices(node *server.Node, gRPCServer *grpc.Server) {
	// Only head nodes handle writes
	// if node.IsHead() {
	razpravljalnica.RegisterMessageBoardWritesServer(gRPCServer, node)
	// }

	// Only tail nodes handle reads
	// if node.IsTail() {
	razpravljalnica.RegisterMessageBoardReadsServer(gRPCServer, node)
	// }

	// All nodes participate in chain replication
	razpravljalnica.RegisterChainReplicationServer(gRPCServer, node)

	razpravljalnica.RegisterMessageBoardSubscriptionsServer(gRPCServer, node)
}

func main() {
	// Port 0 means to pick a random available port
	port := flag.Int("port", 0, "Port to listen on")
	controlPlanePort := flag.Int("control-port", 50051, "Control plane port")
	interfaceType := flag.String("type", "terminal", "Interface type: terminal or gui")

	flag.Parse()
	controlPlaneAddress := fmt.Sprintf("localhost:%d", *controlPlanePort)

	conn, err := grpc.NewClient(
		controlPlaneAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	controlPlaneClient := razpravljalnica.NewControlPlaneClient(conn)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(err)
	}

	addr := lis.Addr().String()

	// Initialize GUI or console mode
	logger, stats, cleanup := gui.StartWithFallback(
		"server-"+addr,
		addr,
		controlPlaneAddress,
		*interfaceType == "gui",
	)
	defer cleanup()

	// Create node with custom logger and stats
	node := server.NewServer("server-"+addr, addr, controlPlaneClient, logger, stats)

	// Initialize and register gRPC server
	gRPCServer := grpc.NewServer()
	razpravljalnica.RegisterNodeUpdateServer(gRPCServer, node)
	registerServices(node, gRPCServer)

	logger.Info(
		"Node started",
		"address", addr,
		"control_plane", controlPlaneAddress,
		"interface_type", *interfaceType,
	)

	// Shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Shutting down node...")
		gRPCServer.GracefulStop()
	}()

	// Start serving
	if err := gRPCServer.Serve(lis); err != nil {
		logger.Info("Server failed", "error", err)
		panic(err)
	}
}
