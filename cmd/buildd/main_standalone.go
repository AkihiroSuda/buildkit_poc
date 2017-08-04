// +build standalone

package main

import (
	"github.com/moby/buildkit/control"
	"github.com/urfave/cli"
)

func init() {
	name := "standalone"
	controllerFactories[name] =
		func(c *cli.Context, root string) (*control.Controller, error) {
			return control.NewStandalone(name, root)
		}
}
