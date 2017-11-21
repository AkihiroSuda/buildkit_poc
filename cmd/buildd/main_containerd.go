// +build containerd

package main

import (
	"os"
	"strings"

	"github.com/moby/buildkit/metaworker"
	"github.com/moby/buildkit/metaworker/containerd"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func init() {
	appFlags = append(appFlags,
		cli.BoolFlag{
			Name:  "disable-containerd",
			Usage: "disable containerd workers",
		},
		cli.StringFlag{
			Name:  "containerd",
			Usage: "containerd socket",
			Value: "/run/containerd/containerd.sock",
		})
	// 1 is less preferred than 0 (runcCtor)
	metaWorkerCtors[1] = containerdCtor
}

func containerdCtor(c *cli.Context, root string) ([]*metaworker.MetaWorker, error) {
	socket := c.GlobalString("containerd")
	if c.GlobalBool("disable-containerd") || skipContainerd(socket) {
		return nil, nil
	}
	opts, err := containerd.NewMetaWorkerOpts(root, socket)
	if err != nil {
		return nil, err
	}
	var mws []*metaworker.MetaWorker
	for _, opt := range opts {
		mw, err := metaworker.NewMetaWorker(opt)
		if err != nil {
			return mws, err
		}
		mws = append(mws, mw)
	}
	return mws, nil
}

func skipContainerd(socket string) bool {
	if strings.HasPrefix(socket, "tcp://") {
		// FIXME(AkihiroSuda): prohibit tcp?
		return false
	}
	socketPath := strings.TrimPrefix(socket, "unix://")
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		// FIXME(AkihiroSuda): add more conditions
		logrus.Warnf("skipping containerd, as %q does not exist", socketPath)
		return true
	}
	return false
}
