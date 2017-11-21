// +build containerd

package main

import (
	"github.com/moby/buildkit/metaworker"
	"github.com/moby/buildkit/metaworker/containerd"
	"github.com/urfave/cli"
)

func init() {
	appFlags = append(appFlags, cli.StringFlag{
		Name:  "containerd",
		Usage: "containerd socket",
		Value: "/run/containerd/containerd.sock",
	})
	// 1 is less preferred than 0 (runcCtor)
	metaWorkerCtors[1] = containerdCtor
}

func containerdCtor(c *cli.Context, root string) ([]*metaworker.MetaWorker, error) {
	socket := c.GlobalString("containerd")
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
