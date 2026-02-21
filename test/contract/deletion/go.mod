module github.com/caesar/all-chat/test/contract/deletion

go 1.25.7

require (
	github.com/caesar/all-chat/services/youtube-listener-innertube v0.0.0
	github.com/stretchr/testify v1.11.1
)

replace github.com/caesar/all-chat/services/youtube-listener-innertube => ../../../services/youtube-listener-innertube

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
	golang.org/x/net v0.49.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
