package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/caesar/all-chat/shared/middleware"
)

// These tests pin the session-only guard on whole-account surfaces.
//
// A personal access token is a long-lived credential whose scopes (chat:write,
// engagement:write) bound what it may do. Surfaces that ignore scopes and act on the
// ENTIRE account must refuse a PAT outright, otherwise a leaked chat token becomes an
// account-destruction or full-PII-disclosure credential. Scope checks are the wrong
// tool here: there is no scope that should unlock account deletion.

// patCtx is a request context as the resolver populates it for a PAT bearer, carrying
// the narrowest scope a plugin would realistically hold.
func patCtx() map[string]string {
	return map[string]string{
		"user_id":                    testUserID,
		middleware.CtxAuthMethod:     middleware.AuthMethodAPIToken,
		middleware.CtxAPITokenID:     "11111111-1111-1111-1111-111111111111",
		middleware.CtxAPITokenScopes: middleware.ScopeChatWrite,
	}
}

// injectCtx applies the given context values before the handler runs, standing in for
// the auth middleware that would have set them.
func injectCtx(values map[string]string) gin.HandlerFunc {
	return func(c *gin.Context) {
		for k, v := range values {
			c.Set(k, v)
		}
		c.Next()
	}
}

// RefuseAPIToken is the single guard both surfaces rely on, so its contract is pinned
// directly: a PAT is refused with 403, and every other auth method passes through.
func TestRefuseAPIToken_RefusesOnlyPATs(t *testing.T) {
	cases := []struct {
		name       string
		authMethod string
		wantPass   bool
	}{
		{"personal access token", middleware.AuthMethodAPIToken, false},
		{"session jwt", middleware.AuthMethodJWT, true},
		// An unset auth method must not be mistaken for a PAT: the JWT path predates
		// this key, so a missing value has to keep behaving exactly as before.
		{"unset auth method", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodDelete, "/me", nil)
			if tc.authMethod != "" {
				c.Set(middleware.CtxAuthMethod, tc.authMethod)
			}

			if got := RefuseAPIToken(c, "nope"); got != tc.wantPass {
				t.Fatalf("RefuseAPIToken = %v, want %v", got, tc.wantPass)
			}
			if !tc.wantPass && w.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", w.Code)
			}
		})
	}
}

// A PAT must never delete the account. The critical assertion is that the refusal
// happens BEFORE any persistence: a 403 returned after the row was already deleted
// would be worthless, and the deletion is irreversible plus cascades every overlay.
//
// h.userRepo is nil here deliberately. If the guard ever regresses, the handler
// proceeds to h.userRepo.Delete and panics, failing this test loudly rather than
// silently passing on a status code.
func TestHandleDeleteAccount_RefusesPAT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := &AuthHandler{logger: zap.NewNop()}
	router.DELETE("/me", injectCtx(patCtx()), h.HandleDeleteAccount)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/me", nil))

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", w.Code, w.Body.String())
	}
}

// The data export is a full PII dump (identities, overlays, viewers, guilds, messages,
// delegations). Same shape as deletion: refused before any read, with a nil repo so a
// regression panics instead of quietly disclosing.
func TestHandleDataExport_RefusesPAT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := &AuthHandler{logger: zap.NewNop()}
	router.GET("/me/data-export", injectCtx(patCtx()), h.HandleDataExport)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/me/data-export", nil))

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), testUserID) {
		t.Fatalf("refusal body leaked user data: %s", w.Body.String())
	}
}
