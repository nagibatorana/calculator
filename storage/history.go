package storage

import (
	"bufio"
	"os"
)

type HistoryRepository struct {
	filename string
}

func NewHistoryRepository() *HistoryRepository {
	return &HistoryRepository{filename: "history.txt"}
}

func (h *HistoryRepository) AddCommand(command string) {
	file, _ := os.OpenFile(h.filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer file.Close()
	file.WriteString(command + "\n")
}

func (h *HistoryRepository) GetLastCommands(n int) []string {
	file, err := os.Open(h.filename)
	if err != nil {
		return []string{}
	}
	defer file.Close()

	var commands []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		commands = append(commands, scanner.Text())
	}

	if len(commands) > n {
		return commands[len(commands)-n:]
	}
	return commands
}

func (h *HistoryRepository) Restore() error {

	return nil
}
