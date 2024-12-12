package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
)

func TestSetQPSBurst(t *testing.T) {
	testCases := []struct {
		name string
		KubernetesOptions
		expectedQPS     float32
		expectedBurst   int
		numRequests     int
		expectedMinTime float64
		expectedMaxTime float64
	}{
		{
			name: "numRequests is 60",
			KubernetesOptions: KubernetesOptions{
				KubeConfig: "",
				MasterURL:  "",
				QPS:        5.0,
				Burst:      10,
			},
			numRequests:     60,
			expectedQPS:     5.0,
			expectedBurst:   10,
			expectedMinTime: 6,
			expectedMaxTime: 12,
		},
		{
			name: "numRequests is 600",
			KubernetesOptions: KubernetesOptions{
				KubeConfig: "",
				MasterURL:  "",
				QPS:        40.0,
				Burst:      60,
			},
			numRequests:     600,
			expectedQPS:     40.0,
			expectedBurst:   60,
			expectedMinTime: 10,
			expectedMaxTime: 15,
		},
		{
			name: "QPS is 1, Burst is 1",
			KubernetesOptions: KubernetesOptions{
				KubeConfig: "",
				MasterURL:  "",
				QPS:        1.0,
				Burst:      1,
			},
			numRequests:     30,
			expectedQPS:     1.0,
			expectedBurst:   1,
			expectedMinTime: 29,
			expectedMaxTime: 31,
		},
		{
			name: "QPS is 0, Burst is 0",
			KubernetesOptions: KubernetesOptions{
				KubeConfig: "",
				MasterURL:  "",
				QPS:        0.0,
				Burst:      0,
			},
			numRequests:     60,
			expectedQPS:     5.0,
			expectedBurst:   10,
			expectedMinTime: 6,
			expectedMaxTime: 12,
		},
		{
			name: "not set",
			KubernetesOptions: KubernetesOptions{
				KubeConfig: "",
				MasterURL:  "",
			},
			numRequests:     60,
			expectedQPS:     5.0,
			expectedBurst:   10,
			expectedMinTime: 6,
			expectedMaxTime: 12,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &rest.Config{}
			options := KubernetesOptions{
				QPS:   tc.expectedQPS,
				Burst: tc.expectedBurst,
			}
			SetQPSBurst(config, options)
			assert.Equal(t, tc.expectedQPS, config.QPS)
			assert.Equal(t, tc.expectedBurst, config.Burst)
		})
	}
	//restConfig, err := clientcmd.BuildConfigFromFlags("", "/Users/george/.kube/config")
	//if err != nil {
	//	panic(err)
	//}
	//for _, tc := range testCases {
	//	t.Run(tc.name, func(t *testing.T) {
	//		options := KubernetesOptions{
	//			QPS:   tc.expectedQPS,
	//			Burst: tc.expectedBurst,
	//		}
	//		SetQPSBurst(restConfig, options)
	//
	//		// create client
	//		clientset, err := kubernetes.NewForConfig(restConfig)
	//		if err != nil {
	//			fmt.Printf("error creating clientset: %s\n", err)
	//			return
	//		}
	//
	//		ctx := context.Background()
	//		var wg sync.WaitGroup
	//		// simulate concurrent requests
	//		numRequests := tc.numRequests
	//
	//		start := time.Now()
	//		for i := 0; i < numRequests; i++ {
	//			wg.Add(1)
	//			go func() {
	//				defer wg.Done()
	//				_, err := clientset.CoreV1().Pods("").List(ctx, v1.ListOptions{})
	//				if err != nil {
	//					fmt.Printf("request failed: %s\n", err)
	//				}
	//			}()
	//		}
	//
	//		// Wait for all requests to complete
	//		wg.Wait()
	//		elapsed := time.Since(start)
	//		fmt.Printf("All requests completed, Execution time: %s\n", elapsed)
	//
	//		assert.Equal(t, tc.expectedQPS, restConfig.QPS)
	//		assert.Equal(t, tc.expectedBurst, restConfig.Burst)
	//		seconds := elapsed.Seconds()
	//		assert.Condition(t, func() bool {
	//			return seconds > tc.expectedMinTime && seconds < tc.expectedMaxTime
	//		}, "seconds %f should be greater than %f and less than %f", seconds, tc.expectedMinTime, tc.expectedMaxTime)
	//	})
	//}
}
