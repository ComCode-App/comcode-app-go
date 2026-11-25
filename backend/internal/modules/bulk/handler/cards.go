package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	bulkusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bulk/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine/adapters"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

// cardsAccess is the job reach of the caller for card bulk jobs (TEC-596): only owners and
// admins of the active account, the role the single toggle-lock needs, and they see every card
// job of that account.
func cardsAccess(p authctx.Principal) (bulkusecase.JobAccess, bool) {
	if p.Account.ID <= 0 || !p.Account.CanWrite() {
		return bulkusecase.JobAccess{}, false
	}
	return bulkusecase.JobAccess{AccountID: p.Account.ID, AnyActor: true}, true
}

const cardsRoleRequired = "Account owner or admin role required"

// ExecuteCards locks or unlocks cards of the active account. Runs under BULK_SYNC_MAX finish
// inline and answer 200 with the job; larger ones 202.
func (h *Handler) ExecuteCards(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	access, ok := cardsAccess(p)
	if !ok {
		response.Forbidden(w, r, cardsRoleRequired)
		return
	}
	var body companyBulkRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid JSON body")
		return
	}
	action := strings.TrimSpace(body.Action)
	def, _, err := h.svc.Registry().ActionDef(adapters.ResourceAppCards, action)
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "unknown bulk action")
		return
	}
	if !p.HasPermission(def.Permission) {
		response.Forbidden(w, r, "Missing permission for bulk action")
		return
	}
	result, err := h.svc.Execute(r.Context(), p.UserInternal, bulkusecase.ExecuteInput{
		Resource:  adapters.ResourceAppCards,
		Action:    action,
		Target:    body.Target,
		Locale:    body.Locale,
		AccountID: access.AccountID,
		Params:    body.Params,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if result.Async {
		response.JSON(w, r, http.StatusAccepted, result.Job)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]any{
		"sync":    true,
		"summary": result.Summary,
		"job":     result.Job,
	})
}

// CardsList lists card bulk jobs of the active account.
func (h *Handler) CardsList(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	access, ok := cardsAccess(p)
	if !ok {
		response.Forbidden(w, r, cardsRoleRequired)
		return
	}
	q := apiquery.Parse(r.URL.Query())
	items, total, err := h.svc.ListAccountJobs(r.Context(), p.UserInternal, access, adapters.ResourceAppCards, q.Limit, q.Offset)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, apiquery.NewPage(items, total, q.Limit, q.Offset))
}

// cardsJob loads a card bulk job of the active account; jobs of other resources are not found.
func (h *Handler) cardsJob(w http.ResponseWriter, r *http.Request) (bulkusecase.BulkJobView, bulkusecase.JobAccess, bool) {
	p := authctx.MustPrincipal(r.Context())
	access, ok := cardsAccess(p)
	if !ok {
		response.Forbidden(w, r, cardsRoleRequired)
		return bulkusecase.BulkJobView{}, access, false
	}
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "uuid is invalid")
		return bulkusecase.BulkJobView{}, access, false
	}
	job, err := h.svc.GetAccountJob(r.Context(), id, p.UserInternal, access)
	if err != nil {
		writeErr(w, r, err)
		return bulkusecase.BulkJobView{}, access, false
	}
	if job.Resource != adapters.ResourceAppCards {
		response.NotFound(w, r, "Bulk job not found")
		return bulkusecase.BulkJobView{}, access, false
	}
	return job, access, true
}

// CardsGet returns one card bulk job of the active account.
func (h *Handler) CardsGet(w http.ResponseWriter, r *http.Request) {
	job, _, ok := h.cardsJob(w, r)
	if !ok {
		return
	}
	response.JSON(w, r, http.StatusOK, job)
}

// CardsRollback restores the lock state a card bulk job changed, within its window.
func (h *Handler) CardsRollback(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	job, access, ok := h.cardsJob(w, r)
	if !ok {
		return
	}
	def, _, err := h.svc.Registry().ActionDef(job.Resource, job.Action)
	if err != nil || !p.HasPermission(def.Permission) {
		response.Forbidden(w, r, "Missing permission for bulk rollback")
		return
	}
	updated, err := h.svc.Rollback(r.Context(), job.UUID, p.UserInternal, access)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, updated)
}
