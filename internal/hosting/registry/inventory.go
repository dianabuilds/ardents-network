package registry

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	hostingreadiness "ardents/internal/hosting/readiness"
	hostingservice "ardents/internal/hosting/service"
)

type Registry struct {
	mu        sync.Mutex
	items     []hostingservice.Spec
	dynamic   map[string]hostingservice.Spec
	observed  map[string]Backing
	readiness *hostingreadiness.Controller
}

type Backing struct {
	Spec       hostingservice.Spec
	WorkloadID string
	Generation int64
	Running    bool
	StartedAt  time.Time
}

type ServiceStatus struct {
	Spec      hostingservice.Spec
	Readiness hostingreadiness.Snapshot
}

func New(specs []hostingservice.Spec) *Registry {
	return NewWithReadiness(specs, hostingreadiness.NewController(hostingreadiness.DefaultPolicy()))
}

func NewWithReadiness(specs []hostingservice.Spec, controller *hostingreadiness.Controller) *Registry {
	items := make([]hostingservice.Spec, 0, len(specs))
	for _, spec := range specs {
		if spec.ID == "" || spec.Type == "" {
			continue
		}
		items = append(items, cloneSpec(spec))
	}
	if controller == nil {
		controller = hostingreadiness.NewController(hostingreadiness.DefaultPolicy())
	}
	return &Registry{items: items, dynamic: map[string]hostingservice.Spec{}, observed: map[string]Backing{}, readiness: controller}
}

func NewStatic(specs []hostingservice.Spec) *Registry {
	return New(specs)
}

func (r *Registry) List() []hostingservice.Spec {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]hostingservice.Spec, 0, len(r.items)+len(r.dynamic))
	seen := make(map[string]struct{}, len(r.items))
	for _, item := range r.items {
		out = append(out, cloneSpec(item))
		seen[item.ID] = struct{}{}
	}
	ids := make([]string, 0, len(r.dynamic))
	for id := range r.dynamic {
		if _, exists := seen[id]; !exists {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		out = append(out, cloneSpec(r.dynamic[id]))
	}
	return out
}

func (r *Registry) Observe(ctx context.Context, backings []Backing, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	current := make(map[string]Backing, len(backings))
	for _, backing := range backings {
		if backing.Spec.ID == "" || backing.WorkloadID == "" {
			return fmt.Errorf("hosted service backing identity is required")
		}
		if _, duplicate := current[backing.Spec.ID]; duplicate {
			return fmt.Errorf("duplicate hosted service backing %s", backing.Spec.ID)
		}
		backing.Spec = cloneSpec(backing.Spec)
		current[backing.Spec.ID] = backing
		r.dynamic[backing.Spec.ID] = cloneSpec(backing.Spec)
		r.readiness.Observe(ctx, readinessObservation(backing), now)
	}
	for id, previous := range r.observed {
		if _, exists := current[id]; exists {
			continue
		}
		previous.Running = false
		r.readiness.Observe(ctx, readinessObservation(previous), now)
	}
	r.observed = current
	return nil
}

func (r *Registry) Readiness(serviceID string, now time.Time) (hostingreadiness.Snapshot, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.readiness.Snapshot(serviceID, now)
}

func (r *Registry) ServiceStatuses(now time.Time) []ServiceStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := make([]string, 0, len(r.dynamic))
	for id := range r.dynamic {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]ServiceStatus, 0, len(ids))
	for _, id := range ids {
		snapshot, ok := r.readiness.Snapshot(id, now)
		if !ok {
			continue
		}
		out = append(out, ServiceStatus{Spec: cloneSpec(r.dynamic[id]), Readiness: snapshot})
	}
	return out
}

func readinessObservation(backing Backing) hostingreadiness.Observation {
	probes := backing.Spec.ProbeEndpoints
	if len(probes) == 0 {
		probes = backing.Spec.Endpoints
	}
	return hostingreadiness.Observation{ServiceID: backing.Spec.ID, WorkloadID: backing.WorkloadID, Generation: backing.Generation,
		Running: backing.Running, StartedAt: backing.StartedAt, Endpoints: append([]string(nil), probes...),
		ExposureEndpoints: append([]string(nil), backing.Spec.Endpoints...)}
}

func cloneSpec(spec hostingservice.Spec) hostingservice.Spec {
	spec.Endpoints = append([]string(nil), spec.Endpoints...)
	spec.ProbeEndpoints = append([]string(nil), spec.ProbeEndpoints...)
	return spec
}
