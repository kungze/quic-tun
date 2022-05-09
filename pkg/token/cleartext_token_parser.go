package token

import (
	"encoding/base64"
	"strings"
)

type cleartextTokerParser struct {
	enctype string
}

func (t cleartextTokerParser) ParseToken(token string) (string, error) {
	switch strings.ToLower(t.enctype) {
	case "base64":
		socket, err := base64.StdEncoding.DecodeString(token)
		if err != nil {
			return "", err
		} else {
			return string(socket), nil
		}
	default:
		return token, nil
	}
}

func NewCleartextTokenParser(key string) cleartextTokerParser {
	return cleartextTokerParser{enctype: key}
}
