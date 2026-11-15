package httpapi_test

import (
	"net/http"
	"testing"

	"littlevillage/internal/store"
)

func (b *browser) planInterest(method string, wantStatus int) store.PlanInterest {
	b.t.Helper()
	var p store.PlanInterest
	if status := b.call(method, "/api/plan/interest", nil, &p); status != wantStatus {
		b.t.Fatalf("%s /api/plan/interest: status %d, want %d", method, status, wantStatus)
	}
	return p
}

func TestPlanInterestIsSavedPerMemberAndCountedPerFamily(t *testing.T) {
	t.Parallel()
	srv, _ := newServer(t)
	mom := newBrowser(t, srv)
	mom.createFamily("Smiths", "Anna", "Mom")
	grandma := newBrowser(t, srv)
	grandma.join(mom.family().Invite.Token, "Mary", "Grandma")
	other := newBrowser(t, srv)
	other.createFamily("Others", "Bob", "Dad")

	if p := mom.planInterest(http.MethodGet, http.StatusOK); p.Interested || p.FamilyInterested != 0 || p.FamilyMembers != 2 {
		t.Fatalf("before anyone asked: %+v", p)
	}
	// Registering twice counts once.
	mom.planInterest(http.MethodPost, http.StatusCreated)
	if p := mom.planInterest(http.MethodPost, http.StatusCreated); !p.Interested || p.FamilyInterested != 1 {
		t.Fatalf("after mom asked twice: %+v", p)
	}
	// Grandma sees the family's count but has not asked herself.
	if p := grandma.planInterest(http.MethodGet, http.StatusOK); p.Interested || p.FamilyInterested != 1 || p.FamilyMembers != 2 {
		t.Fatalf("grandma's view: %+v", p)
	}
	// Another family is not affected.
	if p := other.planInterest(http.MethodGet, http.StatusOK); p.Interested || p.FamilyInterested != 0 || p.FamilyMembers != 1 {
		t.Fatalf("other family's view: %+v", p)
	}
}
