FROM golang:1.16.3-alpine

RUN apk update && apk add --no-cache git=2.30.2-r0

WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o jsonrpc-fsproxy .
