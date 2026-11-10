# ABAPer REST server / CLI image (ghcr.io/bluefunda/abaper).
# Build from the repository root:
#   docker build -f abaper/Dockerfile -t ghcr.io/bluefunda/abaper .
# CI passes GO_VERSION from go.mod (release.yml); the default is for local builds.
ARG GO_VERSION=1.26.10
FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 go build \
    -ldflags "-X github.com/bluefunda/bluefunda-ai/internal/cmd.Version=${VERSION}" \
    -o /abaper ./abaper/cmd/abaper

FROM alpine:3.21
RUN apk --no-cache add ca-certificates man-db
COPY --from=builder /abaper /usr/local/bin/abaper
COPY abaper/man/abaper.1 /usr/local/share/man/man1/abaper.1
# Links the GHCR package to this repo (bluefunda/abaper, its old source, is archived).
LABEL org.opencontainers.image.source="https://github.com/bluefunda/bluefunda-ai"
ENTRYPOINT ["abaper"]
