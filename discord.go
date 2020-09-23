package main

import (
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
	return discordMessageID(message.GuildID, message.ChannelID, message.ID)
}

func (d DiscordMessageID) Guild() string   { return strings.Split(string(d), "/")[0] }
func (d DiscordMessageID) Channel() string { return strings.Split(string(d), "/")[1] }
func (d DiscordMessageID) Message() string { return strings.Split(string(d), "/")[2] }

func (d DiscordMessageID) URL() string {
	return "https://discord.com/channels/" + string(d)
}

var slashToDollarSignReplacer = regexp.MustCompile(`([^\w\/]|^)/(\w)`)
var mdConverter = html_to_markdown.NewConverter("", true, &html_to_markdown.Options{
	EmDelimiter:     "__",
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

func appendToDiscordMessage(channelId, messageId, text string) {
	message, err := discord.ChannelMessage(channelId, messageId)
	if err != nil || len(message.Embeds) < 1 {
		return
	}

	embed := message.Embeds[0]
	embed.Description += " " + text
	discord.ChannelMessageEditEmbed(channelId, messageId, embed)
}
