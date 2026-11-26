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

package discord

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/caesar/all-chat/services/support-bot/access"
	"github.com/caesar/all-chat/services/support-bot/config"
	"go.uber.org/zap"
)

// fakeChannel records what the sticky reposter did to one channel so a test can
// assert on posts and deletes without touching the Discord API.
type fakeChannel struct {
	mu       sync.Mutex
	existing []*discordgo.Message // newest-first, as Discord returns history
	posted   []string             // contents sent, in order
	deleted  []string             // message IDs deleted, in order
	nextID   int
	sendErr  error
	delErr   error
	// posts is signalled after every send so tests can wait instead of sleeping.
	posts chan struct{}
}

func newFakeChannel() *fakeChannel {
	return &fakeChannel{posts: make(chan struct{}, 16)}
}

func (f *fakeChannel) ChannelMessageSend(channelID, content string, _ ...discordgo.RequestOption) (*discordgo.Message, error) {
	f.mu.Lock()
	if f.sendErr != nil {
		err := f.sendErr
		f.mu.Unlock()
		return nil, err
	}
	f.nextID++
	msg := &discordgo.Message{ID: "sticky-" + strconv.Itoa(f.nextID), ChannelID: channelID, Content: content}
	f.posted = append(f.posted, content)
	f.mu.Unlock()
	f.posts <- struct{}{}
	return msg, nil
}

func (f *fakeChannel) ChannelMessageDelete(_, messageID string, _ ...discordgo.RequestOption) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.delErr != nil {
		return f.delErr
	}
	f.deleted = append(f.deleted, messageID)
	return nil
}

func (f *fakeChannel) ChannelMessages(_ string, _ int, _, _, _ string, _ ...discordgo.RequestOption) ([]*discordgo.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.existing, nil
}

func (f *fakeChannel) snapshot() (posted, deleted []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.posted...), append([]string(nil), f.deleted...)
}

// waitForPosts blocks until n sends have happened, failing the test on timeout.
func (f *fakeChannel) waitForPosts(t *testing.T, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-f.posts:
		case <-time.After(2 * time.Second):
			posted, _ := f.snapshot()
			t.Fatalf("timed out waiting for post %d of %d; got %d: %v", i+1, n, len(posted), posted)
		}
	}
}

const (
	testStickyChannel = "chan-1"
	testBotID         = "bot-42"
	testStickyText    = "Ping {bot} for help."
	testStickyGuild   = "guild-7"
)

func newTestSticky(t *testing.T, api stickyAPI, channelIDs []string, text string) *stickyReposter {
	t.Helper()
	return newStickyReposter(stickyDeps{
		API:        api,
		ChannelIDs: channelIDs,
		Text:       text,
		BotID:      func() string { return testBotID },
		Debounce:   20 * time.Millisecond,
		Log:        zap.NewNop(),
	})
}

// The reposter is off unless both the channel list and the text are configured,
// so emptying either env var turns the feature off without a code change.
func TestStickyDisabledWhenEitherSettingIsEmpty(t *testing.T) {
	cases := map[string]struct {
		channels []string
		text     string
	}{
		"no channels": {nil, testStickyText},
		"no text":     {[]string{testStickyChannel}, ""},
		"neither":     {nil, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fake := newFakeChannel()
			s := newTestSticky(t, fake, tc.channels, tc.text)
			if s.enabled() {
				t.Fatal("reposter reports enabled with an empty setting")
			}
			s.onMessage(testStickyChannel)
			time.Sleep(60 * time.Millisecond)
			posted, deleted := fake.snapshot()
			if len(posted) != 0 || len(deleted) != 0 {
				t.Fatalf("disabled reposter acted: posted=%v deleted=%v", posted, deleted)
			}
		})
	}
}

// One message in a configured channel posts the sticky exactly once, with {bot}
// rendered as a real Discord mention.
func TestStickyPostsOnceAndRendersBotMention(t *testing.T) {
	fake := newFakeChannel()
	s := newTestSticky(t, fake, []string{testStickyChannel}, testStickyText)

	s.onMessage(testStickyChannel)
	fake.waitForPosts(t, 1)

	posted, deleted := fake.snapshot()
	want := "Ping <@" + testBotID + "> for help."
	if len(posted) != 1 || posted[0] != want {
		t.Fatalf("posted = %v, want exactly [%q]", posted, want)
	}
	if len(deleted) != 0 {
		t.Fatalf("deleted = %v, want none on the first post", deleted)
	}
}

// A burst within the debounce window must collapse into a single repost: posting
// per message would hit Discord's rate limit and leave stickies behind.
func TestStickyBurstCollapsesIntoOnePost(t *testing.T) {
	fake := newFakeChannel()
	s := newTestSticky(t, fake, []string{testStickyChannel}, testStickyText)

	for i := 0; i < 10; i++ {
		s.onMessage(testStickyChannel)
	}
	fake.waitForPosts(t, 1)
	time.Sleep(60 * time.Millisecond)

	posted, deleted := fake.snapshot()
	if len(posted) != 1 {
		t.Fatalf("posted = %v, want exactly one message for a burst", posted)
	}
	if len(deleted) != 0 {
		t.Fatalf("deleted = %v, want none when there was no previous sticky", deleted)
	}
}

// The second cycle deletes the sticky posted by the first before posting again,
// so only the newest copy survives.
func TestStickyDeletesPreviousStickyOnNextCycle(t *testing.T) {
	fake := newFakeChannel()
	s := newTestSticky(t, fake, []string{testStickyChannel}, testStickyText)

	s.onMessage(testStickyChannel)
	fake.waitForPosts(t, 1)
	s.onMessage(testStickyChannel)
	fake.waitForPosts(t, 1)

	posted, deleted := fake.snapshot()
	if len(posted) != 2 {
		t.Fatalf("posted = %v, want two messages", posted)
	}
	if len(deleted) != 1 || deleted[0] != "sticky-1" {
		t.Fatalf("deleted = %v, want [sticky-1]", deleted)
	}
}

// A pod restart loses the in-memory ID, so the first cycle sweeps the recent
// history for the bot's own identical message and removes it.
func TestStickyCleansUpOrphanAfterRestart(t *testing.T) {
	fake := newFakeChannel()
	rendered := "Ping <@" + testBotID + "> for help."
	fake.existing = []*discordgo.Message{
		{ID: "other", Author: &discordgo.User{ID: "someone"}, Content: rendered},
		{ID: "orphan", Author: &discordgo.User{ID: testBotID}, Content: rendered},
		{ID: "bot-chatter", Author: &discordgo.User{ID: testBotID}, Content: "unrelated bot message"},
	}
	s := newTestSticky(t, fake, []string{testStickyChannel}, testStickyText)

	s.onMessage(testStickyChannel)
	fake.waitForPosts(t, 1)

	_, deleted := fake.snapshot()
	if len(deleted) != 1 || deleted[0] != "orphan" {
		t.Fatalf("deleted = %v, want only [orphan]", deleted)
	}
}

// Channels that are not configured are ignored entirely.
func TestStickyIgnoresUnconfiguredChannel(t *testing.T) {
	fake := newFakeChannel()
	s := newTestSticky(t, fake, []string{testStickyChannel}, testStickyText)

	s.onMessage("some-other-channel")
	time.Sleep(60 * time.Millisecond)

	posted, deleted := fake.snapshot()
	if len(posted) != 0 || len(deleted) != 0 {
		t.Fatalf("acted on an unconfigured channel: posted=%v deleted=%v", posted, deleted)
	}
}

// Posting before the gateway has reported the bot's identity would render a
// broken mention, so that cycle is skipped instead.
func TestStickySkipsCycleWhileBotIDUnknown(t *testing.T) {
	fake := newFakeChannel()
	s := newStickyReposter(stickyDeps{
		API:        fake,
		ChannelIDs: []string{testStickyChannel},
		Text:       testStickyText,
		BotID:      func() string { return "" },
		Debounce:   20 * time.Millisecond,
		Log:        zap.NewNop(),
	})

	s.onMessage(testStickyChannel)
	time.Sleep(60 * time.Millisecond)

	posted, _ := fake.snapshot()
	if len(posted) != 0 {
		t.Fatalf("posted = %v while the bot ID was unknown", posted)
	}
}

// Discord returns unknown-message when the sticky was already deleted by hand.
// That is not a failure: the new sticky still goes up.
func TestStickyPostsWhenPreviousStickyIsAlreadyGone(t *testing.T) {
	fake := newFakeChannel()
	s := newTestSticky(t, fake, []string{testStickyChannel}, testStickyText)

	s.onMessage(testStickyChannel)
	fake.waitForPosts(t, 1)

	fake.mu.Lock()
	fake.delErr = &discordgo.RESTError{Message: &discordgo.APIErrorMessage{Code: discordgo.ErrCodeUnknownMessage}}
	fake.mu.Unlock()

	s.onMessage(testStickyChannel)
	fake.waitForPosts(t, 1)

	posted, _ := fake.snapshot()
	if len(posted) != 2 {
		t.Fatalf("posted = %v, want the new sticky despite the stale delete", posted)
	}
}

// Manage Messages is granted by hand after the deploy, so the bot can be running
// in a channel where every delete is refused. Posting anyway would pile up one
// stray copy per debounce cycle, so a delete that failed for any reason other
// than the message already being gone has to abort the cycle.
func TestStickyDoesNotPostWhenPreviousStickyCannotBeDeleted(t *testing.T) {
	fake := newFakeChannel()
	s := newTestSticky(t, fake, []string{testStickyChannel}, testStickyText)

	s.onMessage(testStickyChannel)
	fake.waitForPosts(t, 1)

	// Missing Manage Messages, as Discord reports it.
	fake.mu.Lock()
	fake.delErr = &discordgo.RESTError{
		Message: &discordgo.APIErrorMessage{Code: discordgo.ErrCodeMissingPermissions},
	}
	fake.mu.Unlock()

	for i := 0; i < 3; i++ {
		s.onMessage(testStickyChannel)
		time.Sleep(60 * time.Millisecond)
	}

	posted, _ := fake.snapshot()
	if len(posted) != 1 {
		t.Fatalf("posted = %d copies, want 1: a refused delete must not add another sticky", len(posted))
	}
	if got := s.lastStickyID(testStickyChannel); got != "sticky-1" {
		t.Fatalf("lastStickyID = %q, want sticky-1 to stay recorded so the next cycle retries the delete", got)
	}
}

// A send failure must not leave a stale ID behind that the next cycle would try
// to delete; the next cycle re-runs the restart sweep instead.
func TestStickyForgetsIDWhenPostFails(t *testing.T) {
	fake := newFakeChannel()
	fake.sendErr = errors.New("discord unavailable")
	s := newTestSticky(t, fake, []string{testStickyChannel}, testStickyText)

	s.onMessage(testStickyChannel)
	time.Sleep(60 * time.Millisecond)

	if got := s.lastStickyID(testStickyChannel); got != "" {
		t.Fatalf("lastStickyID = %q after a failed send, want empty", got)
	}
}

// stop cancels armed timers so a shutdown does not post after Close.
func TestStickyStopCancelsPendingRepost(t *testing.T) {
	fake := newFakeChannel()
	s := newTestSticky(t, fake, []string{testStickyChannel}, testStickyText)

	s.onMessage(testStickyChannel)
	s.stop()
	time.Sleep(60 * time.Millisecond)

	posted, _ := fake.snapshot()
	if len(posted) != 0 {
		t.Fatalf("posted = %v after stop", posted)
	}
}

// A message arriving while a cycle is already running arms a fresh timer. stop
// must still be able to cancel that timer: if the running cycle drops it from the
// map, nothing can cancel it and the pod posts a sticky on its way out that it
// will never clean up.
func TestStickyStopCancelsTimerArmedDuringRepost(t *testing.T) {
	fake := newFakeChannel()
	// Park the first cycle inside the history scan so a second cycle can queue up
	// behind it. That queued cycle is the one that clears the timer map, and it does
	// so long after a third timer has been armed in the same slot.
	release := make(chan struct{})
	slow := newBlockingScan(fake, release)
	s := newTestSticky(t, slow, []string{testStickyChannel}, testStickyText)

	s.onMessage(testStickyChannel)
	slow.waitUntilScanning(t)

	s.onMessage(testStickyChannel)
	time.Sleep(60 * time.Millisecond) // this timer fires; its repost queues behind the parked one

	s.onMessage(testStickyChannel) // arms the timer that must survive until stop
	close(release)

	fake.waitForPosts(t, 2) // both queued cycles finish
	s.stop()
	time.Sleep(200 * time.Millisecond)

	if posted, _ := fake.snapshot(); len(posted) != 2 {
		t.Fatalf("posted = %v, want two; stop could not cancel a timer the running cycle dropped from the map", posted)
	}
}

// Rendering is exercised directly so a change to the placeholder is caught even
// if the posting path changes.
func TestRenderStickyReplacesEveryBotPlaceholder(t *testing.T) {
	got := renderSticky("{bot} and {bot}", "9")
	want := "<@9> and <@9>"
	if got != want {
		t.Fatalf("renderSticky = %q, want %q", got, want)
	}
}

// Two cycles for one channel must not overlap: a message arriving while a repost
// is mid-flight arms a new timer, and if that ran concurrently both sends would
// race, each record an ID, and one sticky would be left orphaned in the channel.
func TestStickyRepostsForOneChannelDoNotOverlap(t *testing.T) {
	fake := newFakeChannel()
	// Block the first send until the second cycle has had its chance to start.
	release := make(chan struct{})
	slow := newBlockingChannel(fake, release)
	s := newTestSticky(t, slow, []string{testStickyChannel}, testStickyText)

	s.onMessage(testStickyChannel)
	slow.waitUntilSending(t)
	s.onMessage(testStickyChannel)
	time.Sleep(60 * time.Millisecond) // the second timer fires while the first send is stuck
	close(release)

	fake.waitForPosts(t, 2)
	posted, deleted := fake.snapshot()
	if len(posted) != 2 {
		t.Fatalf("posted = %v, want two sequential posts", posted)
	}
	if len(deleted) != 1 || deleted[0] != "sticky-1" {
		t.Fatalf("deleted = %v, want the second cycle to remove [sticky-1]", deleted)
	}
}

// blockingChannel holds the first send open so a test can drive two cycles into
// each other on purpose.
type blockingChannel struct {
	*fakeChannel
	release chan struct{} // closed by the test to let the first send finish
	sending chan struct{} // closed once the first send has been entered
	once    sync.Once
}

func newBlockingChannel(fake *fakeChannel, release chan struct{}) *blockingChannel {
	return &blockingChannel{fakeChannel: fake, release: release, sending: make(chan struct{})}
}

func (b *blockingChannel) ChannelMessageSend(channelID, content string, opts ...discordgo.RequestOption) (*discordgo.Message, error) {
	b.once.Do(func() {
		close(b.sending)
		<-b.release
	})
	return b.fakeChannel.ChannelMessageSend(channelID, content, opts...)
}

// waitUntilSending blocks until the first send has been entered and parked.
func (b *blockingChannel) waitUntilSending(t *testing.T) {
	t.Helper()
	select {
	case <-b.sending:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the first send to start")
	}
}

// blockingScan holds the first history scan open. repost reaches the scan after
// it has already read and cleared the channel's timer, so parking there lets a
// test act in the window where a re-armed timer can be lost.
type blockingScan struct {
	*fakeChannel
	release  chan struct{} // closed by the test to let the first scan finish
	scanning chan struct{} // closed once the first scan has been entered
	once     sync.Once
}

func newBlockingScan(fake *fakeChannel, release chan struct{}) *blockingScan {
	return &blockingScan{fakeChannel: fake, release: release, scanning: make(chan struct{})}
}

func (b *blockingScan) ChannelMessages(channelID string, limit int, before, after, around string, opts ...discordgo.RequestOption) ([]*discordgo.Message, error) {
	b.once.Do(func() {
		close(b.scanning)
		<-b.release
	})
	return b.fakeChannel.ChannelMessages(channelID, limit, before, after, around, opts...)
}

// waitUntilScanning blocks until the first history scan has been entered and parked.
func (b *blockingScan) waitUntilScanning(t *testing.T) {
	t.Helper()
	select {
	case <-b.scanning:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the first history scan to start")
	}
}

// An ordinary message — one that neither mentions the bot nor sits in a bot
// thread — is what the handler returns early on, so the sticky hook has to run
// before that check or the reminder never moves.
func TestOnMessageCreateArmsStickyForPlainMessage(t *testing.T) {
	fake := newFakeChannel()
	b := newStickyTestBot(t, fake)

	b.onMessageCreate(b.session, &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: testStickyChannel,
		GuildID:   testStickyGuild,
		Author:    &discordgo.User{ID: "human"},
		Content:   "anyone around?",
	}})
	fake.waitForPosts(t, 1)
}

// The bot's own repost must not arm another cycle, or the sticky would move for
// ever on its own.
func TestOnMessageCreateIgnoresBotAuthors(t *testing.T) {
	fake := newFakeChannel()
	b := newStickyTestBot(t, fake)

	b.onMessageCreate(b.session, &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: testStickyChannel,
		GuildID:   testStickyGuild,
		Author:    &discordgo.User{ID: testBotID, Bot: true},
		Content:   "the sticky itself",
	}})
	time.Sleep(60 * time.Millisecond)

	if posted, _ := fake.snapshot(); len(posted) != 0 {
		t.Fatalf("posted = %v in response to the bot's own message", posted)
	}
}

// A guild the bot does not serve must not get a sticky even if the channel ID
// happens to be listed.
func TestOnMessageCreateIgnoresForeignGuild(t *testing.T) {
	fake := newFakeChannel()
	b := newStickyTestBot(t, fake)

	b.onMessageCreate(b.session, &discordgo.MessageCreate{Message: &discordgo.Message{
		ChannelID: testStickyChannel,
		GuildID:   "some-other-guild",
		Author:    &discordgo.User{ID: "human"},
		Content:   "hello",
	}})
	time.Sleep(60 * time.Millisecond)

	if posted, _ := fake.snapshot(); len(posted) != 0 {
		t.Fatalf("posted = %v for a guild the bot does not serve", posted)
	}
}

// Close must cancel armed timers so a shutting-down pod does not post a sticky
// it will never clean up.
func TestCloseCancelsPendingSticky(t *testing.T) {
	fake := newFakeChannel()
	b := newStickyTestBot(t, fake)

	b.sticky.onMessage(testStickyChannel)
	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	time.Sleep(60 * time.Millisecond)

	if posted, _ := fake.snapshot(); len(posted) != 0 {
		t.Fatalf("posted = %v after Close", posted)
	}
}

// newStickyTestBot builds the minimum Bot that onMessageCreate needs, with the
// sticky reposter wired to a fake Discord.
func newStickyTestBot(t *testing.T, api stickyAPI) *Bot {
	t.Helper()
	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("discordgo.New: %v", err)
	}
	// The session is never opened: onMessageCreate only reads its cached state for
	// thread lookups, and the sticky itself talks to the fake.
	return &Bot{
		session: session,
		cfg:     &config.Config{DiscordGuildID: testStickyGuild},
		policy:  access.NewPolicy(nil),
		log:     zap.NewNop(),
		queues:  newSerialQueues(),
		sticky:  newTestSticky(t, api, []string{testStickyChannel}, testStickyText),
	}
}

// Documents the fake's ID scheme the other tests depend on.
func TestFakeChannelAssignsSequentialIDs(t *testing.T) {
	fake := newFakeChannel()
	for i := 1; i <= 2; i++ {
		msg, err := fake.ChannelMessageSend("c", "x")
		if err != nil {
			t.Fatalf("send: %v", err)
		}
		if want := fmt.Sprintf("sticky-%d", i); msg.ID != want {
			t.Fatalf("ID = %q, want %q", msg.ID, want)
		}
		<-fake.posts
	}
}
