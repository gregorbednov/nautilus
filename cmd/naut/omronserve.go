package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/joyautomation/nautilus/omron/monitor"
)

func serveOmron(ctx context.Context, dial monitor.Dial, tags, addr string, poll, stale time.Duration) error {
	if tags == "" {
		return errors.New("serve needs -tags BINDINGS.json")
	}
	if poll <= 0 || stale <= poll {
		return errors.New("poll must be positive and stale-after must exceed poll")
	}
	data, err := readOmronFile(tags)
	if err != nil {
		return err
	}
	var bindings []monitor.Binding
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&bindings); err != nil {
		return err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return errors.New("trailing JSON after bindings")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	driver, err := monitor.New(ctx, dial, bindings, stale)
	if err != nil {
		return err
	}
	facade, err := monitor.NewFacade(driver, poll)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: facade.Handler(), ReadHeaderTimeout: 5 * time.Second}
	defer srv.Close()
	loopDone := make(chan struct{})
	go func() { facade.Run(ctx); close(loopDone) }()
	serverDone := make(chan error, 1)
	go func() { serverDone <- srv.Serve(listener) }()
	fmt.Fprintf(os.Stderr, "Omron read-only monitor: http://%s (%d tags)\n", listener.Addr(), len(bindings))
	select {
	case err = <-serverDone:
		cancel()
	case <-ctx.Done():
		shutdownCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
		_ = srv.Shutdown(shutdownCtx)
		stop()
		_ = srv.Close()
		err = <-serverDone
	}
	<-loopDone
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
