package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
)

var (
	server, tlsServer *http.Server
	port, tlsPort     int
	verifyClient      bool
)

func main() {
	log.Println("parsing flags")
	flag.BoolVar(&verifyClient, "verify-client-cert", false, "set to true to turn on client cert verification; client cert can be retrieved by querying /cert on http port")
	flag.IntVar(&port, "port", 8080, "http port")
	flag.IntVar(&tlsPort, "tls-port", 8443, "https port")
	flag.Parse()
	log.Println("flags parsed")
	go startHttpServer()
	go startHttpsServer()
	forever := make(<-chan bool)
	<-forever
}

func startHttpServer() {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}

	var handlerFunc http.HandlerFunc

	mux := http.NewServeMux()
	handlerFunc = func(w http.ResponseWriter, r *http.Request) {
		sharedHandleFunc("http", w, r)
	}
	mux.Handle("/cert", http.HandlerFunc(getCertHandler))
	mux.Handle("/key", http.HandlerFunc(getKeyHandler))
	mux.Handle("/", handlerFunc)
	server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	go func() {
		log.Printf("server is running on localhost:%d\n", port)
		log.Fatal(server.Serve(listener))
	}()
}

func startHttpsServer() {
	var tlsHandlerFunc http.HandlerFunc
	tlsHandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		sharedHandleFunc("https", w, r)
	}
	tlsServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", tlsPort),
		Handler: tlsHandlerFunc,
	}
	cer, err := tls.X509KeyPair([]byte(MtlsCertificate()), []byte(MtlsPrivateKey()))
	if err != nil {
		log.Fatal(err)
	}

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM([]byte(MtlsCertificate()))
	tlsCfg := &tls.Config{
		GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return &cer, nil
		},
		ClientAuth: tls.RequireAnyClientCert,
		ClientCAs:  certPool,
	}
	if verifyClient {
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	tlsListener, err := tls.Listen("tcp", fmt.Sprintf(":%d", tlsPort), tlsCfg)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("TLS server is running on localhost:%d\n", tlsPort)
	log.Fatal(tlsServer.Serve(tlsListener))
}

func getCertHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(MtlsCertificate()))
}
func getKeyHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(MtlsPrivateKey()))
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
