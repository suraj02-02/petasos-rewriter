FROM golang:1.14.2-stretch as builder

LABEL maintainer="Sachin  Narang"

WORKDIR  /go/src/github.com/Sachin4403/petasos-rewriter
COPY . .
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w"

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /go/src/github.com/Sachin4403/petasos-rewriter/petasos-rewriter /petasos-rewriter
RUN mkdir /etc/petasos-rewriter/ && touch /etc/petasos-rewriter/petasos-rewriter.yaml && chmod 666 /etc/petasos-rewriter/petasos-rewriter.yaml

RUN adduser 'dtuser' -D --home /opt/dtenv/ --shell /bin/bash dtuser
WORKDIR /opt/dtenv/
RUN chown dtuser:dtuser /var/log
RUN chown dtuser:dtuser /var/run/
RUN chown -R dtuser:dtuser /opt/dtenv/

ENTRYPOINT ["/petasos-rewriter"]
