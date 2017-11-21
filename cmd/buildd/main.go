package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/containerd/containerd/sys"
	"github.com/docker/go-connections/sockets"
	"github.com/moby/buildkit/control"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/frontend/dockerfile"
	"github.com/moby/buildkit/frontend/gateway"
	"github.com/moby/buildkit/metaworker"
	"github.com/moby/buildkit/util/appcontext"
	"github.com/moby/buildkit/util/appdefaults"
	"github.com/moby/buildkit/util/profiler"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var (
	appFlags []cli.Flag
	// key: priority (+: less preferred, -: more preferred)
	metaWorkerCtors = make(map[int]func(c *cli.Context, root string) ([]*metaworker.MetaWorker, error), 0)
)

func main() {
	app := cli.NewApp()
	app.Name = "buildd"
	app.Usage = "build daemon"

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug output in logs",
		},
		cli.StringFlag{
			Name:  "root",
			Usage: "path to state directory",
			Value: appdefaults.Root,
		},
		cli.StringFlag{
			Name:  "addr",
			Usage: "listening address (socket or tcp)",
			Value: appdefaults.Address,
		},
		cli.StringFlag{
			Name:  "debugaddr",
			Usage: "debugging address (eg. 0.0.0.0:6060)",
			Value: "",
		},
	}

	app.Flags = append(app.Flags, appFlags...)

	app.Action = func(c *cli.Context) error {
		ctx, cancel := context.WithCancel(appcontext.Context())

		if debugAddr := c.GlobalString("debugaddr"); debugAddr != "" {
			if err := setupDebugHandlers(debugAddr); err != nil {
				return err
			}
		}

		server := grpc.NewServer(unaryInterceptor(ctx))

		// relative path does not work with nightlyone/lockfile
		root, err := filepath.Abs(c.GlobalString("root"))
		if err != nil {
			return err
		}

		if err := os.MkdirAll(root, 0700); err != nil {
			return errors.Wrapf(err, "failed to create %s", root)
		}

		controller, err := newController(c, root)
		if err != nil {
			return err
		}

		controller.Register(server)

		errCh := make(chan error, 1)
		if err := serveGRPC(server, c.GlobalString("addr"), errCh); err != nil {
			return err
		}

		select {
		case serverErr := <-errCh:
			err = serverErr
			cancel()
		case <-ctx.Done():
			err = ctx.Err()
		}

		logrus.Infof("stopping server")
		server.GracefulStop()

		return err
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

func serveGRPC(server *grpc.Server, addr string, errCh chan error) error {
	if addr == "" {
		return errors.New("--addr cannot be empty")
	}
	l, err := getListener(addr)
	if err != nil {
		return err
	}
	go func() {
		defer l.Close()
		logrus.Infof("running server on %s", addr)
		errCh <- server.Serve(l)
	}()
	return nil
}

func getListener(addr string) (net.Listener, error) {
	addrSlice := strings.SplitN(addr, "://", 2)
	proto := addrSlice[0]
	listenAddr := addrSlice[1]
	switch proto {
	case "unix", "npipe":
		return sys.GetLocalListener(listenAddr, os.Getuid(), os.Getgid())
	case "tcp":
		return sockets.NewTCPSocket(listenAddr, nil)
	default:
		return nil, errors.Errorf("addr %s not supported", addr)
	}
}

func unaryInterceptor(globalCtx context.Context) grpc.ServerOption {
	return grpc.UnaryInterceptor(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		go func() {
			select {
			case <-ctx.Done():
			case <-globalCtx.Done():
				cancel()
			}
		}()

		resp, err = handler(ctx, req)
		if err != nil {
			logrus.Errorf("%s returned error: %+v", info.FullMethod, err)
		}
		return
	})
}

func newController(c *cli.Context, root string) (*control.Controller, error) {
	mws, err := metaWorkers(c, root)
	if err != nil {
		return nil, err
	}
	frontends := map[string]frontend.Frontend{}
	frontends["dockerfile.v0"] = dockerfile.NewDockerfileFrontend()
	frontends["gateway.v0"] = gateway.NewGatewayFrontend()
	return control.NewController(control.Opt{
		MetaWorkers: mws,
		Frontends:   frontends,
	})
}

func metaWorkers(c *cli.Context, root string) ([]*metaworker.MetaWorker, error) {
	type ctorEntry struct {
		priority int
		ctor     func(c *cli.Context, root string) ([]*metaworker.MetaWorker, error)
	}
	var ctors []ctorEntry
	for p, ctor := range metaWorkerCtors {
		ctors = append(ctors, ctorEntry{priority: p, ctor: ctor})
	}
	sort.Slice(ctors, func(i, j int) bool { return ctors[i].priority < ctors[j].priority })
	var ret []*metaworker.MetaWorker
	for _, e := range ctors {
		mws, err := e.ctor(c, root)
		if err != nil {
			return ret, err
		}
		for _, mw := range mws {
			logrus.Infof("Found metaworker %q", mw.Name)
			ret = append(ret, mw)
		}
	}
	if len(ret) == 0 {
		return nil, errors.New("no metaworker found, build the buildkit daemon with tags? (e.g. \"standalone\", \"containerd\")")
	}
	logrus.Infof("Default metaworker %q", ret[0].Name)
	logrus.Warn("Currently, only the default metaworker can be used.")
	return ret, nil
}
