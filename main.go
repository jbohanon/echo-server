package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

var (
	server, tlsServer *http.Server
	port, tlsPort     int
	useTlsTunnel      bool
)

func main() {
	log.Println("parsing flags")
	var useConnectProxy bool
	flag.BoolVar(&useConnectProxy, "use-connect-proxy", false, "set to true to turn into CONNECT proxy")
	flag.BoolVar(&useTlsTunnel, "use-tls-tunnel", false, "set to true to make proxy set up tls tunnel to destination")
	flag.IntVar(&port, "port", 8080, "http port")
	flag.IntVar(&tlsPort, "tls-port", 8443, "https port")
	flag.Parse()
	log.Println("flags parsed")

	var (
		handlerFunc, tlsHandlerFunc http.HandlerFunc
	)
	if useConnectProxy {
		log.Println("using connect proxy")
		handlerFunc = connectProxy
		tlsHandlerFunc = connectProxy
	} else {
		log.Println("using standard echo server")
		handlerFunc = func(w http.ResponseWriter, r *http.Request) {
			sharedHandleFunc("http", w, r)
		}
		tlsHandlerFunc = func(w http.ResponseWriter, r *http.Request) {
			sharedHandleFunc("https", w, r)
		}
	}
	server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handlerFunc,
	}
	tlsServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", tlsPort),
		Handler: tlsHandlerFunc,
	}
	connectStr := func() string {
		if useConnectProxy {
			return "CONNECT "
		}
		return ""
	}
	go func() {
		log.Printf("%sserver is running on localhost:%d\n", connectStr(), port)
		log.Fatal(server.ListenAndServe())
	}()
	log.Printf("TLS %sserver is running on localhost:%d\n", connectStr(), tlsPort)
	log.Fatal(tlsServer.ListenAndServeTLS("/app/tls/localhost.crt", "/app/tls/localhost.key"))
}

func sharedHandleFunc(handler string, w http.ResponseWriter, r *http.Request) {
	log.Printf("handing request from %s handler\n", handler)
	defer r.Body.Close()
	m := printHeaders(r, w)
	if err := copyBody(m, w, r); err != nil {
		log.Printf("[ERROR] %v\n", err)
	}
}

func printHeaders(r *http.Request, w http.ResponseWriter) map[string]string {
	m := map[string]string{
		"Host":           r.Host,
		"Method":         r.Method,
		"Proto":          r.Proto,
		"RemoteAddr":     r.RemoteAddr,
		"RequestURI":     r.RequestURI,
		"Content-Length": strconv.Itoa(int(r.ContentLength)),
	}
	for k, v := range r.Header {
		vjoined := strings.Join(v, ",")
		log.Printf("[%s]: %s\n", k, vjoined)
		//w.Header().Set(k, vjoined)
		m[k] = vjoined
	}
	return m
}
func copyBody(m map[string]string, w http.ResponseWriter, r *http.Request) error {
	var buf *bytes.Buffer
	if b, err := json.Marshal(m); err == nil {
		buf = bytes.NewBuffer(b)
	}
	if buf == nil {
		return errors.New("nil buf")
	}
	_, err := io.Copy(buf, r.Body)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Length", strconv.Itoa(len(buf.Bytes())))
	_, err = io.Copy(w, buf)
	if err != nil {
		return err
	}

	return nil
}
