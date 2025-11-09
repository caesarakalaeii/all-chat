package twitch

import (
	"strings"

	"github.com/gempir/go-twitch-irc/v4"
)

// IRCClient wraps the go-twitch-irc client
type IRCClient struct {
	client   *twitch.Client
	username string
	oauth    string
}

// NewIRCClient creates a new Twitch IRC client
func NewIRCClient(username, oauth string) *IRCClient {
	client := twitch.NewClient(username, oauth)
	return &IRCClient{
		client:   client,
		username: username,
		oauth:    oauth,
	}
}

// Connect establishes connection to Twitch IRC
func (c *IRCClient) Connect() error {
	return c.client.Connect()
}

// Disconnect closes the connection
func (c *IRCClient) Disconnect() error {
	return c.client.Disconnect()
}

// Join joins the specified channels
func (c *IRCClient) Join(channels ...string) {
	c.client.Join(channels...)
}

// Part leaves the specified channels
func (c *IRCClient) Part(channels ...string) {
	for _, channel := range channels {
		c.client.Depart(channel)
	}
}

// OnMessage registers a callback for incoming messages
func (c *IRCClient) OnMessage(callback func(channel, user, message string, tags map[string]string)) {
	c.client.OnPrivateMessage(func(msg twitch.PrivateMessage) {
		// Normalize channel name (remove # prefix if present)
		channel := strings.TrimPrefix(msg.Channel, "#")

		// Convert tags to map
		tags := make(map[string]string)
		tags["user-id"] = msg.User.ID
		tags["display-name"] = msg.User.DisplayName
		tags["color"] = msg.User.Color

		// Parse badges
		if len(msg.User.Badges) > 0 {
			badges := make([]string, 0, len(msg.User.Badges))
			for badge := range msg.User.Badges {
				badges = append(badges, badge)
			}
			tags["badges"] = strings.Join(badges, ",")
		}

		callback(channel, msg.User.Name, msg.Message, tags)
	})
}
