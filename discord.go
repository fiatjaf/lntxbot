package main

import (
	"net/url"
	"regexp"
	"strings"

	html_to_markdown "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/bwmarrin/discordgo"
)

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

func sendDiscordMessage(channelId, html string) (id interface{}) {
	message, err := discord.ChannelMessageSendEmbed(channelId, &discordgo.MessageEmbed{
		Description: convertToDiscord(html),
	})
	if err != nil {
		log.Warn().Str("message", html).Err(err).Msg("sending discord text message")
		return ""
	}

	return DiscordMessage{
		GuildID:   message.GuildID,
		ChannelID: message.ChannelID,
		MessageID: message.ID,
	}
}

func sendDiscordMessageWithPicture(
	channelId string,
	pictureURL *url.URL,
	html string,
) (id interface{}) {
	var message *discordgo.Message

	// at this point we have all we need to send an embed
	message, err := discord.ChannelMessageSendEmbed(channelId, &discordgo.MessageEmbed{
		Description: convertToDiscord(html),
		Image: &discordgo.MessageEmbedImage{
			URL: pictureURL.String(),
		},
	})
	if err != nil {
		log.Warn().Str("message", html).Err(err).Msg("sending discord text message")
		return ""
	}

	return DiscordMessage{
		GuildID:   message.GuildID,
		ChannelID: message.ChannelID,
		MessageID: message.ID,
	}
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

type DiscordMessage struct {
	GuildID   string
	ChannelID string
	MessageID string
}

func (dmi DiscordMessage) URL() string {
	guild := dmi.GuildID
	if guild == "" {
		guild = "@me"
	}
	return "https://discord.com/channels/" + guild + "/" +
		dmi.ChannelID + "/" + dmi.MessageID
}
