package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var flagSocket string

var rootCmd = &cobra.Command{
	Use:   "chaind",
	Short: "Determine base image relationships between container images",
}

// verdictExitError carries a verdict exit code through cobra's error handling.
type verdictExitError struct {
	code int
}

func (e *verdictExitError) Error() string { return "" }

func init() {
	rootCmd.PersistentFlags().StringVar(&flagSocket, "socket", "", "Docker socket path (default: DOCKER_HOST or /var/run/docker.sock)")
	rootCmd.AddCommand(compareCmd)
}

func Execute() {
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		var ve *verdictExitError
		if errors.As(err, &ve) {
			os.Exit(ve.code)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(10)
	}
}
