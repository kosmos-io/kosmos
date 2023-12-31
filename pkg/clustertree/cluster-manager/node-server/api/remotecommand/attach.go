// This code is directly lifted from the Kubernetes
// For reference:
// https://github.com/kubernetes/kubernetes/staging/src/k8s.io/kubelet/pkg/cri/streaming/remotecommand/attach.go

package remotecommand

import (
	"fmt"
	"io"
	"net/http"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	remotecommandconsts "k8s.io/apimachinery/pkg/util/remotecommand"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/remotecommand"
	utilexec "k8s.io/utils/exec"
)

// Attacher knows how to attach to a container in a pod.
type Attacher interface {
	// AttachToContainer attaches to a container in the pod, copying data
	// between in/out/err and the container's stdin/stdout/stderr.
	AttachToContainer(name string, uid types.UID, container string, in io.Reader, out, err io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error
}

// ServeAttach handles requests to attach to a container. After
// creating/receiving the required streams, it delegates the actual attachment
// to the attacher.
func ServeAttach(w http.ResponseWriter, req *http.Request, attacher Attacher, podName string, uid types.UID, container string, streamOpts *Options, idleTimeout, streamCreationTimeout time.Duration, supportedProtocols []string) {
	ctx, ok := createStreams(req, w, streamOpts, supportedProtocols, idleTimeout, streamCreationTimeout)
	if !ok {
		// error is handled by createStreams
		return
	}
	defer ctx.conn.Close()

	err := attacher.AttachToContainer(podName, uid, container, ctx.stdinStream, ctx.stdoutStream, ctx.stderrStream, ctx.tty, ctx.resizeChan, 0)
	if err != nil {
		if exitErr, ok := err.(utilexec.ExitError); ok && exitErr.Exited() {
			rc := exitErr.ExitStatus()
			// nolint:errcheck
			ctx.writeStatus(&apierrors.StatusError{ErrStatus: metav1.Status{
				Status: metav1.StatusFailure,
				Reason: remotecommandconsts.NonZeroExitCodeReason,
				Details: &metav1.StatusDetails{
					Causes: []metav1.StatusCause{
						{
							Type:    remotecommandconsts.ExitCodeCauseType,
							Message: fmt.Sprintf("%d", rc),
						},
					},
				},
				Message: fmt.Sprintf("command terminated with non-zero exit code: %v", exitErr),
			}})
			return
		}
		err = fmt.Errorf("error attaching to container: %v", err)
		runtime.HandleError(err)
		// nolint:errcheck
		ctx.writeStatus(apierrors.NewInternalError(err))
		return
	}
	// nolint:errcheck
	ctx.writeStatus(&apierrors.StatusError{ErrStatus: metav1.Status{
		Status: metav1.StatusSuccess,
	}})
}
