//go:build windows

package winhost

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
	"hdmi-cec-host/internal/core"
	"hdmi-cec-host/internal/ipc"
)

const pipeSDDL = "D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GRGW;;;AU)"

type PipeServer struct {
	ctx        context.Context
	controller *core.Controller
	listener   net.Listener
	wg         sync.WaitGroup
}

func StartPipeServer(ctx context.Context, controller *core.Controller, sddl string) (*PipeServer, error) {
	// go-winio creates pipes with FILE_PIPE_REJECT_REMOTE_CLIENTS.
	listener, err := winio.ListenPipe(ipc.PipeName, &winio.PipeConfig{SecurityDescriptor: sddl, InputBufferSize: 4096, OutputBufferSize: 4096})
	if err != nil {
		return nil, err
	}
	s := &PipeServer{ctx: ctx, controller: controller, listener: listener}
	s.wg.Add(1)
	go s.serve()
	return s, nil
}

func (s *PipeServer) Close() error { err := s.listener.Close(); s.wg.Wait(); return err }
func (s *PipeServer) serve() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func() { defer s.wg.Done(); defer conn.Close(); s.handle(conn) }()
	}
}

func (s *PipeServer) handle(conn net.Conn) {
	var req ipc.Request
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err := json.NewDecoder(io.LimitReader(conn, 4096)).Decode(&req); err != nil {
		_ = json.NewEncoder(conn).Encode(ipc.Response{OK: false, Message: "invalid request"})
		return
	}
	_ = conn.SetReadDeadline(time.Time{})
	ctx := s.ctx
	var err error
	switch req.Command {
	case "on":
		err = s.controller.ManualOn(ctx)
	case "off":
		err = s.controller.Off(ctx)
	case "input":
		err = s.controller.SetInput(ctx, req.Input)
	case "probe":
		err = s.controller.Probe(ctx, req.Input)
	case "status":
		status := s.controller.Status()
		_ = json.NewEncoder(conn).Encode(ipc.Response{OK: true, Status: &status})
		return
	default:
		err = errors.New("unknown command")
	}
	response := ipc.Response{OK: err == nil}
	if err != nil {
		response.Message = err.Error()
	}
	status := s.controller.Status()
	response.Status = &status
	_ = json.NewEncoder(conn).Encode(response)
}

func Send(req ipc.Request) (ipc.Response, error) {
	conn, err := winio.DialPipe(ipc.PipeName, nil)
	if err != nil {
		return ipc.Response{}, err
	}
	defer conn.Close()
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return ipc.Response{}, err
	}
	var response ipc.Response
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		return response, err
	}
	return response, nil
}
