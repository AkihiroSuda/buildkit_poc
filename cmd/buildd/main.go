package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/sys"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/control"
	"github.com/moby/buildkit/control/dispatcher"
	"github.com/moby/buildkit/util/appcontext"
	"github.com/moby/buildkit/util/appdefaults"
	"github.com/moby/buildkit/util/profiler"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// DefaultController can be filled with `-ldflags "-X main.DefaultController"`
var DefaultController = "containerd"

type controllerFactory func(c *cli.Context, root string) (*control.Controller, error)

// initialized during init().
var (
	extraFlags []cli.Flag
	// root must be an absolute path
	controllerFactories map[string]controllerFactory = make(map[string]controllerFactory, 0)
)

func main() {
	app := cli.NewApp()
	app.Name = "buildd"
	app.Usage = "build daemon"

	app.Flags = append([]cli.Flag{
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
			Name:  "socket",
			Usage: "listening socket",
			Value: appdefaults.Socket,
		},
		cli.StringFlag{
			Name:  "debugaddr",
			Usage: "Debugging address (eg. 0.0.0.0:6060)",
			Value: "",
		},
		cli.StringFlag{
			Name:  "default-controller",
			Usage: "default controller instance name",
			Value: DefaultController,
		},
	}, extraFlags...)

	app.Action = func(c *cli.Context) error {
		ctx, cancel := context.WithCancel(appcontext.Context())

		if debugAddr := c.GlobalString("debugaddr"); debugAddr != "" {
			if err := setupDebugHandlers(debugAddr); err != nil {
				return err
			}
		}

		server := grpc.NewServer(unaryInterceptor(ctx))
		controller, err := newController(c)
		if err != nil {
			return err
		}
		controlapi.RegisterControlServer(server, controller)

		errCh := make(chan error, 1)
		if err := serveGRPC(server, c.GlobalString("socket"), errCh); err != nil {
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

// TODO: port over containerd/containerd/plugin model
func newController(c *cli.Context) (controlapi.ControlServer, error) {
	// relative path does not work with nightlyone/lockfile
	root, err := filepath.Abs(c.GlobalString("root"))
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, errors.Wrapf(err, "failed to create %s", root)
	}

	defaultControllerName := c.GlobalString("default-controller")
	defaultControllerIdx := -1
	var controllers []dispatcher.DispatchableController
	for name, f := range controllerFactories {
		ctlr, err := f(c, filepath.Join(root, name))
		if err != nil {
			logrus.Warnf("Error while loading controller %q: %v", name, err)
			continue
		}
		logrus.Infof("Loaded controller %q", name)
		if name == defaultControllerName {
			logrus.Infof("Controller %q is the default controller instance", name)
			defaultControllerIdx = len(controllers)
		}
		controllers = append(controllers, ctlr)
	}
	if len(controllers) == 0 {
		return nil, errors.New("no controller loaded")
	}
	if defaultControllerIdx < 0 {
		return nil, errors.Errorf("default controller %q is not loaded, probably you want to specify --default-controller to one of the keys of %v?", defaultControllerName, controllerFactories)
	}
	controllers[0], controllers[defaultControllerIdx] = controllers[defaultControllerIdx], controllers[0]
	return dispatcher.NewDispatcher(controllers)
}

func serveGRPC(server *grpc.Server, path string, errCh chan error) error {
	if path == "" {
		return errors.New("--socket path cannot be empty")
	}
	l, err := sys.GetLocalListener(path, os.Getuid(), os.Getgid())
	if err != nil {
		return err
	}
	go func() {
		defer l.Close()
		logrus.Infof("running server on %s", path)
		errCh <- server.Serve(l)
	}()
	return nil
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
