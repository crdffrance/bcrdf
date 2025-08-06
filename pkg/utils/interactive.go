package utils

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// PromptString prompts the user for a string input
func PromptString(prompt, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)

	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", prompt, defaultValue)
	} else {
		fmt.Printf("%s: ", prompt)
	}

	input, err := reader.ReadString('\n')
	if err != nil {
		return defaultValue
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}

	return input
}

// PromptPassword prompts the user for a password (hidden input would be ideal, but for simplicity we'll use regular input)
func PromptPassword(prompt string) string {
	fmt.Printf("%s: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(input)
}

// PromptInt prompts the user for an integer input
func PromptInt(prompt string, defaultValue int, min, max int) int {
	for {
		defaultStr := fmt.Sprintf("%d", defaultValue)
		input := PromptString(prompt, defaultStr)

		if input == defaultStr {
			return defaultValue
		}

		value, err := strconv.Atoi(input)
		if err != nil {
			fmt.Printf("❌ Invalid number. Please enter a number between %d and %d.\n", min, max)
			continue
		}

		if value < min || value > max {
			fmt.Printf("❌ Number must be between %d and %d.\n", min, max)
			continue
		}

		return value
	}
}

// PromptChoice prompts the user to choose from a list of options
func PromptChoice(prompt string, choices []string, defaultChoice int) int {
	fmt.Printf("\n%s\n", prompt)
	for i, choice := range choices {
		marker := " "
		if i == defaultChoice {
			marker = ">"
		}
		fmt.Printf(" %s %d. %s\n", marker, i+1, choice)
	}

	for {
		input := PromptString(fmt.Sprintf("Choose (1-%d)", len(choices)), fmt.Sprintf("%d", defaultChoice+1))

		choice, err := strconv.Atoi(input)
		if err != nil {
			fmt.Printf("❌ Invalid choice. Please enter a number between 1 and %d.\n", len(choices))
			continue
		}

		if choice < 1 || choice > len(choices) {
			fmt.Printf("❌ Choice must be between 1 and %d.\n", len(choices))
			continue
		}

		return choice - 1
	}
}

// PromptYesNo prompts the user for a yes/no answer
func PromptYesNo(prompt string, defaultValue bool) bool {
	defaultStr := "n"
	if defaultValue {
		defaultStr = "y"
	}

	for {
		input := strings.ToLower(PromptString(prompt+" (y/n)", defaultStr))

		switch input {
		case "y", "yes", "true", "1":
			return true
		case "n", "no", "false", "0":
			return false
		default:
			fmt.Printf("❌ Please answer with 'y' or 'n'.\n")
		}
	}
}

// PrintHeader prints a formatted header
func PrintHeader(title string) {
	separator := strings.Repeat("=", 60)
	fmt.Printf("\n%s\n", separator)
	fmt.Printf("  %s\n", strings.ToUpper(title))
	fmt.Printf("%s\n\n", separator)
}

// PrintSection prints a formatted section header
func PrintSection(title string) {
	separator := strings.Repeat("-", 40)
	fmt.Printf("\n%s\n", separator)
	fmt.Printf("  %s\n", title)
	fmt.Printf("%s\n\n", separator)
}

// PrintSuccess prints a success message
func PrintSuccess(message string) {
	fmt.Printf("✅ %s\n", message)
}

// PrintWarning prints a warning message
func PrintWarning(message string) {
	fmt.Printf("⚠️  %s\n", message)
}

// PrintInfo prints an info message
func PrintInfo(message string) {
	fmt.Printf("ℹ️  %s\n", message)
}
