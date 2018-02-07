package control

import (
	"context"

	"github.com/docker/distribution/reference"
	controlapi "github.com/moby/buildkit/api/services/control"
	trans "github.com/moby/buildkit/cache/transferable/contentstore"
	registrytrans "github.com/moby/buildkit/cache/transferable/registry"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/exporter"
	"github.com/moby/buildkit/frontend"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/grpchijack"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/worker"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	netcontext "golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

type Opt struct {
	SessionManager   *session.Manager
	WorkerController *worker.Controller
	Frontends        map[string]frontend.Frontend
}

type Controller struct { // TODO: ControlService
	opt    Opt
	solver *solver.Solver
}

func NewController(opt Opt) (*Controller, error) {
	c := &Controller{
		opt: opt,
		solver: solver.NewLLBOpSolver(solver.LLBOpt{
			WorkerController: opt.WorkerController,
			Frontends:        opt.Frontends,
		}),
	}
	return c, nil
}

func (c *Controller) Register(server *grpc.Server) error {
	controlapi.RegisterControlServer(server, c)
	return nil
}

func (c *Controller) DiskUsage(ctx netcontext.Context, r *controlapi.DiskUsageRequest) (*controlapi.DiskUsageResponse, error) {
	resp := &controlapi.DiskUsageResponse{}
	workers, err := c.opt.WorkerController.List()
	if err != nil {
		return nil, err
	}
	for _, w := range workers {
		du, err := w.DiskUsage(ctx, client.DiskUsageInfo{
			Filter: r.Filter,
		})
		if err != nil {
			return nil, err
		}

		for _, r := range du {
			resp.Record = append(resp.Record, &controlapi.UsageRecord{
				// TODO: add worker info
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
	}
	return resp, nil
}

func (c *Controller) Prune(req *controlapi.PruneRequest, stream controlapi.Control_PruneServer) error {
	ch := make(chan client.UsageInfo)

	eg, ctx := errgroup.WithContext(stream.Context())
	workers, err := c.opt.WorkerController.List()
	if err != nil {
		return errors.Wrap(err, "failed to list workers for prune")
	}

	for _, w := range workers {
		func(w worker.Worker) {
			eg.Go(func() error {
				return w.Prune(ctx, ch)
			})
		}(w)
	}

	eg2, ctx := errgroup.WithContext(stream.Context())

	eg2.Go(func() error {
		defer close(ch)
		return eg.Wait()
	})

	eg2.Go(func() error {
		for r := range ch {
			if err := stream.Send(&controlapi.UsageRecord{
				// TODO: add worker info
				ID:          r.ID,
				Mutable:     r.Mutable,
				InUse:       r.InUse,
				Size_:       r.Size,
				Parent:      r.Parent,
				UsageCount:  int64(r.UsageCount),
				Description: r.Description,
				CreatedAt:   r.CreatedAt,
				LastUsedAt:  r.LastUsedAt,
			}); err != nil {
				return err
			}
		}
		return nil
	})

	return eg2.Wait()
}

func (c *Controller) Solve(ctx netcontext.Context, req *controlapi.SolveRequest) (*controlapi.SolveResponse, error) {
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
	// TODO: multiworker
	// This is actually tricky, as the exporter should come from the worker that has the returned reference. We may need to delay this so that the solver loads this.
	w, err := c.opt.WorkerController.GetDefault()
	if err != nil {
		return nil, err
	}
	if req.Exporter != "" {
		exp, err := w.Exporter(req.Exporter)
		if err != nil {
			return nil, err
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

	if ref := req.Cache.ImportRef; ref != "" {
		parsed, err := reference.ParseNormalizedNamed(ref)
		if err != nil {
			return nil, err
		}
		importCacheRef := reference.TagNameOnly(parsed).String()
		// TODO: multiworker
		w, err := c.opt.WorkerController.GetDefault()
		if err != nil {
			return nil, err
		}
		ci := w.CacheImporter().(trans.EnsurableImporter)
		regciOpt := registrytrans.ImporterOpt{
			SessionManager:            c.opt.SessionManager,
			ContentStore:              w.ContentStore(),
			ContentStoreCacheImporter: ci,
		}
		regCI := registrytrans.NewImporter(regciOpt)
		ic, err := regCI.Import(ctx, importCacheRef)
		if err != nil {
			return nil, err
		}
		w.InjectInstructionCache(ic)
	}

	solverCE, err := c.solver.Solve(ctx, req.Ref, solver.SolveRequest{
		Frontend:    frontend,
		Definition:  req.Definition,
		Exporter:    expi,
		FrontendOpt: req.FrontendAttrs,
	})
	if err != nil {
		return nil, err
	}
	if exportCacheRef != "" {
		if solverCE == nil {
			return nil, errors.New("exportCacheRef specified, but solverCE is nil")
		}
		records, err := solverCE.Export(ctx)
		if err != nil {
			return nil, err
		}
		// TODO: multiworker
		w, err := c.opt.WorkerController.GetDefault()
		if err != nil {
			return nil, err
		}
		regCEOpt := registrytrans.ExporterOpt{
			SessionManager:            c.opt.SessionManager,
			ContentStoreCacheExporter: w.CacheExporter(),
		}
		regCE := registrytrans.NewExporter(regCEOpt)
		if err = regCE.Export(ctx, records, exportCacheRef); err != nil {
			return nil, err
		}
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
	conn, closeCh, opts := grpchijack.Hijack(stream)
	defer conn.Close()

	ctx, cancel := context.WithCancel(stream.Context())
	go func() {
		<-closeCh
		cancel()
	}()

	err := c.opt.SessionManager.HandleConn(ctx, conn, opts)
	logrus.Debugf("session finished: %v", err)
	return err
}

func (c *Controller) ListWorkers(ctx netcontext.Context, r *controlapi.ListWorkersRequest) (*controlapi.ListWorkersResponse, error) {
	resp := &controlapi.ListWorkersResponse{}
	workers, err := c.opt.WorkerController.List(r.Filter...)
	if err != nil {
		return nil, err
	}
	for _, w := range workers {
		resp.Record = append(resp.Record, &controlapi.WorkerRecord{
			ID:     w.ID(),
			Labels: w.Labels(),
		})
	}
	return resp, nil
}
