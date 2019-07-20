package bech32

import "errors"

func LNURL(lnurl string) (url string, err error) {
	tag, data, err := Decode(lnurl)
	if err != nil {
		return
	}

	if tag != "lnurl" {
		err = errors.New("tag is not 'lnurl', but '" + tag + "'")
		return
	}

	converted, err := ConvertBits(data, 5, 8, false)
	if err != nil {
		return
	}

	url = string(converted)
	return
}
