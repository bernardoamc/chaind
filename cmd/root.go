package cmd

import (
	"encoding/json"
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
	rootCmd.AddCommand(ancestorsCmd)
	rootCmd.AddCommand(compareCmd)
	rootCmd.AddCommand(graphCmd)
}

func applySocket() error {
	if flagSocket != "" {
		if err := os.Setenv("DOCKER_HOST", "unix://"+flagSocket); err != nil {
			return fmt.Errorf("set DOCKER_HOST: %w", err)
		}
	}
	return nil
}

func Execute() {
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		var ve *verdictExitError
		if errors.As(err, &ve) {
			os.Exit(ve.code)
		}
		data, _ := json.Marshal(struct {
			Error string `json:"error"`
			Code  int    `json:"code"`
		}{err.Error(), 10})
		fmt.Fprintf(os.Stderr, "%s\n", data)
		os.Exit(10)
	}
}
