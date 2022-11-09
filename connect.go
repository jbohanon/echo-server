package main

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"syscall"
)

func connectProxy(w http.ResponseWriter, r *http.Request) {
	logError := func(err error) {
		http.Error(w, err.Error(), 400)
		log.Printf("[ERROR] %v", err)
	}

	if r.Method != http.MethodConnect {
		logError(errors.New("not CONNECT request"))
		return
	}

	if r.TLS != nil {
		log.Printf("handshake complete %v\n", r.TLS.HandshakeComplete)
		log.Printf("tls version %v\n", r.TLS.Version)
		log.Printf("cipher suite %v\n", r.TLS.CipherSuite)
		log.Printf("negotiated protocol %v\n", r.TLS.NegotiatedProtocol)
	} else {
		log.Printf("no TLS")
	}

	hij, ok := w.(http.Hijacker)
	if !ok {
		logError(errors.New("no hijacker"))
	}
	var targetConn net.Conn
	var err error
	if useTlsTunnel {
		host := r.URL.Host
		hostport := strings.Split(host, ":")
		tlsTarget := false
		if len(hostport) == 2 {
			if hostport[1] == "443" {
				tlsTarget = true
			}
		}

		if tlsTarget {
			log.Println("creating tls dialer")
			dialer := &tls.Dialer{
				Config: &tls.Config{},
			}
			log.Println("dialing tls")
			targetConn, err = dialer.Dial("tcp", host)
			if err != nil {
				log.Printf("[ERROR] can't connect: %v", err)
				http.Error(w, fmt.Sprintf("can't connect: %v", err), 500)
				return
			}
		} else {
			log.Println("creating net dialer")
			targetConn, err = net.Dial("tcp", host)
			if err != nil {
				log.Printf("[ERROR] can't connect: %v", err)
				http.Error(w, fmt.Sprintf("can't connect: %v", err), 500)
				return
			}
		}
	} else {
		log.Println("creating net dialer")
		targetConn, err = net.Dial("tcp", r.URL.Host)
		if err != nil {
			log.Printf("[ERROR] can't connect: %v", err)
			http.Error(w, fmt.Sprintf("can't connect: %v", err), 500)
			return
		}
	}
	defer targetConn.Close()

	conn, buf, err := hij.Hijack()
	if err != nil {
		logError(err)

	}
	defer conn.Close()

	log.Printf("Accepting CONNECT to %s\n", r.URL.Host)
	// note to devs! will only work with HTTP 1.1 request from envoy!
	conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))

	// now just copy:
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		for {
			// read bytes from buf.Reader until EOF
			bts := []byte{1}
			_, err := targetConn.Read(bts)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				logError(err)
				return
			}

			log.Println("read from target", bts)
			_, err = conn.Write(bts)
			if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, syscall.EPIPE) {
				logError(fmt.Errorf("error writing from target to caller %v\n", err))
				return
			}
		}
		err = buf.Flush()
		if err != nil {
			logError(err)
			return
		}
		wg.Done()
	}()
	go func() {
		for !isEof(buf.Reader) {
			// read bytes from buf.Reader until EOF
			bts := []byte{1}
			_, err := buf.Read(bts)
			log.Println("read from envoy", bts)
			if err != nil {
				logError(err)
				return
			}
			_, err = targetConn.Write(bts)
			if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, syscall.EPIPE) {
				logError(fmt.Errorf("error writing from caller to target %v\n", err))
				return
			}
		}
		wg.Done()
	}()

	wg.Wait()
	log.Printf("done proxying\n")
}

func isEof(r *bufio.Reader) bool {
	_, err := r.Peek(1)
	if err == io.EOF {
		return true
	}
	return false
}
