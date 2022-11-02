package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func main() {
	port := ":9999"
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		m := printHeaders(r, w)
		if err := copyBody(m, w, r); err != nil {
			log.Printf("[ERROR] %v\n", err)
		}
	})

	log.Println("Server is running on localhost:9999")

	log.Fatal(http.ListenAndServe(port, nil))
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
