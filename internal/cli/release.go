package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/cloudyfolks-labs/bedrock/internal/release"
)

func releaseCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "build" {
		fmt.Fprintln(stderr, "usage: bedrock release build --config FILE --version VERSION --image IMAGE --out DIR [--helm helm]")
		return 2
	}
	flags := flag.NewFlagSet("release build", flag.ContinueOnError)
	flags.SetOutput(stderr)
	config := flags.String("config", "release/components.yaml", "build configuration")
	version := flags.String("version", "", "release version")
	image := flags.String("image", "", "bedrock image reference")
	out := flags.String("out", "dist/release", "output directory")
	helm := flags.String("helm", "helm", "helm binary")
	root := flags.String("root", ".", "repository root the config paths are relative to")
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	if *version == "" || *image == "" {
		fmt.Fprintln(stderr, "release build: --version and --image are required")
		return 2
	}
	cfg, err := release.LoadBuildConfig(*config)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := release.Build(cfg, release.BuildOptions{Version: *version, Image: *image, Out: *out, Helm: *helm, Root: *root}); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "release %s written to %s\n", *version, *out)
	return 0
}
