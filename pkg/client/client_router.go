package client

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
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
func route(client *clientSet, command string, args []string) error {
	switch command {
	case "help", "h":
		printHelp()
	case "exit", "quit", "q":
		return errExit
	case "createuser":
		return createUser(client.writes, args)
	case "createtopic":
		return createTopic(client.writes, args)
	case "post":
		return postMessage(client.writes, args)
	case "update":
		return updateMessage(client.writes, args)
	case "delete":
		return deleteMessage(client.writes, args)
	case "like":
		return likeMessage(client.writes, args)
	case "topics":
		return listTopics(client.reads, args)
	case "messages":
		return getMessages(client.reads, args)
	case "user":
		return getUser(client.reads, args)
	case "subscribe":
		return subscribeTopics(client.subscriptions, args)
	case "getsubscribtionnode":
		return getSubscriptionNode(client.subscriptions, args)
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

// loopCommand repeatedly executes the given command until interrupted by SIGINT (Ctrl+C).
func loopCommand(client *clientSet, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: loop <command> [args...]")
	}

	innerCmd := args[0]
	innerArgs := args[1:]

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	fmt.Println("Looping. Press Ctrl+C to stop...")

	for {
		err := route(client, innerCmd, innerArgs)
		if err != nil {
			if errors.Is(err, errExit) {
				return errExit
			}
			fmt.Printf("Error: %v\n", err)
		}

		// time.Sleep(100 * time.Millisecond)

		select {
		case <-sigCh:
			fmt.Println("\nLoop interrupted by user.")
			return nil
		default:
			// continue looping
		}
	}
}
