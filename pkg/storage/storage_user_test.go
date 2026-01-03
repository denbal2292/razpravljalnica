package storage

import (
	"testing"
)

func TestCreateUser(t *testing.T) {
	storage := NewStorage()
	userName := "testuser"

	user, err := storage.CreateUser(userName)

	// User creation should not error here
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Created user should have correct details
	if user.Name != userName {
		t.Fatalf("Expected user name %s, got %s", userName, user.Name)
	}

	// Created user should have ID 1 (first user)
	if user.Id != 1 {
		t.Fatalf("Expected user ID 1, got %d", user.Id)
	}
}

func TestGetUser(t *testing.T) {
	storage := NewStorage()
	userName := "testuser"

	// CreateUser tested in previous test
	createdUser, _ := storage.CreateUser(userName)

	// Test valid user retrieval
	t.Run("ValidUserRetrieval", func(t *testing.T) {
		retrievedUser, err := storage.GetUser(createdUser.Id)

		// Valid user retrieval should not error
		if err != nil {
			t.Fatalf("Expected no error retrieving user, got %v", err)
		}

		// Retrieved user details should match created user
		if retrievedUser.Id != createdUser.Id || retrievedUser.Name != createdUser.Name {
			t.Fatalf(
				"Retrieved user does not match created user, expected %v, got %v", createdUser, retrievedUser,
			)
		}
	})

	// Test invalid user retrieval - multiple edge cases
	tests := []struct {
		name   string
		userId int64
	}{
		{"NegativeUserID", -1},
		{"ZeroUserID", 0},
		{"NonExistentUserID", 999},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			invalidUser, err := storage.GetUser(tc.userId)

			// Error for invalid user ID should be ErrUserNotFound
			if err != ErrUserNotFound {
				t.Fatalf("Expected ErrUserNotFound for user ID %d, got %v", tc.userId, err)
			}

			// Invalid user should be nil
			if invalidUser != nil {
				t.Fatalf("Expected nil user for user ID %d, got %v", tc.userId, invalidUser)
			}
		})
	}
}
