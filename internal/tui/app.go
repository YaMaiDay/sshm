package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/host"
)

func Run(hosts []host.Host, passwords config.PasswordStore) error {
	model := New(hosts, passwords)
	program := tea.NewProgram(model, tea.WithAltScreen())
	_, err := program.Run()
	return err
}

func Fatal(message string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", message, err)
	} else {
		fmt.Fprintln(os.Stderr, message)
	}
	os.Exit(1)
}
