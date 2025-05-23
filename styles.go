//go:build !windows
// +build !windows

package main

import "github.com/charmbracelet/lipgloss"

var (
	docStyle      = lipgloss.NewStyle().Margin(1, 2)
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62")) // Light Purple
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("40"))            // Green
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))           // Red
	infoStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))            // Blue
	warningStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))           // Orange
	spinnerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
)
