// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClashDelayParsesJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer s3cr3t" {
			t.Errorf("missing bearer auth")
		}
		w.Write([]byte(`{"delay": 42}`))
	}))
	defer srv.Close()
	c := &ClashClient{BaseURL: srv.URL, Secret: "s3cr3t", HTTP: srv.Client()}
	d, err := c.Delay(context.Background(), "proxy", "https://www.gstatic.com/generate_204", 5000)
	if err != nil {
		t.Fatalf("delay: %v", err)
	}
	if d != 42 {
		t.Fatalf("want 42ms, got %d", d)
	}
}

func TestClashSwitchPutsName(t *testing.T) {
	got := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			b, _ := io.ReadAll(r.Body) // Read may short-fill a sized buffer; ReadAll is safe
			got = string(b)
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()
	c := &ClashClient{BaseURL: srv.URL, Secret: "", HTTP: srv.Client()}
	if err := c.Switch(context.Background(), "select", "us-1"); err != nil {
		t.Fatalf("switch: %v", err)
	}
	if !strings.Contains(got, "us-1") {
		t.Fatalf("expected body to carry member name, got %q", got)
	}
}
