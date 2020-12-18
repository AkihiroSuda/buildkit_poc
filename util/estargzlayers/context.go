package estargzlayers

import "context"

type contextKeyT string

var contextKey = contextKeyT("buildkit/estargzlayers-on")

func UseEStarGZLayerMode(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKey, true)
}

func hasEStarGZLayerMode(ctx context.Context) bool {
	v := ctx.Value(contextKey)
	if v == nil {
		return false
	}
	return true
}
