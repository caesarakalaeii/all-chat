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

// Package discord is the Discord surface: it resolves each requester's access mode
// from the maintainer allow-list (never from message content), runs the agent loop,
// applies the final-answer redaction pass in support mode, moderates cross-channel
// spam, and delivers answers in threads. It replaces the discord.js bot.
package discord

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/caesar/all-chat/services/support-bot/access"
	"github.com/caesar/all-chat/services/support-bot/agent"
	"github.com/caesar/all-chat/services/support-bot/config"
	"github.com/caesar/all-chat/services/support-bot/llm"
	"github.com/caesar/all-chat/services/support-bot/memory"
	"github.com/caesar/all-chat/services/support-bot/moderation"
	"github.com/caesar/all-chat/services/support-bot/redact"
	"github.com/caesar/all-chat/services/support-bot/sanitize"
	"github.com/caesar/all-chat/services/support-bot/tool"
	"go.uber.org/zap"
)

const (
	discordMaxMessage  = 2000
	historyFetchLimit  = 20
	banDeleteDays      = 1 // deletes ~last day of the banned user's messages
	threadArchiveMins  = 1440
	threadNameMaxRunes = 50
)

var mentionRe = regexp.MustCompile(`<@[!&]?\d+>`)

// Deps are the dependencies the bot needs.
type Deps struct {
	Config   *config.Config
	Policy   *access.Policy
	Registry *tool.Registry
	LLM      llm.ChatClient
	Memory   *memory.Repository
	Redactor *redact.Redactor
	AgentCfg agent.Config
	Log      *zap.Logger
}

// Bot is the Discord adapter.
type Bot struct {
	session  *discordgo.Session
	cfg      *config.Config
	policy   *access.Policy
	reg      *tool.Registry
	llm      llm.ChatClient
	mem      *memory.Repository
	redactor *redact.Redactor
	spam     *moderation.SpamDetector
	agentCfg agent.Config
	log      *zap.Logger

	botThreads sync.Map // channelID -> struct{}: threads the bot manages
	queues     *serialQueues
}

// New builds a Bot (does not connect).
func New(d Deps) (*Bot, error) {
	session, err := discordgo.New("Bot " + d.Config.DiscordToken)
	if err != nil {
		return nil, err
	}
	session.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentMessageContent
	b := &Bot{
		session:  session,
		cfg:      d.Config,
		policy:   d.Policy,
		reg:      d.Registry,
		llm:      d.LLM,
		mem:      d.Memory,
		redactor: d.Redactor,
		spam:     moderation.NewSpamDetector(moderation.Options{}),
		agentCfg: d.AgentCfg,
		log:      d.Log,
		queues:   newSerialQueues(),
	}
	session.AddHandler(b.onReady)
	session.AddHandler(b.onMessageCreate)
	session.AddHandler(b.onInteractionCreate)
	return b, nil
}

// Start opens the gateway connection and registers the /support slash command.
func (b *Bot) Start() error {
	if err := b.session.Open(); err != nil {
		return err
	}
	b.registerCommands()
	return nil
}

// Close disconnects the session.
func (b *Bot) Close() error {
	return b.session.Close()
}

func (b *Bot) onReady(s *discordgo.Session, _ *discordgo.Ready) {
	name := "unknown"
	if s.State != nil && s.State.User != nil {
		name = s.State.User.Username
	}
	b.log.Info("support bot ready",
		zap.String("user", name),
		zap.Int("admin_uids", b.policy.AdminCount()))
}

func (b *Bot) botID() string {
	if b.session.State != nil && b.session.State.User != nil {
		return b.session.State.User.ID
	}
	return ""
}

func (b *Bot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author == nil || m.Author.Bot {
		return
	}

	// Cross-channel spam moderation (only in the configured guild; maintainers exempt).
	if b.cfg.ModerationGuildID != "" && m.GuildID == b.cfg.ModerationGuildID && !b.isPrivileged(m.Author.ID) {
		sizes := make([]int, 0, len(m.Attachments))
		for _, a := range m.Attachments {
			sizes = append(sizes, a.Size)
		}
		if b.spam.Record(m.Author.ID, m.ChannelID, m.Content, sizes) {
			b.autoBan(s, m)
			return
		}
	}

	inThread := b.inBotThread(s, m.ChannelID)
	if !inThread && !b.mentionsBot(m) {
		return
	}

	stripped := strings.TrimSpace(mentionRe.ReplaceAllString(m.Content, ""))
	channelID := m.ChannelID
	authorID := m.Author.ID

	// Serialize per channel (mutual exclusion + FIFO among enqueued tasks) so a thread's
	// history is complete before the next message in it is handled. enqueue returns
	// immediately; the drain goroutine does the work, so the gateway handler is not
	// blocked.
	b.queues.enqueue(channelID, func() {
		b.handleMessage(s, m, inThread, stripped, channelID, authorID)
	})
}

func (b *Bot) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate, inThread bool, stripped, channelID, authorID string) {
	_ = s.ChannelTyping(channelID)
	stopTyping := make(chan struct{})
	go func() {
		t := time.NewTicker(8 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-stopTyping:
				return
			case <-t.C:
				_ = s.ChannelTyping(channelID)
			}
		}
	}()
	defer close(stopTyping)

	var history []string
	if inThread {
		history = b.fetchHistory(s, channelID)
	}

	question := stripped
	if m.MessageReference != nil && m.MessageReference.MessageID != "" {
		if ref, err := s.ChannelMessage(m.MessageReference.ChannelID, m.MessageReference.MessageID); err == nil && ref.Author != nil && !ref.Author.Bot {
			if stripped != "" {
				question = "Context (message being replied to by " + ref.Author.Username + "): " + ref.Content + "\n\nQuestion: " + stripped
			} else {
				question = ref.Content
			}
		}
	}
	if strings.TrimSpace(question) == "" {
		return
	}

	answer := b.answer(authorID, channelID, question, history)

	if inThread {
		b.deliver(s, channelID, answer)
		return
	}
	// Start a thread from the user's message and answer there.
	name := truncateRunes(firstNonEmpty(stripped, question), threadNameMaxRunes)
	th, err := s.MessageThreadStartComplex(channelID, m.ID, &discordgo.ThreadStart{
		Name:                name,
		AutoArchiveDuration: threadArchiveMins,
	})
	if err != nil {
		b.log.Warn("failed to start thread; replying in channel", zap.Error(err))
		b.deliver(s, channelID, answer)
		return
	}
	b.botThreads.Store(th.ID, struct{}{})
	b.deliver(s, th.ID, answer)
}

func (b *Bot) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	data := i.ApplicationCommandData()
	if data.Name != "support" {
		return
	}
	question := ""
	for _, opt := range data.Options {
		if opt.Name == "question" {
			question = opt.StringValue()
		}
	}
	if strings.TrimSpace(question) == "" {
		return
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	uid := interactionUserID(i)
	answer := b.answer(uid, i.ChannelID, question, nil)

	chunks := chunkMessage(answer)
	first := answer
	if len(chunks) > 0 {
		first = chunks[0]
	}
	if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &first}); err != nil {
		b.log.Warn("failed to edit interaction response", zap.Error(err))
		return
	}
	reply, err := s.InteractionResponse(i.Interaction)
	if err != nil {
		return
	}
	th, err := s.MessageThreadStartComplex(i.ChannelID, reply.ID, &discordgo.ThreadStart{
		Name:                truncateRunes(question, threadNameMaxRunes),
		AutoArchiveDuration: threadArchiveMins,
	})
	if err != nil {
		// Thread creation can fail (already in a thread, forum channel, missing perms).
		// Don't silently drop the rest of a long answer — deliver it in the channel.
		b.log.Warn("failed to start /support thread; delivering remainder in channel", zap.Error(err))
		for _, c := range chunks[min(1, len(chunks)):] {
			_, _ = s.ChannelMessageSend(i.ChannelID, c)
		}
		return
	}
	b.botThreads.Store(th.ID, struct{}{})
	_, _ = s.ChannelMessageSend(th.ID, "**Original question:** "+truncateRunes(question, 1500))
	for _, c := range chunks[min(1, len(chunks)):] {
		_, _ = s.ChannelMessageSend(th.ID, c)
	}
}

// answer runs the agent loop for a question and returns the delivery-ready text.
func (b *Bot) answer(uid, channelID, question string, history []string) string {
	mode := b.policy.ModeFor(uid)

	var memStrings []string
	if b.mem != nil {
		if mems, err := b.mem.Retrieve(context.Background(), memory.ExtractTags(question)); err == nil {
			for _, mm := range mems {
				memStrings = append(memStrings, "["+string(mm.Type)+"] "+mm.Content)
			}
		} else {
			b.log.Warn("memory retrieve failed", zap.Error(err))
		}
	}

	system := agent.BuildSystemPrompt(mode, b.cfg.RepoPaths(), b.cfg.GrafanaEnabled())
	userContent := agent.BuildUserContent(memStrings, history, question)
	messages := []llm.Message{
		llm.TextMessage(llm.RoleSystem, system),
		llm.TextMessage(llm.RoleUser, userContent),
	}

	tctx := &tool.ToolCtx{
		Mode:       mode,
		DiscordUID: uid,
		ChannelID:  channelID,
		RepoPaths:  b.cfg.RepoPaths(),
		Namespace:  b.cfg.KubeNamespace,
		Redactor:   b.redactor,
		Log:        b.log,
	}

	ctx, cancel := context.WithTimeout(context.Background(), b.cfg.OverallTimeout)
	defer cancel()

	b.log.Info("handling question",
		zap.String("mode", mode.String()),
		zap.String("discord_uid", uid),
		zap.Int("question_len", len(question)))

	res, err := agent.Run(ctx, b.agentCfg, b.llm, b.reg, tctx, messages)
	if err != nil {
		b.log.Error("agent run failed", zap.Error(err), zap.String("stop", string(res.Stop)))
		if strings.TrimSpace(res.Text) == "" {
			return "Sorry, something went wrong while processing your question. Please try again, or check the bot logs."
		}
	}

	answer := sanitize.StripInternalScaffolds(res.Text)
	if mode == access.ModeSupport {
		answer = b.redactor.Redact(answer)
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		answer = "I couldn't produce an answer for that. Could you rephrase or add detail?"
	}

	if len(res.Effects) > 0 {
		var lines []string
		for _, e := range res.Effects {
			if e.URL != "" {
				lines = append(lines, "- "+e.Summary+": "+e.URL)
			} else {
				lines = append(lines, "- "+e.Summary)
			}
		}
		answer += "\n\n" + strings.Join(lines, "\n")
	}
	if res.Stop == agent.StopMaxIterations || res.Stop == agent.StopNoProgress {
		answer = "_(I stopped early, so this may be partial.)_\n\n" + answer
	}
	// Ping the lead developer when the bot took a side-effecting action.
	if len(res.Effects) > 0 && b.cfg.LeadDeveloperDiscordID != "" {
		answer = "<@" + b.cfg.LeadDeveloperDiscordID + "> " + answer
	}
	return answer
}

func (b *Bot) registerCommands() {
	if b.cfg.DiscordAppID == "" {
		b.log.Warn("DISCORD_CLIENT_ID not set; skipping slash-command registration")
		return
	}
	cmd := &discordgo.ApplicationCommand{
		Name:        "support",
		Description: "Ask a question about All-Chat",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "question",
				Description: "What would you like to know?",
				Required:    true,
			},
		},
	}
	if _, err := b.session.ApplicationCommandCreate(b.cfg.DiscordAppID, b.cfg.DiscordGuildID, cmd); err != nil {
		b.log.Warn("failed to register /support command", zap.Error(err))
		return
	}
	b.log.Info("registered /support slash command", zap.String("guild", b.cfg.DiscordGuildID))
}

func (b *Bot) autoBan(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.GuildID == "" {
		return
	}
	reason := "Auto-ban: same message posted in 3+ channels (suspected compromised account)"
	if err := s.GuildBanCreateWithReason(m.GuildID, m.Author.ID, reason, banDeleteDays); err != nil {
		b.log.Error("failed to auto-ban cross-channel spammer", zap.Error(err), zap.String("user", m.Author.ID))
		return
	}
	b.log.Info("auto-banned cross-channel spammer",
		zap.String("user", m.Author.ID), zap.String("guild", m.GuildID))
}

func (b *Bot) isPrivileged(uid string) bool {
	return uid == b.cfg.LeadDeveloperDiscordID || b.policy.IsAdmin(uid)
}

func (b *Bot) mentionsBot(m *discordgo.MessageCreate) bool {
	id := b.botID()
	if id == "" {
		return false
	}
	for _, u := range m.Mentions {
		if u.ID == id {
			return true
		}
	}
	return false
}

func (b *Bot) inBotThread(s *discordgo.Session, channelID string) bool {
	if _, ok := b.botThreads.Load(channelID); ok {
		return true
	}
	ch, err := s.State.Channel(channelID)
	if err != nil || ch == nil || !ch.IsThread() {
		return false
	}
	return ch.OwnerID == b.botID()
}

func (b *Bot) fetchHistory(s *discordgo.Session, channelID string) []string {
	msgs, err := s.ChannelMessages(channelID, historyFetchLimit, "", "", "")
	if err != nil {
		return nil
	}
	// ChannelMessages returns newest-first; reverse to chronological.
	out := make([]string, 0, len(msgs))
	for i := len(msgs) - 1; i >= 0; i-- {
		msg := msgs[i]
		if msg.Author == nil || strings.TrimSpace(msg.Content) == "" {
			continue
		}
		name := msg.Author.Username
		if msg.Author.Bot {
			name = "Bot"
		}
		out = append(out, "["+name+"]: "+msg.Content)
	}
	return out
}

func (b *Bot) deliver(s *discordgo.Session, channelID, text string) {
	for _, c := range chunkMessage(text) {
		if _, err := s.ChannelMessageSend(channelID, c); err != nil {
			b.log.Warn("failed to send message", zap.Error(err))
			return
		}
	}
}

func interactionUserID(i *discordgo.InteractionCreate) string {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return ""
}
