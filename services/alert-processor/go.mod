module github.com/caesar/all-chat/services/alert-processor

go 1.25.6

replace github.com/caesar/all-chat/shared => ../../shared

replace github.com/caesar/all-chat/services/message-processor => ../message-processor

require (
	github.com/alicebob/miniredis/v2 v2.39.0
	github.com/caesar/all-chat/services/message-processor v0.0.0-00010101000000-000000000000
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.11.0
	github.com/redis/go-redis/v9 v9.22.0
	github.com/stretchr/testify v1.12.1
	go.uber.org/zap v1.28.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
)
