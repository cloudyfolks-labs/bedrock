package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cloudyfolks-labs/bedrock/internal/operator"
	"github.com/cloudyfolks-labs/bedrock/internal/release"
)

const releaseUsage = "usage: bedrock release build --config FILE --version VERSION --image IMAGE --out DIR [--helm helm]\n       bedrock release apply --dir DIR [--timeout 10m] [--interval 2s]"

func releaseCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, releaseUsage)
		return 2
	}
	switch args[0] {
	case "build":
		return releaseBuild(args[1:], stdout, stderr)
	case "apply":
		return releaseApply(args[1:], stdout, stderr)
	}
	fmt.Fprintln(stderr, releaseUsage)
	return 2
}

func releaseBuild(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("release build", flag.ContinueOnError)
	flags.SetOutput(stderr)
	config := flags.String("config", "release/components.yaml", "build configuration")
	version := flags.String("version", "", "release version")
	image := flags.String("image", "", "bedrock image reference")
	out := flags.String("out", "dist/release", "output directory")
	helm := flags.String("helm", "helm", "helm binary")
	root := flags.String("root", ".", "repository root the config paths are relative to")
	if err := flags.Parse(args); err != nil {
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

func releaseApply(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("release apply", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dir := flags.String("dir", "", "release directory")
	timeout := flags.Duration("timeout", 10*time.Minute, "overall timeout")
	interval := flags.Duration("interval", 2*time.Second, "readiness poll interval")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *dir == "" {
		fmt.Fprintln(stderr, "release apply: --dir is required")
		return 2
	}
	bundle, err := release.Load(os.DirFS(*dir))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	cfg, err := ctrl.GetConfig()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	scheme, err := operator.Scheme()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	report := func(group release.Group, err error) {
		if err != nil {
			fmt.Fprintf(stdout, "group %s failed\n", group.Name)
			return
		}
		fmt.Fprintf(stdout, "group %s ready\n", group.Name)
	}
	if err := release.Install(ctx, c, bundle, release.Gates{}, *interval, 0, report); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "release %s applied\n", bundle.Spec.Version)
	return 0
}
