module github.com/caesar/all-chat/services/event-collector

go 1.23

replace github.com/caesar/all-chat/shared => ../../shared

require (
	github.com/caesar/all-chat/shared v0.0.0-00010101000000-000000000000
	github.com/gin-gonic/gin v1.10.0
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.7.2
	github.com/redis/go-redis/v9 v9.7.0
	go.uber.org/zap v1.27.0
)
