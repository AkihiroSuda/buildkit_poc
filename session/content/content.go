package content

import (
	"github.com/moby/buildkit/session"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/content/proxy"
	contentservice	"github.com/containerd/containerd/services/content"
	"google.golang.org/grpc"
	api "github.com/containerd/containerd/api/services/content/v1"
)

type attachable struct {
	service api.ContentServer
}

func NewAttachable(store content.Store) session.Attachable {
	service := contentservice.NewService(store)
	a := attachable{
		service: service,
	}
	return &a
}

func (a *attachable) Register(server *grpc.Server) {
	api.RegisterContentServer(server, a.service)
}

func CallerStore(c session.Caller) content.Store {
	client :=  api.NewContentClient(c.Conn())
	return proxy.NewContentStore(client)
}
