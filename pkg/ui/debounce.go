package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// DefaultDebounceDelay is the default debounce window for async UI operations.
const DefaultDebounceDelay = 180 * time.Millisecond

// Debouncer tracks generation tokens for debounced Bubble Tea tick commands.
// Zero values are usable and fall back to DefaultDebounceDelay on Schedule.
type Debouncer struct {
	id    uint64
	delay time.Duration
}

// NewDebouncer constructs a Debouncer with the given delay, falling back to
// DefaultDebounceDelay when delay is non-positive.
func NewDebouncer(delay time.Duration) Debouncer {
	if delay <= 0 {
		delay = DefaultDebounceDelay
	}
	return Debouncer{delay: delay}
}

// Next increments the generation token and returns the new token ID.
func (d *Debouncer) Next() uint64 {
	if d == nil {
		return 0
	}
	d.id++
	return d.id
}

// Current returns the active generation token ID.
func (d Debouncer) Current() uint64 {
	return d.id
}

// Matches reports whether id matches the active generation token.
func (d Debouncer) Matches(id uint64) bool {
	return d.id == id
}

// Schedule advances the generation token and returns a tea.Tick command that
// emits msgFactory(id) after the debouncer's delay. If msgFactory is nil, the
// token still advances to invalidate prior ticks and nil is returned.
func (d *Debouncer) Schedule(msgFactory func(id uint64) tea.Msg) tea.Cmd {
	if d == nil {
		return nil
	}
	id := d.Next()
	if msgFactory == nil {
		return nil
	}
	delay := d.delay
	if delay <= 0 {
		delay = DefaultDebounceDelay
	}
	return tea.Tick(delay, func(time.Time) tea.Msg {
		return msgFactory(id)
	})
}

// Cancel invalidates any in-flight debounced ticks by advancing the token.
func (d *Debouncer) Cancel() {
	if d == nil {
		return
	}
	d.id++
}
