package openapi

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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

type OpenApiResponse struct {
	RequestId    string      `json:"requestId,omitempty"`
	State        string      `json:"state,omitempty"`
	ErrorCode    string      `json:"errorCode,omitempty"`
	ErrorMessage string      `json:"errorMessage,omitempty"`
	ErrorDetail  string      `json:"errorDetail,omitempty"`
	Body         interface{} `json:"body,omitempty"`
}

func (r *OpenApiResponse) GetError() string {
	return fmt.Sprintf("code: %s, message: %s, detail: %s", r.ErrorCode, r.ErrorMessage, r.ErrorDetail)
}

type ApiClient struct {
	config ApiClientConfig
	client http.Client
}

type ApiClientConfig struct {
	Url       string
	AccessKey string
	SecretKey string
}

type OpenApiParams struct {
	HeaderParams map[string]string
	QueryParams  map[string]string
	BodyParams   BodyParams
	PathParams   map[string]string
}

// nolint
func NewApiClient(config ApiClientConfig) ApiClient {
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

	return ApiClient{
		config: config,
		client: client,
	}
}

func (c *ApiClient) exec(request *http.Request, headerParams map[string]string) (*OpenApiResponse, error) {
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

	data := &OpenApiResponse{}

	err = json.Unmarshal(body, data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (c *ApiClient) Get(urlTemplate string, params OpenApiParams, r interface{}) (*OpenApiResponse, error) {
	return c.Request(GET, urlTemplate, params, r)
}

func (c *ApiClient) Post(urlTemplate string, params OpenApiParams, r interface{}) (*OpenApiResponse, error) {
	return c.Request(POST, urlTemplate, params, r)
}

func (c *ApiClient) Delete(urlTemplate string, params OpenApiParams, r interface{}) (*OpenApiResponse, error) {
	return c.Request(DELETE, urlTemplate, params, r)
}

func (c *ApiClient) Put(urlTemplate string, params OpenApiParams, r interface{}) (*OpenApiResponse, error) {
	return c.Request(PUT, urlTemplate, params, r)
}

func (c *ApiClient) Head(urlTemplate string, params OpenApiParams, r interface{}) (*OpenApiResponse, error) {
	return c.Request(HEAD, urlTemplate, params, r)
}

func (c *ApiClient) Path(urlTemplate string, params OpenApiParams, r interface{}) (*OpenApiResponse, error) {
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

// do request
//
// @param method GET POST DELETE PUT PATCH HEAD
//
// @param urlTemplate   url template
//
// @param params OpenApiParams include head query body param
//
// @param r   convert response to type `r`
//
// @return OpenApiResponse  whole response
func (c *ApiClient) Request(method string, urlTemplate string, params OpenApiParams, r interface{}) (*OpenApiResponse, error) {
	queryParams, headerParams, bodyParams, pathParams := params.QueryParams, params.HeaderParams, params.BodyParams, params.PathParams

	// validate
	if err := ValidateMethod(method); err != nil {
		return nil, err
	}

	latestUrl := ConvertUrl(urlTemplate, pathParams)

	path, err := Sign(queryParams, latestUrl, method, c.config.AccessKey, c.config.SecretKey)
	if err != nil {
		return nil, err
	}

	var body io.Reader

	if len(bodyParams) > 0 {
		jsonData, err := json.Marshal(bodyParams)
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

	if response.State == "OK" {
		if r == nil {
			return response, nil
		}
		jsonData, err := json.Marshal(response.Body)
		if err != nil {
			return response, err
		}

		err = json.Unmarshal([]byte(jsonData), &r)
		if err != nil {
			return response, err
		}
	} else {
		return response, errors.New(response.GetError())
	}
	return response, nil
}
