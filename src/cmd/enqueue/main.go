package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/specvital/collector/internal/infra/db"
	"github.com/specvital/collector/internal/infra/queue"
)

func main() {
	databaseURL := flag.String("database", os.Getenv("DATABASE_URL"), "Database URL")
	flag.Parse()

	if flag.NArg() < 1 {
		printUsage()
		os.Exit(1)
	}

	if *databaseURL == "" {
		fmt.Fprintln(os.Stderr, "Error: Database URL is required (use -database flag or set DATABASE_URL)")
		os.Exit(1)
	}

	owner, repo, err := ParseGitHubURL(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := enqueue(*databaseURL, owner, repo); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to enqueue task: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: enqueue [flags] <github-url>")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Arguments:")
	fmt.Fprintln(os.Stderr, "  <github-url>  GitHub repository URL (e.g., github.com/owner/repo)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Flags:")
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Examples:")
	fmt.Fprintln(os.Stderr, "  enqueue github.com/octocat/Hello-World")
	fmt.Fprintln(os.Stderr, "  enqueue -database postgres://localhost/mydb github.com/owner/repo")
	fmt.Fprintln(os.Stderr, "  enqueue https://github.com/owner/repo.git")
}

func enqueue(databaseURL, owner, repo string) error {
	ctx := context.Background()

	pool, err := db.NewPool(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("database connection: %w", err)
	}
	defer pool.Close()

	client, err := queue.NewClient(ctx, pool)
	if err != nil {
		return fmt.Errorf("create queue client: %w", err)
	}
	defer client.Close()

	if err := client.EnqueueAnalysis(ctx, owner, repo); err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	slog.Info("task enqueued",
		"owner", owner,
		"repo", repo,
	)
	return nil
}
