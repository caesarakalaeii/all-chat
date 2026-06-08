module github.com/caesar/all-chat/test/youtube-stream-test

go 1.25.6

replace github.com/caesar/all-chat/services/youtube-listener => ../../services/youtube-listener

require (
	github.com/caesar/all-chat/services/youtube-listener v0.0.0
	google.golang.org/grpc v1.81.1
	google.golang.org/protobuf v1.36.11
)

require (
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
)
