package main

import (
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

func handle(upd tgbotapi.Update) {
	if upd.Message.MessageID > 0 {
		// it's a message
		handleMessage(upd.Message)
	}
}

func handleMessage(message *tgbotapi.Message) {
	// if message.Text == "/start" {
	// 	// create user
	// 	u := &User{
	// 		Id:       message.From.ID,
	// 		Username: message.From.UserName,
	// 	}

	// 	err := u.createUser()
	// 	if err != nil {
	// 		// notify: failed to create
	// 	}

	// } else if strings.HasPrefix(message.Text, "/pay ") {
	// 	// pay invoice

	// 	confirm := true
	// 	if strings.HasPrefix(message.Text, "/pay now ") {
	// 		confirm = false
	// 	}

	// 	var invoice string
	// 	parts := strings.Split(message.Text, " ")
	// 	for _, p := range parts {
	// 		if strings.HasPrefix(p, "lnbc") {
	// 			invoice = p
	// 			goto pay
	// 		}
	// 	}

	// 	// notify: invoice not found

	// pay:
	// 	if confirm {
	// 		// TODO
	// 	} else {
	// 		u, err := loadUser(message.From.ID)
	// 		if err != nil {
	// 			// notify: user not registered, press /start
	// 		}

	// 		err = u.call("POST", "/pay", InvoicePayload{invoice}, nil)
	// 		if err != nil {
	// 			// notify: failed to pay, must get reason
	// 		}
	// 	}
	// }
}
