package onboarding

import (
	"errors"
	"sync"
)

type Status string

const (
	Pending   Status = "pending"
	Active    Status = "active"
	Suspended Status = "suspended"
	Closed    Status = "closed"
)

var (
	ErrTenantExists   = errors.New("tenant already exists")
	ErrTenantNotFound = errors.New("tenant not found")
	ErrInvalidMove    = errors.New("invalid tenant status transition")
	ErrImagesDisabled = errors.New("image processing requires an active tenant")
)

type Tenant struct {
	ID     string `json:"id"`
	Status Status `json:"status"`
}

type Registry struct {
	mu      sync.RWMutex
	tenants map[string]Tenant
}

func NewRegistry() *Registry {
	return &Registry{tenants: make(map[string]Tenant)}
}

func (r *Registry) Onboard(id string) (Tenant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tenants[id]; exists {
		return Tenant{}, ErrTenantExists
	}
	tenant := Tenant{ID: id, Status: Pending}
	r.tenants[id] = tenant
	return tenant, nil
}

func (r *Registry) Transition(id string, next Status) (Tenant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tenant, exists := r.tenants[id]
	if !exists {
		return Tenant{}, ErrTenantNotFound
	}
	if !allowed(tenant.Status, next) {
		return Tenant{}, ErrInvalidMove
	}
	tenant.Status = next
	r.tenants[id] = tenant
	return tenant, nil
}

func (r *Registry) AuthorizeImage(id string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tenant, exists := r.tenants[id]
	if !exists {
		return ErrTenantNotFound
	}
	if tenant.Status != Active {
		return ErrImagesDisabled
	}
	return nil
}

func allowed(current, next Status) bool {
	switch current {
	case Pending:
		return next == Active || next == Closed
	case Active:
		return next == Suspended || next == Closed
	case Suspended:
		return next == Active || next == Closed
	default:
		return false
	}
}
