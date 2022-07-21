package token

// Used to provide token to client endpoint
type TokenSourcePlugin interface {
	// GetToken return a token string according to the addr (client application address) parameter
	GetToken(addr string) (string, error)
}

// Used to parse token which form client endpoint
type TokenParserPlugin interface {
	// ParseToken parse the token and return the parse result
	ParseToken(token string) (string, error)
}
