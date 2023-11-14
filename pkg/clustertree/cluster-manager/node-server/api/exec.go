package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes/scheme"
	remoteutils "k8s.io/client-go/tools/remotecommand"

	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/node-server/api/errdefs"
	"github.com/kosmos.io/kosmos/pkg/clustertree/cluster-manager/node-server/api/remotecommand"
)

type execIO struct {
	tty      bool
	stdin    io.Reader
	stdout   io.WriteCloser
	stderr   io.WriteCloser
	chResize chan TermSize
}

type ContainerExecHandlerFunc func(ctx context.Context, namespace, podName, containerName string, cmd []string, attach AttachIO, getClient getClientFunc) error

func (e *execIO) TTY() bool {
	return e.tty
}

func (e *execIO) Stdin() io.Reader {
	return e.stdin
}

func (e *execIO) Stdout() io.WriteCloser {
	return e.stdout
}

func (e *execIO) Stderr() io.WriteCloser {
	return e.stderr
}

func (e *execIO) Resize() <-chan TermSize {
	return e.chResize
}

type containerExecutor struct {
	h                         ContainerExecHandlerFunc
	namespace, pod, container string
	ctx                       context.Context
	getClient                 getClientFunc
}

func (c *containerExecutor) ExecInContainer(name string, uid types.UID, container string, cmd []string, in io.Reader, out, err io.WriteCloser, tty bool, resize <-chan remoteutils.TerminalSize, timeout time.Duration) error {
	eio := &execIO{
		tty:    tty,
		stdin:  in,
		stdout: out,
		stderr: err,
	}

	if tty {
		eio.chResize = make(chan TermSize)
	}

	ctx, cancel := context.WithCancel(c.ctx)
	defer cancel()

	if tty {
		go func() {
			send := func(s remoteutils.TerminalSize) bool {
				select {
				case eio.chResize <- TermSize{Width: s.Width, Height: s.Height}:
					return false
				case <-ctx.Done():
					return true
				}
			}

			for {
				select {
				case s := <-resize:
					if send(s) {
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	return c.h(c.ctx, c.namespace, c.pod, c.container, cmd, eio, c.getClient)
}

type AttachIO interface {
	Stdin() io.Reader
	Stdout() io.WriteCloser
	Stderr() io.WriteCloser
	TTY() bool
	Resize() <-chan TermSize
}

type TermSize struct {
	Width  uint16
	Height uint16
}

type termSize struct {
	attach AttachIO
}

func (t *termSize) Next() *remoteutils.TerminalSize {
	resize := <-t.attach.Resize()
	return &remoteutils.TerminalSize{
		Height: resize.Height,
		Width:  resize.Width,
	}
}

type ContainerExecOptions struct {
	StreamIdleTimeout     time.Duration
	StreamCreationTimeout time.Duration
}

func getVarFromReq(req *http.Request) (string, string, string, []string, []string) {
	vars := mux.Vars(req)
	namespace := vars[namespaceVar]
	pod := vars[podVar]
	container := vars[containerVar]

	supportedStreamProtocols := strings.Split(req.Header.Get(httpstream.HeaderProtocolVersion), ",")

	q := req.URL.Query()
	command := q[commandVar]

	return namespace, pod, container, supportedStreamProtocols, command
}

func getExecOptions(req *http.Request) (*remotecommand.Options, error) {
	tty := req.FormValue(execTTYParam) == "1"
	stdin := req.FormValue(execStdinParam) == "1"
	stdout := req.FormValue(execStdoutParam) == "1"
	stderr := req.FormValue(execStderrParam) == "1"

	if tty && stderr {
		return nil, errors.New("cannot exec with tty and stderr")
	}

	if !stdin && !stdout && !stderr {
		return nil, errors.New("you must specify at least one of stdin, stdout, stderr")
	}
	return &remotecommand.Options{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		TTY:    tty,
	}, nil
}

func execInContainer(ctx context.Context, namespace string, podName string, containerName string, cmd []string, attach AttachIO, getClient getClientFunc) error {
	defer func() {
		if attach.Stdout() != nil {
			attach.Stdout().Close()
		}
		if attach.Stderr() != nil {
			attach.Stderr().Close()
		}
	}()

	client, config, err := getClient(ctx, namespace, podName)

	if err != nil {
		return fmt.Errorf("could not get the leaf client, podName: %s, namespace: %s, err: %v", podName, namespace, err)
	}

	req := client.CoreV1().RESTClient().
		Post().
		Namespace(namespace).
		Resource("pods").
		Name(podName).
		SubResource("exec").
		Timeout(0).
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   cmd,
			Stdin:     attach.Stdin() != nil,
			Stdout:    attach.Stdout() != nil,
			Stderr:    attach.Stderr() != nil,
			TTY:       attach.TTY(),
		}, scheme.ParameterCodec)

	exec, err := remoteutils.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("could not make remote command: %v", err)
	}

	ts := &termSize{attach: attach}

	err = exec.StreamWithContext(ctx, remoteutils.StreamOptions{
		Stdin:             attach.Stdin(),
		Stdout:            attach.Stdout(),
		Stderr:            attach.Stderr(),
		Tty:               attach.TTY(),
		TerminalSizeQueue: ts,
	})

	if err != nil {
		return err
	}

	return nil
}

func ContainerExecHandler(cfg ContainerExecOptions, getClient getClientFunc) http.HandlerFunc {
	return handleError(func(w http.ResponseWriter, req *http.Request) error {
		namespace, pod, container, supportedStreamProtocols, command := getVarFromReq(req)

		streamOpts, err := getExecOptions(req)
		if err != nil {
			return errdefs.ConvertToInvalidInput(err)
		}

		ctx, cancel := context.WithCancel(req.Context())
		defer cancel()

		exec := &containerExecutor{
			ctx:       ctx,
			h:         execInContainer,
			pod:       pod,
			namespace: namespace,
			container: container,
			getClient: getClient,
		}
		remotecommand.ServeExec(
			w,
			req,
			exec,
			"",
			"",
			container,
			command,
			streamOpts,
			cfg.StreamIdleTimeout,
			cfg.StreamCreationTimeout,
			supportedStreamProtocols,
		)

		return nil
	})
}
