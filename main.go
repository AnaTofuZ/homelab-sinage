package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	bf "github.com/barefootjs/runtime/bf"
	"golang.org/x/sync/errgroup"
)

func main() {
	renderer := mustNewRenderer()
	service := newDashboardService(httpClient())

	mux := http.NewServeMux()
	MountDevReload(mux)
	mux.Handle("/client/", http.StripPrefix("/client/", http.FileServer(http.Dir("dist/client"))))
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("public"))))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /api/dashboard", func(w http.ResponseWriter, req *http.Request) {
		writeJSON(w, http.StatusOK, service.dashboard(req.Context()))
	})
	mux.HandleFunc("POST /api/attendance/{action}", func(w http.ResponseWriter, req *http.Request) {
		action := req.PathValue("action")
		if action != "clock-in" && action != "clock-out" && action != "break-start" && action != "break-end" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown action"})
			return
		}
		status, err := service.attendanceAction(req.Context(), action)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, status)
	})

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, req *http.Request) {
		props := NewSignageProps(SignageInput{})
		render(w, renderer, http.StatusOK, "Signage", bf.RenderOptions{
			Title: "Home Signal",
			Props: &props,
		})
	})

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8092"
	}
	fmt.Printf("  ➜ http://%s\n", addr)
	server := &http.Server{Addr: addr, Handler: requestLog(mux), ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	group.Go(func() error {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	})
	if err := group.Wait(); err != nil {
		log.Fatal(err)
	}
}

func httpClient() *http.Client {
	return &http.Client{Timeout: 12 * time.Second}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !strings.HasPrefix(req.URL.Path, "/api/") {
			next.ServeHTTP(w, req)
			return
		}
		started := time.Now()
		next.ServeHTTP(w, req)
		log.Printf("%s %s %s", req.Method, req.URL.Path, time.Since(started).Round(time.Millisecond))
	})
}
