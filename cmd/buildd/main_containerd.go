// +build containerd

package main

import (
	"os"
	"strings"

	"github.com/moby/buildkit/worker"
	"github.com/moby/buildkit/worker/containerd"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func init() {
	appFlags = append(appFlags,
		cli.StringFlag{
			Name:  "containerd-worker",
			Usage: "enable containerd workers (true/false/auto)",
			Value: "auto",
		},
		cli.BoolFlag{
			Name:  "containerd-worker-multiple-snapshotters",
			Usage: "use multiple snapshotters. useful for some applications that does not work with the default snapshotter (overlay)",
		},
		cli.StringFlag{
			Name:  "containerd-worker-addr",
			Usage: "containerd socket",
			Value: "/run/containerd/containerd.sock",
		})
	// 1 is less preferred than 0 (runcCtor)
	workerCtors[1] = containerdCtor
}

func containerdCtor(c *cli.Context, common *worker.CommonOpt, root string) ([]*worker.Worker, error) {
	socket := c.GlobalString("containerd-worker-addr")
	boolOrAuto, err := parseBoolOrAuto(c.GlobalString("containerd-worker"))
	if err != nil {
		return nil, err
	}
	if (boolOrAuto == nil && skipContainerd(socket)) || (boolOrAuto != nil && !*boolOrAuto) {
		return nil, nil
	}
	opts, err := containerd.NewWorkerOpts(root, socket)
	if err != nil {
		return nil, err
	}
	if !c.GlobalBool("containerd-worker-multiple-snapshotters") {
		opts = opts[0:1]
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

func skipContainerd(socket string) bool {
	if strings.HasPrefix(socket, "tcp://") {
		// FIXME(AkihiroSuda): prohibit tcp?
		return false
	}
	socketPath := strings.TrimPrefix(socket, "unix://")
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		// FIXME(AkihiroSuda): add more conditions
		logrus.Warnf("skipping containerd worker, as %q does not exist", socketPath)
		return true
	}
	return false
}
