package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/PhuocThinhkkk/ghtrend/pkg/app"
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

var rootCmd = &cobra.Command{
	Use:   "ghtrend",
	Short: "Explore GitHub Trending and user repositories directly from your terminal",
	Run: func(cmd *cobra.Command, args []string) {
		useCache := true
		if cmd.Flags().Changed("no-cache") && noCache {
			useCache = false
		}
		fred := flags.Frequency(since)
		cfg, err := flags.NewConfig(useCache, limit, fred, lang)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if cmd.Flags().Changed("theme") {
			cfg.Theme = theme
		}
		if cmd.Flags().Changed("user") {
			trimmedUser := strings.TrimSpace(user)
			if trimmedUser == "" {
				fmt.Fprintln(os.Stderr, "❌ Error: username cannot be empty")
				os.Exit(1)
			}
			cfg.User = trimmedUser
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
		fmt.Fprintf(os.Stderr, "👉 Try '%s --help' for usage.\n", rootCmd.Use)
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
