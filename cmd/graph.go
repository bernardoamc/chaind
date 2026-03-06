package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bernardoamc/chaind/internal/graph"
	"github.com/bernardoamc/chaind/internal/image"
	"github.com/bernardoamc/chaind/internal/output"
)

var graphCmd = &cobra.Command{
	Use:          "graph",
	Short:        "Map base image relationships across all local images",
	RunE:         runGraph,
	SilenceUsage: true,
}

func runGraph(cmd *cobra.Command, args []string) error {
	if err := applySocket(); err != nil {
		return err
	}

	ctx := cmd.Context()

	refs, err := image.ListRefs(ctx)
	if err != nil {
		return fmt.Errorf("list images: %w", err)
	}

	res, err := graph.Build(ctx, refs)
	if err != nil {
		return fmt.Errorf("build graph: %w", err)
	}

	return output.NewJSONRenderer(os.Stdout).RenderGraph(res)
}
