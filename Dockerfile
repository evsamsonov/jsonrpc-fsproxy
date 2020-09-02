FROM golang:1.15.1-alpine

RUN apk update && apk add git && apk add bash

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o jsonrpc-fsproxy ./cmd/jsonrpc-fsproxy

CMD ["./jsonrpc-fsproxy"]
