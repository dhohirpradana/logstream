package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	apiURL  string
	rootCmd = &cobra.Command{
		Use:   "logstream",
		Short: "Log query CLI",
	}
)

func init() {
	rootCmd.PersistentFlags().StringVar(&apiURL, "api", "http://localhost:8080", "API URL")

	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(tailCmd)
	rootCmd.AddCommand(statsCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
