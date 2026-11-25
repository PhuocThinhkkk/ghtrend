package cmd

import (
	"testing"

	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
	"github.com/spf13/cobra"
)

func newTestRootCmd() *cobra.Command {
	c := &cobra.Command{
		Use:  "ghtrend [owner/repo | @username]",
		Args: cobra.MaximumNArgs(1),
	}
	c.Flags().IntVarP(&limit, "limit", "l", defaultLimit, "")
	c.Flags().StringVarP(&since, "since", "r", "daily", "")
	c.Flags().StringVarP(&lang, "lang", "L", "All", "")
	c.Flags().BoolVar(&noCache, "no-cache", false, "")
	c.Flags().StringVarP(&theme, "theme", "t", "", "")
	c.Flags().StringVarP(&user, "user", "u", "", "")
	return c
}

func resetFlagGlobals() {
	limit = defaultLimit
	since = "daily"
	lang = "All"
	noCache = false
	theme = ""
	user = ""
}

func TestBuildCmdConfig_ZeroArgLaunchHome(t *testing.T) {
	resetFlagGlobals()
	t.Cleanup(resetFlagGlobals)

	c := newTestRootCmd()
	if err := c.ParseFlags([]string{}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	cfg, err := buildCmdConfig(c, nil)
	if err != nil {
		t.Fatalf("buildCmdConfig: %v", err)
	}
	if !cfg.LaunchHome {
		t.Errorf("expected LaunchHome=true for zero-arg launch")
	}

	// Theme flag alone still launches home launcher.
	resetFlagGlobals()
	c2 := newTestRootCmd()
	if err := c2.ParseFlags([]string{"--theme", "dracula"}); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	cfg2, err := buildCmdConfig(c2, nil)
	if err != nil {
		t.Fatalf("buildCmdConfig: %v", err)
	}
	if !cfg2.LaunchHome || cfg2.Theme != "dracula" {
		t.Errorf("expected LaunchHome=true and Theme=dracula, got %+v", cfg2)
	}
}

func TestBuildCmdConfig_FlagsAndPositionalArgs(t *testing.T) {
	tests := []struct {
		name           string
		flags          []string
		args           []string
		wantHome       bool
		wantUser       string
		wantDirectRepo string
		wantSince      flags.Frequency
		wantErr        bool
	}{
		{
			name:      "explicit since skips home",
			flags:     []string{"--since", "weekly"},
			wantHome:  false,
			wantSince: flags.Weekly,
		},
		{
			name:      "explicit lang skips home",
			flags:     []string{"--lang", "go"},
			wantHome:  false,
			wantSince: flags.Daily,
		},
		{
			name:      "explicit user flag skips home",
			flags:     []string{"--user", "@torvalds"},
			wantHome:  false,
			wantUser:  "torvalds",
			wantSince: flags.Daily,
		},
		{
			name:           "positional owner/repo skips home",
			args:           []string{"charmbracelet/bubbletea"},
			wantHome:       false,
			wantDirectRepo: "charmbracelet/bubbletea",
			wantSince:      flags.Daily,
		},
		{
			name:           "positional GitHub URL skips home",
			args:           []string{"https://github.com/golang/go"},
			wantHome:       false,
			wantDirectRepo: "golang/go",
			wantSince:      flags.Daily,
		},
		{
			name:      "positional @username sets user",
			args:      []string{"@octocat"},
			wantHome:  false,
			wantUser:  "octocat",
			wantSince: flags.Daily,
		},
		{
			name:    "invalid positional target errors",
			args:    []string{"../bad/path"},
			wantErr: true,
		},
		{
			name:    "bare @ user flag errors",
			flags:   []string{"--user", "@"},
			wantErr: true,
		},
		{
			name:    "invalid user flag errors",
			flags:   []string{"--user", "../bad"},
			wantErr: true,
		},
		{
			name:      "url user flag normalizes username",
			flags:     []string{"--user", "https://github.com/octocat"},
			wantHome:  false,
			wantUser:  "octocat",
			wantSince: flags.Daily,
		},
		{
			name:    "cannot combine user flag with positional repo",
			flags:   []string{"--user", "torvalds"},
			args:    []string{"golang/go"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetFlagGlobals()
			t.Cleanup(resetFlagGlobals)

			c := newTestRootCmd()
			if err := c.ParseFlags(tc.flags); err != nil {
				t.Fatalf("ParseFlags: %v", err)
			}
			cfg, err := buildCmdConfig(c, tc.args)
			if (err != nil) != tc.wantErr {
				t.Fatalf("buildCmdConfig err=%v, wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if cfg.LaunchHome != tc.wantHome {
				t.Errorf("LaunchHome = %v, want %v", cfg.LaunchHome, tc.wantHome)
			}
			if cfg.User != tc.wantUser {
				t.Errorf("User = %q, want %q", cfg.User, tc.wantUser)
			}
			if cfg.DirectRepo != tc.wantDirectRepo {
				t.Errorf("DirectRepo = %q, want %q", cfg.DirectRepo, tc.wantDirectRepo)
			}
			if cfg.Since != tc.wantSince {
				t.Errorf("Since = %q, want %q", cfg.Since, tc.wantSince)
			}
		})
	}
}
