package lib

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
)

// Level defines the severity of the log message
// Higher values = more severe = always shown
type Level int

const (
	Trace Level = iota // Most verbose, lowest priority
	Debug
	Info
	Warn
	Error // Always visible
	Fatal // Always visible, causes exit
)

var levelNames = map[Level]string{
	Trace: "trace",
	Debug: "debug",
	Info:  "info",
	Warn:  "warn",
	Error: "error",
	Fatal: "fatal",
}

var levelFromName = map[string]Level{
	"trace": Trace,
	"debug": Debug,
	"info":  Info,
	"warn":  Warn,
	"error": Error,
	"fatal": Fatal,
}

// AlwaysVisibleLevel is the minimum level that's always shown regardless of config
const AlwaysVisibleLevel = Error

func ParseLevel(level string) Level {
	if lvl, ok := levelFromName[strings.ToLower(level)]; ok {
		return lvl
	}
	return Info // Default to Info as the out-of-the-box config
}

// Predefined terminal colors
var (
	colorTrace   = color.New(color.FgWhite)
	colorDebug   = color.New(color.FgYellow)
	colorInfo    = color.New(color.FgCyan)
	colorWarn    = color.New(color.FgHiYellow)
	colorError   = color.New(color.FgRed)
	colorFatal   = color.New(color.FgHiRed, color.Bold)
	colorAccent  = color.New(color.FgHiGreen)
	colorSuccess = color.New(color.FgGreen)
	colorTitle   = color.New(color.FgHiMagenta, color.Bold)
)

var levelColors = map[Level]*color.Color{
	Trace: colorTrace,
	Debug: colorDebug,
	Info:  colorInfo,
	Warn:  colorWarn,
	Error: colorError,
	Fatal: colorFatal,
}

// ColumnAlign defines text alignment for columns
type ColumnAlign int

const (
	AlignLeft ColumnAlign = iota
	AlignRight
)

// Column represents a formatted column for structured log output
type Column struct {
	Text  string
	Width int
	Align ColumnAlign
}

// Logger represents a customizable logger
type Logger struct {
	WithTimestamp bool
	Level         Level
}

// NewLogger creates a new Logger instance
func NewLogger(withTimestamp bool, level ...Level) *Logger {
	lvl := Info // Default level
	if len(level) > 0 {
		lvl = level[0]
	}

	return &Logger{
		WithTimestamp: withTimestamp,
		Level:         lvl,
	}
}

// shouldLog determines if a message at the given level should be logged
func (l *Logger) shouldLog(lvl Level) bool {
	// Error and Fatal are always visible
	if lvl >= AlwaysVisibleLevel {
		return true
	}
	// Otherwise, only log if message level >= configured level
	return lvl >= l.Level
}

// log prints a message with the specified color and level
func (l *Logger) log(lvl Level, message string) {
	if !l.shouldLog(lvl) {
		return
	}

	c := levelColors[lvl]
	prefix := fmt.Sprintf("[%s]", strings.ToUpper(levelNames[lvl]))

	var output string
	if l.WithTimestamp {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		output = fmt.Sprintf("[%s] %s %s", timestamp, prefix, message)
	} else {
		output = fmt.Sprintf("%s %s", prefix, message)
	}

	c.Println(output)
}

// FormatColumns formats an array of columns into a single string
func FormatColumns(columns []Column) string {
	var parts []string
	for _, col := range columns {
		if col.Width == 0 {
			parts = append(parts, col.Text)
		} else if col.Align == AlignRight {
			parts = append(parts, fmt.Sprintf("%*s", col.Width, col.Text))
		} else {
			parts = append(parts, fmt.Sprintf("%-*s", col.Width, col.Text))
		}
	}
	return strings.Join(parts, "")
}

// Spacer prints blank lines
func (l *Logger) Spacer(lines ...int) {
	n := 1
	if len(lines) > 0 && lines[0] > 0 {
		n = lines[0]
	}
	for i := 0; i < n; i++ {
		fmt.Println()
	}
}

// Divider prints a nice divider, with optional centered text
func (l *Logger) Divider(format string, a ...any) {
	const width = 75

	fmt.Println()
	msg := fmt.Sprintf(format, a...)
	remaining := width - 2 - len(msg)
	if remaining < 1 {
		remaining = 1
	}
	fmt.Printf("- %s %s\n", msg, strings.Repeat("-", remaining))
	fmt.Println()
}

// Init prints an initial message for program startup with timestamp
func (l *Logger) Init() {
	currentDate := time.Now().Format(time.RFC3339)
	divider := strings.Repeat("-", len(currentDate))

	fmt.Println()
	fmt.Println(divider)
	fmt.Println(currentDate)
	fmt.Println(divider)
	fmt.Println()
}

// Title prints a title message
func (l *Logger) Title(message string) {
	fmt.Println()
	colorTitle.Printf("▶︎ %s\n", message)
	fmt.Println()
}

// Terminate prints a termination message with dividers
func (l *Logger) Terminate(message string) {
	divider := strings.Repeat("-", len(message))

	fmt.Println()
	fmt.Println(divider)
	fmt.Println(message)
	fmt.Println(divider)
	fmt.Println()
}

// Success logs a success message
func (l *Logger) Success(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	colorSuccess.Printf("✓ %s\n", msg)
}

// SuccessColumns logs a success message with formatted columns
func (l *Logger) SuccessColumns(columns []Column) {
	colorSuccess.Printf("✓ %s\n", FormatColumns(columns))
}

// Progress logs a progress message
func (l *Logger) Progress(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	colorInfo.Printf("• %s\n", msg)
}

// ProgressColumns logs a progress message with formatted columns
func (l *Logger) ProgressColumns(columns []Column) {
	colorInfo.Printf("• %s\n", FormatColumns(columns))
}

// Trace logs a trace-level message (most verbose)
func (l *Logger) Trace(format string, a ...any) {
	l.log(Trace, fmt.Sprintf(format, a...))
}

// Debug logs a debug-level message
func (l *Logger) Debug(format string, a ...any) {
	l.log(Debug, fmt.Sprintf(format, a...))
}

// Info logs an info-level message
func (l *Logger) Info(format string, a ...any) {
	l.log(Info, fmt.Sprintf(format, a...))
}

// InfoColumns logs an info message with formatted columns
func (l *Logger) InfoColumns(columns []Column) {
	l.log(Info, FormatColumns(columns))
}

// Warn logs a warning-level message
func (l *Logger) Warn(format string, a ...any) {
	l.log(Warn, fmt.Sprintf(format, a...))
}

// Error logs an error-level message (always visible) and returns an error
func (l *Logger) Error(err error, format string, a ...any) error {
	msg := fmt.Sprintf(format, a...)
	l.log(Error, msg)
	return errors.New(msg)
}

// Errorf logs a formatted error message with an error value (always visible) and returns the error
func (l *Logger) Errorf(message string, err error) error {
	if err != nil {
		msg := fmt.Sprintf("%s: %v", message, err)
		l.log(Error, msg)
		return fmt.Errorf("%s: %w", message, err)
	}
	l.log(Error, message)
	return errors.New(message)
}

// Fatal logs a fatal error message, returns an error, and exits (always visible)
func (l *Logger) Fatal(format string, a ...any) error {
	msg := fmt.Sprintf(format, a...)
	l.log(Fatal, msg)
	err := errors.New(msg)
	os.Exit(1)
	return err // unreachable, but satisfies return type
}

// Fatalf logs a formatted fatal error with an error value, returns the error, and exits (always visible)
func (l *Logger) Fatalf(format string, a ...any) error {
	var resultErr error
	l.log(Fatal, fmt.Sprintf(format, a...))
	resultErr = errors.New(fmt.Sprintf(format, a...))
	os.Exit(1)
	return resultErr // unreachable, but satisfies return type
}

// Accent logs a message with accent color at Info level
func (l *Logger) Accent(format string, a ...any) {
	if !l.shouldLog(Info) {
		return
	}
	msg := fmt.Sprintf(format, a...)
	colorAccent.Println(msg)
}

// Custom logs a message with a custom terminal color
func (l *Logger) Custom(message string, c *color.Color, lvl Level) {
	if !l.shouldLog(lvl) {
		return
	}
	c.Println(message)
}

func (l *Logger) Clear() {
	fmt.Print("\033c")
}

func (l *Logger) SetLogLevel(level string) {
	l.Level = ParseLevel(level)
}

var AppLogger = NewLogger(true, Info)

// PerformanceTracker represents a timer for tracking operation performance
type PerformanceTracker struct {
	start     time.Time
	operation string
	logger    *Logger
}

// StartPerformanceTracker creates a new performance tracker
func (l *Logger) StartPerformanceTracker(operation string) *PerformanceTracker {
	l.Debug(fmt.Sprintf("[%s] Starting operation", operation))
	return &PerformanceTracker{
		start:     time.Now(),
		operation: operation,
		logger:    l,
	}
}

// End stops the performance tracker and logs the duration
func (pt *PerformanceTracker) End() {
	duration := time.Since(pt.start)
	pt.logger.Debug(fmt.Sprintf("[%s] Operation completed in %v", pt.operation, duration))
}

// ProgressBar prints a progress bar
func (l *Logger) ProgressBar(current, total int) {
	if !l.shouldLog(Info) {
		return
	}
	colorAccent.Printf("[%d/%d]\n", current, total)
}