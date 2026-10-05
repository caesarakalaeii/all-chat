module github.com/caesar/all-chat/services/alert-processor

go 1.25.6

replace github.com/caesar/all-chat/shared => ../../shared

replace github.com/caesar/all-chat/services/message-processor => ../message-processor

require (
	github.com/caesar/all-chat/services/message-processor v0.0.0-00010101000000-000000000000
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.11.0
	github.com/stretchr/testify v1.12.1
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/text v0.41.0 // indirect
)
