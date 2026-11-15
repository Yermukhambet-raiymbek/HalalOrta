package httpapi

import (
	"net/http"

	"littlevillage/internal/auth"
	"littlevillage/internal/store"
)

type planHandlers struct {
	store *store.Store
}

// GET /api/plan/interest (private): has the active member asked for the
// paid plan, and how many of their family have.
func (h *planHandlers) interest(w http.ResponseWriter, r *http.Request) {
	h.respond(w, r, http.StatusOK)
}

// POST /api/plan/interest (private): the active member says they would use
// the paid plan. No payment is involved. Repeating it changes nothing.
func (h *planHandlers) register(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.SessionFrom(r.Context())
	if err := h.store.RegisterPlanInterest(r.Context(), sess.MemberID); err != nil {
		writeInternal(w, r, err)
		return
	}
	h.respond(w, r, http.StatusCreated)
}

func (h *planHandlers) respond(w http.ResponseWriter, r *http.Request, status int) {
	sess, _ := auth.SessionFrom(r.Context())
	p, err := h.store.PlanInterestFor(r.Context(), sess.FamilyID, sess.MemberID)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, status, p)
}
