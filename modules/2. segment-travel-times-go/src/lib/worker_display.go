package lib

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

// WorkerDisplay manages a live-updating terminal display for parallel workers.
// It uses ANSI escape codes to update worker statuses, progress, and stats in place.
type WorkerDisplay struct {
	mu        sync.Mutex
	workers   []workerState
	total     int
	completed int
	skipped   int
	errors    int
	records   int
	rendered  bool
	lineCount int
	done      chan struct{}
	isTTY     bool
	// Non-TTY fallback tracking
	lastLoggedCompleted int
}

type workerState struct {
	lineID int
	status string
	active bool
}

// Display colors
var (
	colorWorkerLabel = color.New(color.FgCyan, color.Bold)
	colorWorkerIdle  = color.New(color.Faint)
	colorBarFilled   = color.New(color.FgGreen)
	colorBarEmpty    = color.New(color.Faint)
	colorStats       = color.New(color.FgGreen)
	colorStatsLabel  = color.New(color.Faint)
)

// NewWorkerDisplay creates a new WorkerDisplay and starts the refresh loop.
func NewWorkerDisplay(workerCount, totalLines int) *WorkerDisplay {
	wd := &WorkerDisplay{
		workers: make([]workerState, workerCount),
		total:   totalLines,
		done:    make(chan struct{}),
		isTTY:   isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()),
	}

	if wd.isTTY {
		go wd.refreshLoop()
	}

	return wd
}

// UpdateWorker updates a worker's current status.
func (wd *WorkerDisplay) UpdateWorker(id int, lineID int, status string) {
	wd.mu.Lock()
	defer wd.mu.Unlock()

	if id < 0 || id >= len(wd.workers) {
		return
	}

	wd.workers[id] = workerState{
		lineID: lineID,
		status: status,
		active: true,
	}
}

// IdleWorker marks a worker as idle.
func (wd *WorkerDisplay) IdleWorker(id int) {
	wd.mu.Lock()
	defer wd.mu.Unlock()

	if id < 0 || id >= len(wd.workers) {
		return
	}

	wd.workers[id] = workerState{active: false}
}

// LineCompleted increments the completed count and adds to total records.
func (wd *WorkerDisplay) LineCompleted(records int) {
	wd.mu.Lock()
	defer wd.mu.Unlock()

	wd.completed++
	wd.records += records

	// Non-TTY fallback: log periodically
	if !wd.isTTY {
		total := wd.completed + wd.skipped + wd.errors
		if total-wd.lastLoggedCompleted >= 50 || total == wd.total {
			AppLogger.Info("Progress: %d/%d lines (%d completed, %d skipped, %d errors, %s records)",
				total, wd.total, wd.completed, wd.skipped, wd.errors, FormatCount(wd.records))
			wd.lastLoggedCompleted = total
		}
	}
}

// LineSkipped increments the skipped count.
func (wd *WorkerDisplay) LineSkipped() {
	wd.mu.Lock()
	defer wd.mu.Unlock()

	wd.skipped++

	// Non-TTY fallback
	if !wd.isTTY {
		total := wd.completed + wd.skipped + wd.errors
		if total-wd.lastLoggedCompleted >= 50 || total == wd.total {
			AppLogger.Info("Progress: %d/%d lines (%d completed, %d skipped, %d errors, %s records)",
				total, wd.total, wd.completed, wd.skipped, wd.errors, FormatCount(wd.records))
			wd.lastLoggedCompleted = total
		}
	}
}

// LineErrored increments the error count.
func (wd *WorkerDisplay) LineErrored() {
	wd.mu.Lock()
	defer wd.mu.Unlock()

	wd.errors++
}

// Stop stops the refresh loop and clears the display.
func (wd *WorkerDisplay) Stop() {
	close(wd.done)

	if wd.isTTY {
		// Small delay to ensure the last render completes
		time.Sleep(200 * time.Millisecond)

		wd.mu.Lock()
		defer wd.mu.Unlock()

		// Clear the display block
		if wd.rendered {
			wd.clearDisplay()
		}
	}
}

// refreshLoop periodically redraws the display.
func (wd *WorkerDisplay) refreshLoop() {
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-wd.done:
			return
		case <-ticker.C:
			wd.mu.Lock()
			wd.render()
			wd.mu.Unlock()
		}
	}
}

// render redraws the full display block. Must be called with mu held.
func (wd *WorkerDisplay) render() {
	if !wd.isTTY {
		return
	}

	// Calculate line count: workers + blank line + progress bar + stats line
	newLineCount := len(wd.workers) + 3

	// If already rendered, move cursor up to overwrite
	if wd.rendered {
		fmt.Printf("\033[%dA", wd.lineCount)
	}

	// Render each worker
	for i, w := range wd.workers {
		fmt.Print("\033[K") // Clear line
		if w.active {
			label := colorWorkerLabel.Sprintf("  [W%d]", i+1)
			fmt.Printf("%s Line %-6d %s\n", label, w.lineID, w.status)
		} else {
			fmt.Printf("  ")
			colorWorkerIdle.Printf("[W%d] ── idle ──", i+1)
			fmt.Println()
		}
	}

	// Blank line
	fmt.Print("\033[K\n")

	// Progress bar
	wd.renderProgressBar()

	// Stats line
	fmt.Print("\033[K")
	wd.renderStats()

	wd.lineCount = newLineCount
	wd.rendered = true
}

// renderProgressBar renders a custom progress bar. Must be called with mu held.
func (wd *WorkerDisplay) renderProgressBar() {
	fmt.Print("\033[K") // Clear line

	done := wd.completed + wd.skipped + wd.errors
	barWidth := 40
	filled := 0
	if wd.total > 0 {
		filled = (done * barWidth) / wd.total
	}
	if filled > barWidth {
		filled = barWidth
	}
	pct := 0.0
	if wd.total > 0 {
		pct = float64(done) / float64(wd.total) * 100
	}

	fmt.Print("  ")
	colorBarFilled.Print(repeatRune('█', filled))
	colorBarEmpty.Print(repeatRune('░', barWidth-filled))
	fmt.Printf(" %d/%d (%.1f%%)\n", done, wd.total, pct)
}

// renderStats renders the stats line. Must be called with mu held.
func (wd *WorkerDisplay) renderStats() {
	fmt.Print("  ")
	colorStats.Printf("✓ %d", wd.completed)
	colorStatsLabel.Print(" completed")
	colorStatsLabel.Print(" · ")
	fmt.Printf("%d", wd.skipped)
	colorStatsLabel.Print(" skipped")
	if wd.errors > 0 {
		colorStatsLabel.Print(" · ")
		colorError.Printf("%d", wd.errors)
		colorStatsLabel.Print(" errors")
	}
	colorStatsLabel.Print(" · ")
	colorStats.Printf("%s", FormatCount(wd.records))
	colorStatsLabel.Print(" records")
	fmt.Println()
}

// clearDisplay clears the display block. Must be called with mu held.
func (wd *WorkerDisplay) clearDisplay() {
	if wd.lineCount > 0 {
		fmt.Printf("\033[%dA", wd.lineCount)
		for i := 0; i < wd.lineCount; i++ {
			fmt.Print("\033[K\n")
		}
		fmt.Printf("\033[%dA", wd.lineCount)
	}
}

// FormatCount formats large numbers for readability (e.g. 245.3K, 1.2M).
func FormatCount(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

// repeatRune creates a string by repeating a rune n times.
func repeatRune(r rune, n int) string {
	if n <= 0 {
		return ""
	}
	result := make([]rune, n)
	for i := range result {
		result[i] = r
	}
	return string(result)
}
