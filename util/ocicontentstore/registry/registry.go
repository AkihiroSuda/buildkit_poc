package registry

import (
	"context"
	"io"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/docker/distribution/reference"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth"
	"github.com/moby/buildkit/util/tracing"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// TODO: deduplicate
func getCredentialsFunc(ctx context.Context, sm *session.Manager) func(string) (string, string, error) {
	id := session.FromContext(ctx)
	if id == "" {
		return nil
	}
	return func(host string) (string, string, error) {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		caller, err := sm.Get(timeoutCtx, id)
		if err != nil {
			return "", "", err
		}

		return auth.CredentialsFunc(context.TODO(), caller)(host)
	}
}

func NewStore(ctx context.Context, sm *session.Manager, ref string, insecure bool) (*Store, remotes.Resolver, error) {
	parsed, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return nil, nil, err
	}
	ref = reference.TagNameOnly(parsed).String()

	resolver := docker.NewResolver(docker.ResolverOptions{
		Client:      tracing.DefaultClient,
		Credentials: getCredentialsFunc(ctx, sm),
		PlainHTTP:   insecure,
	})
	store := &Store{
		ref:      ref,
		resolver: resolver,
	}
	return store, resolver, nil
}

type Store struct {
	ref      string
	resolver remotes.Resolver
}

func (s *Store) ReaderAt(ctx context.Context, dgst digest.Digest) (content.ReaderAt, error) {
	desc := ocispec.Descriptor{
		MediaType: "",
		Digest:    dgst,
		Size:      -1,
	}
	return s.ReaderAtWithOCIDescriptor(ctx, desc)
}

func (s *Store) ReaderAtWithOCIDescriptor(ctx context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	fetcher, err := s.resolver.Fetcher(ctx, s.ref)
	if err != nil {
		return nil, err
	}
	// fetcher requires desc.MediaType to determine the GET URL, especially for manifest blobs.
	r, err := fetcher.Fetch(ctx, desc)
	if err != nil {
		return nil, err
	}
	return &readerAt{
		r:    r,
		desc: desc,
	}, nil
}

type readerAt struct {
	r    io.ReadCloser
	desc ocispec.Descriptor
}

func (r *readerAt) ReadAt(p []byte, off int64) (n int, err error) {
	if ra, ok := r.r.(io.ReaderAt); ok {
		return ra.ReadAt(p, off)
	}
	if off != 0 {
		return 0, errors.Wrap(errdefs.ErrInvalidArgument, "fetcher does not support non-zero offset")
	}
	return r.r.Read(p)
}

func (r *readerAt) Close() error {
	return r.r.Close()
}

func (r *readerAt) Size() int64 {
	return r.desc.Size
}

func (s *Store) Writer(ctx context.Context, ref string, size int64, expected digest.Digest) (content.Writer, error) {
	desc := ocispec.Descriptor{
		MediaType: "",
		Digest:    expected,
		Size:      size,
	}
	return s.WriterWithOCIDescriptor(ctx, ref, desc)
}

func (s *Store) WriterWithOCIDescriptor(ctx context.Context, ref string, desc ocispec.Descriptor) (content.Writer, error) {
	pusher, err := s.resolver.Pusher(ctx, s.ref)
	if err != nil {
		return nil, err
	}
	// pusher requires desc.MediaType to determine the PUT URL, especially for manifest blobs.
	contentWriter, err := pusher.Push(ctx, desc)
	if err != nil {
		return nil, err
	}
	return &writer{
		Writer:           contentWriter,
		contentWriterRef: ref,
	}, nil
}

type writer struct {
	content.Writer          // returned from pusher.Push
	contentWriterRef string // ref passed for Writer()
}

func (w *writer) Status() (content.Status, error) {
	st, err := w.Writer.Status()
	if err != nil {
		return st, err
	}
	if w.contentWriterRef != "" {
		st.Ref = w.contentWriterRef
	}
	return st, nil
}
