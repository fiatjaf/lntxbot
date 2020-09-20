package main

func sendDiscordMessage(targetChannelId string, text string) {
	discord.ChannelMessageSend(targetChannelId, text)
}
