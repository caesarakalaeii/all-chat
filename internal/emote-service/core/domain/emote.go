package domain

type Emote struct {
	Code     string
	Provider string // "twitch", "7tv", "bttv", "ffz"
	URL      string
	Animated bool
	Channel  string // Empty for global emotes
}

type EmoteResponse struct {
	Code     string `json:"code"`
	Provider string `json:"provider"`
	URL      string `json:"url"`
	Animated bool   `json:"animated"`
}

// 7TV API response structures
type SevenTVEmote struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Data struct {
		Animated bool `json:"animated"`
		Host     struct {
			URL string `json:"url"`
		} `json:"host"`
	} `json:"data"`
}

type SevenTVGlobalResponse struct {
	Emotes []SevenTVEmote `json:"emotes"`
}

type SevenTVChannelResponse struct {
	EmoteSet struct {
		Emotes []SevenTVEmote `json:"emotes"`
	} `json:"emote_set"`
}

// BTTV API response structures
type BTTVEmote struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	ImageType string `json:"imageType"`
}

type BTTVGlobalResponse []BTTVEmote

type BTTVChannelResponse struct {
	ChannelEmotes []BTTVEmote `json:"channelEmotes"`
	SharedEmotes  []BTTVEmote `json:"sharedEmotes"`
}

// FFZ API response structures
type FFZEmote struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	URLs struct {
		One string `json:"1"`
		Two string `json:"2,omitempty"`
	} `json:"urls"`
}

type FFZSet struct {
	Emoticons []FFZEmote `json:"emoticons"`
}

type FFZGlobalResponse struct {
	Sets map[string]FFZSet `json:"sets"`
}

type FFZChannelResponse struct {
	Sets map[string]FFZSet `json:"sets"`
}
