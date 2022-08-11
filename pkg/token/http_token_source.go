package token

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
)

type httpTokenSourcePlugin struct {
	urlPath string
}

type result struct {
	Token string `json:"token"`
}

func (t httpTokenSourcePlugin) GetToken(addr string) (string, error) {
	params := url.Values{}
	url, err := url.Parse(t.urlPath)
	if err != nil {
		return "", err
	}
	params.Set("addr", addr)
	url.RawQuery = params.Encode()
	urlPath := url.String()
	resp, err := http.Get(urlPath)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var res result
	_ = json.Unmarshal(body, &res)
	return res.Token, nil
}

// NewHttpTokenPlugin return a ``Http`` type token source plugin.
// ``Http`` token source plugin will initiate an http request and
// get the token based on addr
func NewHttpTokenPlugin(tokenSource string) httpTokenSourcePlugin {
	return httpTokenSourcePlugin{urlPath: tokenSource}
}
