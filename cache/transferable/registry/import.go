package registry

import (
	"context"

	"net/http"
	"time"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"

	"github.com/moby/buildkit/cache/instructioncache"
	"github.com/moby/buildkit/cache/transferable/contentstore"

	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type ImporterOpt struct {
	SessionManager            *session.Manager
	ContentStore              content.Store
	ContentStoreCacheImporter contentstore.EnsurableImporter
}

func NewImporter(opt ImporterOpt) *Importer {
	return &Importer{opt: opt}
}

type Importer struct {
	opt ImporterOpt
}

func (ci *Importer) getCredentialsFromSession(ctx context.Context) func(string) (string, string, error) {
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

func (ci *Importer) pull(ctx context.Context, ref string) (*ocispec.Descriptor, remotes.Fetcher, error) {
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

	if _, err := remotes.FetchHandler(ci.opt.ContentStore, fetcher)(ctx, desc); err != nil {
		return nil, nil, err
	}

	return &desc, fetcher, err
}

func (ci *Importer) Import(ctx context.Context, ref string) (instructioncache.InstructionCache, error) {
	desc, fetcher, err := ci.pull(ctx, ref)
	if err != nil {
		return nil, err
	}
	ensurer := func(xctx context.Context, xcs content.Store, xdesc ocispec.Descriptor) error {
		_, err := remotes.FetchHandler(xcs, fetcher)(xctx, xdesc)
		return err
	}
	tr := contentstore.Transfer{
		ContentProvider: ci.opt.ContentStore,
		Digest:          desc.Digest,
	}
	return ci.opt.ContentStoreCacheImporter.ImportWithContentEnsurer(ctx, tr, ensurer)
}
