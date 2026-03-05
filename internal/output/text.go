package output

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/bernardoamc/chaind/internal/result"
)

const (
	colorReset   = "\033[0m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorRed     = "\033[31m"
	colorCyan    = "\033[36m"
	colorBold    = "\033[1m"
	colorDim     = "\033[2m"
)

// TextRenderer renders a CompareResult as human-readable ANSI output.
type TextRenderer struct {
	w       io.Writer
	color   bool
	quiet   bool
}

// NewTextRenderer creates a new TextRenderer.
// Color is enabled when the output is a TTY and NO_COLOR is not set, unless noColor=true.
func NewTextRenderer(w io.Writer, noColor, quiet bool) *TextRenderer {
	useColor := !noColor && isColorTTY(w) && os.Getenv("NO_COLOR") == ""
	return &TextRenderer{w: w, color: useColor, quiet: quiet}
}

func isColorTTY(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}

func (r *TextRenderer) c(color, s string) string {
	if !r.color {
		return s
	}
	return color + s + colorReset
}

func (r *TextRenderer) bold(s string) string {
	if !r.color {
		return s
	}
	return colorBold + s + colorReset
}

func (r *TextRenderer) dim(s string) string {
	if !r.color {
		return s
	}
	return colorDim + s + colorReset
}

// Render writes the human-readable output to the writer.
func (r *TextRenderer) Render(res *result.CompareResult) error {
	verdictColor, verdictLabel := verdictStyle(res.Verdict)
	headline := r.c(verdictColor, verdictLabel) + "  " + verdictDescription(res)
	fmt.Fprintln(r.w, headline)

	if r.quiet {
		r.renderWarnings(res)
		return nil
	}

	fmt.Fprintln(r.w)
	r.renderMeta(res)

	if res.Verdict == result.VerdictConfirmedBase {
		fmt.Fprintln(r.w)
		r.renderLayers(res)
	}

	r.renderWarnings(res)
	return nil
}

func (r *TextRenderer) renderMeta(res *result.CompareResult) {
	fmt.Fprintf(r.w, "  %-14s %s\n", r.bold("Platform"), res.Platform)
	fmt.Fprintf(r.w, "  %-14s %-20s %s  (%d layers)\n",
		r.bold("Image A"), res.ImageA.Reference,
		r.dim(shortHash(res.ImageA.Digest)), res.ImageA.LayerCount)
	fmt.Fprintf(r.w, "  %-14s %-20s %s  (%d layers)\n",
		r.bold("Image B"), res.ImageB.Reference,
		r.dim(shortHash(res.ImageB.Digest)), res.ImageB.LayerCount)
}

func (r *TextRenderer) renderLayers(res *result.CompareResult) {
	fmt.Fprintf(r.w, "  %s\n", r.bold("Layer comparison"))
	sep := strings.Repeat("─", 68)
	fmt.Fprintf(r.w, "  %s\n", r.dim(sep))
	fmt.Fprintf(r.w, "  %-4s %-64s %s\n", r.bold("#"), r.bold("DiffID"), r.bold("Status"))

	for _, l := range res.MatchedLayers {
		fmt.Fprintf(r.w, "  %-4d %-64s %s\n",
			l.Index,
			shortHash(l.DiffID.String()),
			r.c(colorGreen, "matched"),
		)
	}
	for _, l := range res.ExtraLayers {
		fmt.Fprintf(r.w, "  %-4d %-64s %s\n",
			l.Index,
			shortHash(l.DiffID.String()),
			r.dim("extra (B only)"),
		)
	}

	fmt.Fprintf(r.w, "  %s\n", r.dim(sep))
	fmt.Fprintf(r.w, "  Matched %d/%d layers from base. Derived image adds %d layer(s).\n",
		len(res.MatchedLayers), res.ImageA.LayerCount, len(res.ExtraLayers))
}


func (r *TextRenderer) renderWarnings(res *result.CompareResult) {
	for _, w := range res.Warnings {
		fmt.Fprintf(r.w, "\n  %s %s\n", r.c(colorYellow, "!"), r.c(colorYellow, "WARNING: "+w))
	}
}

func verdictStyle(v result.Verdict) (string, string) {
	switch v {
	case result.VerdictConfirmedBase:
		return colorGreen, "CONFIRMED BASE"
	case result.VerdictNotBase:
		return colorRed, "NOT BASE"
	case result.VerdictSameImage:
		return colorCyan, "SAME IMAGE"
	default:
		return colorReset, "UNKNOWN"
	}
}

func verdictDescription(res *result.CompareResult) string {
	switch res.Verdict {
	case result.VerdictConfirmedBase:
		return fmt.Sprintf("%s is a base image of %s", res.ImageA.Reference, res.ImageB.Reference)
	case result.VerdictNotBase:
		return fmt.Sprintf("%s is NOT a base image of %s", res.ImageA.Reference, res.ImageB.Reference)
	case result.VerdictSameImage:
		return fmt.Sprintf("%s and %s refer to the same image", res.ImageA.Reference, res.ImageB.Reference)
	default:
		return ""
	}
}

func shortHash(s string) string {
	if len(s) > 19 {
		return s[:19] + "..."
	}
	return s
}
