package emop

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

const (
	GET    = "GET"
	POST   = "POST"
	DELETE = "DELETE"
	PUT    = "PUT"
	PATCH  = "PATCH"
	HEAD   = "HEAD"
)

var METHOD_MAP = map[string]string{
	GET:    GET,
	POST:   POST,
	DELETE: DELETE,
	PUT:    PUT,
	PATCH:  PATCH,
	HEAD:   HEAD,
}

type QueryParams map[string]string
type HeaderParams map[string]string
type BodyParams map[string]interface{}

type EMOPApiResponse struct {
	RespCode string      `json:"respCode,omitempty"`
	RespDesc string      `json:"respDesc,omitempty"`
	Result   interface{} `json:"result,omitempty"`
}

func (r *EMOPApiResponse) GetError() string {
	return fmt.Sprintf("code: %s, desc: %s", r.RespCode, r.RespDesc)
}

type EMOPClient struct {
	config    EMOPClientConfig
	client    http.Client
	signature *Signature
}

type EMOPClientConfig struct {
	Url        string
	PublicKey  string
	PrivateKey string
	AppId      string
	Format     string
	Status     string
	Key        []byte
}

type EMOPApiParams struct {
	HeaderParams map[string]string
	QueryParams  map[string]string
	BodyParams   BodyParams
	PathParams   map[string]string
}

// nolint
func NewEMOPClient(config EMOPClientConfig) EMOPClient {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := http.Client{
		Timeout:   40 * time.Second,
		Transport: tr,
	}

	if config.Url == "" {
		config.Url = DefaultEndpoint
	}

	return EMOPClient{
		config:    config,
		client:    client,
		signature: NewSignature(config.PublicKey, config.PrivateKey),
	}
}

func (c *EMOPClient) exec(request *http.Request, headerParams map[string]string) (*EMOPApiResponse, error) {
	request.Header.Set("Connection", "Keep-Alive")
	request.Header.Set("User-Agent", "OpenAPI/2.0/Golang")
	request.Header.Set("Accept-Encoding", "gzip")
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	for v, k := range headerParams {
		request.Header.Set(v, k)
	}

	response, err := c.client.Do(request)

	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(response.Body)

	if err != nil {
		if response.Body != nil {
			_ = response.Body.Close()
		}
		return nil, err
	}

	if len(c.config.Key) > 0 {
		body = Decrypt(body, c.config.Key)
	}

	data := &EMOPApiResponse{}

	err = json.Unmarshal(body, data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (c *EMOPClient) Get(urlTemplate string, params EMOPApiParams, r interface{}) (*EMOPApiResponse, error) {
	return c.Request(GET, urlTemplate, params, r)
}

func (c *EMOPClient) Post(urlTemplate string, params EMOPApiParams, r interface{}) (*EMOPApiResponse, error) {
	return c.Request(POST, urlTemplate, params, r)
}

func (c *EMOPClient) Delete(urlTemplate string, params EMOPApiParams, r interface{}) (*EMOPApiResponse, error) {
	return c.Request(DELETE, urlTemplate, params, r)
}

func (c *EMOPClient) Put(urlTemplate string, params EMOPApiParams, r interface{}) (*EMOPApiResponse, error) {
	return c.Request(PUT, urlTemplate, params, r)
}

func (c *EMOPClient) Head(urlTemplate string, params EMOPApiParams, r interface{}) (*EMOPApiResponse, error) {
	return c.Request(HEAD, urlTemplate, params, r)
}

func (c *EMOPClient) Path(urlTemplate string, params EMOPApiParams, r interface{}) (*EMOPApiResponse, error) {
	return c.Request(PATCH, urlTemplate, params, r)
}

// conver urlTemplate string to url
func ConvertUrl(urlTemplate string, pathParams map[string]string) string {
	latestUrl := ""
	if len(pathParams) == 0 {
		latestUrl = urlTemplate
	} else {
		latestUrl = os.Expand(urlTemplate, func(key string) string {
			if v, ok := pathParams[key]; ok {
				return v
			} else {
				// to remind you, I won't replace the placeholders
				return fmt.Sprintf("${%s}", key)
			}
		})
	}
	return latestUrl
}

func ValidateMethod(method string) error {
	if _, ok := METHOD_MAP[method]; !ok {
		return errors.New("method not supported")
	}
	return nil
}

func ValidateQueryParam(query map[string]string) error {
	if _, ok := query["method"]; !ok {
		return errors.New("emop query params method is nil")
	}
	return nil
}

// do request
//
// @param method GET POST DELETE PUT PATCH HEAD
//
// @param urlTemplate   url template
//
// @param params EMOPApiParams include head query body param
//
// @param r   convert response to type `r`
//
// @return EMOPApiResponse  whole response
func (c *EMOPClient) Request(method string, urlTemplate string, params EMOPApiParams, r interface{}) (*EMOPApiResponse, error) {
	queryParams, headerParams, bodyParams, pathParams := params.QueryParams, params.HeaderParams, params.BodyParams, params.PathParams

	// validate
	if err := ValidateMethod(method); err != nil {
		return nil, err
	}

	if err := ValidateQueryParam(queryParams); err != nil {
		return nil, err
	}

	latestUrl := ConvertUrl(urlTemplate, pathParams)

	m := make(map[string]string)
	m["appId"] = c.config.AppId
	m["method"] = queryParams["method"]
	m["format"] = c.config.Format
	m["status"] = c.config.Status

	if err := c.signature.Sign(m); err != nil {
		return nil, err
	}

	np := url.Values{}

	for k, v := range m {
		np.Add(k, v)
	}

	path := fmt.Sprintf("%s?%s", latestUrl, np.Encode())

	var body io.Reader

	if len(bodyParams) > 0 {
		jsonData, err := json.Marshal(bodyParams)

		if len(c.config.Key) > 0 {
			jsonData = Encrypt(jsonData, c.config.Key)
		}
		if err != nil {
			return nil, err
		}
		body = bytes.NewBuffer(jsonData)
	}

	request, err := http.NewRequest(method, c.config.Url+path, body)

	if err != nil {
		return nil, err
	}

	response, err := c.exec(request, headerParams)

	if err != nil {
		return nil, err
	}

	if response.RespDesc == "ok" {
		if r == nil {
			return response, nil
		}
		jsonData, err := json.Marshal(response.Result)
		if err != nil {
			return response, err
		}

		err = json.Unmarshal([]byte(jsonData), &r)
		if err != nil {
			fmt.Print(Encrypt(nil, nil))
			return response, err
		}
	} else {
		return response, errors.New(response.GetError())
	}
	return response, nil
}
