package main

import (
	"strings"

	"git.alhur.es/fiatjaf/lntxbot/t"

	"github.com/docopt/docopt-go"
	"github.com/kballard/go-shellquote"
)

type def struct {
	aliases        []string
	flags          []flag
	inline         bool
	inline_example string
}

func (d def) argstr(lang string) string {
	key := t.Key(d.aliases[0] + "HelpArgs")
	if _, ok := t.EN[key]; !ok {
		return ""
	}
	return translate(key, lang)
}

func (d def) desc(lang string) string {
	key := t.Key(d.aliases[0] + "HelpDesc")
	if _, ok := t.EN[key]; !ok {
		return ""
	}
	return translate(key, lang)
}

func (d def) examples(lang string) string {
	key := t.Key(d.aliases[0] + "HelpExample")
	if _, ok := t.EN[key]; !ok {
		return ""
	}
	return translate(key, lang)
}

type flag struct {
	Name        string
	Explanation string
}

var methods = []def{
	def{
		aliases: []string{"start"},
	},
	def{
		aliases: []string{"receive", "invoice", "fund"},
		flags: []flag{
			{
				"--preimage",
				"If you want to generate an invoice with a specific preimage, write it here as a 32b / 64char hex string.",
			},
		},
		inline:         true,
		inline_example: "invoice <satoshis>",
	},
	def{
		aliases: []string{"pay", "decode", "paynow", "withdraw"},
	},
	def{
		aliases: []string{"send", "tip", "sendanonymously"},
		flags: []flag{
			{
				"--anonymous",
				"The receiver will never know who sent him the satoshis.",
			},
		},
	},
	def{
		aliases: []string{"balance"},
	},
	def{
		aliases: []string{"transactions"},
		flags: []flag{
			{
				"--page",
				"To show older transactions, specify a page number greater than 1.",
			},
		},
	},
	def{
		aliases:        []string{"giveaway"},
		inline:         true,
		inline_example: "giveaway <satoshis>",
	},
	def{
		aliases:        []string{"coinflip", "lottery"},
		inline:         true,
		inline_example: "coinflip <satoshis> <num_participants>",
	},
	def{
		aliases:        []string{"giveflip"},
		inline:         true,
		inline_example: "giveflip <satoshis> <num_participants>",
	},
	def{
		aliases: []string{"fundraise", "crowdfund"},
	},
	def{
		aliases: []string{"hide"},
	},
	def{
		aliases:        []string{"reveal"},
		inline:         true,
		inline_example: "reveal [hidden_message_id]",
	},
	def{
		aliases: []string{"app", "lapp"},
	},
	def{
		aliases: []string{"bluewallet", "lndhub"},
	},
	def{
		aliases: []string{"toggle"},
	},
	def{
		aliases: []string{"help"},
	},
	def{
		aliases: []string{"stop"},
	},
}

var commandList []string
var commandIndex = make(map[string]def)

func setupCommands() {
	s.Usage = docoptFromMethodDefinitions()

	for _, def := range methods {
		for _, alias := range def.aliases {
			commandList = append(commandList, alias)
			commandIndex[alias] = def
		}
	}
}

func docoptFromMethodDefinitions() string {
	var lines []string

	for _, def := range methods {
		for _, alias := range def.aliases {
			lines = append(lines, "  c "+alias+" "+def.argstr("en"))
		}
	}

	return s.ServiceId + "\n\nUsage:\n" + strings.Join(lines, "\n")
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

	// turn /app_microbet_bets, for example, into /app microbet bet
	parts := strings.SplitN(message, " ", 2)
	parts[0] = strings.ReplaceAll(parts[0], "_", " ")
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

func handleHelp(u User, method string) (handled bool) {
	var def def
	var mainName string
	var aliases []map[string]string
	var helpString string
	var ok bool
	method = strings.ToLower(strings.TrimSpace(method))
	if method == "" {
		helpString = translateTemplate(t.HELPINTRO, u.Locale, t.T{
			"Help": escapeHTML(strings.Replace(s.Usage, "  c ", "  /", -1)),
		})
		goto gothelpstring
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
			u.notify(t.HELPSIMILAR, t.T{
				"Method":  method,
				"Similar": similar,
			})
			return true
		} else {
			return false
		}
	}

	// here we have a working method definition
	helpString = translateTemplate(t.HELPMETHOD, u.Locale, t.T{
		"MainName":      mainName,
		"Argstr":        escapeHTML(def.argstr(u.Locale)),
		"Desc":          def.desc(u.Locale),
		"Examples":      def.examples(u.Locale),
		"HasInline":     def.inline,
		"InlineExample": escapeHTML(def.inline_example),
		"Aliases":       aliases,
		"ServiceId":     s.ServiceId,
	})

	goto gothelpstring

gothelpstring:
	sendMessage(u.ChatId, helpString)
	return true
}
