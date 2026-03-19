package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

func IsConfigured() bool {
	// In Go, typically we use godotenv.Load() to load .env files into os.Environ()
	// For this TDD exercise, we just check the environment variable directly.
	return os.Getenv("MSWEA_CONFIGURED") == "true"
}

func RunSetupWizard(in io.Reader, out io.Writer, envFilePath string) error {
	scanner := bufio.NewScanner(in)

	fmt.Fprintln(out, "To get started, we need to set up your global config file.")
	fmt.Fprint(out, "Enter your default model (e.g., openai/gpt-4o): ")
	scanner.Scan()
	model := strings.TrimSpace(scanner.Text())

	fmt.Fprint(out, "Enter your API key name (e.g., OPENAI_API_KEY): ")
	scanner.Scan()
	keyName := strings.TrimSpace(scanner.Text())

	var keyValue string
	if keyName != "" {
		fmt.Fprintf(out, "Enter your API key value for %s: ", keyName)
		scanner.Scan()
		keyValue = strings.TrimSpace(scanner.Text())
	}

	// Write to file (in Python this uses dotenv.set_key)
	f, err := os.OpenFile(envFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	if model != "" {
		fmt.Fprintf(f, "MSWEA_MODEL_NAME=\"%s\"\n", model)
	}
	if keyName != "" && keyValue != "" {
		fmt.Fprintf(f, "%s=\"%s\"\n", keyName, keyValue)
	}
	fmt.Fprintf(f, "MSWEA_CONFIGURED=\"true\"\n")

	return nil
}
