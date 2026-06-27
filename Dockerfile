FROM golang:1.25-alpine AS builder

WORKDIR /src
ENV GOPROXY=https://goproxy.cn,direct

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/agent-im-server ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /out/agent-im-server /app/agent-im-server

ENV PORT=9800
EXPOSE 9800

CMD ["/app/agent-im-server"]
