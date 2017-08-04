package main

import (
	"fmt"
	"os"

	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/util/appcontext"
	"github.com/moby/buildkit/util/appdefaults"
	"github.com/moby/buildkit/util/profiler"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
)

func main() {
	app := cli.NewApp()
	app.Name = "buildctl"
	app.Usage = "build utility"

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug output in logs",
		},
		cli.StringFlag{
			Name:  "socket",
			Usage: "listening socket",
			Value: appdefaults.Socket,
		},
		cli.StringFlag{
			Name:  "controller",
			Usage: "controller constraint",
		},
	}

	app.Commands = []cli.Command{
		diskUsageCommand,
		buildCommand,
		debugCommand,
	}

	app.Before = func(context *cli.Context) error {
		if context.GlobalBool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}

	profiler.Attach(app)

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "buildd: %s\n", err)
		os.Exit(1)
	}
}

func resolveClient(c *cli.Context) (*client.Client, error) {
	return client.New(c.GlobalString("socket"), client.WithBlock())
}

func resolveContext(c *cli.Context) context.Context {
	md := controlapi.Metadata{
		Constraints: []controlapi.Constraint{
			controlapi.Constraint{
				Controller: c.GlobalString("controller"),
			},
		},
	}
	return client.ContextWithMetadata(appcontext.Context(), md)
}
