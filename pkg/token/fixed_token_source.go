package token

type fixedTokenSourcePlugin struct {
	token string
}

func (t fixedTokenSourcePlugin) GetToken(addr string) (string, error) {
	return t.token, nil
}

func NewFixedTokenPlugin(tokenSource string) fixedTokenSourcePlugin {
	return fixedTokenSourcePlugin{token: tokenSource}
}
