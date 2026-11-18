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

package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/caesar/all-chat/services/engagement-service/announcer"
	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// sendFailureTTL drops a streamer's failures after a week without any bot post.
const sendFailureTTL = 7 * 24 * time.Hour

func sendFailuresKey(owner string) string { return "chatcmd:sendfail:" + owner }

// SendStatus wraps the bot sender that command replies and timer posts both go
// through, and remembers per streamer and platform the last post that was not
// delivered (cleared by the next delivered one), so the commands editor can say
// why replies are missing instead of failing silently.
type SendStatus struct {
	chatOutput
	rdb *redis.Client
	log *zap.Logger
}

// NewSendStatus wraps out.
func NewSendStatus(rdb *redis.Client, out chatOutput, log *zap.Logger) *SendStatus {
	return &SendStatus{chatOutput: out, rdb: rdb, log: log}
}

// BotSend posts through the wrapped sender and records the outcome. The error is
// returned unchanged.
func (s *SendStatus) BotSend(ctx context.Context, userID, message, platform string, purpose announcer.SendPurpose) error {
	err := s.chatOutput.BotSend(ctx, userID, message, platform, purpose)
	key := sendFailuresKey(userID)
	if err == nil {
		if derr := s.rdb.HDel(ctx, key, platform).Err(); derr != nil {
			s.log.Debug("clear bot send failure", zap.Error(derr))
		}
		return nil
	}
	f := models.SendFailure{Platform: platform, ErrorKind: "send_failed", At: time.Now().UTC()}
	var notDelivered *announcer.BotSendError
	if errors.As(err, &notDelivered) {
		f.ErrorKind = "not_connected" // auth-service found no identity to post with
		for _, r := range notDelivered.Results {
			if r.Platform == platform {
				f.Sender, f.ErrorKind, f.Detail = r.Sender, r.ErrorKind, r.Detail
			}
		}
	}
	payload, _ := json.Marshal(f)
	pipe := s.rdb.Pipeline()
	pipe.HSet(ctx, key, platform, payload)
	pipe.Expire(ctx, key, sendFailureTTL)
	if _, perr := pipe.Exec(ctx); perr != nil {
		s.log.Warn("record bot send failure", zap.String("owner", userID), zap.Error(perr))
	}
	return err
}

// Failures returns the streamer's remembered failures, one per platform.
func (s *SendStatus) Failures(ctx context.Context, owner uuid.UUID) ([]models.SendFailure, error) {
	raw, err := s.rdb.HGetAll(ctx, sendFailuresKey(owner.String())).Result()
	if err != nil {
		return nil, err
	}
	out := make([]models.SendFailure, 0, len(raw))
	for _, v := range raw {
		var f models.SendFailure
		if json.Unmarshal([]byte(v), &f) == nil {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Platform < out[j].Platform })
	return out, nil
}
