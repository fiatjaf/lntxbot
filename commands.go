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
  c (receive | invoice | fund) <satoshis> [<description>...] [--preimage=<preimage>]
  c (pay [now] | paynow | withdraw) [<invoice>] [<satoshis>]
  c (send | tip) <satoshis> [<username>...]
  c giveaway <satoshis>
  c balance
  c transactions
  c help
  c stop

Things to know:
  - ` + "`/send` or `/tip`" + ` can send funds to any Telegram user, he'll be able to claim the funds once he registers with ` + "`/start`" + `;
  - To send funds to users without username, use the autocomplete prompt Telegram gives you when you start typing "@" and their name: ` + "`/send 500 @autocomplete-here`" + `;
  - Replying to a message with ` + "`/tip <satoshis>`" + ` will send that amount to the messge author (the bot must be added to the group for this to work).
  - ` + "`/pay now` or `/paynow`" + ` will skip the payment confirmation;
  - ` + "`/giveaway`" + ` will create a button in a chat and the first user to click that button will claim the satoshis you're giving away;
  - ` + "`/stop`" + ` will make you stop receiving notifications from the bot, but you'll not lose your account. You can call ` + "`/start`" + ` to receive notifications again.

Plus:
  - Forward any message containing an invoice to this chat to pay it (after confirmation);
  - In a group, reply to a message containing an invoice with ` + "`/pay`, `/pay now` or `/paynow`" + ` to pay it;
  - Take a picture of a QR code to pay it (after confirmation);
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
