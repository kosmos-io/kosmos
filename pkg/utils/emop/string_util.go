package emop

import (
	"encoding/json"
	"net/url"
	"strings"
)

const (
	DefaultEndpoint = "https://ecloud.10086.cn"
)

func PercentEncode(urlStr string) string {
	urlStr = url.QueryEscape(urlStr)

	urlStr = strings.ReplaceAll(urlStr, "+", "%20")

	urlStr = strings.ReplaceAll(urlStr, "*", "%2A")

	urlStr = strings.ReplaceAll(urlStr, "%7E", "~")

	return urlStr
}

func Beautify(i interface{}) string {
	resp, _ := json.MarshalIndent(i, "", "   ")

	return string(resp)
}

func ToJsonString(i interface{}) string {
	resp, _ := json.Marshal(i)

	return string(resp)
}
