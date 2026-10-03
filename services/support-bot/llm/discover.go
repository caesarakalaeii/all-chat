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

package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
)

// DiscoverModel resolves the model id at boot against the gateway's model list,
// so an upstream model swap needs no manifest edit and no rollout. The pin wins
// while it is still served; discovery only picks a replacement when it is not.
//
// Resolution order:
//  1. pin set AND listed          -> pin
//  2. pin set but NOT listed      -> warn, fall through to discovery
//  3. exactly 1 model             -> it; >1 -> lexicographically smallest, warn
//  4. list fetch fails            -> pin set: warn, use pin; no pin: error
//
// Only the model id is resolved here; contextWindow/maxTokens stay pinned config.
func DiscoverModel(ctx context.Context, cfg Config, log *zap.Logger) (string, error) {
	models, err := listModels(ctx, cfg)
	if err != nil {
		if cfg.Model != "" {
			log.Warn("model list fetch failed; keeping pinned model (gateway may still serve it)",
				zap.String("pinned", cfg.Model), zap.String("url", modelsURL(cfg.BaseURL)), zap.Error(err))
			return cfg.Model, nil
		}
		return "", fmt.Errorf("llm: model discovery failed and no LOCAL_LLM_MODEL pin set (tried %s): %w", modelsURL(cfg.BaseURL), err)
	}

	ids := make([]string, 0, len(models))
	for _, m := range models {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}

	if cfg.Model != "" {
		for _, id := range ids {
			if id == cfg.Model {
				return cfg.Model, nil
			}
		}
		log.Warn("pinned model no longer listed by gateway; discovering replacement",
			zap.String("pinned", cfg.Model), zap.Strings("available", ids))
	}

	switch {
	case len(ids) == 1:
		return ids[0], nil
	case len(ids) > 1:
		sort.Strings(ids)
		log.Warn("model discovery ambiguous, picking lexicographically smallest id",
			zap.Strings("candidates", ids), zap.String("picked", ids[0]))
		return ids[0], nil
	default:
		if cfg.Model != "" {
			return "", fmt.Errorf("llm: pinned model %q not listed and gateway list is empty", cfg.Model)
		}
		return "", fmt.Errorf("llm: gateway lists no models (tried %s)", modelsURL(cfg.BaseURL))
	}
}

// listModels fetches GET <BaseURL>/v1/models and returns the listed model ids.
// The endpoint is unauthenticated on the gateway, but the optional API key is
// sent when set, mirroring the chat client.
func listModels(ctx context.Context, cfg Config) ([]modelEntry, error) {
	modelsCtx, cancel := context.WithTimeout(ctx, discoverTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(modelsCtx, http.MethodGet, modelsURL(cfg.BaseURL), nil)
	if err != nil {
		return nil, err
	}
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	// A bare default client is fine here: this runs once at boot, and the
	// client's redirect guard belongs to the credential-bearing chat POST.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: unexpected status %s", modelsURL(cfg.BaseURL), resp.Status)
	}

	var list struct {
		Data []modelEntry `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("decoding model list from %s: %w", modelsURL(cfg.BaseURL), err)
	}
	return list.Data, nil
}

func modelsURL(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/v1/models"
}

type modelEntry struct {
	ID string `json:"id"`
}

// discoverTimeout bounds the whole boot-time discovery round-trip. Boot only:
// it must never wedge startup the way the chat timeout tolerates a slow model.
const discoverTimeout = 10 * time.Second
