// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package monitor

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler builds dzo-exporter's http.Handler: GET /metrics (Prometheus
// exposition text) and GET /status[/<instance>] (JSON), both pulling a
// fresh Snapshot from source on every request (§C9: "scrapes never block
// on game servers themselves" - source is expected to serve a cached
// value with its own refresh interval, not query the servers inline).
func Handler(source Source) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		snap, err := source.Snapshot()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_ = WriteMetrics(w, snap)
	})
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		snap, err := source.Snapshot()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, snap)
	})
	mux.HandleFunc("/status/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/status/")
		if name == "" {
			http.NotFound(w, r)
			return
		}
		snap, err := source.Snapshot()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		inst, ok := snap.Find(name)
		if !ok {
			http.Error(w, "instance not found", http.StatusNotFound)
			return
		}
		writeJSON(w, inst)
	})
	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
