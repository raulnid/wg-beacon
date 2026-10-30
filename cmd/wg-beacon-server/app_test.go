package main

import (
	"bytes"
	"crypto/rand"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/nacl/box"

	"wg-beacon/internal/beacon"
)

const peerKey = "PEERKEY="

// fakeWG answers `wg show` with endpoints and records every call.
type fakeWG struct {
	endpoints string
	calls     [][]string
}

func (f *fakeWG) run(name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	if len(args) > 0 && args[0] == "show" {
		return []byte(f.endpoints), nil
	}
	return nil, nil
}

func newTestApp(wg *fakeWG) *App {
	return &App{
		cfg: &Config{Endpoint: "beacon", Interface: "wg0", PeerPublicKey: peerKey, PeerPort: "51820"},
		run: wg.run,
	}
}

func TestSetEndpoint(t *testing.T) {
	t.Run("unchanged", func(t *testing.T) {
		wg := &fakeWG{endpoints: "OTHER=\t198.51.100.1:51820\n" + peerKey + "\t[2001:db8::1]:51820\n"}
		if err := newTestApp(wg).setEndpoint("2001:db8::1"); err != nil {
			t.Fatal(err)
		}
		if len(wg.calls) != 1 {
			t.Fatalf("expected only wg show, got %v", wg.calls)
		}
	})
	t.Run("changed", func(t *testing.T) {
		wg := &fakeWG{endpoints: peerKey + "\t(none)\n"}
		if err := newTestApp(wg).setEndpoint("203.0.113.7"); err != nil {
			t.Fatal(err)
		}
		want := []string{"wg", "set", "wg0", "peer", peerKey, "endpoint", "203.0.113.7:51820"}
		if len(wg.calls) != 2 || !slices.Equal(wg.calls[1], want) {
			t.Fatalf("got %v, want second call %v", wg.calls, want)
		}
	})
	t.Run("peer missing", func(t *testing.T) {
		wg := &fakeWG{endpoints: "OTHER=\t198.51.100.1:51820\n"}
		if err := newTestApp(wg).setEndpoint("203.0.113.7"); err == nil {
			t.Fatal("expected an error")
		}
		if len(wg.calls) != 1 {
			t.Fatalf("wg set must not run, got %v", wg.calls)
		}
	})
	t.Run("wg show fails", func(t *testing.T) {
		a := newTestApp(nil)
		a.run = func(string, ...string) ([]byte, error) { return nil, errors.New("boom") }
		if err := a.setEndpoint("203.0.113.7"); err == nil {
			t.Fatal("expected an error")
		}
	})
}

func TestHandler(t *testing.T) {
	cPub, cPriv, _ := box.GenerateKey(rand.Reader)
	sPub, sPriv, _ := box.GenerateKey(rand.Reader)
	a := newTestApp(&fakeWG{endpoints: peerKey + "\t203.0.113.7:51820\n"})
	a.keys = beacon.Keys{Peer: *cPub, Private: *sPriv}
	h := a.routes()

	do := func(method string, body []byte) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/beacon", bytes.NewReader(body)))
		return rec.Code
	}

	msg, err := beacon.Seal("203.0.113.7", time.Now(), beacon.Keys{Peer: *sPub, Private: *cPriv})
	if err != nil {
		t.Fatal(err)
	}

	if got := do(http.MethodGet, nil); got != http.StatusMethodNotAllowed {
		t.Errorf("GET: got %d", got)
	}
	if got := do(http.MethodPost, []byte("short")); got != http.StatusBadRequest {
		t.Errorf("short body: got %d", got)
	}
	if got := do(http.MethodPost, []byte(strings.Repeat("x", maxBodySize+1))); got != http.StatusBadRequest {
		t.Errorf("oversized body: got %d", got)
	}
	if got := do(http.MethodPost, msg); got != http.StatusOK {
		t.Errorf("first message: got %d", got)
	}
	if got := do(http.MethodPost, msg); got != http.StatusUnauthorized {
		t.Errorf("replayed message: got %d", got)
	}
}
