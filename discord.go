package main

import (
	"errors"
	"regexp"
	"strings"

	html_to_markdown "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/bwmarrin/discordgo"
)

type DiscordMessageID string // a string in the format "guild/channel/message"

func discordMessageID(guildId, channelId, messageId string) DiscordMessageID {
	if guildId == "" {
		guildId = "@me"
	}
	return DiscordMessageID(guildId + "/" + channelId + "/" + messageId)
}

func discordIDFromMessage(message *discordgo.Message) DiscordMessageID {
	if message == nil {
		return ""
	}
	return discordMessageID(message.GuildID, message.ChannelID, message.ID)
}

func discordIDFromReaction(reaction *discordgo.MessageReaction) DiscordMessageID {
	return discordMessageID(reaction.GuildID, reaction.ChannelID, reaction.MessageID)
}

func (d DiscordMessageID) Guild() string   { return strings.Split(string(d), "/")[0] }
func (d DiscordMessageID) Channel() string { return strings.Split(string(d), "/")[1] }
func (d DiscordMessageID) Message() string { return strings.Split(string(d), "/")[2] }

func (d DiscordMessageID) URL() string {
	return "https://discord.com/channels/" + string(d)
}

var slashToDollarSignReplacer = regexp.MustCompile(`([^\w\/]|^)/(\w)`)
var mdConverter = html_to_markdown.NewConverter("", true, &html_to_markdown.Options{
	EmDelimiter:     "_",
	StrongDelimiter: "**",
	CodeBlockStyle:  "fenced",
	Fence:           "```",
})

func convertToDiscord(v string) string {
	v, err := mdConverter.ConvertString(v)
	if err != nil {
		log.Warn().Str("html", v).Err(err).Msg("converting to discord markdown")
		return v
	}
	v = strings.ReplaceAll(v, "Telegram", "Discord")
	v = slashToDollarSignReplacer.ReplaceAllString(v, "$1$$$2")
	return v
}

func examineDiscordUsername(name string) (u *User, err error) {
	if !strings.HasPrefix(name, "<@!") || name[len(name)-1] != '>' || len(name) < 10 {
		return nil, errors.New("invalid discord user reference")
	}

	did := name[3 : len(name)-1]
	st, err := discord.User(did)
	if err != nil {
		return nil, err
	}

	user, err := ensureDiscordUser(did, st.Username+"#"+st.Discriminator, st.Locale)
	if err != nil {
		return nil, err
	}

	return &user, nil
}
