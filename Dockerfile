# syntax=docker/dockerfile:1
ARG GO_VERSION=1.24
ARG BUILDER_REGISTRY=docker.io
ARG RUNTIME_REGISTRY=gcr.io
ARG GOPROXY=https://proxy.golang.org,direct

FROM ${BUILDER_REGISTRY}/golang:${GO_VERSION}-alpine AS builder

ARG GOPROXY
ENV GOPROXY=${GOPROXY}

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download && go mod verify

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -trimpath \
    -o /build/bin/service \
    ./cmd/server

FROM ${RUNTIME_REGISTRY}/distroless/static-debian12:nonroot

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/bin/service /app/service

WORKDIR /app

ENTRYPOINT ["/app/service"]
