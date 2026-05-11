package main

import (
	"flag"
	"fmt"
	"os"

	"golang.design/x/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"ghclassroom/internal/config"
	"ghclassroom/internal/tui"
	"golang.org/x/term"
)

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	reconfigure := flag.Bool("reconfigure", false, "delete saved token and re-run first-run setup")
	thresholdFlag := flag.Int("threshold", 0, "override inactivity threshold in days for this session")
	flag.Parse()

	if *versionFlag {
		fmt.Println("ghclassroom v0.1.0")
		os.Exit(0)
	}

	if *reconfigure {
		if err := config.Delete(); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "error deleting config: %v\n", err)
			os.Exit(1)
		}
	}

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

	// Probe clipboard once at startup; pass the result to the TUI so it never
	// calls Init() again and can degrade gracefully on headless/SSH sessions.
	clipboardAvailable := clipboard.Init() == nil

	threshold := cfg.InactivityThresholdDays
	if *thresholdFlag > 0 {
		threshold = *thresholdFlag
	}

	p := tea.NewProgram(tui.New(cfg.Token, clipboardAvailable, threshold, cfg.LastDownloadDir), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error running TUI: %v\n", err)
		os.Exit(1)
	}
}
