package cli

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/denbal2292/razpravljalnica/pkg/client/shared"
)

const help = `Available commands:
  help, h                                     - Show this help message
  exit, quit, q                               - Exit the client

Write operations:
  createuser <name>                           - Create a new user
  createtopic <name>                          - Create a new topic
  post <user_id> <topic_id> <content>         - Post a message to a topic
  update <topic_id> <user_id> <msg_id> <text> - Update a message
  delete <topic_id> <user_id> <msg_id>        - Delete a message
  like <topic_id> <msg_id> <user_id>          - Like a message

Read operations:
  topics                                      - List all topics
  user <user_id>                              - Get user information
  messages <topic_id> <from_id> <limit>       - Get messages from a topic

Subscription operations:
  subscribe <user_id> <topic_id>...           - Subscribe to topics (stream)
  getsubscribtionnode <user_id> <topic_id>... - Get subscription node info
`

// Route the command to the appropriate client method
func route(client *shared.ClientSet, command string, args []string) error {
	switch command {
	case "help", "h":
		printHelp()
	case "exit", "quit", "q":
		return errExit
	case "createuser":
		return createUser(client.Writes, args)
	case "createtopic":
		return createTopic(client.Writes, args)
	case "post":
		return postMessage(client.Writes, args)
	case "update":
		return updateMessage(client.Writes, args)
	case "delete":
		return deleteMessage(client.Writes, args)
	case "like":
		return likeMessage(client.Writes, args)
	case "topics":
		return listTopics(client.Reads, args)
	case "messages":
		return getMessages(client.Reads, args)
	case "user":
		return getUser(client.Reads, args)
	case "subscribe":
		return subscribeTopics(client.Subscriptions, args)
	case "getsubscriptionnode":
		return getSubscriptionNode(client.Subscriptions, args)
	case "loop":
		return loopCommand(client, args)
	default:
		return fmt.Errorf("unknown command: %s\n", command)
	}

	return nil
}

// Print the help message with available commands
func printHelp() {
	fmt.Print(help)
}

func loopCommand(client *shared.ClientSet, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: loop <command> [args...]")
	}

	innerCmd := args[0]
	innerArgs := args[1:]

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	const maxConcurrent = 100
	sem := make(chan struct{}, maxConcurrent)

	fmt.Println("Looping. Press Ctrl+C to stop...")

	var sent, errCount uint64
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastSent uint64
	go func() {
		for range ticker.C {
			currentSent := atomic.LoadUint64(&sent)
			rate := currentSent - lastSent
			lastSent = currentSent

			e := atomic.LoadUint64(&errCount)
			fmt.Printf("\rSent: %d | Errors: %d | Rate: %d/s", currentSent, e, rate)
		}
	}()

	for {
		select {
		case <-sigCh:
			fmt.Printf("\nLoop interrupted. Sent %d requests, %d errors.\n",
				atomic.LoadUint64(&sent), atomic.LoadUint64(&errCount))
			return nil
		case sem <- struct{}{}:
			go func() {
				defer func() { <-sem }()

				err := route(client, innerCmd, innerArgs)
				if err != nil && !errors.Is(err, errExit) {
					atomic.AddUint64(&errCount, 1)
				}
			}()
			atomic.AddUint64(&sent, 1)
		}
	}
}
