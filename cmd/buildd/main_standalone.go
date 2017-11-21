// +build standalone

// TODO(AkihiroSuda): s/standalone/oci/g

package main

import (
	"os/exec"

	"github.com/moby/buildkit/worker"
	"github.com/moby/buildkit/worker/runc"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func init() {
	appFlags = append(appFlags,
		cli.StringFlag{
			Name:  "oci-worker",
			Usage: "enable oci workers (true/false/auto)",
			Value: "auto",
		},
	)
	// TODO: allow multiple oci runtimes and snapshotters
	workerCtors[0] = runcCtor
}

func runcCtor(c *cli.Context, common *worker.CommonOpt, root string) ([]*worker.Worker, error) {
	boolOrAuto, err := parseBoolOrAuto(c.GlobalString("oci-worker"))
	if err != nil {
		return nil, err
	}
	if (boolOrAuto == nil && skipRunc()) || (boolOrAuto != nil && !*boolOrAuto) {
		return nil, nil
	}
	opts, err := runc.NewWorkerOpts(root)
	if err != nil {
		return nil, err
	}
	var ws []*worker.Worker
	for _, opt := range opts {
		w, err := worker.NewWorker(common, opt)
		if err != nil {
			return ws, err
		}
		ws = append(ws, w)
	}
	return ws, nil
}

func skipRunc() bool {
	_, err := exec.LookPath("runc")
	if err != nil {
		logrus.Warnf("skipping oci worker, as runc does not exist")
		return true
	}
	return false
}
