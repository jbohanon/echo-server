package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
)

func main() {
	port := ":9999"

	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/headers", handleHeaders)
	http.HandleFunc("/body", handleBody)

	log.Println("Server is running on localhost:9999")

	log.Fatal(http.ListenAndServe(port, nil))
}

type responseBodyConstructor func(*request) error

type request struct {
	w             http.ResponseWriter
	body          []byte
	responseBody  *bytes.Buffer
	headers       http.Header
	contentLength int
}

func (r *request) writeResponse(code int) {
	r.w.Header().Set("Content-Length", strconv.Itoa(r.contentLength))
	_, err := io.Copy(r.w, r.responseBody)
	if err != nil {
		r.w.Header().Add("status", strconv.Itoa(http.StatusInternalServerError))
		r.w.Write([]byte(err.Error()))
	}
}

// newRequest reads the body and closes it
func newRequest(w http.ResponseWriter, r *http.Request) (*request, error) {
	defer r.Body.Close()

	b, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	req := &request{
		headers:      r.Header,
		body:         b,
		responseBody: &bytes.Buffer{},
		w:            w,
	}
	req.headers.Add("Host", r.Host)
	req.headers.Add("Method", r.Method)
	req.headers.Add("Proto", r.Proto)
	req.headers.Add("RemoteAddr", r.RemoteAddr)
	req.headers.Add("RequestURI", r.RequestURI)
	req.headers.Add("X-Received-Content-Length", strconv.Itoa(int(r.ContentLength)))

	return req, nil
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	handle(w, r, responseBodyWithHeaders)
}

func handleBody(w http.ResponseWriter, r *http.Request) {
	handle(w, r, responseBodyEcho)
}

func handleHeaders(w http.ResponseWriter, r *http.Request) {
	handle(w, r, responseBodyHeadersOnly)
}

func handle(w http.ResponseWriter, r *http.Request, bodyConstructor responseBodyConstructor) {
	req, err := newRequest(w, r)
	if err != nil {
		returnError(w, err, http.StatusBadRequest)
	}

	err = bodyConstructor(req)
	if err != nil {
		returnError(w, err, http.StatusInternalServerError)
	}

	req.writeResponse(http.StatusOK)

}

func returnError(w http.ResponseWriter, err error, code int) {
	w.Header().Add("status", strconv.Itoa(code))
	w.Write([]byte(err.Error()))
}

func responseBodyHeadersOnly(req *request) error {
	b, err := json.Marshal(req.headers)
	if err != nil {
		return err
	}

	req.contentLength, err = req.responseBody.Write(b)
	return nil
}

func responseBodyEcho(req *request) error {
	var err error
	req.contentLength, err = req.responseBody.Write(req.body)
	return err
}

func responseBodyWithHeaders(req *request) error {
	m := map[string]any{
		"headers": req.headers,
		"body":    req.body,
	}

	b, err := json.Marshal(m)
	if err != nil {
		return err
	}

	req.contentLength, err = req.responseBody.Write(b)
	return nil
}
