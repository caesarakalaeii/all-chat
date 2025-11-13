module github.com/caesar/all-chat/services/auth-service

go 1.23

require (
	github.com/caesar/all-chat/shared v0.0.0
	github.com/gin-gonic/gin v1.10.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.7.1
	github.com/redis/go-redis/v9 v9.7.0
	go.uber.org/zap v1.27.0
	golang.org/x/oauth2 v0.24.0
)

replace github.com/caesar/all-chat/shared => ../../shared
