package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bernardoamc/chaind/internal/compare"
	"github.com/bernardoamc/chaind/internal/image"
	"github.com/bernardoamc/chaind/internal/output"
	"github.com/bernardoamc/chaind/internal/platform"
	"github.com/bernardoamc/chaind/internal/result"
)

var flagPlatform string

var compareCmd = &cobra.Command{
	Use:          "compare <image1> <image2>",
	Short:        "Determine the base image relationship between two images",
	Args:         cobra.ExactArgs(2),
	RunE:         runCompare,
	SilenceUsage: true,
}

func init() {
	compareCmd.Flags().StringVar(&flagPlatform, "platform", "", "Target platform, e.g. linux/arm64/v8 (default: host platform)")
}

func runCompare(cmd *cobra.Command, args []string) error {
	if err := applySocket(); err != nil {
		return err
	}

	plat := platform.HostPlatform()
	if flagPlatform != "" {
		p, err := platform.Parse(flagPlatform)
		if err != nil {
			return fmt.Errorf("invalid --platform: %w", err)
		}
		plat = p
	}

	cli, err := image.NewClient()
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}
	defer cli.Close()

	imgA, err := cli.Load(args[0], plat)
	if err != nil {
		return fmt.Errorf("load image %s: %w", args[0], err)
	}

	imgB, err := cli.Load(args[1], plat)
	if err != nil {
		return fmt.Errorf("load image %s: %w", args[1], err)
	}

	metaA, err := image.Extract(args[0], imgA)
	if err != nil {
		return fmt.Errorf("extract metadata %s: %w", args[0], err)
	}

	metaB, err := image.Extract(args[1], imgB)
	if err != nil {
		return fmt.Errorf("extract metadata %s: %w", args[1], err)
	}

	res := compare.Compare(metaA, metaB, platform.String(plat))

	renderer := output.NewJSONRenderer(os.Stdout)
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
