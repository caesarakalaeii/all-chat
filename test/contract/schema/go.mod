module github.com/caesar/all-chat/test/contract

go 1.24

require (
	github.com/caesar/all-chat/test/shared v0.0.0
	github.com/redis/go-redis/v9 v9.20.0
	github.com/sebdah/goldie/v2 v2.5.3
	github.com/stretchr/testify v1.8.4
	go.uber.org/zap v1.28.0
)

replace github.com/caesar/all-chat/test/shared => ../../shared

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/nsf/jsondiff v0.0.0-20210926074059-1e845ec5d249 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sergi/go-diff v1.0.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
