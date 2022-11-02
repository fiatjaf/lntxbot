package main

import (
	"context"
	"strings"

	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/lntxbot/t"
	"github.com/kballard/go-shellquote"
)

type def struct {
	aliases        []string
	argstr         string
	inline         bool
	inline_example string
}

func (d def) help(ctx context.Context) string {
	key := t.Key(strings.ReplaceAll(d.aliases[0], " ", "_") + "Help")
	if _, ok := t.EN[key]; !ok {
		return ""
	}
	return translateTemplate(ctx, key, t.T{})
}

var methods = []def{
	{
		aliases: []string{"start"},
	},
	{
		aliases: []string{"lnurl"},
		argstr:  "[--anonymous] <lnurl>",
	},
	{
		aliases:        []string{"receive", "invoice", "fund"},
		argstr:         "(lnurl | (any | <satoshis>) [<description>...])",
		inline:         true,
		inline_example: "invoice <satoshis>",
	},
	{
		aliases: []string{"deposit", "depositbtc", "fundbtc"},
		argstr:  "",
	},
	{
		aliases: []string{"sms", "smsreceive"},
		argstr:  "",
	},
	{
		aliases: []string{"pay", "decode", "paynow", "withdraw"},
		argstr:  "(lnurl <satoshis> | [now] [<invoice>] [<satoshis>])",
	},
	{
		aliases:        []string{"send", "tip", "sendanonymously"},
		argstr:         "[anonymously] <satoshis> [<receiver>] [<description>...] [--anonymous]",
		inline_example: "give <satoshis> <username>",
	},
	{
		aliases: []string{"balance"},
		argstr:  "[apps]",
	},
	{
		aliases: []string{"apps"},
	},
	{
		aliases: []string{"tx"},
		argstr:  "<hash>",
	},
	{
		aliases: []string{"transactions"},
		argstr:  "[<tag>] [--in] [--out]",
	},
	{
		aliases:        []string{"giveaway"},
		argstr:         "<satoshis>",
		inline:         true,
		inline_example: "giveaway <satoshis>",
	},
	{
		aliases:        []string{"coinflip", "lottery"},
		argstr:         "<satoshis> [<num_participants>]",
		inline:         true,
		inline_example: "coinflip <satoshis> <num_participants>",
	},
	{
		aliases:        []string{"giveflip"},
		argstr:         "<satoshis> [<num_participants>]",
		inline:         true,
		inline_example: "giveflip <satoshis> <num_participants>",
	},
	{
		aliases: []string{"fundraise", "crowdfund"},
		argstr:  "<satoshis> <num_participants> <receiver>",
	},
	{
		aliases: []string{"hide"},
		argstr:  "<satoshis> [<message>...] [--revealers=<num_revealers>] [--crowdfund=<num_participants>] [--private]",
	},
	{
		aliases:        []string{"reveal"},
		argstr:         "<hidden_message_id>",
		inline:         true,
		inline_example: "reveal [hidden_message_id]",
	},
	{
		aliases: []string{"sats4ads"},
		argstr:  "(on [<msat_per_character>] | off | rate | rates | broadcast <satoshis> [<text>...] [--max-rate=<maxrate>] [--skip=<offset>] | preview)",
	},
	{
		aliases: []string{"api"},
		argstr:  "[full | invoice | readonly | url | refresh]",
	},
	{
		aliases: []string{"lightningatm"},
	},
	{
		aliases: []string{"bluewallet", "zeus", "lndhub"},
		argstr:  "[refresh]",
	},
	{
		aliases: []string{"rename"},
		argstr:  "<name>...",
	},
	{
		aliases: []string{"fine"},
		argstr:  "<satoshis> [for <reason>...]",
	},
	{
		aliases: []string{"toggle"},
		argstr:  "(ticket [<satoshis>] | renamable [<satoshis>] | spammy | expensive [<satoshis> <pattern>] | language [<lang>] | coinflips)",
	},
	{
		aliases: []string{"satoshis", "calc"},
		argstr:  "<expression>",
	},
	{
		aliases: []string{"moon"},
	},
	{
		aliases: []string{"help"},
		argstr:  "[<command>]",
	},
	{
		aliases: []string{"stop"},
	},
}

var (
	commandList  []string
	commandIndex = make(map[string]def)
)

func setupCommands() {
	s.Usage = docoptFromMethodDefinitions()

	for _, def := range methods {
		for _, alias := range def.aliases {
			commandList = append(commandList, strings.Replace(alias, " ", "_", -1))
			commandIndex[alias] = def
		}
	}
}

func docoptFromMethodDefinitions() string {
	var lines []string

	for _, def := range methods {
		for _, alias := range def.aliases {
			lines = append(lines, "  c "+alias+" "+def.argstr)
		}
	}

	return s.ServiceId + "\n\nUsage:\n" + strings.Join(lines, "\n")
}

func commandsListFromDefinitions() string {
	var lines []string

	for _, def := range methods {
		lines = append(lines, "/"+def.aliases[0]+" "+def.argstr)
	}

	return strings.Join(lines, "\n")
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

	// turn '_' into ' '
	parts := strings.SplitN(message, " ", 2)
	parts[0] = strings.ReplaceAll(parts[0], "_", " ")
	message = strings.Join(parts, " ")
	parts = strings.SplitN(message, " ", 2)
	parts[0] = strings.ToLower(parts[0])
	message = strings.Join(parts, " ")

	// apply this to get shell-like quoting rules
	var argv []string
	argv, err = shellquote.Split(message)
	if err != nil {
		// fallback to just a split when people do `/app satellite 23 can't quote`
		// which would normally break here
		argv = strings.Split(message, " ")
	}

	// parse using docopt
	opts, err = parser.ParseArgs(s.Usage, argv, "")
	return
}

func handleHelp(ctx context.Context, method string) (handled bool) {
	var (
		def        def
		mainName   string
		aliases    []map[string]string
		params     t.T
		helpString string
		ok         bool
	)

	method = strings.ToLower(strings.TrimSpace(method))
	if method == "" {
		briefUsage := commandsListFromDefinitions()
		helpString = translateTemplate(ctx, t.HELPINTRO, t.T{
			"Help": escapeHTML(briefUsage),
		})
		send(ctx, helpString)
		return true
	}

	// render specific help instructions for the given method
	def, ok = commandIndex[method]
	if ok {
		mainName = method
		aliases := ""
		for _, alias := range def.aliases {
			if alias != mainName {
				aliases += " " + alias
			}
		}
	} else {
		similar := findSimilar(method, commandList)
		if len(similar) > 0 {
			send(ctx, t.HELPSIMILAR, t.T{
				"Method":  method,
				"Similar": similar,
			})
			return true
		} else {
			return false
		}
	}

	// here we have a working method definition
	params = t.T{
		"MainName":      mainName,
		"Argstr":        escapeHTML(def.argstr),
		"Help":          def.help(ctx),
		"HasInline":     def.inline,
		"InlineExample": escapeHTML(def.inline_example),
		"Aliases":       aliases,
		"ServiceId":     s.ServiceId,
	}

	send(ctx, t.HELPMETHOD, params)
	return true
}
