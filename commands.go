package main

import (
	"strings"

	"github.com/docopt/docopt-go"
	shellquote "github.com/kballard/go-shellquote"
)

const USAGE = `lntxbot

*start*: create a new account.
    as an account will also be created if you try to call any of the
    other commands.
*decode*: show information about a bolt11 invoice.
*receive*: generates an invoice.
*pay*: paste an invoice to pay.
    use /pay now <bolt11> to pay without asking for confirmation,
    append a value in satoshis to pay invoices without a specified amount.
*balance*: show your balance and some information.
*transactions*: show your transaction history.
*help*: show usage information.

Usage:
  c start
  c decode <invoice>
  c receive <amount> [<description>...]
  c pay <invoice> [<satoshis>]
  c pay now <invoice> [<satoshis>]
  c pay @<person>
  c balance
  c transactions
  c help

To verify the state of an invoice, forward the message that contains its encoded payment request to the chat.
`

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

	opts, err = docopt.ParseArgs(USAGE, argv, "")
	return
}
