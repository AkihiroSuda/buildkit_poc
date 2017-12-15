package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/containerd/console"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/util/appcontext"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
)

// TODO: call `docker load` rather than creating a file
var exporterOptDefault = cli.StringSlice([]string{"output=./oci.tar"})

var dockerIncompatibleFlags = []cli.Flag{
	cli.StringFlag{
		Name:  "buildkit-exporter",
		Usage: "Define exporter for build result",
		Value: "oci", // TODO: docker v1 exporter (unless moby supports OCI importer)
	},
	cli.StringSliceFlag{
		Name:  "buildkit-exporter-opt",
		Usage: "Define custom options for exporter",
		Value: &exporterOptDefault,
	},
}

var budCommand = cli.Command{
	Name:      "build-using-dockerfile",
	ShortName: "bud",
	UsageText: `buildctl build-using-dockerfile [OPTIONS] PATH | URL | -`,
	Description: `
build using Dockerfile.

This command mimics behavior of "docker build" command so that people can easily get started with BuildKit.
This command is NOT the replacement of "docker build", and should NOT be used for building production images.

By default, the built image is exported as a tar archive with OCI Image Layout ("./oci.tar").

You can import oci.tar to Docker as follows:

  $ mkdir tmp
  $ tar Cxvf tmp oci.tar
  $ skopeo copy oci:tmp docker-daemon:foo/bar:latest 
`,
	Action: bud,
	Flags: append([]cli.Flag{
		cli.StringFlag{
			Name:  "file, f",
			Usage: "Name of the Dockerfile (Default is 'PATH/Dockerfile')",
		},
		cli.StringFlag{
			Name:  "target",
			Usage: "Set the target build stage to build.",
		},
		cli.StringSliceFlag{
			Name:  "build-arg",
			Usage: "Set build-time variables",
		},
	}, dockerIncompatibleFlags...),
}

func newBudSolveOpt(clicontext *cli.Context) (*client.SolveOpt, error) {
	buildCtx := clicontext.Args().First()
	if buildCtx == "" {
		return nil, errors.New("please specify build context (e.g. \".\" for the current directory)")
	} else if buildCtx == "-" {
		return nil, errors.New("stdin not supported yet")
	}

	file := clicontext.String("file")
	if file == "" {
		file = filepath.Join(buildCtx, "Dockerfile")
	}
	exporterAttrs, err := attrMap(clicontext.StringSlice("buildkit-exporter-opt"))
	if err != nil {
		return nil, errors.Wrap(err, "invalid buildkit-exporter-opt")
	}

	localDirs := map[string]string{
		"context":    buildCtx,
		"dockerfile": filepath.Dir(file),
	}

	frontendAttrs := map[string]string{
		"filename": filepath.Base(file),
	}
	if target := clicontext.String("target"); target != "" {
		frontendAttrs["target"] = target
	}
	buildArgs, err := attrMap(clicontext.StringSlice("build-arg"))
	if err != nil {
		return nil, err
	}
	for k, v := range buildArgs {
		frontendAttrs["build-arg:"+k] = v
	}
	return &client.SolveOpt{
		Exporter:      clicontext.String("buildkit-exporter"),
		ExporterAttrs: exporterAttrs,
		LocalDirs:     localDirs,
		Frontend:      "dockerfile.v0", // TODO: use gateway
		FrontendAttrs: frontendAttrs,
	}, nil
}

func bud(clicontext *cli.Context) error {
	solveOpt, err := newBudSolveOpt(clicontext)
	if err != nil {
		return err
	}
	c, err := resolveClient(clicontext)
	if err != nil {
		return err
	}
	ch := make(chan *client.SolveStatus)
	eg, ctx := errgroup.WithContext(appcontext.Context())
	eg.Go(func() error {
		return c.Solve(ctx, nil, *solveOpt, ch)
	})
	eg.Go(func() error {
		if c, err := console.ConsoleFromFile(os.Stderr); err == nil {
			// not using shared context to not disrupt display but let is finish reporting errors
			return progressui.DisplaySolveStatus(context.TODO(), c, ch)
		}
		return nil
	})
	return eg.Wait()
}
