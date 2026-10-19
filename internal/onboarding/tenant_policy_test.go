package onboarding

import (
	"errors"
	"testing"
)

func TestImageAdmissionFollowsTenantLifecycle(t *testing.T) {
	tests := []struct {
		name       string
		moves      []Status
		wantStatus Status
		wantErr    error
	}{
		{name: "new account waits for review", wantStatus: Pending, wantErr: ErrImagesDisabled},
		{name: "active account may process", moves: []Status{Active}, wantStatus: Active},
		{name: "suspended account stops processing", moves: []Status{Active, Suspended}, wantStatus: Suspended, wantErr: ErrImagesDisabled},
		{name: "reactivated account resumes", moves: []Status{Active, Suspended, Active}, wantStatus: Active},
		{name: "closed account stays disabled", moves: []Status{Closed}, wantStatus: Closed, wantErr: ErrImagesDisabled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry()
			tenant, err := registry.Onboard("acme")
			if err != nil {
				t.Fatal(err)
			}
			for _, next := range tt.moves {
				tenant, err = registry.Transition(tenant.ID, next)
				if err != nil {
					t.Fatal(err)
				}
			}
			if tenant.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q", tenant.Status, tt.wantStatus)
			}
			if err := registry.AuthorizeImage(tenant.ID); !errors.Is(err, tt.wantErr) {
				t.Fatalf("AuthorizeImage() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestClosedTenantCannotBeReactivated(t *testing.T) {
	registry := NewRegistry()
	if _, err := registry.Onboard("acme"); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Transition("acme", Closed); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Transition("acme", Active); !errors.Is(err, ErrInvalidMove) {
		t.Fatalf("Transition() error = %v, want %v", err, ErrInvalidMove)
	}
}
