package storage

import (
	"os"
	"testing"
)

func TestHistoryRepository(t *testing.T) {

	tempFile, err := os.CreateTemp("", "history_test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	repo := &HistoryRepository{filename: tempFile.Name()}
	commands := []string{"2+2", "5*5", "x=10"}
	for _, cmd := range commands {
		repo.AddCommand(cmd)
	}

	t.Run("get last commands", func(t *testing.T) {
		lastCommands := repo.GetLastCommands(2)

		if len(lastCommands) != 2 {
			t.Errorf("Expected 2 commands, got %d", len(lastCommands))
		}

		expected := []string{"5*5", "x=10"}
		for i, cmd := range lastCommands {
			if cmd != expected[i] {
				t.Errorf("Expected '%s', got '%s'", expected[i], cmd)
			}
		}
	})

	t.Run("get more than available", func(t *testing.T) {
		lastCommands := repo.GetLastCommands(10)

		if len(lastCommands) != 3 {
			t.Errorf("Expected 3 commands, got %d", len(lastCommands))
		}
	})

	t.Run("empty history", func(t *testing.T) {
		emptyRepo := &HistoryRepository{filename: "nonexistent_file.txt"}
		commands := emptyRepo.GetLastCommands(5)

		if len(commands) != 0 {
			t.Errorf("Expected 0 commands for empty history, got %d", len(commands))
		}
	})
}
