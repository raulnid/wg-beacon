FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /wg-beacon ./cmd/wg-beacon-client

# the client reads ./.env and ./pem/* relative to its cwd, so mount them under /app
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /wg-beacon /usr/local/bin/wg-beacon
ENTRYPOINT ["/usr/local/bin/wg-beacon"]
