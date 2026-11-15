package httpapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"littlevillage/internal/auth"
	"littlevillage/internal/store"
)

type familyHandlers struct {
	store        *store.Store
	cookieSecure bool
}

type createFamilyRequest struct {
	FamilyName   string `json:"family_name"`
	MemberName   string `json:"member_name"`
	Relationship string `json:"relationship"`
}

type joinFamilyRequest struct {
	Token        string `json:"token"`
	MemberName   string `json:"member_name"`
	Relationship string `json:"relationship"`
}

type switchRequest struct {
	MemberID string `json:"member_id"`
}

type familyResponse struct {
	Family  store.FamilyRef `json:"family"`
	Members []store.Member  `json:"members"`
	Invite  inviteLink      `json:"invite"`
}

type inviteLink struct {
	Token string `json:"token"`
	// Path is relative; the client adds its own origin.
	Path string `json:"path"`
}

// POST /api/families
func (h *familyHandlers) create(w http.ResponseWriter, r *http.Request) {
	var req createFamilyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	familyName, ok := cleanName(req.FamilyName)
	if !ok {
		writeError(w, http.StatusBadRequest, codeValidation, "Family name must be 1 to 40 characters.")
		return
	}
	memberName, problem := validateMember(req.MemberName, req.Relationship)
	if problem != "" {
		writeError(w, http.StatusBadRequest, codeValidation, problem)
		return
	}

	b, err := h.store.CreateFamily(r.Context(), auth.DeviceToken(r), familyName, memberName, req.Relationship)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	h.respondBound(w, r, b)
}

// POST /api/families/join
func (h *familyHandlers) join(w http.ResponseWriter, r *http.Request) {
	var req joinFamilyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	memberName, problem := validateMember(req.MemberName, req.Relationship)
	if problem != "" {
		writeError(w, http.StatusBadRequest, codeValidation, problem)
		return
	}

	b, err := h.store.JoinFamily(r.Context(), auth.DeviceToken(r), req.Token, memberName, req.Relationship)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeInviteNotFound(w)
		return
	case errors.Is(err, store.ErrFamilyFull):
		writeError(w, http.StatusConflict, codeFamilyFull, "This family already has the maximum of 10 members.")
		return
	case err != nil:
		writeInternal(w, r, err)
		return
	}
	h.respondBound(w, r, b)
}

// respondBound sets the device cookie if one was just created and returns
// the new active member, as GET /api/me would.
func (h *familyHandlers) respondBound(w http.ResponseWriter, r *http.Request, b store.Binding) {
	if b.NewDeviceToken != "" {
		auth.SetDeviceCookie(w, r, b.NewDeviceToken, h.cookieSecure)
	}
	me, err := h.store.Me(r.Context(), b.Session)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, me)
}

// GET /api/invites/{token} (public)
func (h *familyHandlers) invite(w http.ResponseWriter, r *http.Request) {
	inv, err := h.store.InviteByToken(r.Context(), chi.URLParam(r, "token"))
	if errors.Is(err, store.ErrNotFound) {
		writeInviteNotFound(w)
		return
	}
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, inv)
}

func writeInviteNotFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, codeInviteNotFound,
		"This invite link is not valid. Ask your family for a new link.")
}

// GET /api/me (private)
func (h *familyHandlers) me(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.SessionFrom(r.Context())
	me, err := h.store.Me(r.Context(), sess)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, me)
}

// GET /api/family (private). The family comes from the session only.
func (h *familyHandlers) family(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.SessionFrom(r.Context())
	fam, err := h.store.FamilyByID(r.Context(), sess.FamilyID)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, familyResponse{
		Family:  fam.Family,
		Members: fam.Members,
		Invite:  inviteLink{Token: fam.InviteToken, Path: "/join/" + fam.InviteToken},
	})
}

// POST /api/session/switch (private)
func (h *familyHandlers) switchMember(w http.ResponseWriter, r *http.Request) {
	sess, _ := auth.SessionFrom(r.Context())
	var req switchRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	var id pgtype.UUID
	if err := id.Scan(req.MemberID); err != nil {
		writeError(w, http.StatusBadRequest, codeValidation, "member_id must be a UUID.")
		return
	}

	err := h.store.SwitchActiveMember(r.Context(), sess.DeviceID, req.MemberID)
	if errors.Is(err, store.ErrNotDeviceMember) {
		writeError(w, http.StatusForbidden, codeForbidden, "You can only switch to members added in this browser.")
		return
	}
	if err != nil {
		writeInternal(w, r, err)
		return
	}

	sess.MemberID = req.MemberID
	me, err := h.store.Me(r.Context(), sess)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, me)
}
