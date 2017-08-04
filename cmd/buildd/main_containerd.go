// +build containerd

package main

import (
	"github.com/moby/buildkit/control"
	"github.com/urfave/cli"
)

func init() {
	extraFlags = append(extraFlags, []cli.Flag{
		cli.StringFlag{
			Name:  "containerd",
			Usage: "containerd socket",
			Value: "/run/containerd/containerd.sock",
		},
	}...)
	name := "containerd"
	controllerFactories[name] =
		func(c *cli.Context, root string) (*control.Controller, error) {
			socket := c.GlobalString("containerd")
			return control.NewContainerd(name, root, socket)
		}
}
