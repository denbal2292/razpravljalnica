package client

import (
	"errors"
	"fmt"

	pb "github.com/denbal2292/razpravljalnica/pkg/pb"
)

// Help message listing available commands
const HELP = `
Available commands:
help, h                                         - Show this help message
exit, quit, q                                   - Exit the client
createuser <name>                               - Create a new user
createtopic <name>                              - Create a new topic
post <topic_id> <user_id> <text>                - Post a message to a topic
update <topic_id> <user_id> <msg_id> <text>     - Update a message
delete <topic_id> <user_id> <msg_id>            - Delete a message
like <topic_id> <msg_id> <user_id>              - Like a message
topics                                          - List all topics
messages <topic_id> [from_id] [limit]           - Get messages from a topic
subscribe <user_id> <topic_id>... [from_msg_id] - Subscribe to topics
`

// Custom exit error to signal client termination
var ErrExit = errors.New("exit")

// Route the command to the appropriate client method
func route(client pb.MessageBoardClient, command string, args []string) error {
	switch command {
	case "help", "h":
		printHelp()
	case "exit", "quit", "q":
		return ErrExit
	case "createuser":
		return createUser(client, args)
	case "createtopic":
		return createTopic(client, args)
	case "post":
		return postMessage(client, args)
	case "update":
		return updateMessage(client, args)
	case "delete":
		return deleteMessage(client, args)
	case "like":
		return likeMessage(client, args)
	case "topics":
		return listTopics(client, args)
	case "messages":
		return getMessages(client, args)
	case "subscribe":
		return subscribeTopics(client, args)
	default:
		fmt.Printf("Unknown command: %s\n", command)
	}

	return nil
}

// Print the help message with available commands
func printHelp() {
	fmt.Println(HELP)
}
