package identity

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
)

// Loader adapts the auth use case to middleware.IdentityLoader.
type Loader struct {
	UC *usecase.AuthUseCase
}

// LoadPrincipal hydrates the request principal; see usecase.AuthUseCase.LoadPrincipal.
func (l Loader) LoadPrincipal(r *http.Request, claims jwt.Claims) (authctx.Principal, error) {
	return l.UC.LoadPrincipal(r.Context(), claims)
}
