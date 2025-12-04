package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tripsearch",
	Short: "Search for bus/train trips",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Trip searcher initialized")
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
