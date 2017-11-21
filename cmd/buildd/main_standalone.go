// +build standalone

// TODO(AkihiroSuda): s/standalone/runc/g

package main

import (
	"github.com/moby/buildkit/metaworker"
	"github.com/moby/buildkit/metaworker/runc"
	"github.com/urfave/cli"
)

func init() {
	appFlags = append(appFlags,
		cli.BoolFlag{
			Name:  "disable-runc",
			Usage: "disable runc workers",
		})
	metaWorkerCtors[0] = runcCtor
}

func runcCtor(c *cli.Context, root string) ([]*metaworker.MetaWorker, error) {
	if c.GlobalBool("disable-runc") {
		return nil, nil
	}
	opts, err := runc.NewMetaWorkerOpts(root)
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
