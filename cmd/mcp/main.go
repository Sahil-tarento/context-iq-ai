package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/example/contextiq/internal/mcp"
)

func main() {
	daemonURL := flag.String("daemon-url", "http://localhost:9009", "URL of the running ContextIQ daemon")
	flag.Parse()

	fmt.Fprintf(os.Stderr, "[ContextIQ MCP] Starting MCP server, daemon=%s\n", *daemonURL)

	server := mcp.NewServer(*daemonURL)
	if err := server.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[ContextIQ MCP] Fatal: %v\n", err)
		os.Exit(1)
	}
}
