FROM quay.io/giantswarm/golang:1.20.2 AS builder

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go install github.com/jstemmer/go-junit-report/v2@latest \
  && apt-get update && apt-get install -y nodejs npm \
  && npm install -g junit-report-merger \
  && go mod download

COPY . .
RUN go build ./...

ENTRYPOINT ["/app/run_go_test.sh"]
