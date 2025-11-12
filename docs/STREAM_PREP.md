# 🎥 Stream Preparation - Overlay Manager Service

**Date**: Ready for your stream
**Duration**: 3 hours
**Difficulty**: ⭐⭐⭐ Intermediate (but achievable!)

---

## ✅ What's Already Done For You

### 1. All Tests Pre-Written (TDD Style)
- ✅ `models/overlay_test.go` - 8 test cases
- ✅ `models/chat_source_test.go` - 13 test cases
- ✅ `repository/overlay_repo_test.go` - 12 test cases (with Testcontainers!)
- ✅ `handlers/overlay_test.go` - 12 test cases
- ✅ `handlers/health_test.go` - 3 test cases

**Total: 48 tests waiting for you to make them pass!**

### 2. Infrastructure Ready
- ✅ Go workspace configured
- ✅ Shared packages ready (logger, database, redis, auth)
- ✅ Docker Compose configured
- ✅ Database migrations ready
- ✅ Makefile with helpful commands
- ✅ Auth Service complete and tested

### 3. Documentation
- ✅ `STREAM_GUIDE.md` - Hour-by-hour breakdown
- ✅ Architecture docs updated
- ✅ All decisions documented

---

## 🎯 Your Task: Implement Code to Pass Tests

You need to create **5 implementation files**:

1. ⏱️ **models/overlay.go** (15-20 min)
   - Simple struct with Validate() method
   - ~50 lines of code

2. ⏱️ **models/chat_source.go** (10-15 min)
   - Struct with Validate() and IsValidPlatform()
   - ~60 lines of code

3. ⏱️ **repository/overlay_repo.go** (40-50 min)
   - PostgreSQL CRUD operations
   - ~200 lines of code
   - Use auth-service repository as reference!

4. ⏱️ **handlers/overlay.go** (45-60 min)
   - 5 HTTP handlers (Create, List, Get, Update, Delete)
   - ~250 lines of code
   - Use auth-service handlers as reference!

5. ⏱️ **handlers/health.go** (10 min)
   - Simple health checks
   - ~40 lines of code

6. ⏱️ **cmd/main.go** (20-30 min)
   - Wire everything together
   - ~150 lines of code
   - Copy from auth-service and modify!

**Total: ~750 lines of code to write**

---

## 📋 Pre-Stream Checklist

### Environment Setup (Do NOW, before stream)

```bash
# 1. Verify Go installation
go version  # Should be 1.24.0 or higher

# 2. Verify Docker is running
docker ps

# 3. Start infrastructure
cd /home/caesar/git/all-chat
make docker-up

# 4. Check services are healthy
docker ps  # Should see postgres and redis

# 5. Run migrations
make migrate

# 6. Verify Auth Service works
curl http://localhost:8081/health/live
# Should return: {"status":"alive"}

# 7. Test database connection
PGPASSWORD=allchat_dev_password psql -h localhost -U allchat -d allchat -c "\dt"
# Should list tables: users, overlays, overlay_configs, etc.

# 8. Download dependencies
cd services/overlay-manager
go mod download
go mod tidy

# 9. Verify tests exist and FAIL (they should!)
go test ./...
# Should see failures - GOOD! That's TDD!
```

### IDE Setup
- [ ] Open `services/overlay-manager/` in your IDE
- [ ] Have `services/auth-service/` open in split view (for reference)
- [ ] Terminal ready for test running
- [ ] `STREAM_GUIDE.md` open for reference

### Stream Tools
- [ ] OBS configured
- [ ] Terminal visible (large font)
- [ ] IDE visible (zoom in on code)
- [ ] Browser ready for manual testing

---

## 🎬 Stream Outline

### Intro (5 min)
```
"Today we're building the Overlay Manager microservice for All-Chat.
All tests are already written - I just need to make them pass!
This is Test-Driven Development (TDD) in Go."

- Show architecture diagram (8 microservices)
- Explain: Building service #2 of 8
- Show test files (already written)
- Run tests: watch them FAIL (red)
```

### Hour 1: Models (55 min)
```
"Let's start with the domain models - Overlay and ChatSource"

1. Create models/overlay.go (show overlay_test.go first)
2. Implement Validate() method
3. Run tests: go test ./models/...
4. Watch tests turn GREEN ✅
5. Repeat for chat_source.go
6. Celebrate first wins!
```

### Hour 2: Repository (60 min)
```
"Now the database layer - this is where Testcontainers shine!"

1. Show repository_test.go (uses real PostgreSQL!)
2. Create repository/overlay_repo.go
3. Implement CRUD methods
4. Run tests: watch Docker pull PostgreSQL image
5. Tests run against REAL database
6. All GREEN ✅
7. Explain: "This is production-quality testing!"
```

### Hour 3: Handlers + Integration (60 min)
```
"HTTP handlers - connecting everything together"

1. Implement handlers/overlay.go (5 endpoints)
2. Implement handlers/health.go
3. Wire everything in cmd/main.go
4. Build and run: go run ./cmd
5. Manual testing with curl/Postman
6. Full flow: Login → Create Overlay → List → Update → Delete
7. VICTORY MOMENT! 🎉
```

### Wrap-up (5 min)
```
- Show final test coverage: go test -cover ./...
- Commit to Git
- Preview Phase 2: "Next time we build API Gateway!"
- Thank viewers
```

---

## 🎓 Learning Objectives (What Viewers Learn)

1. **Test-Driven Development** - Write tests first, code second
2. **Go patterns** - Structs, interfaces, error handling
3. **Table-driven tests** - Go testing best practice
4. **Testcontainers** - Integration testing with real databases
5. **Microservices** - Service communication, JWT auth
6. **PostgreSQL** - CRUD operations with pgx
7. **Gin framework** - HTTP handlers, middleware, routing
8. **Production patterns** - Health checks, graceful shutdown

---

## 🚦 Traffic Light System (Use During Stream)

### 🔴 RED Phase
"Tests are failing - this is EXPECTED in TDD!"
- Show failing tests
- Read error messages
- Plan implementation

### 🟢 GREEN Phase
"Make tests pass - minimal code first!"
- Implement just enough to pass
- Run tests frequently
- Celebrate each GREEN ✅

### 🔵 REFACTOR Phase
"Tests pass - now make it better!"
- Clean up code
- Add comments
- Extract functions if needed
- Tests still GREEN ✅

---

## 📞 Help Resources (If Stuck)

### Quick References
- Auth Service code: `services/auth-service/` (working example)
- pgx docs: https://pkg.go.dev/github.com/jackc/pgx/v5
- Gin docs: https://gin-gonic.com/docs/
- Test examples: All `*_test.go` files have patterns

### Common Patterns to Copy

**Gin handler pattern**:
```go
func (h *Handler) HandleSomething(c *gin.Context) {
    // 1. Extract user_id from context
    userID, exists := c.Get("user_id")
    if !exists {
        c.JSON(401, gin.H{"error": "unauthorized"})
        return
    }

    // 2. Parse request
    var req SomeRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "invalid request"})
        return
    }

    // 3. Call repository
    result, err := h.repo.DoSomething(c.Request.Context(), ...)
    if err != nil {
        h.logger.Error("Operation failed", zap.Error(err))
        c.JSON(500, gin.H{"error": "internal error"})
        return
    }

    // 4. Return response
    c.JSON(200, result)
}
```

**pgx query pattern**:
```go
func (r *Repo) GetByID(ctx context.Context, id string) (*Model, error) {
    var model Model
    err := r.pool.QueryRow(ctx,
        "SELECT id, field1, field2 FROM table WHERE id = $1",
        id,
    ).Scan(&model.ID, &model.Field1, &model.Field2)

    if err != nil {
        if err == pgx.ErrNoRows {
            return nil, ErrNotFound
        }
        return nil, err
    }

    return &model, nil
}
```

---

## ⏰ Time Tracking Template

Use this during stream to stay on track:

```
[00:00] Stream start
[00:05] Intro complete
[00:30] Overlay model done ✅
[00:45] ChatSource model done ✅
[01:30] Repository done ✅
[02:30] Handlers done ✅
[02:45] Main.go done ✅
[03:00] E2E test success! 🎉
```

If behind schedule:
- Skip ChatSource model (do it off-stream)
- Use more copy-paste from auth-service
- Skip some bonus features

---

## 🎉 Victory Conditions

### Minimum (Must achieve):
- ✅ Can create an overlay via API
- ✅ Can list overlays
- ✅ Tests pass for models + repository

### Target (Aim for this):
- ✅ All tests passing
- ✅ Full CRUD working
- ✅ Coverage ≥ 80%

### Stretch (If you're fast):
- ✅ Chat source management
- ✅ Config management
- ✅ Kubernetes deployment

---

**You've got this! The tests guide you step-by-step.** 💪

See `STREAM_GUIDE.md` for detailed hour-by-hour breakdown.
