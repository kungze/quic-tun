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

// NewCleartextTokenParserPlugin return a ``Cleartext`` type token parser plugin.
// The token parser plugin require the token from client endpoint mustn't be encrypted.
// The key specify the token's enctype, it can be ``base64`` or a ""(null chart string).
func NewCleartextTokenParserPlugin(key string) cleartextTokerParser {
	return cleartextTokerParser{enctype: key}
}
