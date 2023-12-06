package utils

import (
	"encoding/json"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/controllers/pod/leaf-pod/serverless/model"
	"github.com/kosmos.io/kosmos/pkg/utils"
	"github.com/kosmos.io/kosmos/pkg/utils/openapi"
)

type ServerlessClient struct {
	openapi.ApiClient
	PoolIp string
}

const KosmosServerlessAppendLabel = "kosmos.io/serverless.append"

func NewServerlessClient(accessKey, secretKey, poolId, url string) *ServerlessClient {
	apiClient := openapi.NewApiClient(openapi.ApiClientConfig{
		AccessKey: accessKey,
		SecretKey: secretKey,
		Url:       url,
	})
	return &ServerlessClient{
		apiClient, poolId,
	}
}

func (s *ServerlessClient) ListEciContainer(eciName string, total int) ([]model.ECIContainerGet, error) {
	queryParams := make(map[string]string)
	headerParams := make(map[string]string)
	headerParams["Pool-Id"] = s.PoolIp
	num := total
	queryParams["maxResults"] = strconv.Itoa(num)
	queryParams[utils.KosmosPodLabel] = "true"
	if len(eciName) > 0 {
		queryParams["name"] = eciName
	}

	pr := model.ListResult{}

	if _, err := s.Request(openapi.GET, "/api/web/eci-backend-service/containergroup", openapi.OpenApiParams{
		QueryParams:  queryParams,
		HeaderParams: headerParams,
	}, &pr); err != nil {
		return nil, err
	}

	return pr.Content, nil
}

func (s *ServerlessClient) GetEciContainer(eciName string) (*model.ECIContainerGet, error) {
	containers, err := s.ListEciContainer(eciName, 2)
	if err != nil {
		return nil, err
	}

	if len(containers) > 1 {
		return nil, fmt.Errorf("server eci containers len > 1")
	}

	if len(containers) == 0 {
		return nil, errors.NewNotFound(v1alpha1.Resource("cluster"), eciName)
	}
	return &containers[0], nil
}

func (s *ServerlessClient) GetPods(eciName string, orderId string) (*corev1.Pod, error) {
	eciContainer, err := s.GetEciContainer(eciName)
	if err != nil {
		if errors.IsNotFound(err) {
			// TODO: query order by emop
			return nil, fmt.Errorf("order failed %s, ", orderId)
		}
		return nil, err
	}

	return eciContainer.ToK8sPod()
}

func (s *ServerlessClient) CreatePod(eciName string, pod *corev1.Pod) (*model.CreateResult, error) {
	eciContainer, err := model.NewECIContainer(pod)
	if err != nil {
		return nil, fmt.Errorf("create eci container failed, err: %s", err)
	}

	queryParams := make(map[string]string)
	headerParams := make(map[string]string)
	bodyParams := make(openapi.BodyParams)

	jsonData, err := json.Marshal(eciContainer)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}

	err = json.Unmarshal([]byte(jsonData), &bodyParams)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}

	headerParams["Pool-Id"] = s.PoolIp

	orderInfo := &model.CreateResult{}

	if _, err := s.Request(openapi.POST, "/api/web/eci-backend-service/open/containergroup", openapi.OpenApiParams{
		QueryParams:  queryParams,
		HeaderParams: headerParams,
		BodyParams:   bodyParams,
	}, orderInfo); err != nil {
		return nil, err
	}

	return orderInfo, nil
}

func (s *ServerlessClient) DeletePod(eciName string) error {
	eciContainer, err := s.GetEciContainer(eciName)
	if err != nil {
		return err
	}

	queryParams := make(map[string]string)
	headerParams := make(map[string]string)
	pathParams := make(map[string]string)
	pathParams["eciId"] = eciContainer.EciId
	headerParams["Pool-Id"] = s.PoolIp

	deleteBody := model.DeleteResult(false)

	if _, err := s.Request(openapi.DELETE, "/api/web/eci-backend-service/containergroup/${eciId}", openapi.OpenApiParams{
		QueryParams:  queryParams,
		HeaderParams: headerParams,
		PathParams:   pathParams,
	}, &deleteBody); err != nil {
		return err
	}

	return nil
}

func (s *ServerlessClient) DoUpdate(cb func(string)) error {
	// TODO: list all
	containers, err := s.ListEciContainer("", 20)
	if err != nil {
		return err
	}

	for _, c := range containers {
		if c.Labels == nil {
			continue
		}
		if c.Labels[utils.KosmosPodLabel] == "true" {
			cb(c.Name)
		}
	}
	return nil
}
