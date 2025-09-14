package internal

import (
	"fmt"
)

// Logger provides consistent styled output methods
type Logger struct{}

// NewLogger creates a new logger instance
func NewLogger() *Logger {
	return &Logger{}
}

// Info prints an info message with consistent styling
func (l *Logger) Info(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Println(InfoStyle.Render(message))
}

// Error prints an error message with consistent styling
func (l *Logger) Error(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Println(ErrorStyle.Render(message))
}

// Success prints a success message with consistent styling
func (l *Logger) Success(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Println(SuccessStyle.Render(message))
}

// Warning prints a warning message with consistent styling
func (l *Logger) Warning(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Println(WarningStyle.Render(message))
}

// Title prints a title message with consistent styling
func (l *Logger) Title(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	fmt.Println(TitleStyle.Render(message))
}

// Plain prints a plain message without styling
func (l *Logger) Plain(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

// Global logger instance for convenience
var Log = NewLogger()