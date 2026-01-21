package main

import (
	"fmt"
	"os"
)

// setupTestEnvironment подготавливает окружение для тестов
func setupTestEnvironment() (string, error) {
	// Создаем временную директорию для тестов
	tempDir, err := os.MkdirTemp("", "calculator_test")
	if err != nil {
		return "", fmt.Errorf("не удалось создать временную директорию: %v", err)
	}

	// Создаем тестовые файлы
	testFiles := map[string]string{
		"test.txt": "Это тестовый файл",
		"test.pdf": "fake pdf content",
	}

	for filename, content := range testFiles {
		filePath := tempDir + "/" + filename
		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			return "", fmt.Errorf("не удалось создать файл %s: %v", filename, err)
		}
	}

	return tempDir, nil
}

// cleanupTestEnvironment очищает тестовое окружение
func cleanupTestEnvironment(tempDir string) {
	if tempDir != "" {
		os.RemoveAll(tempDir)
	}
}
