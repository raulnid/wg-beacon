package beacon

import (
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/nacl/box"
)

// keyPairs returns the client-side and server-side Keys for a fresh pair of keypairs.
func keyPairs(t *testing.T) (client, server Keys) {
	t.Helper()
	cPub, cPriv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sPub, sPriv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return Keys{Peer: *sPub, Private: *cPriv}, Keys{Peer: *cPub, Private: *sPriv}
}

func seal(t *testing.T, ip string, ts time.Time, k Keys) []byte {
	t.Helper()
	msg, err := Seal(ip, ts, k)
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

func TestRoundtrip(t *testing.T) {
	c, s := keyPairs(t)
	now := time.Now()
	for _, ip := range []string{"203.0.113.7", "2001:db8::1"} {
		ip2, ts, err := Open(seal(t, ip, now, c), now, 0, s)
		if err != nil {
			t.Fatalf("%s: %v", ip, err)
		}
		if ip2 != ip || ts != now.Unix() {
			t.Fatalf("got %q %d, want %q %d", ip2, ts, ip, now.Unix())
		}
	}
}

func TestRejects(t *testing.T) {
	c, s := keyPairs(t)
	other, _ := keyPairs(t)
	now := time.Now()

	flipped := seal(t, "203.0.113.7", now, c)
	flipped[len(flipped)-1] ^= 1

	tests := []struct {
		name string
		msg  []byte
		last int64
		want error
	}{
		{"short", make([]byte, 10), 0, ErrMalformed},
		{"flipped byte", flipped, 0, ErrAuth},
		{"wrong keys", seal(t, "203.0.113.7", now, other), 0, ErrAuth},
		{"too old", seal(t, "203.0.113.7", now.Add(-MaxSkew-time.Second), c), 0, ErrStale},
		{"too far in future", seal(t, "203.0.113.7", now.Add(MaxSkew+time.Second), c), 0, ErrStale},
		{"same ts twice", seal(t, "203.0.113.7", now, c), now.Unix(), ErrReplay},
		{"not an IP", seal(t, "nope", now, c), 0, ErrMalformed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := Open(tt.msg, now, tt.last, s); !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}
}
