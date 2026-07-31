module github.com/caesar/all-chat/test/youtube-stream-test

go 1.23

replace github.com/caesar/all-chat => ../..

require (
	github.com/caesar/all-chat v0.0.0
	google.golang.org/grpc v1.69.4
	google.golang.org/protobuf v1.36.5
)
