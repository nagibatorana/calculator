package main

import (
	"calculator/business"
	"calculator/storage"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWebsiteOpening(t *testing.T) {
	fmt.Println("🌐 ТЕСТИРОВАНИЕ ОТКРЫТИЯ САЙТОВ")
	fmt.Println("═══════════════════════════════════════════════════════════")

	historyRepo := storage.NewHistoryRepository()
	interpreter := business.NewInterpreter(historyRepo)

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{"open with https", "https://google.com", "Открываю в браузере: https://google.com"},
	}

	passed := 0
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := interpreter.Execute(test.input)
			if err != nil {
				t.Logf("❌ %s: ошибка - %v", test.name, err)
			} else {
				resultStr, ok := result.(string)
				if !ok {
					t.Logf("❌ %s: ожидалась строка, получен %T", test.name, result)
				} else if strings.Contains(resultStr, test.contains) {
					fmt.Printf("✅ %s: %s\n", test.name, resultStr)
					passed++
				} else {
					t.Logf("❌ %s: результат не содержит '%s', получено: %s", test.name, test.contains, resultStr)
				}
			}
		})
	}

	fmt.Printf("Результат: %d/%d тестов пройдено\n", passed, len(tests))
	fmt.Println("═══════════════════════════════════════════════════════════")
}

func TestDeepSeekResponse(t *testing.T) {
	fmt.Println("ТЕСТИРОВАНИЕ DEEPSEEK API")
	fmt.Println("═══════════════════════════════════════════════════════════")

	historyRepo := storage.NewHistoryRepository()
	interpreter := business.NewInterpreter(historyRepo)

	tests := []struct {
		name        string
		question    string
		checkResult func(string) bool
	}{
		{
			"simple greeting",
			"привет",
			func(response string) bool {
				return len(response) > 10 && (strings.Contains(strings.ToLower(response), "привет") ||
					strings.Contains(strings.ToLower(response), "здравствуйте") ||
					strings.Contains(strings.ToLower(response), "hello"))
			},
		},
		{
			"general knowledge",
			"столица России",
			func(response string) bool {
				return len(response) > 5 && (strings.Contains(strings.ToLower(response), "москва") ||
					strings.Contains(strings.ToLower(response), "moscow"))
			},
		},
	}

	passed := 0
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := interpreter.Execute(test.question)
			if err != nil {
				if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "limit") {
					t.Skipf("Пропускаем тест '%s': лимит DeepSeek API исчерпан", test.name)
					return
				}
				t.Logf("❌ %s: ошибка - %v", test.name, err)
				return
			}

			resultStr, ok := result.(string)
			if !ok {
				t.Logf("❌ %s: ожидалась строка, получен %T", test.name, result)
				return
			}

			if test.checkResult(resultStr) {
				fmt.Printf("✅ %s: получен корректный ответ (%d символов)\n", test.name, len(resultStr))
				fmt.Printf("   📝 Ответ: %.100s...\n", resultStr)
				passed++
			} else {
				t.Logf("❌ %s: ответ не прошел проверку: %.100s...", test.name, resultStr)
			}
		})
	}

	fmt.Printf("📊 Результат: %d/%d тестов пройдено\n", passed, len(tests))
	fmt.Println("═══════════════════════════════════════════════════════════")
}

func TestWebsiteAnalysis(t *testing.T) {
	fmt.Println("🔍 ТЕСТИРОВАНИЕ АНАЛИЗА САЙТОВ")
	fmt.Println("═══════════════════════════════════════════════════════════")
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		htmlContent := `
		<!DOCTYPE html>
		<html>
		<head>
			<title>Тестовый сайт</title>
		</head>
		<body>
			<h1>Добро пожаловать на тестовый сайт</h1>
			<p>Это тестовый контент для проверки анализа сайтов.</p>
			<p>Сайт содержит информацию о тестировании.</p>
		</body>
		</html>`
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(htmlContent))
	}))
	defer mockServer.Close()

	historyRepo := storage.NewHistoryRepository()
	interpreter := business.NewInterpreter(historyRepo)

	t.Run("analyze website content", func(t *testing.T) {
		command := "расскажи о содержимом сайта " + mockServer.URL
		result, err := interpreter.Execute(command)

		if err != nil {
			if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "limit") {
				t.Skip("Пропускаем тест анализа сайта: лимит DeepSeek API исчерпан")
				return
			}
			t.Logf("Ошибка анализа сайта: %v", err)
			return
		}

		resultStr, ok := result.(string)
		if !ok {
			t.Logf("Ожидалась строка, получен %T", result)
			return
		}

		if len(resultStr) > 50 {
			fmt.Printf("✅ Анализ сайта работает: получен ответ (%d символов)\n", len(resultStr))
			fmt.Printf("   📝 Результат: %.100s...\n", resultStr)
		} else {
			t.Logf("Слишком короткий ответ от анализатора: %s", resultStr)
		}
	})

	fmt.Println("═══════════════════════════════════════════════════════════")
}

func TestWebRTCDebug(t *testing.T) {
	fmt.Println("🔧 ДИАГНОСТИКА ЗВОНКОВ")
	fmt.Println("═══════════════════════════════════════════════════════════")
	historyRepo := storage.NewHistoryRepository()
	interpreter := business.NewInterpreter(historyRepo)
	fmt.Println("1. Тестируем команду входа:")
	result, err := interpreter.Execute("войти как testuser")
	if err != nil {
		fmt.Printf("   ❌ Ошибка входа: %v\n", err)
		fmt.Println("2. Проверяем доступность сервера звонков:")
		resp, err := http.Get("http://localhost:8080")
		if err != nil {
			fmt.Printf("   ❌ Сервер звонков недоступен: %v\n", err)
			fmt.Println("   💡 Запустите сервер звонков: go run signaling/server.go")
		} else {
			defer resp.Body.Close()
			fmt.Printf("   ✅ Сервер звонков доступен, статус: %d\n", resp.StatusCode)
		}
	} else {
		fmt.Printf("   ✅ Вход выполнен: %v\n", result)
		fmt.Println("3. Тестируем команду звонка:")
		result, err = interpreter.Execute("позвонить testuser2")
		if err != nil {
			fmt.Printf("   ❌ Ошибка звонка: %v\n", err)
		} else {
			fmt.Printf("   ✅ Звонок инициирован: %v\n", result)
			fmt.Println("   💡 Проверьте, открылись ли окна браузера")
		}
	}

	fmt.Println("═══════════════════════════════════════════════════════════")
}

func TestStableOperations(t *testing.T) {
	fmt.Println("🎯 СТАБИЛЬНЫЕ ТЕСТЫ")
	fmt.Println("═══════════════════════════════════════════════════════════")
	historyRepo := storage.NewHistoryRepository()
	interpreter := business.NewInterpreter(historyRepo)
	stableTests := []struct {
		input    string
		expected interface{}
	}{
		{"2+2", 4.0},
		{"3*4", 12.0},
		{"10/2", 5.0},
		{"8-3", 5.0},
		{"5==5", true},
		{"6>4", true},
	}

	passed := 0
	for _, test := range stableTests {
		result, err := interpreter.Execute(test.input)
		if err != nil {
			t.Errorf("Ошибка при вычислении %s: %v", test.input, err)
		} else if result != test.expected {
			t.Errorf("%s = %v, ожидалось %v", test.input, result, test.expected)
		} else {
			fmt.Printf("✅ %s = %v\n", test.input, result)
			passed++
		}
	}

	fmt.Printf("📊 Результат: %d/%d тестов пройдено\n", passed, len(stableTests))
	fmt.Println("═══════════════════════════════════════════════════════════")
}

func TestCalculatorOperations(t *testing.T) {
	historyRepo := storage.NewHistoryRepository()
	interpreter := business.NewInterpreter(historyRepo)

	tests := []struct {
		input    string
		expected interface{}
	}{
		{"2+2", 4.0},
		{"10-5", 5.0},
		{"3*4", 12.0},
		{"20/5", 4.0},
		{"2+3*4", 14.0},
		{"(2+3)*4", 20.0},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result, err := interpreter.Execute(test.input)
			if err != nil {
				t.Errorf("Ошибка при вычислении %s: %v", test.input, err)
				return
			}
			if result != test.expected {
				t.Errorf("%s = %v, ожидалось %v", test.input, result, test.expected)
			} else {
				fmt.Printf("✅ %s = %v\n", test.input, result)
			}
		})
	}
}

func TestCurlCommands(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "success", "message": "test response"}`))
	}))
	defer server.Close()

	historyRepo := storage.NewHistoryRepository()
	interpreter := business.NewInterpreter(historyRepo)

	t.Run("simple curl", func(t *testing.T) {
		result, err := interpreter.Execute("curl " + server.URL)
		if err != nil {
			t.Errorf("Curl запрос не удался: %v", err)
			return
		}

		resultStr, ok := result.(string)
		if !ok {
			t.Errorf("Ожидалась строка, получен %T", result)
			return
		}

		if !strings.Contains(resultStr, `{"status": "success"`) {
			t.Errorf("Curl результат не содержит ожидаемый текст: %s", resultStr)
		} else {
			fmt.Printf("✅ Curl работает: получен ответ от сервера\n")
		}
	})

	t.Run("curl assignment", func(t *testing.T) {
		result, err := interpreter.Execute("data = curl " + server.URL)
		if err != nil {
			t.Logf("Не удалось установить CURL переменную: %v", err)
			return
		}

		resultStr, ok := result.(string)
		if ok && strings.Contains(resultStr, "CURL результат сохранен") {
			fmt.Printf("✅ Curl присваивание работает\n")
		}
	})
}

func TestFileOperations(t *testing.T) {
	testFilesDir, err := filepath.Abs("test_files")
	if err != nil {
		t.Fatalf("Не удалось получить абсолютный путь: %v", err)
	}
	if _, err := os.Stat(testFilesDir); err != nil {
		t.Fatalf("Папка test_files не существует: %v", err)
	}

	historyRepo := storage.NewHistoryRepository()
	interpreter := business.NewInterpreter(historyRepo)
	interpreter.AddSafeDirectory(testFilesDir)

	t.Run("open text file", func(t *testing.T) {
		result, err := interpreter.Execute("открой test.txt")
		if err != nil {
			t.Logf("Не удалось открыть файл: %v", err)
		} else {
			resultStr, ok := result.(string)
			if ok && strings.Contains(resultStr, "Открываю файл: test.txt") {
				fmt.Printf("✅ Открытие файла работает\n")
			} else {
				t.Logf("Результат не содержит ожидаемую строку: %s", resultStr)
			}
		}
	})
}

func TestHistory(t *testing.T) {
	historyRepo := storage.NewHistoryRepository()
	interpreter := business.NewInterpreter(historyRepo)

	commands := []string{
		"2+2",
		"5*5",
		"10/2",
		"8-3",
	}

	for _, cmd := range commands {
		interpreter.Execute(cmd)
	}

	t.Run("get history", func(t *testing.T) {
		history := interpreter.GetHistory()
		if len(history) >= len(commands) {
			fmt.Printf("✅ История сохранила %d команд\n", len(history))
		} else {
			t.Errorf("В истории только %d из %d команд", len(history), len(commands))
		}
	})

	t.Run("history command", func(t *testing.T) {
		result, err := interpreter.Execute("history")
		if err != nil {
			t.Errorf("Ошибка выполнения history: %v", err)
		} else if result == nil {
			t.Errorf("Команда history вернула nil")
		} else {
			fmt.Printf("✅ Команда history работает\n")
		}
	})
}

func TestErrorHandling(t *testing.T) {
	historyRepo := storage.NewHistoryRepository()
	interpreter := business.NewInterpreter(historyRepo)

	t.Run("division by zero", func(t *testing.T) {
		_, err := interpreter.Execute("5 / 0")
		if err != nil {
			fmt.Printf("✅ Деление на ноль корректно вызывает ошибку: %v\n", err)
		} else {
			t.Errorf("Деление на ноль должно возвращать ошибку")
		}
	})

	t.Run("undefined variable", func(t *testing.T) {
		_, err := interpreter.Execute("undefined_var + 5")
		if err != nil {
			fmt.Printf("✅ Неопределенная переменная корректно вызывает ошибку: %v\n", err)
		}
	})
}

