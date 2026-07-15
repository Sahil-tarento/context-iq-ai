package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/example/contextiq/internal/config"
	"github.com/example/contextiq/internal/server"
)

func main() {
	// 1. Define command-line flags
	restPortFlag := flag.String("port", "", "Port for REST HTTP Server")
	grpcPortFlag := flag.String("grpc-port", "", "Port for gRPC Server")
	indexPathFlag := flag.String("index", "", "Trigger immediate repository indexing at the given path")
	flag.Parse()

	// 2. Load Configuration
	cfg := config.LoadConfig()

	// Override config with CLI flags if provided
	if *restPortFlag != "" {
		cfg.RESTPort = *restPortFlag
	}
	if *grpcPortFlag != "" {
		cfg.GRPCPort = *grpcPortFlag
	}

	fmt.Println("==================================================================")
	fmt.Println("                   ContextIQ Core Optimization Engine             ")
	fmt.Println("==================================================================")
	fmt.Printf("Default LLM Provider: %s (Model: %s)\n", cfg.DefaultProv, cfg.DefaultModel)
	fmt.Printf("Database URL:         %s\n", maskConnStr(cfg.DatabaseURL))
	fmt.Printf("Redis URL:            %s\n", cfg.RedisURL)
	fmt.Printf("Qdrant Vector URL:    %s\n", cfg.QdrantURL)

	// 3. Initialize Platform Engine
	platform, err := server.NewPlatform(cfg)
	if err != nil {
		fmt.Printf("CRITICAL: Failed to initialize ContextIQ: %v\n", err)
		os.Exit(1)
	}

	// 4. Handle CLI-triggered Indexing Mode
	if *indexPathFlag != "" {
		fmt.Printf("CLI Mode: Starting indexing for repository: %s...\n", *indexPathFlag)
		start := time.Now()
		files, symbols, err := platform.IndexRepository(context.Background(), *indexPathFlag)
		if err != nil {
			fmt.Printf("ERROR: Indexing failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("SUCCESS: Index completed in %v. Files: %d, Symbols: %d\n", time.Since(start), files, symbols)
		os.Exit(0)
	}

	// 5. Start Servers in Daemon Mode
	// Boot gRPC Server
	gServer, err := server.StartGRPCServer(platform, cfg.GRPCPort)
	if err != nil {
		fmt.Printf("CRITICAL: Failed to start gRPC Server: %v\n", err)
		os.Exit(1)
	}

	// Boot HTTP REST Server
	restServer, err := server.StartRESTServer(platform, cfg.RESTPort)
	if err != nil {
		fmt.Printf("CRITICAL: Failed to start REST Server: %v\n", err)
		os.Exit(1)
	}

	// 6. Wait for Shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigChan
	fmt.Printf("\nReceived shutdown signal: %v. Initiating graceful shutdown...\n", sig)

	// Graceful shutdown REST Server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := restServer.Shutdown(ctx); err != nil {
		fmt.Printf("Error shutting down HTTP server: %v\n", err)
	} else {
		fmt.Println("HTTP REST server stopped gracefully.")
	}

	// Graceful shutdown gRPC Server
	gServer.GracefulStop()
	fmt.Println("gRPC server stopped gracefully.")

	fmt.Println("ContextIQ stopped. Goodbye!")
}

// maskConnStr masks passwords in database connection strings for log safety.
func maskConnStr(connStr string) string {
	if connStr == "" {
		return "[local-in-memory fallback]"
	}
	// Check if connection contains a password
	if strings.Contains(connStr, "@") && strings.Contains(connStr, ":") {
		parts := strings.Split(connStr, "@")
		credentials := strings.Split(parts[0], ":")
		if len(credentials) > 2 {
			return credentials[0] + ":" + "[MASKED]" + "@" + parts[1]
		}
	}
	return connStr
}
