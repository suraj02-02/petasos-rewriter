FROM golang:1.14.2-stretch as builder

LABEL maintainer="Sachin  Narang"

WORKDIR  /go/src/github.com/Sachin4403/petasos-rewriter
COPY . .
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w"

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /go/src/github.com/Sachin4403/petasos-rewriter/petasos-rewriter /petasos-rewriter
ENTRYPOINT ["/petasos-rewriter"]