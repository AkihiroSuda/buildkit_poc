package client

import (
	"encoding/json"
	context "golang.org/x/net/context"

	controlapi "github.com/moby/buildkit/api/services/control"
	"google.golang.org/grpc/metadata"
)

func ContextWithMetadata(ctx context.Context, md controlapi.Metadata) context.Context {
	b, _ := json.Marshal(md)
	return metadata.NewOutgoingContext(ctx, metadata.Pairs(controlapi.MetadataKey, string(b)))
}
