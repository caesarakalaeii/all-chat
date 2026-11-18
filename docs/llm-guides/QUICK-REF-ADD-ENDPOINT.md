# Quick Reference: Add HTTP Endpoint

Goal: add a new HTTP endpoint to an existing Go service.

## Steps

1. **Pick the service** that owns the functionality and a RESTful route, e.g. `GET /api/v1/<resource>/:id`.

2. **Handler** in `services/<service>/handlers/<feature>.go`: a struct holding the repository (and logger), a constructor, and a Gin handler function. Validate input, map not-found to 404 and errors to 500:

```go
func (h *FeatureHandler) GetItem(c *gin.Context) {
    item, err := h.repo.GetByID(c.Request.Context(), c.Param("id"))
    if err != nil {
        h.logger.Error("get item failed", zap.Error(err))
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
        return
    }
    if item == nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
        return
    }
    c.JSON(http.StatusOK, item)
}
```

3. **Register the route** where the router is set up in `services/<service>/cmd/main.go`, under the existing `/api/v1` group.

4. **Auth**: wrap protected routes with `shared/middleware` auth middleware, e.g. `middleware.JWTAuthWithRevocation(userKeyChain, redisClient)` or `middleware.ServiceJWTAuth(...)` for service-to-service routes. Health endpoints stay unauthenticated. See `services/auth-service/cmd/main.go` for real examples.

5. **Validation**: bind JSON body with `ShouldBindJSON` and Gin binding tags (`binding:"required,min=1,max=255"`).

6. **Tests** in `handlers/<feature>_test.go`: use `gin.SetMode(gin.TestMode)`, `httptest.NewRequest`/`httptest.NewRecorder`, cover 200, 400, 404, 401, 500. Run `go test ./... -v` in the service directory (`make test` runs all services).

7. **Check manually**: `go run ./cmd` in the service directory, then curl the endpoint.

8. **Document** the endpoint in `services/<service>/README.md`.
