FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/lightbridge-mail-service .

FROM alpine:3.21
RUN addgroup -S lightbridge && adduser -S -G lightbridge lightbridge
WORKDIR /app
COPY --from=builder /out/lightbridge-mail-service /app/lightbridge-mail-service
USER lightbridge
EXPOSE 8091
ENTRYPOINT ["/app/lightbridge-mail-service"]
