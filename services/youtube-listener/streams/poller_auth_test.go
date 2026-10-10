// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package streams

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/api"
	"github.com/caesar/all-chat/services/youtube-listener/models"
	"github.com/caesar/all-chat/shared/youtubeclaim"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	grpcoauth "google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/grpc/testdata"
)

// newHTTPAPIClient returns an api.Client without a gRPC client, so chat streaming takes the
// HTTP path against a server that answers every request with status and body.
func newHTTPAPIClient(t *testing.T, statusCode int, body string) *api.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	service, err := youtube.NewService(context.Background(),
		option.WithEndpoint(srv.URL+"/"), option.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("youtube.NewService: %v", err)
	}
	return api.NewClient(service, srv.Client(), nil, zap.NewNop())
}

const unauthorizedBody = `{"error":{"code":401,"message":"Invalid Credentials","errors":[{"reason":"authError","message":"Invalid Credentials"}]}}`

// A running poller is the only API caller for its channel: syncStreams skips discovery while it
// runs and the stream-state fast path starts it without a call. So a token revoked mid-stream
// has to be handed back from the poller itself.
func TestPollerAuthError_ReleasesClaimAndStopsPoller(t *testing.T) {
	verifier := &fakeVerifier{}
	m, mr := newClaimsManager(t, &fakeEligibility{}, verifier)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	m.redisClient = rc
	m.pollers = make(map[string]*Poller)
	m.activeStreams = make(map[string]*models.YouTubeStream)
	m.channelConnectedOverlays = map[string]map[string]struct{}{"UCowned": {"ov1": {}}}
	if err := mr.Set("overlay:connected:ov1", "1"); err != nil {
		t.Fatal(err)
	}
	m.claimed["UCowned"] = "user-1"
	if err := mr.Set(youtubeclaim.ClaimKey("UCowned"), "user-1"); err != nil {
		t.Fatal(err)
	}

	stream := &models.YouTubeStream{StreamID: "vid1", ChannelID: "UCowned", LiveChatID: "chat1", OverlayID: "ov1", IsLive: true}
	poller := m.newPoller(stream, newHTTPAPIClient(t, http.StatusUnauthorized, unauthorizedBody), "user-1")
	m.mu.Lock()
	m.pollers[stream.StreamID] = poller
	m.activeStreams[stream.StreamID] = stream
	m.mu.Unlock()
	t.Cleanup(func() {
		select {
		case <-poller.IsDone():
		default:
			poller.Stop()
		}
	})
	if err := poller.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for mr.Exists(youtubeclaim.ClaimKey("UCowned")) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if mr.Exists(youtubeclaim.ClaimKey("UCowned")) {
		t.Fatal("a 401 while streaming must release the claim so innertube takes the channel over")
	}

	select {
	case <-poller.IsDone():
	case <-time.After(time.Second):
		t.Fatal("poller must stop instead of retrying a rejected token")
	}
	m.mu.RLock()
	_, stillTracked := m.pollers[stream.StreamID]
	m.mu.RUnlock()
	if stillTracked {
		t.Fatal("rejected poller must leave the poller map")
	}
	if _, ok := m.claimedOwner("UCowned"); ok {
		t.Fatal("channel must leave the claimed set")
	}
	verifier.mu.Lock()
	forgotten := append([]string(nil), verifier.forgotten...)
	verifier.mu.Unlock()
	if len(forgotten) != 1 || forgotten[0] != "user-1|UCowned" {
		t.Fatalf("forgotten verdicts = %v, want [user-1|UCowned]", forgotten)
	}
}

// liveChatEnded comes back as a 403 too; it ends the stream, not the owner's token.
func TestPollerAuthError_ChatEndedKeepsClaim(t *testing.T) {
	ended := `{"error":{"code":403,"message":"The live chat is no longer live.","errors":[{"reason":"liveChatEnded","message":"The live chat is no longer live."}]}}`
	err := pollOnce(t, newHTTPAPIClient(t, http.StatusForbidden, ended))
	if err == nil {
		t.Fatal("expected a poll error")
	}
	if pollerOwnerRejected(err) {
		t.Fatalf("pollerOwnerRejected(%v) = true, want false for an ended chat", err)
	}
}

func TestPollerAuthError_HTTP401IsOwnerRejection(t *testing.T) {
	err := pollOnce(t, newHTTPAPIClient(t, http.StatusUnauthorized, unauthorizedBody))
	if !pollerOwnerRejected(err) {
		t.Fatalf("pollerOwnerRejected(%v) = false, want true", err)
	}
}

func pollOnce(t *testing.T, client *api.Client) error {
	t.Helper()
	p := NewPoller(&models.YouTubeStream{StreamID: "vid1", ChannelID: "UCowned", LiveChatID: "chat1"}, client, nil, zap.NewNop(), nil, nil)
	_, err := p.poll(context.Background(), "")
	return err
}

type failingTokenSource struct{ err error }

func (f failingTokenSource) Token() (*oauth2.Token, error) { return nil, f.err }

// grpcCallWithToken makes one RPC over TLS with ts as the per-RPC credentials, which is how
// syncChannel hands the owner token to the gRPC chat stream, and returns its error.
func grpcCallWithToken(t *testing.T, ts oauth2.TokenSource) error {
	t.Helper()
	serverCreds, err := credentials.NewServerTLSFromFile(testdata.Path("x509/server1_cert.pem"), testdata.Path("x509/server1_key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer(grpc.Creds(serverCreds))
	grpc_health_v1.RegisterHealthServer(srv, health.NewServer())
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	clientCreds, err := credentials.NewClientTLSFromFile(testdata.Path("x509/server_ca_cert.pem"), "x.test.example.com")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(clientCreds),
		grpc.WithPerRPCCredentials(grpcoauth.TokenSource{TokenSource: ts}),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	return err
}

// grpc-go reports any non-status per-RPC credentials error as Unauthenticated, so without
// ownerTokenSource a token-endpoint outage would look like a revoked grant and bounce the channel.
func TestPollerAuthError_GRPCTokenRefreshClassification(t *testing.T) {
	revoked := &oauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusBadRequest}, ErrorCode: "invalid_grant"}
	endpointDown := &oauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusServiceUnavailable}}

	for _, tc := range []struct {
		name  string
		cause error
		want  bool
	}{
		{"invalid_grant", revoked, true},
		{"token_endpoint_503", endpointDown, false},
		{"network", errors.New("dial tcp: connection refused"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := grpcCallWithToken(t, ownerTokenSource{failingTokenSource{tc.cause}})
			if err == nil {
				t.Fatal("expected the RPC to fail on the token source")
			}
			if got := isOwnerAuthError(err); got != tc.want {
				t.Fatalf("isOwnerAuthError(%v) = %v, want %v", err, got, tc.want)
			}
		})
	}
}
