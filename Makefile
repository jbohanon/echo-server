.PHONY: build-and-push
build-and-push:
	go mod tidy
	docker build -t bohanon/echo-server .
	docker push bohanon/echo-server
