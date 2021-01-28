FROM quay.io/giantswarm/golang:1.15.3 AS builder

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go get -u github.com/jstemmer/go-junit-report \
  && apt-get update && apt-get install -y nodejs npm \
  && npm install -g junit-report-merger \
  && go mod download

COPY . .
RUN go build ./...

ENTRYPOINT ["/app/run_go_test.sh"]
