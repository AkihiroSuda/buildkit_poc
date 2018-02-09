package registry

import (
	"context"
	"sync"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/docker/distribution/reference"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth"
	"github.com/moby/buildkit/util/tracing"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
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

func NewStore(ctx context.Context, sm *session.Manager, readCache content.Store, ref string, insecure bool) (*Store, remotes.Resolver, error) {
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
		cs:       readCache,
		ref:      ref,
		resolver: resolver,
		m:        make(map[digest.Digest]string, 0),
	}
	return store, resolver, nil
}

type Store struct {
	cs       content.Store
	ref      string
	resolver remotes.Resolver
	mmu      sync.Mutex
	m        map[digest.Digest]string
}

func (s *Store) SetMediaType(d digest.Digest, mt string) {
	s.mmu.Lock()
	s.m[d] = mt
	s.mmu.Unlock()
}

func (s *Store) ReleaseMediaType(d digest.Digest) {
	s.mmu.Lock()
	delete(s.m, d)
	s.mmu.Unlock()
}

func (s *Store) ensure(ctx context.Context, dgst digest.Digest) error {
	fetcher, err := s.resolver.Fetcher(ctx, s.ref)
	if err != nil {
		return err
	}
	s.mmu.Lock()
	mediaType := s.m[dgst]
	s.mmu.Unlock()
	desc := ocispec.Descriptor{
		Digest:    dgst,
		MediaType: mediaType,
		Size:      -1, // FIXME
	}
	_, err = remotes.FetchHandler(s.cs, fetcher)(ctx, desc)
	return err
}

func (s *Store) ReaderAt(ctx context.Context, dgst digest.Digest) (content.ReaderAt, error) {
	cached, err := s.cs.ReaderAt(ctx, dgst)
	if cached != nil && err == nil {
		return cached, err
	}
	if err := s.ensure(ctx, dgst); err != nil {
		return nil, err
	}
	return s.cs.ReaderAt(ctx, dgst)
}

func (s *Store) Writer(ctx context.Context, ref string, size int64, expected digest.Digest) (content.Writer, error) {
	// TODO(AkihiroSuda): tee to s.cs as well
	pusher, err := s.resolver.Pusher(ctx, s.ref)
	if err != nil {
		return nil, err
	}
	s.mmu.Lock()
	mediaType := s.m[expected]
	s.mmu.Unlock()
	desc := ocispec.Descriptor{
		Digest:    expected,
		MediaType: mediaType,
		Size:      size,
	}
	cWriter, err := pusher.Push(ctx, desc)
	if err != nil {
		return nil, err
	}
	return &writer{
		Writer: cWriter,
		cRef:   ref,
	}, nil
}

type writer struct {
	content.Writer
	cRef string
}

func (w *writer) Status() (content.Status, error) {
	st, err := w.Writer.Status()
	if err != nil {
		return st, err
	}
	if w.cRef != "" {
		st.Ref = w.cRef
	}
	return st, nil
}
