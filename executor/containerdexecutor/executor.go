package containerdexecutor

import (
	"io"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/executor/oci"
	"github.com/moby/buildkit/identity"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

type containerdExecutor struct {
	client *containerd.Client
}

func New(client *containerd.Client) executor.Executor {
	return containerdExecutor{
		client: client,
	}
}

func (w containerdExecutor) Exec(ctx context.Context, meta executor.Meta, root cache.Mountable, mounts []executor.Mount, stdin io.ReadCloser, stdout, stderr io.WriteCloser) error {
	id := identity.NewID()

	spec, cleanup, err := oci.GenerateSpec(ctx, meta, mounts, id)
	if err != nil {
		return err
	}
	defer cleanup()

	rootMounts, err := root.Mount(ctx, false)
	if err != nil {
		return err
	}

	container, err := w.client.NewContainer(ctx, id,
		containerd.WithSpec(spec),
	)
	if err != nil {
		return err
	}
	defer container.Delete(ctx)

	if stdin == nil {
		stdin = &emptyReadCloser{}
	}

	task, err := container.NewTask(ctx, cio.NewIO(stdin, stdout, stderr), containerd.WithRootFS(rootMounts))
	if err != nil {
		return err
	}
	defer task.Delete(ctx)

	// TODO: Configure bridge networking

	// TODO: support sending signals

	if err := task.Start(ctx); err != nil {
		return err
	}

	statusCh, err := task.Wait(ctx)
	if err != nil {
		return err
	}
	status := <-statusCh
	if status.ExitCode() != 0 {
		return errors.Errorf("process returned non-zero exit code: %d", status.ExitCode())
	}

	return nil
}

type emptyReadCloser struct{}

func (*emptyReadCloser) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (*emptyReadCloser) Close() error {
	return nil
}
