FROM golang:1.24-alpine AS builder
ENV GOPROXY=https://goproxy.io,direct
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o fxtrader ./cmd/server/main.go

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata
WORKDIR /home/appuser

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
COPY --from=builder /app/fxtrader .
RUN chmod 755 ./fxtrader
COPY --chown=appuser:appgroup static/ ./static/
COPY --chown=appuser:appgroup docs/ ./docs/

RUN chmod -R 777 ./static ./docs
USER appuser
EXPOSE 7000 7003
ENV TZ=UTC

HEALTHCHECK --interval=30s --timeout=3s CMD wget --no-verbose --tries=1 --spider http://localhost:7000/health || exit 1
CMD ["./fxtrader"]