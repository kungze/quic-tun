package token

type TokenSourcePlugin interface {
	GetToken(addr string) (string, error)
}

type TokenParsePlugin interface {
	ParseToken(token string) (string, error)
}
