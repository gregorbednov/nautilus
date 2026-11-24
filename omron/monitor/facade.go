package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/joyautomation/nautilus/runtime"
	"github.com/joyautomation/nautilus/server"
)

type Facade struct {
	driver *Driver
	rt     *runtime.Runtime
	srv    *server.Server
}

func NewFacade(driver *Driver, poll time.Duration) (*Facade, error) {
	if driver == nil || poll <= 0 {
		return nil, fmt.Errorf("poll must be positive")
	}
	var tags []runtime.TagDef
	for _, b := range driver.bindings {
		tags = append(tags, runtime.TagDef{Name: b.Name, Role: runtime.RoleInput, Comms: true, DriverType: b.Type})
	}
	rt, err := runtime.New(runtime.Options{Program: "PROGRAM OmronMonitor\nEND_PROGRAM", Driver: driver, Tags: tags, Scan: poll})
	if err != nil {
		return nil, err
	}
	f := &Facade{driver: driver, rt: rt}
	f.srv = server.New(rt, server.Options{Drivers: func() []server.DriverStatus {
		h := driver.Health()
		state := "connecting"
		if h.Connected {
			state = "connected"
		}
		if h.LastError != "" {
			state = "error"
		}
		return []server.DriverStatus{{Kind: "omron-fins", Name: "CJ1", State: state, LastError: h.LastError, Detail: fmt.Sprintf("read-only monitor; CPU mode %02X; host polling %s", h.Mode, poll)}}
	}, TagWriter: func(string, any) error { return fmt.Errorf("Omron monitor is read-only") }})
	return f, nil
}

func (f *Facade) Handler() http.Handler {
	api := f.srv.Handler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Omron monitor is read-only; native compilation and deployment are not implemented"})
			return
		}
		switch r.URL.Path {
		case "/api/omron/status":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(f.driver.Health())
			return
		case "/api/program", "/api/program/history":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "CPU user memory is not Nautilus source; use naut omron upload for an opaque backup"})
			return
		}
		api.ServeHTTP(w, r)
	})
}

// Run owns the driver until cancellation and waits for its last poll to exit.
func (f *Facade) Run(ctx context.Context) {
	done := make(chan struct{})
	go func() { f.rt.Run(ctx); close(done) }()
	f.srv.Run(ctx)
	<-done
	_ = f.driver.Close()
}
