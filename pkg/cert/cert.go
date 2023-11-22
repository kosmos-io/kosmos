package cert

import (
	_ "embed"
	"encoding/base64"
)

//go:embed crt.pem
var crt []byte

//go:embed key.pem
var key []byte

func GetCrtEncode() string {
	return base64.StdEncoding.EncodeToString(crt)
}

func GetKeyEncode() string {
	return base64.StdEncoding.EncodeToString(key)
}

func GetCrt() []byte {
	return crt
}

func GetKey() []byte {
	return key
}
