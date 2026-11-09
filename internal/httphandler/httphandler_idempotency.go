package httphandler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/ppa/itprj-0007-sso/internal/domain"
	"github.com/ppa/itprj-0007-sso/internal/pkg"
)

// checkIdempotency validates an Idempotency-Key and replays a stored response.
// It returns false when it has already written the response or an error.
func (h *handlers) checkIdempotency(w http.ResponseWriter, r *http.Request, realmID domain.RealmID, endpoint, key string, request any) (string, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", true
	}
	if len(key) > maxIdempotencyKeySize {
		WriteProblem(w, r, http.StatusBadRequest, domain.CodeBadRequest,
			"Idempotency-Key must be at most 128 characters")
		return "", false
	}
	canonical, err := json.Marshal(request)
	if err != nil {
		h.logInternal(r.Context(), "idempotency_hash_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return "", false
	}
	requestHash := pkg.SHA256Hex(string(canonical))
	if h.deps.Idempotency == nil {
		return requestHash, true
	}
	record, err := h.deps.Idempotency.Get(r.Context(), realmID, endpoint, key)
	if err == nil {
		if record.RequestHash != requestHash {
			WriteProblem(w, r, http.StatusConflict, domain.CodeIdempotencyMismatch,
				"Idempotency-Key was already used with a different request")
			return "", false
		}
		var payload idempotencyPayload
		if len(record.Response) > 0 && json.Unmarshal(record.Response, &payload) == nil &&
			payload.Status != 0 && len(payload.Body) > 0 {
			writeRawJSON(w, payload.Status, payload.Body)
			return "", false
		}
		WriteProblem(w, r, http.StatusConflict, domain.CodeConflict,
			"the idempotent request is still being processed")
		return "", false
	}
	if !errors.Is(err, domain.ErrNotFound) {
		h.logInternal(r.Context(), "idempotency_lookup_failed", err)
		WriteProblem(w, r, http.StatusInternalServerError, domain.CodeInternal,
			"the server could not complete the request")
		return "", false
	}
	return requestHash, true
}

// saveIdempotency stores the response for replay. Failure is logged, not
// returned: the resource already committed.
func (h *handlers) saveIdempotency(ctx context.Context, realmID domain.RealmID, endpoint, key, requestHash string, status int, body []byte) {
	if key == "" || h.deps.Idempotency == nil {
		return
	}
	payload, err := json.Marshal(idempotencyPayload{Status: status, Body: body})
	if err != nil {
		return
	}
	record := &domain.IdempotencyKey{
		ID:          uuid.NewString(),
		RealmID:     realmID,
		Key:         key,
		Endpoint:    endpoint,
		RequestHash: requestHash,
		Response:    payload,
		CreatedAt:   nowUTC(),
		ExpiresAt:   nowUTC().Add(idempotencyLifetime),
	}
	if err := h.deps.Idempotency.Save(ctx, record); err != nil {
		h.logInternal(ctx, "idempotency_save_failed", err)
	}
}
