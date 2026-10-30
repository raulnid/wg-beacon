// Package beacon seals and opens the client's IP report.
//
// Wire format: nonce(24) || box.Seal(ts(8, big-endian unix seconds) || ip).
package beacon

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/nacl/box"
)

// MaxSkew is how far a message timestamp may be from the server clock.
const MaxSkew = 2 * time.Minute

const (
	nonceSize = 24
	tsSize    = 8
)

var (
	ErrMalformed = errors.New("malformed message")
	ErrAuth      = errors.New("decryption failed")
	ErrStale     = errors.New("timestamp outside the allowed window")
	ErrReplay    = errors.New("timestamp not newer than the last accepted one")
)

// Keys holds our private key and the other side's public key.
type Keys struct {
	Peer    [32]byte
	Private [32]byte
}

// ReadKey reads a hex-encoded 32-byte key from path.
func ReadKey(path string) ([32]byte, error) {
	var key [32]byte
	raw, err := os.ReadFile(path) //nolint:gosec // path is a fixed key location, not user input
	if err != nil {
		return key, err
	}
	decoded, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		return key, err
	}
	if len(decoded) != 32 {
		return key, fmt.Errorf("key at %s must be 32 bytes, got %d", path, len(decoded))
	}
	copy(key[:], decoded)
	return key, nil
}

// Seal encrypts ip together with the timestamp now.
func Seal(ip string, now time.Time, k Keys) ([]byte, error) {
	var nonce [nonceSize]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}
	plain := binary.BigEndian.AppendUint64(nil, uint64(now.Unix())) //nolint:gosec // unix time is positive
	plain = append(plain, ip...)
	return box.Seal(nonce[:], plain, &nonce, &k.Peer, &k.Private), nil
}

// Open decrypts msg and returns the IP and its timestamp. It rejects
// messages whose timestamp is more than MaxSkew from now or not newer than last.
func Open(msg []byte, now time.Time, last int64, k Keys) (string, int64, error) {
	if len(msg) < nonceSize+box.Overhead+tsSize {
		return "", 0, ErrMalformed
	}
	var nonce [nonceSize]byte
	copy(nonce[:], msg[:nonceSize])
	plain, ok := box.Open(nil, msg[nonceSize:], &nonce, &k.Peer, &k.Private)
	if !ok {
		return "", 0, ErrAuth
	}
	ts := int64(binary.BigEndian.Uint64(plain[:tsSize])) //nolint:gosec // overflow just yields a rejected ts
	if d := now.Sub(time.Unix(ts, 0)); d > MaxSkew || d < -MaxSkew {
		return "", 0, ErrStale
	}
	if ts <= last {
		return "", 0, ErrReplay
	}
	ip := string(plain[tsSize:])
	if net.ParseIP(ip) == nil {
		return "", 0, fmt.Errorf("%w: invalid IP %q", ErrMalformed, ip)
	}
	return ip, ts, nil
}
