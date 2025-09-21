package cmd

import (
	"fmt"
	"github.com/PhuocThinhkkk/ghtrend/pkg/app"
	"github.com/PhuocThinhkkk/ghtrend/pkg/configs/flags"
	"os"

	"github.com/spf13/cobra"
)
const defauleLimit = 2202

var limit int
var since string
var lang string
var noCache bool

var rootCmd = &cobra.Command{
	Use:   "ghtrend",
	Short: "Explore GitHub Trending directly from your terminal",
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
	rootCmd.Flags().IntVarP(&limit, "limit", "l", defauleLimit, "Set the limit repos")
	rootCmd.Flags().StringVarP(&since, "since", "r", "daily", "Date range (daily/weekly/monthly)")
	rootCmd.Flags().StringVarP(&lang, "lang", "L", "All", "filter by programming languages")
	rootCmd.Flags().BoolVar(&noCache, "no-cache", false, "Ignore cached data and fetch fresh")
}
