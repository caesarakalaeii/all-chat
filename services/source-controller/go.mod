module github.com/caesar/all-chat/services/source-controller

go 1.25.3

require (
	github.com/caesar/all-chat/shared v0.0.0-00010101000000-000000000000
	github.com/gin-gonic/gin v1.11.0
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.7.6
	github.com/redis/go-redis/v9 v9.16.0
	go.uber.org/zap v1.27.0
)

replace github.com/caesar/all-chat/shared => ../../shared
