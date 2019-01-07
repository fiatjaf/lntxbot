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
	Path string

	reqcount int
	waiting  map[string]chan gjson.Result
	conn     net.Conn
}

func Connect(path string) (*Client, error) {
	ln := &Client{Path: path}
	ln.waiting = make(map[string]chan gjson.Result)

	return ln, nil
}

func (ln *Client) reconnect() error {
	if ln.conn != nil {
		ln.conn.Close()
	}

	conn, err := net.Dial("unix", ln.Path)
	if err != nil {
		return err
	}

	ln.conn = conn
	return nil
}

func (ln *Client) Listen(errorHandler func(error)) {
	err := ln.reconnect()
	if err != nil {
		errorHandler(err)
		return
	}

	errored := make(chan bool, 1)

	go func(conn net.Conn) {
		for {
			message := make([]byte, 4096)
			length, err := conn.Read(message)
			if err != nil {
				errorHandler(err)
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
				errorHandler(err)
				continue
			}

			if respchan, ok := ln.waiting[response.Id]; ok {
				log.Print("got response from lightningd: " + string(response.Result))
				respchan <- gjson.ParseBytes(response.Result)
				delete(ln.waiting, response.Id)
			} else {
				errorHandler(errors.New("got response without a waiting caller: " +
					string(message)))
				continue
			}
		}
	}(ln.conn)

	go func() {
		select {
		case <-errored:
			log.Print("error break")
			// start again after an error break
			ln.Listen(errorHandler)
		}
	}()
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
