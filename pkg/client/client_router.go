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
func route(client *ClientSet, command string, args []string) error {
	switch command {
	case "help", "h":
		printHelp()
	case "exit", "quit", "q":
		return ErrExit
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
		return subscribeTopics(client.Subsciptions, args)
	case "getsubscribtionnode":
		return getSubscriptionNode(client.Subsciptions, args)
	default:
		return fmt.Errorf("unknown command: %s\n", command)
	}

	return nil
}

// Print the help message with available commands
func printHelp() {
	fmt.Print(help)
}
