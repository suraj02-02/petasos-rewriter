FROM docker.io/library/golang:1.14-alpine as builder

LABEL maintainer="Matthias `Equanox` Noesner <matthiasnoesner@benchkram.de>"

WORKDIR /go/src/github.com/Equanox/petasos-rewriter

ARG VERSION=unknown
ARG GITCOMMIT=unknown
ARG BUILDTIME=unknown

RUN apk add --no-cache --no-progress \
    ca-certificates \
    make \
    git \
    openssh \
    gcc \
    libc-dev \
    upx

COPY . .
RUN make build

FROM alpine:latest
COPY --from=builder /go/src/github.com/Equanox/petasos-rewriter /petasos-rewriter
ENTRYPOINT ["/petasos-rewriter"]