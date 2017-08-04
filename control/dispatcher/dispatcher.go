package dispatcher

import (
	"encoding/json"
	"errors"
	context "golang.org/x/net/context"

	controlapi "github.com/moby/buildkit/api/services/control"
	"google.golang.org/grpc/metadata"
)

var (
	ErrUnsatisfiableConstraint = errors.New("unsatisfiable constraint")
)

type DispatchableController interface {
	controlapi.ControlServer
	// OR-match
	MeetConstraint(constraint *controlapi.Constraint) bool
}

type dispatcher struct {
	controllers []DispatchableController
}

func NewDispatcher(controllers []DispatchableController) (controlapi.ControlServer, error) {
	return &dispatcher{controllers: controllers}, nil
}

func (d *dispatcher) getMetadata(ctx context.Context) *controlapi.Metadata {
	m, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil
	}
	ss, ok := m[controlapi.MetadataKey]
	if !ok {
		return nil
	}
	if len(ss) == 0 {
		return nil
	}
	s := ss[0]
	var md controlapi.Metadata
	if err := json.Unmarshal([]byte(s), &s); err != nil {
		return nil
	}
	return &md
}

// Fields within structure: OR-match
// Slice of structures: AND-match (but zero-length slice matches anything)
func (d *dispatcher) findMatchingController(ctx context.Context) controlapi.ControlServer {
	md := d.getMetadata(ctx)
	if md == nil {
		return d.controllers[0]
	}
controllerLoop:
	for _, ctlr := range d.controllers {
		for _, constr := range md.Constraints {
			if !ctlr.MeetConstraint(&constr) {
				continue controllerLoop
			}
		}
		return ctlr
	}
	return nil
}

func (d *dispatcher) DiskUsage(ctx context.Context, r *controlapi.DiskUsageRequest) (*controlapi.DiskUsageResponse, error) {
	if ctlr := d.findMatchingController(ctx); ctlr != nil {
		return ctlr.DiskUsage(ctx, r)
	}
	return nil, ErrUnsatisfiableConstraint
}

func (d *dispatcher) Solve(ctx context.Context, r *controlapi.SolveRequest) (*controlapi.SolveResponse, error) {
	if ctlr := d.findMatchingController(ctx); ctlr != nil {
		return ctlr.Solve(ctx, r)
	}
	return nil, ErrUnsatisfiableConstraint
}

func (d *dispatcher) Status(r *controlapi.StatusRequest, ss controlapi.Control_StatusServer) error {
	if ctlr := d.findMatchingController(ss.Context()); ctlr != nil {
		return ctlr.Status(r, ss)
	}
	return ErrUnsatisfiableConstraint
}

func (d *dispatcher) Session(ss controlapi.Control_SessionServer) error {
	if ctlr := d.findMatchingController(ss.Context()); ctlr != nil {
		return ctlr.Session(ss)
	}
	return ErrUnsatisfiableConstraint
}
