// SPDX-FileCopyrightText: 2026 Bernd Zeimetz <bernd@bzed.de>
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func exporterSite(t *testing.T, extra string) string {
	t.Helper()
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	writeFile(t, cfg, "paths:\n  data: "+dir+"\nexporter:\n  listen: 127.0.0.1:0\n"+extra)
	writeFile(t, filepath.Join(dir, "site", "instances", "x", "instance.yaml"), `name: x
product: dayz-stable
map: empty.m
mission_source: {git: g, ref: r, path: p}
ports: {game: 2302, rcon: 2306, query: 27016}
network: host
`)
	return cfg
}

func TestExporterServesMetricsAndStatus(t *testing.T) {
	cfg := exporterSite(t, "")
	root := newRootCmd()
	var out syncBuf
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"exporter", "--config", cfg})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- root.ExecuteContext(ctx) }()

	var addr string
	for deadline := time.Now().Add(10 * time.Second); addr == "" && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if m := regexp.MustCompile(`exporter on (\S+)`).FindStringSubmatch(out.String()); m != nil {
			addr = m[1]
		}
	}
	if addr == "" {
		t.Fatalf("the exporter did not start:\n%s", out.String())
	}
	get := func(path string) (int, string) {
		resp, err := http.Get("http://" + addr + path)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	if code, body := get("/metrics"); code != 200 || !strings.Contains(body, `dzo_instance_up{instance="x"} 0`) || !strings.Contains(body, "dzo_build_info") {
		t.Errorf("metrics: %d\n%s", code, body)
	}
	if code, body := get("/status/x"); code != 200 || !strings.Contains(body, `"name": "x"`) {
		t.Errorf("status: %d %s", code, body)
	}
	if code, _ := get("/status/nope"); code != 404 {
		t.Errorf("unknown instance: %d", code)
	}
	cancel()
	if err := <-done; err != nil {
		t.Errorf("the exporter must stop cleanly: %v", err)
	}
}

func TestExporterNeedsItsTokenAndRefusesBadConfig(t *testing.T) {
	cfg := exporterSite(t, "  bearer_token_file: /nonexistent\n")
	if _, err := runCmd(t, "exporter", "--config", cfg); err == nil || !strings.Contains(err.Error(), "bearer_token_file") {
		t.Errorf("a missing token file: %v", err)
	}
	bad := filepath.Join(t.TempDir(), "c.yaml")
	writeFile(t, bad, "exporter:\n  allow: [nonsense]\n")
	if _, err := runCmd(t, "exporter", "--config", bad); err == nil || !strings.Contains(err.Error(), "CIDR") {
		t.Errorf("a bad allow-list is a config error: %v", err)
	}
	writeFile(t, bad, "exporter:\n  tls: {cert_file: /a}\n")
	if _, err := runCmd(t, "exporter", "--config", bad); err == nil || !strings.Contains(err.Error(), "go together") {
		t.Errorf("a certificate without a key: %v", err)
	}
}

func TestStatusCommand(t *testing.T) {
	cfg := exporterSite(t, "")
	out, err := runCmd(t, "status", "--config", cfg)
	if err != nil || !strings.HasPrefix(out, "x: ") {
		t.Fatalf("status: %v\n%s", err, out)
	}
	out, err = runCmd(t, "status", "x", "--json", "--config", cfg)
	var i struct{ Name, Product string }
	if err != nil || json.Unmarshal([]byte(out), &i) != nil || i.Name != "x" || i.Product != "dayz-stable" {
		t.Fatalf("status x --json: %v\n%s", err, out)
	}
	dir := filepath.Join(t.TempDir(), "status")
	if _, err := runCmd(t, "status", "--write", dir, "--config", cfg); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"x.json", "global.json"} {
		b, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil || !json.Valid(b) {
			t.Errorf("%s: %v", f, err)
		}
	}
	if _, err := runCmd(t, "status", "nope", "--config", cfg); err == nil {
		t.Error("an unknown instance")
	}
	if out, err := runCmd(t, "status", "--json", "--config", cfg); err != nil || !strings.Contains(out, `"global"`) {
		t.Errorf("status --json: %v\n%s", err, out)
	}
}
