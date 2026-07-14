package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Default daemon endpoint
const DaemonURL = "http://localhost:9009/v1"

func main() {
	// CLI subcommands
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "ask":
		handleAsk(os.Args[2:])
	case "index":
		handleIndex(os.Args[2:])
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("ContextIQ Universal IDE CLI")
	fmt.Println("Usage: contextiq-cli <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  ask      Ask AI a question based on current file context")
	fmt.Println("  index    Trigger repository indexing")
}

func handleAsk(args []string) {
	askCmd := flag.NewFlagSet("ask", flag.ExitOnError)
	query := askCmd.String("query", "", "The question to ask the AI")
	cursorFile := askCmd.String("cursor-file", "", "Absolute path to the active file in the IDE")
	cursorLine := askCmd.Int("cursor-line", 1, "Line number of the cursor")
	repoPath := askCmd.String("repo-path", "", "Absolute path to the workspace root")
	openFiles := askCmd.String("open-files", "", "Comma-separated list of currently open files")

	askCmd.Parse(args)

	if *query == "" {
		fmt.Println("Error: --query is required")
		askCmd.PrintDefaults()
		os.Exit(1)
	}

	openFilesList := []string{}
	if *openFiles != "" {
		// Split mock logic
		// In a real CLI you would use strings.Split(*openFiles, ",")
		openFilesList = append(openFilesList, *cursorFile) 
	}

	payload := map[string]interface{}{
		"query":       *query,
		"cursor_file": *cursorFile,
		"cursor_line": *cursorLine,
		"repo_path":   *repoPath,
		"open_files":  openFilesList,
		"provider":    "mock", // Let the daemon use its defaults
		"model":       "mock-model",
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(DaemonURL+"/chat", "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Printf("Error connecting to ContextIQ Daemon at %s: %v\n", DaemonURL, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Daemon returned error (%d): %s\n", resp.StatusCode, string(respBody))
		os.Exit(1)
	}

	var result struct {
		Response       string  `json:"response"`
		RawTokens      int     `json:"raw_tokens"`
		OptimizedTokens int    `json:"optimized_tokens"`
		Savings        float64 `json:"token_savings"`
	}

	json.Unmarshal(respBody, &result)

	// Output cleanly so IDEs can capture standard output easily
	fmt.Println("==================================================")
	fmt.Printf("Tokens Reduced: %d -> %d (Savings: %.1f%%)\n", result.RawTokens, result.OptimizedTokens, result.Savings)
	fmt.Println("==================================================")
	fmt.Println(result.Response)
}

func handleIndex(args []string) {
	indexCmd := flag.NewFlagSet("index", flag.ExitOnError)
	repoPath := indexCmd.String("repo-path", "", "Absolute path to the workspace root")
	indexCmd.Parse(args)

	if *repoPath == "" {
		fmt.Println("Error: --repo-path is required")
		os.Exit(1)
	}

	payload := map[string]interface{}{
		"repo_path": *repoPath,
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(DaemonURL+"/index", "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Printf("Error connecting to ContextIQ Daemon: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Println("Indexing completed successfully!")
	} else {
		respBody, _ := io.ReadAll(resp.Body)
		fmt.Printf("Indexing failed: %s\n", string(respBody))
	}
}
