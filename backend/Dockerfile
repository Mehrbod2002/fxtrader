FROM golang:latest AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o fxtrader ./cmd/server/main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/fxtrader .
COPY static/ ./static/
EXPOSE 8080
CMD ["./fxtrader"]