package control

import (
	"github.com/docker/distribution/reference"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/metaworker"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/grpchijack"
	"github.com/moby/buildkit/solver"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

type Opt struct {
	MetaWorkers []*metaworker.MetaWorker
	Frontends   map[string]frontend.Frontend
	// TODO: split more from metaworkers
}

type Controller struct { // TODO: ControlService
	opt    Opt
	solver *solver.Solver
}

func NewController(opt Opt) (*Controller, error) {
	c := &Controller{
		opt: opt,
		solver: solver.NewLLBSolver(solver.LLBOpt{
			MetaWorkers: opt.MetaWorkers,
			Frontends:   opt.Frontends,
		}),
	}
	return c, nil
}

func (c *Controller) Register(server *grpc.Server) error {
	controlapi.RegisterControlServer(server, c)
	return nil
}

func (c *Controller) DiskUsage(ctx context.Context, r *controlapi.DiskUsageRequest) (*controlapi.DiskUsageResponse, error) {
	// FIXME mw0
	mw0 := c.opt.MetaWorkers[0]
	du, err := mw0.CacheManager.DiskUsage(ctx, client.DiskUsageInfo{
		Filter: r.Filter,
	})
	if err != nil {
		return nil, err
	}

	resp := &controlapi.DiskUsageResponse{}
	for _, r := range du {
		resp.Record = append(resp.Record, &controlapi.UsageRecord{
			ID:          r.ID,
			Mutable:     r.Mutable,
			InUse:       r.InUse,
			Size_:       r.Size,
			Parent:      r.Parent,
			UsageCount:  int64(r.UsageCount),
			Description: r.Description,
			CreatedAt:   r.CreatedAt,
			LastUsedAt:  r.LastUsedAt,
		})
	}
	return resp, nil
}

func (c *Controller) Solve(ctx context.Context, req *controlapi.SolveRequest) (*controlapi.SolveResponse, error) {
	var frontend frontend.Frontend
	if req.Frontend != "" {
		var ok bool
		frontend, ok = c.opt.Frontends[req.Frontend]
		if !ok {
			return nil, errors.Errorf("frontend %s not found", req.Frontend)
		}
	}

	ctx = session.NewContext(ctx, req.Session)

	var expi exporter.ExporterInstance
	var err error
	// FIXME mw0
	mw0 := c.opt.MetaWorkers[0]
	if req.Exporter != "" {
		exp, ok := mw0.Exporters[req.Exporter]
		if !ok {
			return nil, errors.Errorf("exporter %q could not be found", req.Exporter)
		}
		expi, err = exp.Resolve(ctx, req.ExporterAttrs)
		if err != nil {
			return nil, err
		}
	}

	exportCacheRef := ""
	if ref := req.Cache.ExportRef; ref != "" {
		parsed, err := reference.ParseNormalizedNamed(ref)
		if err != nil {
			return nil, err
		}
		exportCacheRef = reference.TagNameOnly(parsed).String()
	}

	importCacheRef := ""
	if ref := req.Cache.ImportRef; ref != "" {
		parsed, err := reference.ParseNormalizedNamed(ref)
		if err != nil {
			return nil, err
		}
		importCacheRef = reference.TagNameOnly(parsed).String()
	}

	if err := c.solver.Solve(ctx, req.Ref, solver.SolveRequest{
		Frontend:       frontend,
		Definition:     req.Definition,
		Exporter:       expi,
		FrontendOpt:    req.FrontendAttrs,
		ExportCacheRef: exportCacheRef,
		ImportCacheRef: importCacheRef,
	}); err != nil {
		return nil, err
	}
	return &controlapi.SolveResponse{}, nil
}

func (c *Controller) Status(req *controlapi.StatusRequest, stream controlapi.Control_StatusServer) error {
	ch := make(chan *client.SolveStatus, 8)

	eg, ctx := errgroup.WithContext(stream.Context())
	eg.Go(func() error {
		return c.solver.Status(ctx, req.Ref, ch)
	})

	eg.Go(func() error {
		for {
			ss, ok := <-ch
			if !ok {
				return nil
			}
			sr := controlapi.StatusResponse{}
			for _, v := range ss.Vertexes {
				sr.Vertexes = append(sr.Vertexes, &controlapi.Vertex{
					Digest:    v.Digest,
					Inputs:    v.Inputs,
					Name:      v.Name,
					Started:   v.Started,
					Completed: v.Completed,
					Error:     v.Error,
					Cached:    v.Cached,
				})
			}
			for _, v := range ss.Statuses {
				sr.Statuses = append(sr.Statuses, &controlapi.VertexStatus{
					ID:        v.ID,
					Vertex:    v.Vertex,
					Name:      v.Name,
					Current:   v.Current,
					Total:     v.Total,
					Timestamp: v.Timestamp,
					Started:   v.Started,
					Completed: v.Completed,
				})
			}
			for _, v := range ss.Logs {
				sr.Logs = append(sr.Logs, &controlapi.VertexLog{
					Vertex:    v.Vertex,
					Stream:    int64(v.Stream),
					Msg:       v.Data,
					Timestamp: v.Timestamp,
				})
			}
			if err := stream.SendMsg(&sr); err != nil {
				return err
			}
		}
	})

	return eg.Wait()
}

func (c *Controller) Session(stream controlapi.Control_SessionServer) error {
	logrus.Debugf("session started")
	conn, opts := grpchijack.Hijack(stream)
	defer conn.Close()
	// FIXME mw0
	mw0 := c.opt.MetaWorkers[0]
	err := mw0.SessionManager.HandleConn(stream.Context(), conn, opts)
	logrus.Debugf("session finished: %v", err)
	return err
}
