FROM golang:1.19 as builder

WORKDIR /var/build
COPY go.mod ./
COPY go.sum ./
RUN go mod download all
COPY *.go ./

RUN GOARCH=amd64 GOOS=linux CGO_ENABLED=0 go build -o echo-server .

FROM scratch
EXPOSE 9999
COPY --from=builder /var/build/echo-server /app/echo-server
CMD ["./app/echo-server"]
