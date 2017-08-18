package main

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/appcontext"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
)

var buildCommand = cli.Command{
	Name:   "build",
	Usage:  "build",
	Action: build,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "exporter",
			Usage: "Define exporter for build result",
		},
		cli.StringSliceFlag{
			Name:  "exporter-opt",
			Usage: "Define custom options for exporter",
		},
		cli.BoolFlag{
			Name:  "no-progress",
			Usage: "Don't show interactive progress",
		},
		cli.StringSliceFlag{
			Name:  "local",
			Usage: "Allow build access to the local directory",
		},
		cli.BoolFlag{
			Name:  "no-cache",
			Usage: "Disable cache lookup",
		},
	},
}

func read(r io.Reader, clicontext *cli.Context) ([][]byte, pb.Metadata, error) {
	def, err := llb.ReadFrom(r)
	if err != nil {
		return nil, pb.Metadata{}, errors.Wrap(err, "failed to parse input")
	}
	var meta pb.Metadata
	meta.Entries = make(map[digest.Digest]pb.MetadataEntry, 0)
	for _, dt := range def {
		var op pb.Op
		if err := (&op).Unmarshal(dt); err != nil {
			return nil, pb.Metadata{}, errors.Wrap(err, "failed to parse llb proto op")
		}
		dig := digest.FromBytes(dt)
		metaEntry := pb.MetadataEntry{}
		metaEntry.IgnoreCache = clicontext.Bool("no-cache")
		meta.Entries[dig] = metaEntry
	}

	return def, meta, nil
}

func build(clicontext *cli.Context) error {
	c, err := resolveClient(clicontext)
	if err != nil {
		return err
	}

	traceFile, err := ioutil.TempFile("", "buildctl")
	if err != nil {
		return err
	}
	defer traceFile.Close()
	traceEnc := json.NewEncoder(traceFile)

	logrus.Infof("tracing logs to %s", traceFile.Name())

	ch := make(chan *client.SolveStatus)
	displayCh := make(chan *client.SolveStatus)
	eg, ctx := errgroup.WithContext(appcontext.Context())

	exporterAttrs, err := attrMap(clicontext.StringSlice("exporter-opt"))
	if err != nil {
		return errors.Wrap(err, "invalid exporter-opt")
	}

	localDirs, err := attrMap(clicontext.StringSlice("local"))
	if err != nil {
		return errors.Wrap(err, "invalid local")
	}

	def, metadata, err := read(os.Stdin, clicontext)
	if err != nil {
		return err
	}
	eg.Go(func() error {
		return c.Solve(ctx, def, metadata, client.SolveOpt{
			Exporter:      clicontext.String("exporter"),
			ExporterAttrs: exporterAttrs,
			LocalDirs:     localDirs,
		}, ch)
	})

	eg.Go(func() error {
		defer close(displayCh)
		for s := range ch {
			if err := traceEnc.Encode(s); err != nil {
				logrus.Error(err)
			}
			displayCh <- s
		}
		return nil
	})

	eg.Go(func() error {
		if clicontext.Bool("no-progress") {
			for s := range displayCh {
				for _, v := range s.Vertexes {
					logrus.Debugf("vertex: %s %s %v %v", v.Digest, v.Name, v.Started, v.Completed)
				}
				for _, s := range s.Statuses {
					logrus.Debugf("status: %s %s %d", s.Vertex, s.ID, s.Current)
				}
				for _, l := range s.Logs {
					logrus.Debugf("log: %s\n%s", l.Vertex, l.Data)
				}
			}
			return nil
		}
		// not using shared context to not disrupt display but let is finish reporting errors
		return progressui.DisplaySolveStatus(context.TODO(), displayCh)
	})

	return eg.Wait()
}

func attrMap(sl []string) (map[string]string, error) {
	m := map[string]string{}
	for _, v := range sl {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) != 2 {
			return nil, errors.Errorf("invalid value %s", v)
		}
		m[parts[0]] = parts[1]
	}
	return m, nil
}
