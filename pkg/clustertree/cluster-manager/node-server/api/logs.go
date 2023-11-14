package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/node-server/api/errdefs"
)

type ContainerLogsHandlerFunc func(ctx context.Context, namespace, podName, containerName string, opts ContainerLogOpts) (io.ReadCloser, error)

type ContainerLogOpts struct {
	Tail         int
	LimitBytes   int
	Timestamps   bool
	Follow       bool
	Previous     bool
	SinceSeconds int
	SinceTime    time.Time
}

func parseLogOptions(q url.Values) (opts ContainerLogOpts, err error) {
	if tailLines := q.Get("tailLines"); tailLines != "" {
		opts.Tail, err = strconv.Atoi(tailLines)
		if err != nil {
			return opts, errdefs.ConvertToInvalidInput(errors.Wrap(err, "could not parse \"tailLines\""))
		}
		if opts.Tail < 0 {
			return opts, errdefs.InvalidInputf("\"tailLines\" is %d", opts.Tail)
		}
	}
	if follow := q.Get("follow"); follow != "" {
		opts.Follow, err = strconv.ParseBool(follow)
		if err != nil {
			return opts, errdefs.ConvertToInvalidInput(errors.Wrap(err, "could not parse \"follow\""))
		}
	}
	if limitBytes := q.Get("limitBytes"); limitBytes != "" {
		opts.LimitBytes, err = strconv.Atoi(limitBytes)
		if err != nil {
			return opts, errdefs.ConvertToInvalidInput(errors.Wrap(err, "could not parse \"limitBytes\""))
		}
		if opts.LimitBytes < 1 {
			return opts, errdefs.InvalidInputf("\"limitBytes\" is %d", opts.LimitBytes)
		}
	}
	if previous := q.Get("previous"); previous != "" {
		opts.Previous, err = strconv.ParseBool(previous)
		if err != nil {
			return opts, errdefs.ConvertToInvalidInput(errors.Wrap(err, "could not parse \"previous\""))
		}
	}
	if sinceSeconds := q.Get("sinceSeconds"); sinceSeconds != "" {
		opts.SinceSeconds, err = strconv.Atoi(sinceSeconds)
		if err != nil {
			return opts, errdefs.ConvertToInvalidInput(errors.Wrap(err, "could not parse \"sinceSeconds\""))
		}
		if opts.SinceSeconds < 1 {
			return opts, errdefs.InvalidInputf("\"sinceSeconds\" is %d", opts.SinceSeconds)
		}
	}
	if sinceTime := q.Get("sinceTime"); sinceTime != "" {
		opts.SinceTime, err = time.Parse(time.RFC3339, sinceTime)
		if err != nil {
			return opts, errdefs.ConvertToInvalidInput(errors.Wrap(err, "could not parse \"sinceTime\""))
		}
		if opts.SinceSeconds > 0 {
			return opts, errdefs.InvalidInput("both \"sinceSeconds\" and \"sinceTime\" are set")
		}
	}
	if timestamps := q.Get("timestamps"); timestamps != "" {
		opts.Timestamps, err = strconv.ParseBool(timestamps)
		if err != nil {
			return opts, errdefs.ConvertToInvalidInput(errors.Wrap(err, "could not parse \"timestamps\""))
		}
	}
	return opts, nil
}

func getContainerLogs(ctx context.Context, namespace string,
	podName string, containerName string, opts ContainerLogOpts, getClient getClientFunc) (io.ReadCloser, error) {
	tailLine := int64(opts.Tail)
	limitBytes := int64(opts.LimitBytes)
	sinceSeconds := opts.SinceSeconds
	options := &corev1.PodLogOptions{
		Container:  containerName,
		Timestamps: opts.Timestamps,
		Follow:     opts.Follow,
	}
	if tailLine != 0 {
		options.TailLines = &tailLine
	}
	if limitBytes != 0 {
		options.LimitBytes = &limitBytes
	}
	if !opts.SinceTime.IsZero() {
		*options.SinceTime = metav1.Time{Time: opts.SinceTime}
	}
	if sinceSeconds != 0 {
		*options.SinceSeconds = int64(sinceSeconds)
	}
	if opts.Previous {
		options.Previous = opts.Previous
	}
	if opts.Follow {
		options.Follow = opts.Follow
	}

	client, _, err := getClient(ctx, namespace, podName)

	if err != nil {
		return nil, fmt.Errorf("could not get the leaf client, podName: %s, namespace: %s, err: %v", podName, namespace, err)
	}

	logs := client.CoreV1().Pods(namespace).GetLogs(podName, options)
	stream, err := logs.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get stream from logs request: %v", err)
	}
	return stream, nil
}

func ContainerLogsHandler(getClient getClientFunc) http.HandlerFunc {
	return handleError(func(w http.ResponseWriter, req *http.Request) error {
		vars := mux.Vars(req)
		if len(vars) != 3 {
			return errdefs.NotFound("not found")
		}

		ctx := req.Context()

		namespace := vars[namespaceVar]
		pod := vars[podVar]
		container := vars[containerVar]

		query := req.URL.Query()
		opts, err := parseLogOptions(query)
		if err != nil {
			return err
		}

		logs, err := getContainerLogs(ctx, namespace, pod, container, opts, getClient)
		if err != nil {
			return errors.Wrap(err, "error getting container logs?)")
		}

		defer logs.Close()

		req.Header.Set("Transfer-Encoding", "chunked")

		if _, ok := w.(writeFlusher); !ok {
			klog.V(4).Info("http response writer does not support flushes")
		}

		if _, err := io.Copy(flushOnWrite(w), logs); err != nil {
			return errors.Wrap(err, "error writing response to client")
		}
		return nil
	})
}
