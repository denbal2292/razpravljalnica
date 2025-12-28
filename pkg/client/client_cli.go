package client

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

func startCLIClient(clients *clientSet) {
	// Simple REPL loop
	// Useful for proper line reading
	scanner := bufio.NewScanner(os.Stdin)
	for {
		// Read the command
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		// Trim leading and trailing space and ignore empty input
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Split the input into fields
		fields := strings.Fields(input)
		command := fields[0]
		args := fields[1:]

		if err := route(clients, command, args); err != nil {
			if errors.Is(err, errExit) {
				// Client decided to exit
				break
			}
			fmt.Printf("Error: %v\n", err)
		}
	}
}
