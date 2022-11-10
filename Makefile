VERSION ?= latest
IMAGE_NAME ?= bohanon/echo-server:$(VERSION)
.PHONY: build-and-push
build-and-push:
	go mod tidy
	docker build -t $(IMAGE_NAME) .
	docker push $(IMAGE_NAME)
