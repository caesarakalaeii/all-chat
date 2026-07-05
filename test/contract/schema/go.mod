module github.com/caesar/all-chat/test/contract

go 1.23

require (
	github.com/caesar/all-chat/test/shared v0.0.0
	github.com/redis/go-redis/v9 v9.6.3
	github.com/sebdah/goldie/v2 v2.5.3
	github.com/stretchr/testify v1.8.4
	go.uber.org/zap v1.24.0
)

replace github.com/caesar/all-chat/test/shared => ../../shared

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/nsf/jsondiff v0.0.0-20210926074059-1e845ec5d249 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sergi/go-diff v1.0.0 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
