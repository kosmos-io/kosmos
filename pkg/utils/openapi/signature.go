package openapi

// nolint
import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	AccessKey       = "AccessKey"
	TIMESTAMP       = "Timestamp"
	Version         = "Version"
	TimestampFormat = "2006-01-02T15:04:05Z"
	// TimestampFormat       = "2017-01-11T15:15:11Z"
	Signature             = "Signature"
	SecretKeyPrefix       = "BC_SIGNATURE&"
	SignatureMethod       = "SignatureMethod"
	SignatureMethodValue  = "HmacSHA1"
	SignatureVersion      = "SignatureVersion"
	SignatureVersionValue = "V2.0"
	SignatureNonce        = "SignatureNonce"
	LineSeparator         = "\n"
	ParameterSeparator    = "&"
	QueryStartSymbol      = "?"
	QuerySeparator        = "="
	VersionValue          = "2016-12-05"
)
const (
	HighMask = 0xf0
	LowMask  = 0x0f
)

var HexCodeTable = []string{
	"0", "1", "2", "3",
	"4", "5", "6", "7",
	"8", "9", "a", "b",
	"c", "d", "e", "f",
}

func Sign(queryparams QueryParams, path string, method string, accessKey string, secretKey string) (string, error) {
	params := make(map[string]string)
	for key, value := range queryparams {
		params[key] = value
	}
	// params[Version] = VersionValue
	params[AccessKey] = accessKey
	now := time.Now()
	params[TIMESTAMP] = now.Format(TimestampFormat)
	params[SignatureMethod] = SignatureMethodValue
	params[SignatureVersion] = SignatureVersionValue
	params[SignatureNonce] = nonce()
	// params[SignatureNonce] = "9d81ffbeaaf7477390db5df577bb3299"
	// 9d81ffbeaaf7477390db5df577bb3299
	keys := make([]string, len(params))
	index := 0
	for key := range params {
		keys[index] = key
		index++
	}
	sort.Strings(keys)
	builder := strings.Builder{}
	pos := 0
	paramsLen := len(keys)
	for _, key := range keys {
		value := params[key]
		builder.WriteString(PercentEncode(key))
		builder.WriteString(QuerySeparator)
		builder.WriteString(PercentEncode(value))
		if pos != paramsLen-1 {
			builder.WriteString(ParameterSeparator)
			pos++
		}
	}
	canonicalQueryString := builder.String()

	hashString := convertToHexString(sha256Encode(canonicalQueryString))

	unescapedPath, err := url.QueryUnescape(path)
	if nil != err {
		return "", err
	}
	builder.Reset()
	builder.WriteString(strings.ToUpper(method))
	builder.WriteString(LineSeparator)
	builder.WriteString(PercentEncode(unescapedPath))
	builder.WriteString(LineSeparator)
	builder.WriteString(hashString)
	stringToSign := builder.String()

	signature := convertToHexString(hmacSha1(stringToSign, SecretKeyPrefix+secretKey))

	builder.Reset()
	builder.WriteString(unescapedPath)
	builder.WriteString(QueryStartSymbol)
	builder.WriteString(canonicalQueryString)
	builder.WriteString(ParameterSeparator)
	builder.WriteString(Signature)
	builder.WriteString(QuerySeparator)
	builder.WriteString(PercentEncode(signature))
	return builder.String(), nil
}

func hmacSha1(text string, keyStr string) []byte {
	key := []byte(keyStr)
	mac := hmac.New(sha1.New, key)
	mac.Write([]byte(text))
	return mac.Sum(nil)
}

func convertToHexString(data []byte) string {
	if data == nil {
		return ""
	}
	builder := strings.Builder{}
	for _, d := range data {
		builder.WriteString(HexCodeTable[(HighMask&d)>>4])
		builder.WriteString(HexCodeTable[LowMask&d])
	}
	return builder.String()
}

func sha256Encode(text string) []byte {
	h := sha256.New()
	h.Write([]byte(text))
	return h.Sum(nil)
}

func nonce() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}
