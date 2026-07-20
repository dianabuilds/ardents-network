package controller

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	db "ardents/internal/persistence"
	"ardents/internal/workload/desiredstate"
	workloadregistry "ardents/internal/workload/registry"
)

type Service struct {
	mu            sync.Mutex
	path          string
	state         string
	executor      Executor
	admission     AdmissionFunc
	items         map[string]Status
	restartBudget int
}

func New(path string, executor Executor) *Service {
	if executor == nil {
		panic("controller.New requires executor")
	}
	return &Service{
		path:          path,
		state:         "new",
		executor:      executor,
		items:         map[string]Status{},
		restartBudget: DefaultRestartBudget,
	}
}

func NewInDir(dir string) *Service {
	return NewWithExecutorInDir(dir, NewLocalExecutor())
}

func NewWithExecutorInDir(dir string, executor Executor) *Service {
	return New(db.PathInDir(dir), executor)
}

func (s *Service) SetAdmission(fn AdmissionFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.admission = fn
}

func (s *Service) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.path == "" {
		s.state = "ready"
		return s.reconcileRuntimeInventoryLocked()
	}

	var stored persistentState
	found, err := db.LoadJSON(s.path, "workload", "snapshot", &stored)
	if err != nil {
		return err
	}
	if found {
		s.items = workloadregistry.NormalizeItems(stored.Items)
		s.state = "ready"
		if err := s.recoverLoadedProcessesLocked(); err != nil {
			return err
		}
		return s.reconcileRuntimeInventoryLocked()
	}

	s.items = map[string]Status{}
	s.state = "ready"
	return s.reconcileRuntimeInventoryLocked()
}

func (s *Service) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *Service) Seed(specs []Spec) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, spec := range specs {
		spec = workloadregistry.NormalizeSpec(spec)
		if spec.ID == "" || spec.Kind == "" {
			continue
		}
		if _, ok := s.items[spec.ID]; ok {
			continue
		}
		s.items[spec.ID] = Status{
			Spec:              spec,
			Observed:          ObservedAccepted,
			LastTransitionAt:  time.Now().UTC(),
			PublishedServices: workloadregistry.ServiceStatuses(spec, false, "workload not running"),
		}
	}
	s.state = "ready"
	return s.saveLocked()
}

func (s *Service) Register(spec Spec) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	spec = workloadregistry.NormalizeSpec(spec)
	if spec.ID == "" || spec.Kind == "" {
		return fmt.Errorf("workload id and kind are required")
	}
	if _, exists := s.items[spec.ID]; exists {
		return fmt.Errorf("workload %s already exists", spec.ID)
	}
	s.items[spec.ID] = Status{
		Spec:              spec,
		Observed:          ObservedAccepted,
		LastTransitionAt:  time.Now().UTC(),
		PublishedServices: workloadregistry.ServiceStatuses(spec, false, "workload not running"),
	}
	s.state = "ready"
	return s.saveLocked()
}

func (s *Service) Get(id string) (Status, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[id]
	if !ok {
		return Status{}, false
	}
	return workloadregistry.CloneStatus(item), true
}

func (s *Service) SetDesired(id, desired string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[id]
	if !ok {
		return fmt.Errorf("workload %s not found", id)
	}
	nextDesired := desiredstate.Normalize(desired)
	if item.Spec.Desired != DesiredRunning && nextDesired == DesiredRunning {
		item.RestartCount = 0
		item.NeedsOperatorAction = false
		item.Reason = ""
	}
	item.Spec.Desired = nextDesired
	s.items[id] = normalizeStatus(item)
	return s.saveLocked()
}

func (s *Service) Reconcile(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for id, item := range s.items {
		next, keep, err := s.reconcileLocked(ctx, item, now)
		if err != nil {
			return err
		}
		if keep {
			s.items[id] = next
			continue
		}
		delete(s.items, id)
	}
	s.state = "ready"
	return s.saveLocked()
}

func (s *Service) List() []Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make([]string, 0, len(s.items))
	for id := range s.items {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := make([]Status, 0, len(ids))
	for _, id := range ids {
		out = append(out, workloadregistry.CloneStatus(s.items[id]))
	}
	return out
}

func (s *Service) Published() []Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Status, 0)
	for _, item := range s.items {
		if item.Observed != ObservedRunning || len(item.PublishedServices) == 0 {
			continue
		}
		out = append(out, workloadregistry.CloneStatus(item))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Spec.ID < out[j].Spec.ID
	})
	return out
}

func (s *Service) State() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Service) Desired() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, item := range s.items {
		switch item.Spec.Desired {
		case DesiredPresent, DesiredRunning, DesiredStopped, DesiredDisabled:
			count++
		}
	}
	return count
}

func (s *Service) Active() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, item := range s.items {
		if item.Observed == ObservedRunning {
			count++
		}
	}
	return count
}
