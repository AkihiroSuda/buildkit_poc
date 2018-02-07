package registry

import (
	"context"

	"github.com/moby/buildkit/cache/transferable/contentstore"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/util/push"
)

type ExporterOpt struct {
	ContentStoreCacheExporter contentstore.Exporter
	SessionManager            *session.Manager
}

func NewExporter(opt ExporterOpt) *Exporter {
	return &Exporter{opt: opt}
}

type Exporter struct {
	opt ExporterOpt
}

// Export exports the cache to a Docker registry.
// By design, the prototype of this method differs from transferable/contentstore.CacheExporter
func (ce *Exporter) Export(ctx context.Context, rec []contentstore.CacheRecord, target string) error {
	tr, err := ce.opt.ContentStoreCacheExporter.Export(ctx, rec)
	if err != nil {
		return err
	}
	// TODO: Create layers and push them concurrently?
	return push.Push(ctx, ce.opt.SessionManager, tr.ContentProvider, tr.Digest, target, false)
}
