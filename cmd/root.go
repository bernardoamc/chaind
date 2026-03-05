package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bernardoamc/chaind/internal/compare"
	"github.com/bernardoamc/chaind/internal/image"
	"github.com/bernardoamc/chaind/internal/output"
	"github.com/bernardoamc/chaind/internal/platform"
	"github.com/bernardoamc/chaind/internal/result"
)

var (
	flagPlatform string
	flagJSON     bool
	flagNoColor  bool
	flagQuiet    bool
	flagSocket   string
)

var rootCmd = &cobra.Command{
	Use:          "chaind <image1> <image2>",
	Short:        "Determine the base image relationship between two images by comparing layer DiffIDs",
	Args:         cobra.ExactArgs(2),
	RunE:         run,
	SilenceUsage: true,
}

func init() {
	rootCmd.Flags().StringVar(&flagPlatform, "platform", "", "Target platform, e.g. linux/arm64/v8 (default: host platform)")
	rootCmd.Flags().BoolVar(&flagJSON, "json", false, "JSON output")
	rootCmd.Flags().BoolVar(&flagNoColor, "no-color", false, "Disable ANSI colors")
	rootCmd.Flags().BoolVarP(&flagQuiet, "quiet", "q", false, "Only the verdict line and warnings")
	rootCmd.Flags().StringVar(&flagSocket, "socket", "", "Docker socket path (default: DOCKER_HOST or /var/run/docker.sock)")
}

// verdictExitError carries a verdict exit code through cobra's error handling.
type verdictExitError struct {
	code int
}

func (e *verdictExitError) Error() string { return "" }

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

func run(cmd *cobra.Command, args []string) error {
	if flagSocket != "" {
		if err := os.Setenv("DOCKER_HOST", "unix://"+flagSocket); err != nil {
			return fmt.Errorf("set DOCKER_HOST: %w", err)
		}
	}

	plat := platform.HostPlatform()
	if flagPlatform != "" {
		p, err := platform.Parse(flagPlatform)
		if err != nil {
			return fmt.Errorf("invalid --platform: %w", err)
		}
		plat = p
	}

	imgA, err := image.Load(args[0], plat)
	if err != nil {
		return fmt.Errorf("load image %s: %w", args[0], err)
	}

	imgB, err := image.Load(args[1], plat)
	if err != nil {
		return fmt.Errorf("load image %s: %w", args[1], err)
	}

	res, err := compare.Compare(
		compare.Input{Ref: args[0], Img: imgA},
		compare.Input{Ref: args[1], Img: imgB},
		platform.String(plat),
	)
	if err != nil {
		return fmt.Errorf("compare images: %w", err)
	}

	var renderer output.Renderer
	if flagJSON {
		renderer = output.NewJSONRenderer(os.Stdout)
	} else {
		renderer = output.NewTextRenderer(os.Stdout, flagNoColor, flagQuiet)
	}
	if err := renderer.Render(res); err != nil {
		return fmt.Errorf("render: %w", err)
	}

	if code := exitCode(res.Verdict); code != 0 {
		return &verdictExitError{code: code}
	}
	return nil
}

func exitCode(v result.Verdict) int {
	switch v {
	case result.VerdictConfirmedBase:
		return 0
	case result.VerdictNotBase:
		return 1
	case result.VerdictSameImage:
		return 2
	default:
		return 10
	}
}
