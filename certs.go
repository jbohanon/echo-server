package main

// from here: https://golang.org/src/crypto/tls/generate_cert.go

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

func publicKey(priv interface{}) interface{} {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}

}

func pemBlockForKey(priv interface{}) *pem.Block {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to marshal ECDSA private key: %v", err)
			os.Exit(2)
		}
		return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
	default:
		return nil
	}

}

type Params struct {
	Hosts            string             // Comma-separated hostnames and IPs to generate a certificate for
	ValidFrom        *time.Time         // Creation date
	ValidFor         *time.Duration     // Duration that certificate is valid for
	IsCA             bool               // whether this cert should be its own Certificate Authority
	RsaBits          int                // Size of RSA key to generate. Ignored if EcdsaCurve is set
	EcdsaCurve       string             // ECDSA curve to use to generate a key. Valid values are P224, P256 (recommended), P384, P521
	AdditionalUsages []x509.ExtKeyUsage // Usages to define in addition to default x509.ExtKeyUsageServerAuth
}

func GetCerts(params Params) (string, string) {

	if len(params.Hosts) == 0 {
		log.Fatal("Missing required --host parameter")
	}

	var priv interface{}
	var err error
	switch params.EcdsaCurve {
	case "":
		if params.RsaBits == 0 {
			params.RsaBits = 2048
		}
		priv, err = rsa.GenerateKey(rand.Reader, params.RsaBits)
	case "P224":
		priv, err = ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	case "P256":
		priv, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case "P384":
		priv, err = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	case "P521":
		priv, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	default:
		log.Fatal(fmt.Sprintf("Unrecognized elliptic curve: %q", params.EcdsaCurve))
	}
	if err != nil {
		log.Fatal(err)
	}

	var notBefore time.Time
	if params.ValidFrom == nil {
		notBefore = time.Now().Add(-time.Minute)
	} else {
		notBefore = *params.ValidFrom
	}

	if params.ValidFor == nil {
		tmp := time.Hour * 24
		params.ValidFor = &tmp
	}

	notAfter := notBefore.Add(*params.ValidFor)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("failed to generate serial number: %s", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	template.ExtKeyUsage = append(template.ExtKeyUsage, params.AdditionalUsages...)

	hosts := strings.Split(params.Hosts, ",")
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	if params.IsCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(priv), priv)
	if err != nil {
		log.Fatalf("Failed to create certificate: %s", err)
	}

	var certOut bytes.Buffer
	err = pem.Encode(&certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		log.Fatal(err)
	}

	var keyOut bytes.Buffer

	err = pem.Encode(&keyOut, pemBlockForKey(priv))
	if err != nil {
		log.Fatal(err)
	}

	return certOut.String(), keyOut.String()
}

var (
	getCerts     sync.Once
	getMtlsCerts sync.Once
	mtlsCert     string
	mtlsPrivKey  string
	cert         string
	privKey      string
)

func gencerts() {

	cert, privKey = GetCerts(Params{
		Hosts: "gateway-proxy,knative-proxy,ingress-proxy",
		IsCA:  true,
	})
}

func genmtlscerts() {
	mtlsCert, mtlsPrivKey = GetCerts(Params{
		Hosts:            "localhost",
		IsCA:             true,
		AdditionalUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
}

func Certificate() string {
	getCerts.Do(gencerts)
	return cert
}

func PrivateKey() string {
	getCerts.Do(gencerts)
	return privKey
}

func MtlsCertificate() string {
	getMtlsCerts.Do(genmtlscerts)
	return mtlsCert
}

func MtlsPrivateKey() string {
	getMtlsCerts.Do(genmtlscerts)
	return mtlsPrivKey
}
