package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/fiatjaf/lntxbot/t"
	"github.com/fogleman/primitive/primitive"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/nfnt/resize"
)

func handleTriangles(ctx context.Context, opts docopt.Opts, message *tgbotapi.Message) {
	u := ctx.Value("initiator").(*User)

	msats, err := parseAmountString(opts["<n>"].(string))
	if err != nil {
		send(ctx, message, u, t.ERROR, t.T{"Err": err})
		return
	}
	n := int(msats / 1000)
	if n > 150 {
		send(ctx, message, u, t.ERROR, t.T{"Err": "max is 150"})
		return
	}

	// check balance
	if !u.checkBalanceFor(ctx, msats, "triangles") {
		send(ctx, message, u, t.ERROR, t.T{"Err": "no enough balance"})
		return
	}

	if message.ReplyToMessage == nil {
		send(ctx, message, u, t.ERROR, t.T{"Err": "must be sent as a reply to an image"})
		return
	}
	if message.ReplyToMessage.Photo == nil {
		send(ctx, message, u, t.ERROR, t.T{"Err": "must be sent as a reply to an image"})
		return
	}
	photo := (*message.ReplyToMessage.Photo)[0]
	if photo.FileSize > 1000000 {
		send(ctx, message, u, t.ERROR, t.T{"Err": "image too large"})
		return
	}

	// prepare files (this is not really necessary, we should just load stuff from memory)
	inputpath := filepath.Join(os.TempDir(), "triangles-"+photo.FileID+"in.png")
	outputpath := filepath.Join(os.TempDir(), "triangles-"+photo.FileID+"out.png")
	defer os.RemoveAll(inputpath)
	defer os.RemoveAll(outputpath)

	// download file
	dl, err := bot.GetFile(tgbotapi.FileConfig{FileID: photo.FileID})
	if err != nil {
		send(ctx, message, u, t.ERROR, t.T{"Err": err.Error()})
		return
	}
	resp, err := http.Get(dl.Link(s.TelegramBotToken))
	if err != nil {
		send(ctx, message, u, t.ERROR, t.T{"Err": err.Error()})
		return
	}
	defer resp.Body.Close()
	file, err := os.Create(inputpath)
	if err != nil {
		send(ctx, message, u, t.ERROR, t.T{"Err": err.Error()})
		return
	}
	defer file.Close()
	if _, err := io.Copy(file, resp.Body); err != nil {
		send(ctx, message, u, t.ERROR, t.T{"Err": err.Error()})
		return
	}

	// generate primitive image
	rand.Seed(time.Now().UTC().UnixNano())
	input, err := primitive.LoadImage(inputpath)
	if _, err := io.Copy(file, resp.Body); err != nil {
		send(ctx, message, u, t.ERROR, t.T{"Err": err.Error()})
		return
	}
	size := uint(256)
	if size > 0 {
		input = resize.Thumbnail(size, size, input, resize.Bilinear)
	}
	bg := primitive.MakeColor(primitive.AverageImageColor(input))
	model := primitive.NewModel(input, bg, 1024, 1)
	for i := 0; i < int(n); i++ {
		model.Step(primitive.ShapeTypeTriangle, 128, 0)
	}
	if err := primitive.SavePNG(outputpath, model.Context.Image()); err != nil {
		send(ctx, message, u, t.ERROR, t.T{"Err": err.Error()})
		return
	}

	// send message
	sendable := tgbotapi.NewPhotoUpload(u.TelegramChatId, outputpath)
	sendable.BaseChat.ReplyToMessageID = message.MessageID
	imgMessage, err := bot.Send(sendable)
	if err != nil {
		send(ctx, message, u, t.ERROR, t.T{"Err": "failed to send result"})
		return
	}

	// subtract balance and notify
	if err := u.payToInternalService(ctx, msats, fmt.Sprintf("Trianglize image with %d triangles.", n), "triangles"); err == nil {
		send(ctx, imgMessage, u, t.PAIDMESSAGE, t.T{"Sats": float64(msats) / 1000})
		return
	}
}
