//go:build jsoniter
// +build jsoniter

package json

import jsoniter "github.com/json-iterator/go"

type RawMessage = jsoniter.RawMessage

var (
	json          = jsoniter.ConfigCompatibleWithStandardLibrary
	Marshal       = json.Marshal
	Unmarshal     = json.Unmarshal
	MarshalIndent = json.MarshalIndent
	NewDecoder    = json.NewDecoder
	NewEncoder    = json.NewEncoder
)
