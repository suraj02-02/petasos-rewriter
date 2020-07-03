

.PHONY: build
build:
	@CGO_ENABLED=0 go build -ldflags "-w -s"


.PHONY: build-docker
build-docker:
	@docker build -f build/Dockerfile . -t petasos-rewriter:latest