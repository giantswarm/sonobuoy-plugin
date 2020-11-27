FROM quay.io/giantswarm/golang:1.15.3 AS builder

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .
RUN go build ./...

ENTRYPOINT ["go", "test", "-v", "./..."]
