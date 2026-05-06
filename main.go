package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"ghclassroom/internal/config"
	"ghclassroom/internal/tui"
	"golang.org/x/term"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "error reading config: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("GitHub personal access token required.")
		fmt.Println("Required scopes: repo, read:org")
		fmt.Println("Generate one at: https://github.com/settings/tokens")
		fmt.Println()
		fmt.Print("Paste token: ")

		tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading token: %v\n", err)
			os.Exit(1)
		}
		token := string(tokenBytes)

		if err := config.ValidateToken(token); err != nil {
			fmt.Fprintf(os.Stderr, "invalid token: %v\n", err)
			os.Exit(1)
		}

		cfg = config.Config{Token: token}
		if err := config.SaveConfig(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "error saving config: %v\n", err)
			os.Exit(1)
		}
	}

	p := tea.NewProgram(tui.New(cfg.Token), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error running TUI: %v\n", err)
		os.Exit(1)
	}
}
