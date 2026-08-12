module github.com/caesar/all-chat/test/contract/dual-listener

go 1.25.7

replace github.com/caesar/all-chat/test/shared => ../../shared

require (
	github.com/caesar/all-chat/test/shared v0.0.0-00010101000000-000000000000
	github.com/redis/go-redis/v9 v9.22.0
	go.uber.org/zap v1.27.1
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/nsf/jsondiff v0.0.0-20210926074059-1e845ec5d249 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
)
