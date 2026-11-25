package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type testDebounceMsg struct {
	id uint64
}

func TestNewDebouncer(t *testing.T) {
	if DefaultDebounceDelay < 150*time.Millisecond || DefaultDebounceDelay > 200*time.Millisecond {
		t.Fatalf("DefaultDebounceDelay = %v, want between 150ms and 200ms", DefaultDebounceDelay)
	}

	tests := []struct {
		name      string
		delay     time.Duration
		wantDelay time.Duration
	}{
		{
			name:      "zero falls back to default",
			delay:     0,
			wantDelay: DefaultDebounceDelay,
		},
		{
			name:      "negative falls back to default",
			delay:     -25 * time.Millisecond,
			wantDelay: DefaultDebounceDelay,
		},
		{
			name:      "positive custom delay preserved",
			delay:     160 * time.Millisecond,
			wantDelay: 160 * time.Millisecond,
		},
		{
			name:      "sub-millisecond positive delay preserved",
			delay:     500 * time.Microsecond,
			wantDelay: 500 * time.Microsecond,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDebouncer(tc.delay)
			if d.delay != tc.wantDelay {
				t.Errorf("delay = %v, want %v", d.delay, tc.wantDelay)
			}
			if d.Current() != 0 {
				t.Errorf("Current() = %d, want 0", d.Current())
			}
		})
	}
}

func TestDebouncer_NextAndCurrent(t *testing.T) {
	t.Run("increments monotonically", func(t *testing.T) {
		d := NewDebouncer(10 * time.Millisecond)
		steps := []uint64{1, 2, 3, 4}
		for _, want := range steps {
			got := d.Next()
			if got != want {
				t.Fatalf("Next() = %d, want %d", got, want)
			}
			if d.Current() != want {
				t.Fatalf("Current() = %d, want %d", d.Current(), want)
			}
		}
	})

	t.Run("nil receiver returns zero safely", func(t *testing.T) {
		var d *Debouncer
		if got := d.Next(); got != 0 {
			t.Errorf("(*Debouncer)(nil).Next() = %d, want 0", got)
		}
	})
}

func TestDebouncer_Matches(t *testing.T) {
	tests := []struct {
		name  string
		setup func() Debouncer
		id    uint64
		want  bool
	}{
		{
			name:  "initial state matches zero",
			setup: func() Debouncer { return NewDebouncer(10 * time.Millisecond) },
			id:    0,
			want:  true,
		},
		{
			name: "matches current after Next",
			setup: func() Debouncer {
				d := NewDebouncer(10 * time.Millisecond)
				d.Next()
				return d
			},
			id:   1,
			want: true,
		},
		{
			name: "stale token does not match after Next",
			setup: func() Debouncer {
				d := NewDebouncer(10 * time.Millisecond)
				d.Next()
				d.Next()
				return d
			},
			id:   1,
			want: false,
		},
		{
			name: "zero token does not match after Next",
			setup: func() Debouncer {
				d := NewDebouncer(10 * time.Millisecond)
				d.Next()
				return d
			},
			id:   0,
			want: false,
		},
		{
			name: "future token does not match",
			setup: func() Debouncer {
				d := NewDebouncer(10 * time.Millisecond)
				d.Next()
				return d
			},
			id:   99,
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := tc.setup()
			if got := d.Matches(tc.id); got != tc.want {
				t.Errorf("Matches(%d) = %v, want %v (current=%d)", tc.id, got, tc.want, d.Current())
			}
		})
	}
}

func TestDebouncer_Cancel(t *testing.T) {
	tests := []struct {
		name        string
		cancelCount int
		priorNexts  int
	}{
		{
			name:        "single cancel invalidates active token",
			priorNexts:  1,
			cancelCount: 1,
		},
		{
			name:        "multiple cancels advance token each time",
			priorNexts:  2,
			cancelCount: 3,
		},
		{
			name:        "cancel from zero state advances token",
			priorNexts:  0,
			cancelCount: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDebouncer(10 * time.Millisecond)
			var lastID uint64
			for i := 0; i < tc.priorNexts; i++ {
				lastID = d.Next()
			}
			for i := 0; i < tc.cancelCount; i++ {
				d.Cancel()
			}

			wantCurrent := uint64(tc.priorNexts + tc.cancelCount)
			if d.Current() != wantCurrent {
				t.Errorf("Current() = %d, want %d", d.Current(), wantCurrent)
			}
			if d.Matches(lastID) {
				t.Errorf("Matches(%d) = true after Cancel, want false", lastID)
			}
		})
	}

	t.Run("nil receiver does not panic", func(t *testing.T) {
		var d *Debouncer
		d.Cancel()
	})
}

func TestDebouncer_Schedule(t *testing.T) {
	tests := []struct {
		name           string
		delay          time.Duration
		schedules      int
		cancelAfter    bool
		nilFactory     bool
		wantNilCmd     bool
		wantLatestOnly bool
	}{
		{
			name:           "single schedule emits matching message",
			delay:          2 * time.Millisecond,
			schedules:      1,
			wantLatestOnly: true,
		},
		{
			name:           "rapid reschedules invalidate earlier ticks",
			delay:          2 * time.Millisecond,
			schedules:      3,
			wantLatestOnly: true,
		},
		{
			name:        "cancel after schedule invalidates tick message",
			delay:       2 * time.Millisecond,
			schedules:   1,
			cancelAfter: true,
		},
		{
			name:       "nil factory advances token and returns nil cmd",
			delay:      2 * time.Millisecond,
			schedules:  1,
			nilFactory: true,
			wantNilCmd: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDebouncer(tc.delay)
			factory := func(id uint64) tea.Msg {
				return testDebounceMsg{id: id}
			}
			if tc.nilFactory {
				factory = nil
			}

			cmds := make([]tea.Cmd, 0, tc.schedules)
			for i := 0; i < tc.schedules; i++ {
				cmd := d.Schedule(factory)
				cmds = append(cmds, cmd)
			}

			if tc.cancelAfter {
				d.Cancel()
			}

			if tc.wantNilCmd {
				for i, cmd := range cmds {
					if cmd != nil {
						t.Fatalf("cmds[%d] != nil, want nil", i)
					}
				}
				if d.Current() != uint64(tc.schedules) {
					t.Errorf("Current() = %d, want %d", d.Current(), tc.schedules)
				}
				return
			}

			for i, cmd := range cmds {
				if cmd == nil {
					t.Fatalf("cmds[%d] == nil, want non-nil tea.Cmd", i)
				}
				msg := cmd()
				debMsg, ok := msg.(testDebounceMsg)
				if !ok {
					t.Fatalf("cmds[%d]() returned %T, want testDebounceMsg", i, msg)
				}
				wantID := uint64(i + 1)
				if debMsg.id != wantID {
					t.Errorf("cmds[%d]() id = %d, want %d", i, debMsg.id, wantID)
				}

				isLatest := i == len(cmds)-1
				wantMatch := tc.wantLatestOnly && isLatest && !tc.cancelAfter
				if gotMatch := d.Matches(debMsg.id); gotMatch != wantMatch {
					t.Errorf("d.Matches(%d) = %v, want %v", debMsg.id, gotMatch, wantMatch)
				}
			}
		})
	}

	t.Run("zero-value debouncer schedules safely with default delay", func(t *testing.T) {
		var d Debouncer
		cmd := d.Schedule(func(id uint64) tea.Msg {
			return testDebounceMsg{id: id}
		})
		if cmd == nil {
			t.Fatal("expected non-nil tea.Cmd for zero-value Debouncer")
		}
		if d.Current() != 1 {
			t.Fatalf("Current() = %d, want 1", d.Current())
		}
		msg, ok := cmd().(testDebounceMsg)
		if !ok {
			t.Fatalf("unexpected msg type %T", msg)
		}
		if !d.Matches(msg.id) {
			t.Errorf("expected Matches(%d) == true", msg.id)
		}
	})

	t.Run("nil receiver returns nil cmd", func(t *testing.T) {
		var d *Debouncer
		cmd := d.Schedule(func(id uint64) tea.Msg {
			return testDebounceMsg{id: id}
		})
		if cmd != nil {
			t.Errorf("expected nil tea.Cmd for nil receiver")
		}
	})
}
