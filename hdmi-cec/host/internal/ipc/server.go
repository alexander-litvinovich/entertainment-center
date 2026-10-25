package ipc

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"sync"
	"time"
)

type Server struct {
	ctx      context.Context
	handler  Handler
	listener net.Listener
	wg       sync.WaitGroup
}

// Serve answers requests on listener until Close.
func Serve(ctx context.Context, listener net.Listener, handler Handler) *Server {
	s := &Server{ctx: ctx, handler: handler, listener: listener}
	s.wg.Add(1)
	go s.serve()
	return s
}

func (s *Server) Close() error { err := s.listener.Close(); s.wg.Wait(); return err }

func (s *Server) serve() {
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

func (s *Server) handle(conn net.Conn) {
	var req Request
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err := json.NewDecoder(io.LimitReader(conn, 4096)).Decode(&req); err != nil {
		_ = json.NewEncoder(conn).Encode(Response{OK: false, Message: "invalid request"})
		return
	}
	_ = conn.SetReadDeadline(time.Time{})
	_ = json.NewEncoder(conn).Encode(s.handler.Handle(s.ctx, req))
}

// Send delivers one request to the local host (service, `run` or `run --dry-run`).
func Send(req Request) (Response, error) {
	conn, err := dial()
	if err != nil {
		return Response{}, err
	}
	defer conn.Close()
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return Response{}, err
	}
	var response Response
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		return response, err
	}
	return response, nil
}
