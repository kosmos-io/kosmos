package emop

import (
	"crypto"
	// nolint:gosec
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
)

type Signature struct {
	PublicKey  string
	PrivateKey string
}

func NewSignature(publicKey, privateKey string) *Signature {
	return &Signature{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}
}

func (s *Signature) Sign(m map[string]string) error {
	sig := url.Values{}
	for key, value := range m {
		sig.Add(key, value)
	}

	quUrl, err := url.QueryUnescape(sig.Encode())
	if err != nil {
		fmt.Println("QueryUnescape", err)
		return err
	}

	if out, err := RsaSignWithMd5(quUrl, s.PrivateKey); err != nil {
		return err
	} else {
		m["sign"] = out
		return nil
	}
}

func (s *Signature) Verify(m map[string]string) error {
	sign := m["sign"]
	delete(m, "sign")

	sig := url.Values{}
	for key, value := range m {
		sig.Add(key, value)
	}

	quUrl, _ := url.QueryUnescape(sig.Encode())

	return RsaVerifySignWithMd5(quUrl, sign, s.PublicKey)
}

func RsaSignWithMd5(data string, prvKey string) (sign string, err error) {
	prvKey = Base64URLDecode(prvKey)

	keyBytes, err := base64.StdEncoding.DecodeString(prvKey)
	if err != nil {
		fmt.Println("DecodeString:", err)
		return "", err
	}

	privateKey, err := x509.ParsePKCS8PrivateKey(keyBytes)
	if err != nil {
		fmt.Println("ParsePKCS8PrivateKey", err)
		return "", err
	}
	// nolint:gosec
	h := md5.New()
	h.Write([]byte(data))
	hash := h.Sum(nil)
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey.(*rsa.PrivateKey), crypto.MD5, hash[:])
	if err != nil {
		fmt.Println("SignPKCS1v15:", err)
		return "", err
	}

	out := base64.StdEncoding.EncodeToString(signature)
	return out, nil
}

func RsaVerifySignWithMd5(originalData, signData, pubKey string) error {
	sign, err := base64.StdEncoding.DecodeString(signData)
	if err != nil {
		fmt.Println("DecodeString:", err)
		return err
	}

	pubKey = Base64URLDecode(pubKey)

	public, err := base64.StdEncoding.DecodeString(pubKey)
	if err != nil {
		fmt.Println("DecodeString")
		return err
	}

	pub, err := x509.ParsePKIXPublicKey(public)
	if err != nil {
		fmt.Println("ParsePKIXPublicKey", err)
		return err
	}
	// nolint:gosec
	hash := md5.New()
	hash.Write([]byte(originalData))
	return rsa.VerifyPKCS1v15(pub.(*rsa.PublicKey), crypto.MD5, hash.Sum(nil), sign)
}

func Base64URLDecode(data string) string {
	var missing = (4 - len(data)%4) % 4
	data += strings.Repeat("=", missing)
	data = strings.Replace(data, "_", "/", -1)
	data = strings.Replace(data, "-", "+", -1)
	return data
}

func Base64UrlSafeEncode(data string) string {
	safeUrl := strings.Replace(data, "/", "_", -1)
	safeUrl = strings.Replace(safeUrl, "+", "-", -1)
	safeUrl = strings.Replace(safeUrl, "=", "", -1)
	return safeUrl
}
