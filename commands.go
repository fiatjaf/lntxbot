package main

import (
	"strings"

	"github.com/docopt/docopt-go"
	shellquote "github.com/kballard/go-shellquote"
)

const USAGE = `Usage:
  c start
  c decode <invoice>
  c receive <amount> [<description>...]
  c pay <invoice>
  c pay now <invoice>
  c pay @<person>
  c balance
  c transactions
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
