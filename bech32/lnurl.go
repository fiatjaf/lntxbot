package bech32

import "errors"

func LNURLDecode(lnurl string) (url string, err error) {
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

func LNURLEncode(actualurl string) (lnurl string, err error) {
	asbytes := []byte(actualurl)
	converted, err := ConvertBits(asbytes, 8, 5, true)
	if err != nil {
		return
	}

	lnurl, err = Encode("lnurl", converted)
	return
}
