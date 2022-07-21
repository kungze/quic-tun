package token

type fixedTokenSourcePlugin struct {
	token string
}

func (t fixedTokenSourcePlugin) GetToken(addr string) (string, error) {
	return t.token, nil
}

// NewFixedTokenPlugin return a ``Fixed`` type token source plugin.
// ``Fixed`` token source plugin will return a fixed token always, this mean that all
// client applications always assess same server application.
// The plugin directly return the value tokenSource
func NewFixedTokenPlugin(tokenSource string) fixedTokenSourcePlugin {
	return fixedTokenSourcePlugin{token: tokenSource}
}
