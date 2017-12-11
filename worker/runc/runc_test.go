// +build linux,standalone

package runc

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/containerd/containerd/namespaces"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/source"
	"github.com/moby/buildkit/worker"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func TestRuncWorker(t *testing.T) {
	t.Parallel()
	if os.Getuid() != 0 {
		t.Skip("requires root")
	}
	if _, err := exec.LookPath("runc"); err != nil {
		t.Skipf("no runc found: %s", err.Error())
	}

	ctx := namespaces.WithNamespace(context.Background(), "buildkit-test")

	sessionManager, err := session.NewManager()
	assert.NoError(t, err)

	commonOpt := &worker.CommonOpt{
		SessionManager: sessionManager,
	}

	// this should be an example or e2e test
	tmpdir, err := ioutil.TempDir("", "workertest")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	workerOpts, err := NewWorkerOpts(tmpdir)
	assert.NoError(t, err)

	w, err := worker.NewWorker(commonOpt, workerOpts[0])
	assert.NoError(t, err)

	img, err := source.NewImageIdentifier("docker.io/library/busybox:latest")
	assert.NoError(t, err)

	src, err := w.SourceManager.Resolve(ctx, img)
	assert.NoError(t, err)

	snap, err := src.Snapshot(ctx)
	assert.NoError(t, err)

	mounts, err := snap.Mount(ctx, false)
	assert.NoError(t, err)

	lm := snapshot.LocalMounter(mounts)

	target, err := lm.Mount()
	assert.NoError(t, err)

	f, err := os.Open(target)
	assert.NoError(t, err)

	names, err := f.Readdirnames(-1)
	assert.NoError(t, err)
	assert.True(t, len(names) > 5)

	err = f.Close()
	assert.NoError(t, err)

	lm.Unmount()
	assert.NoError(t, err)

	du, err := w.CacheManager.DiskUsage(ctx, client.DiskUsageInfo{})
	assert.NoError(t, err)

	// for _, d := range du {
	// 	fmt.Printf("du: %+v\n", d)
	// }

	for _, d := range du {
		assert.True(t, d.Size >= 8192)
	}

	meta := executor.Meta{
		Args: []string{"/bin/sh", "-c", "echo \"foo\" > /bar"},
		Cwd:  "/",
	}

	stderr := bytes.NewBuffer(nil)

	err = w.Executor.Exec(ctx, meta, snap, nil, nil, nil, &nopCloser{stderr})
	assert.Error(t, err) // Read-only root
	// typical error is like `mkdir /.../rootfs/proc: read-only file system`.
	// make sure the error is caused before running `echo foo > /bar`.
	assert.Contains(t, stderr.String(), "read-only file system")

	root, err := w.CacheManager.New(ctx, snap)
	assert.NoError(t, err)

	err = w.Executor.Exec(ctx, meta, root, nil, nil, nil, nil)
	assert.NoError(t, err)

	rf, err := root.Commit(ctx)
	assert.NoError(t, err)

	mounts, err = rf.Mount(ctx, false)
	assert.NoError(t, err)

	lm = snapshot.LocalMounter(mounts)

	target, err = lm.Mount()
	assert.NoError(t, err)

	dt, err := ioutil.ReadFile(filepath.Join(target, "bar"))
	assert.NoError(t, err)
	assert.Equal(t, string(dt), "foo\n")

	lm.Unmount()
	assert.NoError(t, err)

	err = rf.Release(ctx)
	assert.NoError(t, err)

	err = snap.Release(ctx)
	assert.NoError(t, err)

	du2, err := w.CacheManager.DiskUsage(ctx, client.DiskUsageInfo{})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(du2)-len(du))

}

type nopCloser struct {
	io.Writer
}

func (n *nopCloser) Close() error {
	return nil
}
