package main

import (
	"errors"
	"regexp"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/bwmarrin/discordgo"
	cmap "github.com/orcaman/concurrent-map"
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
var mdConverter = md.NewConverter("", true, &md.Options{
	EmDelimiter:     "_",
	StrongDelimiter: "**",
	CodeBlockStyle:  "fenced",
	Fence:           "```",
}).AddRules(
	md.Rule{
		Filter: []string{"br"},
		Replacement: func(_ string, sel *goquery.Selection, opt *md.Options) *string {
			return md.String("\n")
		},
	},
).After(
	func(v string) string {
		v = strings.ReplaceAll(v, "``", "` `")
		v = strings.ReplaceAll(v, "Telegram", "Discord")
		v = slashToDollarSignReplacer.ReplaceAllString(v, "$1$$$2")
		return v
	},
)

func convertToDiscord(v string) string {
	v = strings.ReplaceAll(v, "\n", "<br>")
	v, err := mdConverter.ConvertString(v)
	if err != nil {
		log.Warn().Str("html", v).Err(err).Msg("converting to discord markdown")
		return v
	}
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

var guildLocaleCache = cmap.New()
var guildSpamChannelCache = cmap.New()

func getGuildMetadata(guildId string) (spamChannelId string, locale string) {
	iSpamChannelId, ok1 := guildSpamChannelCache.Get(guildId)
	iLocale, ok2 := guildLocaleCache.Get(guildId)
	if ok1 && ok2 {
		spamChannelId = iSpamChannelId.(string)
		locale = iLocale.(string)
		return
	}

	guild, err := discord.Guild(guildId)
	if err == nil {
		locale = guild.PreferredLocale
	} else {
		// we don't care about locales
		locale = "en"
	}

	channels, err := discord.GuildChannels(guildId)
	if err == nil {
		for _, channel := range channels {
			if channel.Name == "commands" || channel.Name == "lntxbot" {
				spamChannelId = channel.ID
				break
			}
		}
	} else {
		// we also don't care about channels
		spamChannelId = ""
	}

	guildLocaleCache.Set(guildId, locale)
	guildSpamChannelCache.Set(guildId, spamChannelId)

	return
}
