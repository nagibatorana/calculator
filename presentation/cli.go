package presentation

import (
	"bufio"
	"calculator/business"
	"fmt"
	"os"
	"strings"
)

type CLI struct {
	interpreter *business.Interpreter
}

func NewCLI(interpreter *business.Interpreter) *CLI {
	return &CLI{interpreter: interpreter}
}

func (c *CLI) Run() {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if input == "exit" {
			fmt.Println("До свидания!")
			break
		}

		if input == "history" {
			history := c.interpreter.GetHistory()
			fmt.Println("Последние 10 команд:")
			for i, cmd := range history {
				fmt.Printf("%d: %s\n", i+1, cmd)
			}
			continue
		}

		result, err := c.interpreter.Execute(input)
		if err != nil {
			fmt.Printf("Ошибка: %v\n", err)
		} else {
			fmt.Printf("Результат: %v\n", result)
		}
	}
}
