package registry

import (
	"context"

	"github.com/moby/buildkit/cache/transferable"
	"github.com/moby/buildkit/cache/transferable/contentstore"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/util/push"
)

type ExporterOpt struct {
	ContentStoreCacheExporter *contentstore.CacheExporter
	SessionManager            *session.Manager
}

func NewCacheExporter(opt ExporterOpt) *CacheExporter {
	return &CacheExporter{opt: opt}
}

type CacheExporter struct {
	opt ExporterOpt
}

// Export exports the cache to a Docker registry.
// By design, the prototype of this method differs from transferable/contentstore.CacheExporter
func (ce *CacheExporter) Export(ctx context.Context, rec []transferable.CacheRecord, target string) error {
	desc, store, err := ce.opt.ContentStoreCacheExporter.Export(ctx, rec)
	if err != nil {
		return err
	}
	// Future implementation may creating tars and pushing them concurrently
	// FIXME: push should not require store to be writable?
	return push.PushWithDescriptor(ctx, ce.opt.SessionManager, store, *desc, target, false)
}
