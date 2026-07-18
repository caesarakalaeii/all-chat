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

package clients

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"go.uber.org/zap"
)

// TestTwitchEmoteClient_ConcurrentFetchGlobal_NoRace hammers FetchEmotes from many
// goroutines against one shared client. The emote handler fans provider fetches out
// concurrently and reuses a single TwitchEmoteClient, so fetchGlobal ran concurrently
// and wrote a shared map with no lock -> `fatal error: concurrent map writes` crashed
// the pod under load (observed in prod 2026-07-17, emote-service exit code 2). This must
// run clean under -race.
func TestTwitchEmoteClient_ConcurrentFetchGlobal_NoRace(t *testing.T) {
	globalPayload := `{"template":"https://static-cdn.jtvnw.net/emoticons/v2/{id}/{format}/{theme_mode}/{scale}","data":[` +
		`{"id":"1","name":"Kappa","format":["static"],"theme_mode":["dark"],"scale":["1.0"]},` +
		`{"id":"2","name":"PogChamp","format":["static"],"theme_mode":["dark"],"scale":["1.0"]}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost { // app-token endpoint
			fmt.Fprint(w, `{"access_token":"tok","expires_in":3600}`)
			return
		}
		fmt.Fprint(w, globalPayload) // /helix/chat/emotes/global
	}))
	defer srv.Close()

	helix := NewTwitchClient("cid", "secret", zap.NewNop())
	helix.apiBase = srv.URL
	helix.tokenURL = srv.URL + "/oauth2/token"
	helix.usersURL = srv.URL + "/helix/users"

	client := NewTwitchEmoteClient(helix, zap.NewNop())

	const goroutines = 32
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			emotes, err := client.FetchEmotes(context.Background(), "global")
			if err != nil {
				t.Errorf("FetchEmotes: %v", err)
				return
			}
			if len(emotes) == 0 {
				t.Errorf("expected global emotes, got none")
			}
		}()
	}
	wg.Wait()
}
