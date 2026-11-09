FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 go build \
    -ldflags "-X github.com/bluefunda/abaper/internal/commands.version=${VERSION}" \
    -o /abaper ./cmd/abaper

FROM alpine:3.21
RUN apk --no-cache add ca-certificates man-db
COPY --from=builder /abaper /usr/local/bin/abaper
COPY man/abaper.1 /usr/local/share/man/man1/abaper.1
ENTRYPOINT ["abaper"]
