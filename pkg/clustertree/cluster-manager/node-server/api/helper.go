package api

import (
	"context"
	"io"
	"net/http"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/node-server/api/errdefs"
)

const (
	execTTYParam    = "tty"
	execStdinParam  = "input"
	execStdoutParam = "output"
	execStderrParam = "error"
	namespaceVar    = "namespace"
	podVar          = "pod"
	containerVar    = "container"
	commandVar      = "command"
)

type handlerFunc func(http.ResponseWriter, *http.Request) error

type getClientFunc func(ctx context.Context, namespace string, podName string) (kubernetes.Interface, *rest.Config, error)

func handleError(f handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		err := f(w, req)
		if err == nil {
			return
		}

		code := httpStatusCode(err)
		w.WriteHeader(code)
		if _, err := io.WriteString(w, err.Error()); err != nil {
			klog.Error("error writing error response")
		}

		if code >= 500 {
			klog.Error("Internal server error on request")
		} else {
			klog.Error("Error on request")
		}
	}
}

func flushOnWrite(w io.Writer) io.Writer {
	if fw, ok := w.(writeFlusher); ok {
		return &flushWriter{fw}
	}
	return w
}

type flushWriter struct {
	w writeFlusher
}

type writeFlusher interface {
	Flush()
	Write([]byte) (int, error)
}

func (fw *flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if n > 0 {
		fw.w.Flush()
	}
	return n, err
}

func httpStatusCode(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errdefs.IsNotFound(err):
		return http.StatusNotFound
	case errdefs.IsInvalidInput(err):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func NotImplemented(w http.ResponseWriter, r *http.Request) {
	klog.Warning("501 not implemented")
	http.Error(w, "501 not implemented", http.StatusNotImplemented)
}

func NotFound(w http.ResponseWriter, r *http.Request) {
	klog.Warningf("404 request not found, url: %s", r.URL)
	http.Error(w, "404 request not found", http.StatusNotFound)
}
