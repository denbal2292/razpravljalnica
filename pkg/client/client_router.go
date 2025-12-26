package client

import (
	"errors"
	"fmt"
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

// Custom exit error to signal client termination
var ErrExit = errors.New("exit")

// Route the command to the appropriate client method
func route(client *clientSet, command string, args []string) error {
	switch command {
	case "help", "h":
		printHelp()
	case "exit", "quit", "q":
		return ErrExit
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
	default:
		return fmt.Errorf("unknown command: %s\n", command)
	}

	return nil
}

// Print the help message with available commands
func printHelp() {
	fmt.Print(help)
}
