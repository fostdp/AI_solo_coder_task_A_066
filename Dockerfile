FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

COPY go.mod ./
COPY . .
RUN go mod tidy && go mod download

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /bin/dc-cooling-server \
    ./cmd/web

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/dc-cooling-server /usr/local/bin/dc-cooling-server
COPY config/config.json /etc/dc-cooling/config.json
COPY frontend/ /app/frontend/

WORKDIR /app

EXPOSE 8080

ENTRYPOINT ["dc-cooling-server"]
