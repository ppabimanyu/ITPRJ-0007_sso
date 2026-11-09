package httphandler

import (
	"errors"
	"net/http"
	"time"

	"github.com/ppa/itprj-0007-sso/internal/domain"
)

func (h *handlers) authorizeRequestInfo(w http.ResponseWriter, r *http.Request) {
	requestID := r.URL.Query().Get("request_id")
	if requestID == "" || len(requestID) > 128 {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest, "request_id is required")
		return
	}
	request, err := h.loadAuthRequest(r, requestID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "authorization request was not found")
			return
		}
		h.logInternal(r.Context(), "auth_request_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	client, err := h.deps.Clients.GetByClientID(r.Context(), request.RealmID, request.ClientIDString)
	if err != nil {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "client was not found")
		return
	}
	current, err := h.optionalSession(r.Context(), r)
	if err != nil {
		h.logInternal(r.Context(), "session_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}

	loginRequired := current == nil
	if current != nil && request.Prompt == "login" &&
		!current.Session.AuthTime.After(request.CreatedAt) {
		loginRequired = true
	}
	if current != nil && request.MaxAgeSeconds != nil {
		if current.Session.AuthTime.Before(time.Now().UTC().Add(-time.Duration(*request.MaxAgeSeconds) * time.Second)) {
			loginRequired = true
		}
	}
	consentRequired := false
	if client.ThirdParty {
		consentRequired = true
		if current != nil {
			consent, err := h.deps.Clients.GetConsent(r.Context(), current.User.ID, client.ID)
			if err != nil && !errors.Is(err, domain.ErrNotFound) {
				h.logInternal(r.Context(), "consent_lookup_failed", err)
				WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
					"the server could not complete the request")
				return
			}
			consentRequired = !consentCovers(consent, request.Scopes)
		}
	}
	var prompt *string
	if request.Prompt != "" {
		prompt = &request.Prompt
	}
	writeJSON(w, http.StatusOK, domain.AuthRequestInfo{
		RequestID:       request.ID,
		ClientID:        client.ClientID,
		ClientName:      client.ClientID,
		Scopes:          request.Scopes,
		Prompt:          prompt,
		LoginHint:       request.LoginHint,
		LoginRequired:   loginRequired,
		ConsentRequired: consentRequired,
	})
}

func (h *handlers) completeAuthorize(w http.ResponseWriter, r *http.Request) {
	var body domain.CompleteAuthorizeRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	h.finishAuthorization(w, r, body.RequestID, nil)
}

func (h *handlers) grantConsent(w http.ResponseWriter, r *http.Request) {
	var body domain.ConsentRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	if len(body.Scopes) == 0 || !hasScope(body.Scopes, domain.ScopeOpenID) {
		WriteValidationProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
			"scopes must include openid", []FieldError{{Field: "scopes", Message: "must include openid"}})
		return
	}
	h.finishAuthorization(w, r, body.RequestID, body.Scopes)
}

func (h *handlers) finishAuthorization(w http.ResponseWriter, r *http.Request, requestID string, grantedScopes []string) {
	if requestID == "" || len(requestID) > 128 {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest, "request_id is required")
		return
	}
	request, err := h.loadAuthRequest(r, requestID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "authorization request was not found")
			return
		}
		h.logInternal(r.Context(), "auth_request_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return
	}
	current := SessionFromContext(r.Context())
	if current == nil || !sessionUsable(current, time.Now().UTC()) {
		WriteProblem(w, r, http.StatusUnauthorized, domain.CodeSessionExpired,
			"session is missing or expired")
		return
	}
	if current.Session.RealmID != request.RealmID {
		WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
			"session belongs to a different realm")
		return
	}
	if request.Prompt == "login" && !current.Session.AuthTime.After(request.CreatedAt) {
		WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
			"login is required before this request can complete")
		return
	}
	if request.MaxAgeSeconds != nil {
		oldest := time.Now().UTC().Add(-time.Duration(*request.MaxAgeSeconds) * time.Second)
		if current.Session.AuthTime.Before(oldest) {
			WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
				"the session is older than max_age")
			return
		}
	}
	client, err := h.deps.Clients.GetByClientID(r.Context(), request.RealmID, request.ClientIDString)
	if err != nil || client.DisabledAt != nil {
		WriteProblem(w, r, http.StatusConflict, domain.CodeConflict, "client is not available")
		return
	}
	if grantedScopes != nil && !client.ThirdParty {
		WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
			"consent is only required for third-party clients")
		return
	}

	scopes := request.Scopes
	if grantedScopes != nil {
		if !scopesSubset(grantedScopes, request.Scopes) {
			WriteValidationProblem(w, r, http.StatusUnprocessableEntity, domain.CodeBadRequest,
				"granted scopes must be a subset of the requested scopes",
				[]FieldError{{Field: "scopes", Message: "contains a scope that was not requested"}})
			return
		}
		if err := h.deps.Clients.UpsertConsent(r.Context(), &domain.Consent{
			UserID:   current.User.ID,
			ClientID: client.ID,
			Scope:    domain.JoinScopes(grantedScopes),
		}); err != nil {
			h.logInternal(r.Context(), "consent_store_failed", err)
			WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
				"the server could not complete the request")
			return
		}
		scopes = grantedScopes
	} else if client.ThirdParty {
		consent, err := h.deps.Clients.GetConsent(r.Context(), current.User.ID, client.ID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			h.logInternal(r.Context(), "consent_lookup_failed", err)
			WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
				"the server could not complete the request")
			return
		}
		if !consentCovers(consent, request.Scopes) {
			WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
				"consent is required before this request can complete")
			return
		}
	}

	redirect, err := h.issueAuthorizationCode(r.Context(), request.RealmID, client, request, current, scopes)
	if err != nil {
		WriteProblem(w, r, http.StatusServiceUnavailable, domain.CodeServiceUnavailable,
			"authorization codes are unavailable")
		return
	}
	clearAuthRequestCookie(w, h.deps.Config)
	writeJSON(w, http.StatusOK, domain.RedirectResponse{RedirectTo: redirect})
}

func (h *handlers) loadAuthRequest(r *http.Request, requestID string) (*domain.AuthRequest, error) {
	if h.deps.AuthRequests == nil {
		return nil, domain.ErrNotFound
	}
	request, err := h.deps.AuthRequests.Get(r.Context(), requestID)
	if err != nil {
		return nil, err
	}
	if !h.authRequestCookieMatches(r, request) {
		return nil, domain.ErrNotFound
	}
	return request, nil
}
