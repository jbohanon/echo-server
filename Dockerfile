FROM golang:1.20 as builder

WORKDIR /var/build
COPY go.mod ./
COPY go.sum ./
RUN go mod download all
COPY *.go ./

RUN GOARCH=amd64 GOOS=linux CGO_ENABLED=0 go build -o echo-server .

FROM alpine
EXPOSE 8080
EXPOSE 8443
COPY --from=builder /var/build/echo-server /app/echo-server
COPY localhost.key /app/tls/localhost.key
COPY localhost.crt /app/tls/localhost.crt
CMD ["/app/echo-server"]
