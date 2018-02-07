package registry

import (
	"context"

	"net/http"
	"time"

	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"

	"github.com/moby/buildkit/cache/instructioncache"
	"github.com/moby/buildkit/cache/transferable/contentstore"

	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type ImportOpt struct {
	SessionManager            *session.Manager
	ContentStoreCacheImporter *contentstore.CacheImporter
}

func NewCacheImporter(opt ImportOpt) *CacheImporter {
	return &CacheImporter{opt: opt}
}

type CacheImporter struct {
	opt ImportOpt
}

func (ci *CacheImporter) getCredentialsFromSession(ctx context.Context) func(string) (string, string, error) {
	id := session.FromContext(ctx)
	if id == "" {
		return nil
	}

	return func(host string) (string, string, error) {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		caller, err := ci.opt.SessionManager.Get(timeoutCtx, id)
		if err != nil {
			return "", "", err
		}

		return auth.CredentialsFunc(context.TODO(), caller)(host)
	}
}

func (ci *CacheImporter) pull(ctx context.Context, ref string) (*ocispec.Descriptor, remotes.Fetcher, error) {
	resolver := docker.NewResolver(docker.ResolverOptions{
		Client:      http.DefaultClient,
		Credentials: ci.getCredentialsFromSession(ctx),
	})

	ref, desc, err := resolver.Resolve(ctx, ref)
	if err != nil {
		return nil, nil, err
	}

	fetcher, err := resolver.Fetcher(ctx, ref)
	if err != nil {
		return nil, nil, err
	}

	if _, err := remotes.FetchHandler(ci.opt.ContentStoreCacheImporter.ContentStore(), fetcher)(ctx, desc); err != nil {
		return nil, nil, err
	}

	return &desc, fetcher, err
}

func (ci *CacheImporter) Import(ctx context.Context, ref string) (instructioncache.InstructionCache, error) {
	desc, fetcher, err := ci.pull(ctx, ref)
	if err != nil {
		return nil, err
	}
	cs := ci.opt.ContentStoreCacheImporter.ContentStore()
	ensurer := func(xctx context.Context, xdesc ocispec.Descriptor) error {
		_, err := remotes.FetchHandler(cs, fetcher)(xctx, xdesc)
		return err
	}
	return ci.opt.ContentStoreCacheImporter.ImportWithContentEnsurer(ctx, desc.Digest, ensurer)
}
