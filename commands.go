package main

import (
	"strings"

	"github.com/docopt/docopt-go"
	"github.com/kballard/go-shellquote"
)

func renderUsage() string {
	return s.ServiceId + `

Usage:
  c start
  c decode <invoice>
  c (receive | invoice) <satoshis> [<description>...]
  c (pay [now] | paynow) [<invoice>] [<satoshis>]
  c (send | tip) <username> <satoshis>
  c (send | tip) <userid> (<displayname>) <satoshis>
  c giveaway <satoshis>
  c balance
  c transactions
  c help

Things to know:
  - ` + "`/send` or `/tip`" + ` can send funds to any Telegram user, he'll be able to claim the funds once he registers with ` + "`/start`" + `;
  - ` + "`/pay now` or `/paynow`" + ` will skip the payment confirmation;
  - ` + "`/giveaway`" + ` will create a button in a chat and the first user to click that button will claim the satoshis you're giving away.
  - ` + "`<userid> (<displayname>)`" + ` is the format Telegram gives you when you're trying to mention an user without an user without a username, example: "@12345 (Matthew)".

Plus:
  - Forward any message containing an invoice to this chat to pay it (after confirmation);
  - Reply to a message containing an invoice with ` + "`/pay`, `/pay now` or `/paynow`" + ` to pay it;
  - Inline bot actions: you can do stuff in groups and private chats without having to add the bot!
    a. ` + "`@" + s.ServiceId + " invoice <amount>`" + ` to generate an invoice in place,
    b. ` + "`@" + s.ServiceId + " giveaway <amount>`" + ` to start a giveaway anywhere.
`
}

var parser = docopt.Parser{
	HelpHandler:   func(_ error, _ string) {},
	OptionsFirst:  false,
	SkipHelpFlags: true,
}

func parse(message string) (opts docopt.Opts, isCommand bool, err error) {
	if strings.HasPrefix(message, "/") {
		isCommand = true
		message = message[1:]
	} else {
		return
	}

	var argv []string
	argv, err = shellquote.Split(message)
	if err != nil {
		return
	}

	opts, err = parser.ParseArgs(s.Usage, argv, "")
	return
}
