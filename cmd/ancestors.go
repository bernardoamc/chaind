package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bernardoamc/chaind/internal/ancestors"
	"github.com/bernardoamc/chaind/internal/image"
	"github.com/bernardoamc/chaind/internal/output"
)

var flagMinDepth int

var ancestorsCmd = &cobra.Command{
	Use:          "ancestors",
	Short:        "Group images by implied shared ancestry using ChainIDs",
	RunE:         runAncestors,
	SilenceUsage: true,
}

func init() {
	ancestorsCmd.Flags().IntVar(&flagMinDepth, "min-depth", 0, "Minimum number of shared layers required to form a group (0 = no minimum)")
}

func runAncestors(cmd *cobra.Command, args []string) error {
	if err := applySocket(); err != nil {
		return err
	}

	ctx := cmd.Context()

	cli, err := image.NewClient()
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}
	defer cli.Close()

	refs, err := cli.ListRefs(ctx)
	if err != nil {
		return fmt.Errorf("list images: %w", err)
	}

	res, err := ancestors.Build(ctx, refs, cli, flagMinDepth)
	if err != nil {
		return fmt.Errorf("build ancestors: %w", err)
	}

	return output.NewJSONRenderer(os.Stdout).RenderAncestors(res)
}
