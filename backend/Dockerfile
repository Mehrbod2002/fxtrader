FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o fxtrader ./cmd/server/main.go

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /root/
COPY --from=builder /app/fxtrader .
COPY static/ ./static/
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

EXPOSE 7000 7003

ENV TZ=UTC

CMD ["./fxtrader"]