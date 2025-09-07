package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var height int
var name string

var rootCmd = &cobra.Command{
	Use:   "ghtrend",
	Short: "Explore GitHub Trending directly from your terminal",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Hello from ghtrend! 🚀")
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
	rootCmd.Flags().IntVarP(&height, "height", "H", 0, "Set the height")
	rootCmd.Flags().StringVarP(&name, "name", "n", "default", "Set the server name")
}
