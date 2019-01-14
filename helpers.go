package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/kballard/go-shellquote"
	"github.com/lucsky/cuid"
	"github.com/skip2/go-qrcode"
	"github.com/tidwall/gjson"
	"gopkg.in/jmcvetta/napping.v3"
)

func makeLabel(chatId int64, messageId interface{}) string {
	return fmt.Sprintf("%s.%d.%v", s.ServiceId, chatId, messageId)
}

func messageIdFromLabel(label string) int {
	parts := strings.Split(label, ".")
	if len(parts) == 3 {
		id, _ := strconv.Atoi(parts[2])
		return id
	}
	return 0
}

func qrImagePath(label string) string {
	return filepath.Join(os.TempDir(), s.ServiceId+".invoice."+label+".png")
}

func searchForInvoice(message tgbotapi.Message) (bolt11 string, ok bool) {
	text := message.Text
	if text == "" {
		text = message.Caption
	}

	argv, err := shellquote.Split(text)
	if err != nil {
		return
	}

	for _, arg := range argv {
		if strings.HasPrefix(arg, "lnbc") {
			return arg, true
		}
	}

	// receiving a picture, try to decode the qr code
	if message.Photo != nil && len(*message.Photo) > 0 {
		log.Debug().Msg("got photo, looking for qr code.")

		photos := *message.Photo
		photo := photos[len(photos)-1]

		photourl, err := bot.GetFileDirectURL(photo.FileID)
		if err != nil {
			log.Warn().Err(err).Str("fileid", photo.FileID).
				Msg("failed to get photo URL.")
			return
		}

		p := &url.Values{}
		p.Set("fileurl", photourl)
		var r []struct {
			Type   string `json:"type"`
			Symbol []struct {
				Data  string `json:"data"`
				Error string `json:"error"`
			} `json:"symbol"`
		}
		_, err = napping.Get("https://api.qrserver.com/v1/read-qr-code/", p, &r, nil)
		if err != nil {
			log.Warn().Err(err).Str("url", photourl).Msg("failed to call qrserver")
			return
		}
		if len(r) == 0 || len(r[0].Symbol) == 0 {
			log.Warn().Str("url", photourl).Msg("invalid rponse from  qrserver")
			return
		}
		if r[0].Symbol[0].Error != "" {
			log.Debug().Str("err", r[0].Symbol[0].Error).
				Str("url", photourl).Msg("qrserver failed to decode")
			return
		}

		data := r[0].Symbol[0].Data
		if strings.HasPrefix(data, "lightning:") {
			data = data[10:]
		}

		if strings.HasPrefix(data, "lnbc") {
			bolt11 = data
			ok = true
			return
		}

		// found a bolt11 invoice on a qr code on a photo!
		log.Debug().Str("data", data).
			Msg("decoded qr is not a bolt11 invoice")
		return
	}

	return
}

func getBaseEdit(cb *tgbotapi.CallbackQuery) tgbotapi.BaseEdit {
	baseedit := tgbotapi.BaseEdit{
		InlineMessageID: cb.InlineMessageID,
	}

	if cb.Message != nil {
		baseedit.MessageID = cb.Message.MessageID
		baseedit.ChatID = cb.Message.Chat.ID
	}

	return baseedit
}

func giveAwayKeyboard(u User, sats int) tgbotapi.InlineKeyboardMarkup {
	giveawayid := cuid.Slug()
	buttonData := fmt.Sprintf("give=%d-%d-%s", u.Id, sats, giveawayid)

	rds.Set("giveaway:"+giveawayid, buttonData, s.GiveAwayTimeout)

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Cancel", "cancel"),
			tgbotapi.NewInlineKeyboardButtonData(
				"Claim!",
				buttonData,
			),
		),
	)
}

func decodeInvoice(invoice string) (inv gjson.Result, err error) {
	inv, err = ln.Call("decodepay", invoice)
	if err != nil {
		return
	}
	if inv.Get("code").Int() != 0 {
		return inv, errors.New(inv.Get("message").String())
	}

	return
}

func makeInvoice(u User, label string, sats int, desc string) (bolt11 string, qrpath string, err error) {
	log.Debug().Str("label", label).Str("desc", desc).Int("sats", sats).
		Msg("generating invoice")

	// save invoice creator on redis
	rds.Set("recinvoice:"+label+":creator", u.Id, s.InvoiceTimeout)

	// make invoice
	res, err := ln.Call("invoice", strconv.Itoa(sats*1000),
		label, desc, strconv.Itoa(int(s.InvoiceTimeout/time.Second)))
	if err != nil {
		return
	}
	bolt11 = res.Get("bolt11").String()

	// save this bolt11 on redis so we know if someone tries
	// to pay it from this same wallet/bot
	rds.Set("recinvoice.internal:"+bolt11, label, s.InvoiceTimeout)

	// generate qr code
	err = qrcode.WriteFile(bolt11, qrcode.Medium, 256, qrImagePath(label))
	if err != nil {
		log.Warn().Err(err).Str("invoice", bolt11).
			Msg("failed to generate qr.")
		err = nil
	} else {
		qrpath = qrImagePath(label)
	}

	return
}

func notify(chatId int64, msg string) tgbotapi.Message {
	return notifyAsReply(chatId, msg, 0)
}

func notifyAsReply(chatId int64, msg string, replyToId int) tgbotapi.Message {
	chattable := tgbotapi.NewMessage(chatId, msg)
	chattable.BaseChat.ReplyToMessageID = replyToId
	chattable.ParseMode = "Markdown"
	message, err := bot.Send(chattable)
	if err != nil {
		log.Warn().Int64("chat", chatId).Err(err).Msg("error sending message")
	}
	return message
}
