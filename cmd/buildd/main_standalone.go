// +build standalone

package main

import (
	"github.com/moby/buildkit/metaworker"
	"github.com/moby/buildkit/metaworker/runc"
	"github.com/urfave/cli"
)

func init() {
	metaWorkerCtors[0] = runcCtor
}

func runcCtor(c *cli.Context, root string) ([]*metaworker.MetaWorker, error) {
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
