FROM golang:1.14.2-stretch as builder

LABEL maintainer="Matthias `Equanox` Noesner <matthiasnoesner@benchkram.de>"

WORKDIR /go/src/github.com/Equanox/petasos-rewriter
COPY . .
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w"

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /go/src/github.com/Equanox/petasos-rewriter/petasos-rewriter /petasos-rewriter
ENTRYPOINT ["/petasos-rewriter"]