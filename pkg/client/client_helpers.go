package client

import (
	"fmt"
	"strconv"
)

// Helper functions for client input validation and error handling

// ValidationError represents an error that occurs during input validation
type ValidationError struct {
	Field   string
	Message string
}

// Implement the error interface for ValidationError
func (e *ValidationError) Error() string {
	return "Validation error on field '" + e.Field + "': " + e.Message
}

// parseId parses a string argument into an int64 ID,
// returning a ValidationError if parsing fails
func parseId(arg, fieldName string) (int64, error) {
	id, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return 0, &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("must be a valid integer, got '%s'", arg),
		}
	}
	return id, nil
}

// requireArgs checks if the number of arguments is provided
func requireArgs(args []string, expected int, usage string) error {
	if len(args) < expected {
		return fmt.Errorf("usage: %s", usage)
	}
	return nil
}

// parseInt32 parses a string argument into an int32
func parseInt32(arg, fieldName string) (int32, error) {
	val, err := strconv.ParseInt(arg, 10, 32)
	if err != nil {
		return 0, &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("must be a valid integer, got '%s'", arg),
		}
	}
	return int32(val), nil
}
