package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/PhuocThinhkkk/ghtrend/pkg/app"
	"github.com/PhuocThinhkkk/ghtrend/pkg/cache"
	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
	"github.com/spf13/cobra"
)

const defaultLimit = 2202

var limit int
var since string
var lang string
var noCache bool
var theme string
var user string

func buildCmdConfig(cmd *cobra.Command, args []string) (*flags.CmdConfig, error) {
	useCache := true
	if cmd.Flags().Changed("no-cache") && noCache {
		useCache = false
	}
	fred := flags.Frequency(since)
	cfg, err := flags.NewConfig(useCache, limit, fred, lang)
	if err != nil {
		return nil, err
	}
	if cmd.Flags().Changed("theme") {
		cfg.Theme = theme
	}
	if cmd.Flags().Changed("user") {
		trimmedUser := strings.TrimSpace(user)
		u, ok := cache.ParseUserTarget(trimmedUser)
		if !ok {
			return nil, fmt.Errorf("invalid GitHub username %q", trimmedUser)
		}
		cfg.User = u
	}

	if len(args) > 0 {
		target := strings.TrimSpace(args[0])
		if target != "" {
			if cmd.Flags().Changed("user") {
				return nil, fmt.Errorf("cannot combine --user with positional target %q", target)
			}
			if strings.HasPrefix(target, "@") {
				if u, ok := cache.ParseUserTarget(target); ok {
					cfg.User = u
				} else {
					return nil, fmt.Errorf("invalid GitHub username %q", target)
				}
			} else if owner, repo, ok := cache.ParseRepoSlug(target); ok {
				cfg.DirectRepo = owner + "/" + repo
			} else if u, ok := cache.ParseUserTarget(target); ok {
				cfg.User = u
			} else {
				return nil, fmt.Errorf("invalid target %q (expected owner/repo or @username)", target)
			}
		}
	}

	hasActionFlag := cmd.Flags().Changed("user") ||
		cmd.Flags().Changed("since") ||
		cmd.Flags().Changed("lang") ||
		cmd.Flags().Changed("limit") ||
		cmd.Flags().Changed("no-cache")
	if !hasActionFlag && cfg.User == "" && cfg.DirectRepo == "" {
		cfg.LaunchHome = true
	}
	return cfg, nil
}

var rootCmd = &cobra.Command{
	Use:   "ghtrend [owner/repo | @username]",
	Short: "Explore GitHub Trending, user profiles, and repositories directly from your terminal",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := buildCmdConfig(cmd, args)
		if err != nil {
			fmt.Fprintln(os.Stderr, "❌ Error:", err)
			os.Exit(1)
		}
		application := app.NewApp(cfg)
		application.Start()
	},
}

func Execute() {
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		fmt.Fprintf(os.Stderr, "👉 Try '%s --help' for usage.\n", rootCmd.CommandPath())
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().IntVarP(&limit, "limit", "l", defaultLimit, "Set the limit repos")
	rootCmd.Flags().StringVarP(&since, "since", "r", "daily", "Date range (daily/weekly/monthly)")
	rootCmd.Flags().StringVarP(&lang, "lang", "L", "All", "Filter by programming language")
	rootCmd.Flags().BoolVar(&noCache, "no-cache", false, "Ignore cached data and fetch fresh")
	rootCmd.Flags().StringVarP(&theme, "theme", "t", "", "Theme name (catppuccin, tokyonight, dracula, nord, gruvbox, cyberpunk, classic)")
	rootCmd.Flags().StringVarP(&user, "user", "u", "", "Fetch and display repositories of a specific GitHub user")
}
