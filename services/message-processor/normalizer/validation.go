package normalizer

import (
	"errors"
	"fmt"
	"regexp"
)

var channelIDPattern = regexp.MustCompile(`^[A-Za-z0-9_\-]+$`)

func validateChannelID(channelID string) error {
	if channelID == "" {
		return errors.New("channel ID cannot be empty")
	}
	if !channelIDPattern.MatchString(channelID) {
		return fmt.Errorf("channel ID contains unexpected characters: %q", channelID)
	}
	return nil
}
