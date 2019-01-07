package lightning

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net"
	"strconv"
	"time"
	"unicode"

	"github.com/tidwall/gjson"
)

type Client struct {
	Path         string
	ErrorHandler func(error)

	reqcount int
	waiting  map[string]chan gjson.Result
	conn     net.Conn
}

func Connect(path string, errorHandler func(error)) (*Client, error) {
	ln := &Client{Path: path, ErrorHandler: errorHandler}
	ln.waiting = make(map[string]chan gjson.Result)

	err := ln.connect()
	if err != nil {
		return ln, err
	}

	err = ln.listen()
	return ln, err
}

func (ln *Client) reconnect() error {
	if ln.conn != nil {
		err := ln.conn.Close()
		if err != nil {
			log.Print("error closing old connection: " + err.Error())
		}
	}

	return ln.connect()
}

func (ln *Client) connect() error {
	conn, err := net.Dial("unix", ln.Path)
	if err != nil {
		return err
	}

	ln.conn = conn
	return nil
}

func (ln *Client) listen() error {
	errored := make(chan bool, 1)
	go func() {
		for {
			message := make([]byte, 4096)
			length, err := ln.conn.Read(message)
			if err != nil {
				ln.ErrorHandler(err)
				errored <- true
				break
			}
			if length == 0 {
				continue
			}

			var messagerunes []byte
			for _, r := range bytes.Runes(message) {
				if unicode.IsGraphic(r) {
					messagerunes = append(messagerunes, byte(r))
				}
			}

			var response JSONRPCResponse
			err = json.Unmarshal(messagerunes, &response)
			if err != nil || response.Error.Code != 0 {
				if response.Error.Code != 0 {
					err = errors.New(response.Error.Message)
				}
				ln.ErrorHandler(err)
				continue
			}

			if respchan, ok := ln.waiting[response.Id]; ok {
				log.Print("got response from lightningd: " + string(response.Result))
				respchan <- gjson.ParseBytes(response.Result)
				delete(ln.waiting, response.Id)
			} else {
				ln.ErrorHandler(
					errors.New("got response without a waiting caller: " +
						string(message)))
				continue
			}
		}
	}()

	go func() {
		select {
		case <-errored:
			log.Print("error break")

			// start again after an error break
			ln.reconnect()
		}
	}()

	return nil
}

func (ln *Client) Call(method string, params ...string) (gjson.Result, error) {
	id := strconv.Itoa(ln.reqcount)

	if params == nil {
		params = make([]string, 0)
	}

	message, _ := json.Marshal(JSONRPCMessage{
		Version: VERSION,
		Id:      id,
		Method:  method,
		Params:  params,
	})

	respchan := make(chan gjson.Result, 1)
	ln.waiting[id] = respchan

	log.Print("writing to lightningd: " + string(message))

	ln.reqcount++
	ln.conn.Write(message)

	select {
	case v := <-respchan:
		return v, nil
	case <-time.After(3 * time.Second):
		log.Print(ln.reconnect())
		return gjson.Result{}, errors.New("timeout")
	}
}

const VERSION = "2.0"

type JSONRPCMessage struct {
	Version string   `json:"jsonrpc"`
	Id      string   `json:"id"`
	Method  string   `json:"method"`
	Params  []string `json:"params"`
}

type JSONRPCResponse struct {
	Version string          `json:"jsonrpc"`
	Id      string          `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}
