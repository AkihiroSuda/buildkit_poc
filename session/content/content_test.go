package content

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/testutil"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/content/local"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestContentAttachable(t *testing.T) {
	ctx := context.TODO()
	t.Parallel()
	tmpDir, err := ioutil.TempDir("", "contenttest")
	require.NoError(t, err)
	calleeStore, err := local.NewStore(tmpDir)
	require.NoError(t, err)
	blob := []byte("test-content-attachable")
	w, err := calleeStore.Writer(ctx, content.WithRef(string(blob)))
	require.NoError(t, err)
	n, err := w.Write(blob)
	require.NoError(t, err)
	err = w.Commit(ctx, int64(n), "")
	require.NoError(t, err)
	err = w.Close()
	require.NoError(t, err)
	blobDigest := w.Digest()

	s, err := session.NewSession(ctx, "foo", "bar")
	require.NoError(t, err)

	m, err := session.NewManager()
	require.NoError(t, err)

	a := NewAttachable(calleeStore)
	s.Allow(a)

	dialer := session.Dialer(testutil.TestStream(testutil.Handler(m.HandleConn)))

	g, ctx := errgroup.WithContext(context.Background())

	g.Go(func() error {
		return s.Run(ctx, dialer)
	})

	g.Go(func() (reterr error) {
		c, err := m.Get(ctx, s.ID())
		if err != nil {
			return err
		}
		callerStore := CallerStore(c)
		blob2, err := content.ReadBlob(ctx, callerStore, ocispec.Descriptor{Digest: blobDigest})
		if err != nil {
			return err
		}
		assert.Equal(t, blob, blob2)
		return s.Close()
	})

	err = g.Wait()
	require.NoError(t, err)
}
