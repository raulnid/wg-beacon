# wg-beacon

A lightweight client/server tool that automatically updates a WireGuard peer's endpoint when its public IP changes. The client periodically fetches its public IP and sends it to the server, which updates `wg0.conf` and reloads the WireGuard interface.

Communication is encrypted and authenticated using NaCl Box (X25519 + XSalsa20 + Poly1305) — the same cryptography used by WireGuard itself.

## How it works

1. The client fetches its current public IP from `api.ipify.org`
2. It encrypts the IP using the server's public key and its own private key
3. The server receives the payload, decrypts it using the client's public key and its own private key
4. If the IP is valid, the server updates `/etc/wireguard/wg0.conf` and reloads the interface

## Project structure

```
wg-beacon/
  client/
    internal/config/
    pem/              # keys (not committed)
    .env
    app.go
    main.go
  server/
    internal/config/
    pem/              # keys (not committed)
    .env
    app.go
    main.go
```

## Setup

### 1. Generate keypairs

Run this once on each machine. You can use any tool that generates X25519 keypairs, or use the Go `nacl/box` package directly:

```go
pub, priv, _ := box.GenerateKey(rand.Reader)
```

Save the public and private keys as hex-encoded files inside the `pem/` directory of each binary.

### 2. Exchange public keys

- Copy the **client's public key** to `server/pem/client_public`
- Copy the **server's public key** to `client/pem/server_public`

### 3. Configure environment

**client/.env**
```
HOST=your-server-ip
PORT=8080
ENDPOINT=your-endpoint-path
```

**server/.env**
```
PORT=8080
ENDPOINT=your-endpoint-path
```

### 4. Build

```bash
# Server (run on the server or cross-compile)
cd server
GOOS=linux GOARCH=amd64 go build -o wg-beacon .

# Client
cd client
GOOS=linux GOARCH=amd64 go build -o wg-beacon .
```

### 5. Run

The server must run as root to write to `/etc/wireguard/` and execute `wg` commands.

```bash
# Server
sudo ./wg-beacon

# Client
./wg-beacon
```

## Security

- All payloads are encrypted and authenticated — the server rejects anything not signed by the legitimate client
- Keys are never committed to the repository
- The server validates that the decrypted payload is a valid IP address before applying any changes

## Important

Never commit the `pem/` directories or `.env` files. The `.gitignore` should cover:

```
pem/
.env
wg-beacon
```