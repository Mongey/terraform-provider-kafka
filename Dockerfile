FROM golang:1.21

WORKDIR /go/src/github.com/Mongey/terraform-provider-kafka/

COPY go.mod go.sum main.go GNUmakefile ./
COPY kafka kafka
COPY secrets secrets
