package storage

import (
	"testing"
)

func TestCreateTopic(t *testing.T) {
	storage := NewStorage()
	topicName := "testtopic"

	topic, err := storage.CreateTopic(topicName)

	// Topic creation should not error here
	if err != nil {
		t.Fatalf("Expected no error creating topic, got %v", err)
	}

	// Created topic should have correct details
	if topic.Name != topicName {
		t.Fatalf("Expected topic name %s, got %s", topicName, topic.Name)
	}

	// Created topic should have ID 1 (first topic)
	if topic.Id != 1 {
		t.Fatalf("Expected topic ID 1, got %d", topic.Id)
	}
}

func TestListTopics(t *testing.T) {
	storage := NewStorage()

	// Test empty topics
	t.Run("EmptyTopics", func(t *testing.T) {
		listedTopics, err := storage.ListTopics()
		if err != nil {
			t.Fatalf("Expected no error listing topics, got %v", err)
		}
		if len(listedTopics) != 0 {
			t.Fatalf("Expected 0 topics, got %d", len(listedTopics))
		}
	})

	// Test non empty topics
	t.Run("NonEmptyTopics", func(t *testing.T) {
		topics := []string{"topic1", "topic2", "topic3"}

		// Create multiple topics
		for _, name := range topics {
			storage.CreateTopic(name)
		}

		// List topics
		listedTopics, err := storage.ListTopics()
		if err != nil {
			t.Fatalf("Expected no error listing topics, got %v", err)
		}

		for _, topic := range listedTopics {
			// Check if the listed topics match the created ones
			topicId := topic.Id

			if topic.Name != topics[topicId-1] {
				t.Fatalf("Expected topic name %s, got %s", topics[topicId-1], topic.Name)
			}
		}
	})
}
