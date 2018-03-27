package client

import (
	"context"
	"io"
)

type contextKeyT string

var contextKeyFSSyncTargetFile = contextKeyT("buildkit/client/fssynctargetfile")

// ContextWithFSSyncTargetFile creates a context with io.WriteCloser that can be used as the
// target for image exportation.
func ContextWithFSSyncTargetFile(ctx context.Context, w io.WriteCloser) context.Context {
	return context.WithValue(ctx, contextKeyFSSyncTargetFile, w)
}

// FSSyncTargetFileFromContext extracts an io.WriteCloser from a context
// composed with ContextWithFSSyncTargetFile.
func FSSyncTargetFileFromContext(ctx context.Context) io.WriteCloser {
	v := ctx.Value(contextKeyFSSyncTargetFile)
	if v == nil {
		return nil
	}
	return v.(io.WriteCloser)
}
