package httphandler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"github.com/google/uuid"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/infra/logger"
)

func (h *handlers) oidcLogout(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest, "logout parameters are invalid")
		return
	}
	realm, err := h.realm(r.Context())
	if err != nil {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "realm was not found")
		return
	}
	issuer := realmIssuer(h.deps.Config.HTTP.ExternalURL, realm)
	clientID := r.FormValue("client_id")
	hint := r.FormValue("id_token_hint")
	postLogout := r.FormValue("post_logout_redirect_uri")
	state := r.FormValue("state")

	var subject, sessionID string
	if hint != "" {
		claims, err := h.deps.Tokens.VerifyIDToken(r.Context(), realm.ID, hint)
		if err != nil {
			WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest, "id_token_hint is invalid")
			return
		}
		if claims.Issuer != issuer {
			WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest, "id_token_hint was issued by another issuer")
			return
		}
		if clientID != "" && claims.Audience != clientID {
			WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest, "id_token_hint audience does not match client_id")
			return
		}
		if clientID == "" {
			clientID = claims.Audience
		}
		subject = claims.Subject
		sessionID = claims.SessionID
	}
	if clientID == "" {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
			"client_id or id_token_hint is required")
		return
	}
	client, err := h.deps.Clients.GetByClientID(r.Context(), realm.ID, clientID)
	if err != nil || client.DisabledAt != nil {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest, "client is unknown or disabled")
		return
	}
	if postLogout != "" {
		registered, err := h.deps.Clients.FindRedirectURI(r.Context(), client.ID, postLogout, domain.RedirectKindPostLogout)
		if err != nil {
			h.logInternal(r.Context(), "post_logout_lookup_failed", err)
			WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
				"the server could not complete the request")
			return
		}
		if !registered {
			WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
				"post_logout_redirect_uri is not registered for this client")
			return
		}
	}

	revoked := false
	if sessionID != "" && h.deps.Sessions != nil {
		if subject != "" {
			if session, err := h.deps.Sessions.GetByID(r.Context(), domain.SessionID(sessionID)); err == nil &&
				string(session.User.ID) != subject {
				WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
					"id_token_hint subject does not match the session")
				return
			}
		}
		audit := auditEvent(realm.ID, domain.ActorTypeUser, subjectActor(subject),
			domain.AuditOIDCLogout, map[string]any{"client_id": client.ClientID, "sid": sessionID})
		revoked, err = h.deps.Sessions.RevokeByID(r.Context(), domain.SessionID(sessionID), audit)
		if err != nil {
			h.logInternal(r.Context(), "logout_revoke_failed", err)
			WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
				"the server could not complete the request")
			return
		}
	}
	if !revoked {
		if current, err := h.optionalSession(r.Context(), r); err == nil && current != nil {
			audit := auditEvent(current.User.RealmID, domain.ActorTypeUser, userActor(current.User.ID),
				domain.AuditOIDCLogout, map[string]any{"client_id": client.ClientID, "sid": string(current.Session.ID)})
			if _, err := h.deps.Sessions.RevokeByID(r.Context(), current.Session.ID, audit); err != nil {
				h.logInternal(r.Context(), "logout_revoke_failed", err)
				WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
					"the server could not complete the request")
				return
			}
			revoked = true
			subject = string(current.User.ID)
			sessionID = string(current.Session.ID)
		}
	}
	if !revoked {
		h.auditOIDC(r.Context(), realm.ID, domain.ActorTypeSystem, nil, domain.AuditOIDCLogout,
			map[string]any{"client_id": client.ClientID, "session_found": false})
	}

	if client.BackchannelLogoutURI != nil && *client.BackchannelLogoutURI != "" && sessionID != "" {
		h.enqueueBackchannelLogout(r, realm, client, subject, sessionID)
	}

	clearSessionCookie(w, h.deps.Config)
	clearCSRFCookie(w, h.deps.Config)
	clearAuthRequestCookie(w, h.deps.Config)
	if postLogout != "" {
		target, err := url.Parse(postLogout)
		if err == nil {
			query := target.Query()
			if state != "" {
				query.Set("state", state)
			}
			target.RawQuery = query.Encode()
			http.Redirect(w, r, target.String(), http.StatusFound)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) enqueueBackchannelLogout(r *http.Request, realm *domain.Realm, client *domain.Client, subject, sessionID string) {
	if h.deps.Outbox == nil {
		return
	}
	payload, err := json.Marshal(map[string]any{
		"client_id": client.ClientID,
		"uri":       *client.BackchannelLogoutURI,
		"sub":       subject,
		"sid":       sessionID,
		"iss":       realmIssuer(h.deps.Config.HTTP.ExternalURL, realm),
	})
	if err != nil {
		logger.FromContext(r.Context()).Error("logout_payload_failed", "error", err)
		return
	}
	event := &domain.OutboxEvent{
		ID:      domain.OutboxEventID(uuid.NewString()),
		RealmID: realm.ID,
		Type:    domain.OutboxLogoutBackchannel,
		Payload: payload,
		Status:  domain.OutboxStatusPending,
	}
	if err := h.deps.Outbox.Enqueue(r.Context(), event); err != nil {
		logger.FromContext(r.Context()).Error("backchannel_enqueue_failed", "error", err)
	}
}

func (h *handlers) backchannelLogout(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest, "logout_token is invalid")
		return
	}
	realm, err := h.realm(r.Context())
	if err != nil {
		WriteProblem(w, r, http.StatusNotFound, domain.CodeNotFound, "realm was not found")
		return
	}
	raw := r.PostFormValue("logout_token")
	if raw == "" {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest, "logout_token is required")
		return
	}
	token, err := h.deps.Tokens.VerifyLogoutToken(r.Context(), realm.ID, raw)
	if err != nil {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest, "logout_token is invalid")
		return
	}
	if token.Issuer != realmIssuer(h.deps.Config.HTTP.ExternalURL, realm) {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest, "logout_token was issued by another issuer")
		return
	}
	client, err := h.deps.Clients.GetByClientID(r.Context(), realm.ID, token.Audience)
	if err != nil || client.DisabledAt != nil {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest, "logout_token audience is unknown")
		return
	}
	if requested := r.PostFormValue("client_id"); requested != "" && requested != client.ClientID {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest, "client_id does not match the logout_token")
		return
	}
	if token.SessionID == "" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if h.deps.Sessions != nil {
		session, err := h.deps.Sessions.GetByID(r.Context(), domain.SessionID(token.SessionID))
		if err == nil {
			if token.Subject != "" && string(session.User.ID) != token.Subject {
				WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
					"logout_token subject does not match the session")
				return
			}
			audit := auditEvent(realm.ID, domain.ActorTypeClient, &client.ClientID,
				domain.AuditOIDCLogout, map[string]any{
					"client_id": client.ClientID,
					"sid":       token.SessionID,
					"direction": "inbound",
				})
			if _, err := h.deps.Sessions.RevokeByID(r.Context(), domain.SessionID(token.SessionID), audit); err != nil {
				h.logInternal(r.Context(), "backchannel_revoke_failed", err)
				WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
					"the server could not complete the request")
				return
			}
		} else if !errors.Is(err, domain.ErrNotFound) {
			h.logInternal(r.Context(), "backchannel_session_lookup_failed", err)
			WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
				"the server could not complete the request")
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

func subjectActor(subject string) *string {
	if subject == "" {
		return nil
	}
	return &subject
}
