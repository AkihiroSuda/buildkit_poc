package frontend

import (
	"context"
	"io"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/llb"
	digest "github.com/opencontainers/go-digest"
)

type Frontend interface {
	Solve(ctx context.Context, llb FrontendLLBBridge, opt map[string]string) (cache.ImmutableRef, map[string][]byte, error)
}

type FrontendLLBBridge interface {
	Solve(ctx context.Context, req SolveRequest) (cache.ImmutableRef, map[string][]byte, error)
	ResolveImageConfig(ctx context.Context, ref string) (digest.Digest, []byte, error)
	Exec(ctx context.Context, meta executor.Meta, rootfs cache.ImmutableRef, stdin io.ReadCloser, stdout, stderr io.WriteCloser) error
}

type SolveRequest struct {
	Definition  *llb.Definition
	Frontend    string
	FrontendOpt map[string]string
}
