# wg-beacon

A small client/server tool that keeps a WireGuard peer's endpoint current when its public IP changes. The client fetches its public IP every `INTERVAL` and POSTs it to the server, which runs `wg set` to point the peer at the new address.

Messages are encrypted and authenticated with NaCl box (X25519 + XSalsa20-Poly1305). That only shares X25519 with WireGuard: WireGuard itself uses the Noise protocol with ChaCha20-Poly1305 and BLAKE2s. Plain HTTP is fine because box already gives confidentiality and authentication.

## Why

The server is a VPS on OCI and the client is in a homelab with a dynamic IP. OCI's firewall only lets the client's WireGuard traffic in as return traffic: the server has to send first, to the client's current endpoint.

WireGuard roaming and `PersistentKeepalive` do not cover this:

- Roaming updates a peer's endpoint only after the server receives an authenticated packet from the new address. Here the firewall drops those packets before they reach WireGuard, so the server never learns the new IP.
- `PersistentKeepalive` on the server keeps sending to the old endpoint. On the client, its keepalives are dropped like any other packet.

wg-beacon gets the new IP to the server over a separate HTTP port that is allowed in, so the server can start the handshake.

## How it works

1. The client fetches its public IP from `api.ipify.org` and checks that it parses as an IP.
2. It seals `timestamp || ip` with the server's public key and its own private key.
3. The server opens the message with the client's public key and its own private key, then checks the timestamp.
4. The server reads `wg show <WG_INTERFACE> endpoints`. If the peer's endpoint already equals `ip:PEER_PORT`, it does nothing. Otherwise it runs `wg set <WG_INTERFACE> peer <PEER_PUBLIC_KEY> endpoint <ip:PEER_PORT>`. If the peer is not on the interface it returns an error rather than letting `wg set` create it.

`wg0.conf` is never touched and the tunnel is never restarted.

### Message format

```
nonce (24 bytes) || box.Seal( timestamp (8 bytes, big-endian unix seconds) || ip (ASCII, IPv4 or IPv6) )
```

The server rejects a message (HTTP 401) when:

- it does not decrypt with the expected keys;
- `|now - timestamp| > 2m`;
- `timestamp <=` the last accepted timestamp, which is kept in memory (so a restart resets it, and the 2-minute window still applies).

A body that is shorter than the minimum or bigger than 256 bytes, or that holds an invalid IP, gets HTTP 400. Both machines need a reasonably synced clock (NTP).

> **Protocol change:** the timestamp was added in this version. Older clients and servers cannot talk to newer ones, so deploy the client and the server together.

## Project structure

```
wg-beacon/
  cmd/wg-beacon-client/   # runs in the homelab k8s cluster (Dockerfile, Jenkinsfile)
  cmd/wg-beacon-server/   # runs on the VPS under systemd
  internal/beacon/        # key loading, Seal and Open
  deploy/wg-beacon-server.service
```

## Configuration

Both binaries read environment variables. A `.env` file in the working directory is optional, and real environment variables take precedence over it.

**Client**

| Variable   | Required | Default | Description |
|------------|----------|---------|-------------|
| `HOST`     | yes      |         | Server host or IP |
| `PORT`     | yes      |         | Server HTTP port |
| `ENDPOINT` | yes      |         | URL path the server listens on (no leading `/`) |
| `INTERVAL` | no       | `5m`    | Time between reports (Go duration, e.g. `300s`, `5m`) |

**Server**

| Variable          | Required | Default | Description |
|-------------------|----------|---------|-------------|
| `PORT`            | yes      |         | HTTP listen port |
| `ENDPOINT`        | yes      |         | URL path to accept `POST`s on |
| `WG_INTERFACE`    | no       | `wg0`   | WireGuard interface |
| `PEER_PUBLIC_KEY` | yes      |         | WireGuard public key of the client peer |
| `PEER_PORT`       | yes      |         | WireGuard UDP port to use in the client's endpoint |

Both exit at startup if a required value is missing or the keys cannot be read.

## Setup

### 1. Generate keypairs

These are NaCl box keys, separate from the WireGuard keys. Generate one pair per machine, for example with Go:

```go
pub, priv, _ := box.GenerateKey(rand.Reader)
```

Save each key hex-encoded in `pem/` under the binary's working directory:

- client: `pem/client_private`, `pem/server_public`
- server: `pem/server_private`, `pem/client_public`

### 2. Client (k8s)

Jenkins builds `hub.xzr.es/library/wg-beacon` from the `Dockerfile` at the repo root with kaniko. The image runs in `/app`, so mount `pem/` (and `.env`, if you use one) there, or set the variables on the pod.

### 3. Server (VPS)

```bash
GOOS=linux GOARCH=amd64 go build -o wg-beacon-server ./cmd/wg-beacon-server
sudo install -m 0755 wg-beacon-server /usr/local/bin/
sudo useradd --system --no-create-home --shell /usr/sbin/nologin wg-beacon
sudo install -d -o root -g wg-beacon -m 0750 /etc/wg-beacon /etc/wg-beacon/pem
# copy pem/server_private, pem/client_public and .env into /etc/wg-beacon, readable by the wg-beacon group
sudo install -m 0644 deploy/wg-beacon-server.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now wg-beacon-server
```

The unit runs as the unprivileged `wg-beacon` user with only `CAP_NET_ADMIN` (enough for `wg show` and `wg set`), with `NoNewPrivileges`, `ProtectSystem=strict`, `ProtectHome` and `PrivateTmp`. It starts after `wg-quick@wg0.service`; change that if you use another interface.

**Trade-off:** because the server no longer writes `wg0.conf`, the endpoint it sets lives only in the kernel. After a VPS reboot `wg-quick` loads the old endpoint from `wg0.conf`, and the tunnel stays down until the next beacon arrives, at most one `INTERVAL` later.

## Development

```bash
go vet ./...
go test -race ./...
golangci-lint run
```

CI (`.github/workflows/ci.yml`) runs the same checks. Dependabot keeps Go modules and GitHub Actions up to date.

## Security

- Every message is encrypted and authenticated: the server rejects anything not authenticated with the client's key.
- Timestamps and the replay check stop old messages from being replayed.
- The server checks that the payload is a valid IP before running any command, and `wg` gets it as a separate argument, never through a shell.
- Never commit `pem/` or `.env`. `.gitignore` covers them.
