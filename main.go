// Command githubusermanager reconciles GitHub organisation membership against
// GitHub usernames stored on Google Workspace user accounts.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	gh "github.com/sethvargo/go-githubactions"

	"github.com/reMarkable/githubUserManager/internal/app"
	"github.com/reMarkable/githubUserManager/internal/config"
	"github.com/reMarkable/githubUserManager/internal/githuborg"
	"github.com/reMarkable/githubUserManager/internal/googlews"
)

func main() {
	ctx := context.Background()

	action := gh.New()

	cfg, err := config.Load(action)
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		os.Exit(1)
	}

	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	google, err := googlews.New(ctx, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "google setup error:", err)
		os.Exit(1)
	}

	gh, err := githuborg.New(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "github setup error:", err)
		os.Exit(1)
	}

	exitCode, err := app.Run(ctx, cfg, google, gh)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	os.Exit(exitCode)
}
