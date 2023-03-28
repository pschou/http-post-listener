package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

var (
	certFile = flag.String("cert", "someCertFile", "A PEM encoded certificate file.")
	keyFile  = flag.String("key", "someKeyFile", "A PEM encoded private key file.")
	caFile   = flag.String("CA", "someCertCAFile", "A PEM encoded CA's certificate file.")
	ciphers  = flag.String("ciphers", "RSA_WITH_AES_128_GCM_SHA256, RSA_WITH_AES_256_GCM_SHA384, ECDHE_ECDSA_WITH_AES_128_GCM_SHA256, ECDHE_RSA_WITH_AES_128_GCM_SHA256, ECDHE_ECDSA_WITH_AES_256_GCM_SHA384, ECDHE_RSA_WITH_AES_256_GCM_SHA384, ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256, ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256", "List of ciphers to enable")
)

var cipher_list = `
Available ciphers to pick from:
	# TLS 1.0 - 1.2 cipher suites.
	RSA_WITH_RC4_128_SHA
	RSA_WITH_3DES_EDE_CBC_SHA
	RSA_WITH_AES_128_CBC_SHA
	RSA_WITH_AES_256_CBC_SHA
	RSA_WITH_AES_128_CBC_SHA256
	RSA_WITH_AES_128_GCM_SHA256
	RSA_WITH_AES_256_GCM_SHA384
	ECDHE_ECDSA_WITH_RC4_128_SHA
	ECDHE_ECDSA_WITH_AES_128_CBC_SHA
	ECDHE_ECDSA_WITH_AES_256_CBC_SHA
	ECDHE_RSA_WITH_RC4_128_SHA
	ECDHE_RSA_WITH_3DES_EDE_CBC_SHA
	ECDHE_RSA_WITH_AES_128_CBC_SHA
	ECDHE_RSA_WITH_AES_256_CBC_SHA
	ECDHE_ECDSA_WITH_AES_128_CBC_SHA256
	ECDHE_RSA_WITH_AES_128_CBC_SHA256
	ECDHE_RSA_WITH_AES_128_GCM_SHA256
	ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
	ECDHE_RSA_WITH_AES_256_GCM_SHA384
	ECDHE_ECDSA_WITH_AES_256_GCM_SHA384
	ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256
	ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256

	# TLS 1.3 cipher suites.
	AES_128_GCM_SHA256
	AES_256_GCM_SHA384
	CHACHA20_POLY1305_SHA256`

var cipher_map = map[string]uint16{
	"RSA_WITH_RC4_128_SHA":                      tls.TLS_RSA_WITH_RC4_128_SHA,
	"RSA_WITH_3DES_EDE_CBC_SHA":                 tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	"RSA_WITH_AES_128_CBC_SHA":                  tls.TLS_RSA_WITH_AES_128_CBC_SHA,
	"RSA_WITH_AES_256_CBC_SHA":                  tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	"RSA_WITH_AES_128_CBC_SHA256":               tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
	"RSA_WITH_AES_128_GCM_SHA256":               tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	"RSA_WITH_AES_256_GCM_SHA384":               tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	"ECDHE_ECDSA_WITH_RC4_128_SHA":              tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA,
	"ECDHE_ECDSA_WITH_AES_128_CBC_SHA":          tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
	"ECDHE_ECDSA_WITH_AES_256_CBC_SHA":          tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
	"ECDHE_RSA_WITH_RC4_128_SHA":                tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
	"ECDHE_RSA_WITH_3DES_EDE_CBC_SHA":           tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
	"ECDHE_RSA_WITH_AES_128_CBC_SHA":            tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	"ECDHE_RSA_WITH_AES_256_CBC_SHA":            tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	"ECDHE_ECDSA_WITH_AES_128_CBC_SHA256":       tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
	"ECDHE_RSA_WITH_AES_128_CBC_SHA256":         tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
	"ECDHE_RSA_WITH_AES_128_GCM_SHA256":         tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	"ECDHE_ECDSA_WITH_AES_128_GCM_SHA256":       tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	"ECDHE_RSA_WITH_AES_256_GCM_SHA384":         tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	"ECDHE_ECDSA_WITH_AES_256_GCM_SHA384":       tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	"ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256":   tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
	"ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256": tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
	"AES_128_GCM_SHA256":                        tls.TLS_AES_128_GCM_SHA256,
	"AES_256_GCM_SHA384":                        tls.TLS_AES_256_GCM_SHA384,
	"CHACHA20_POLY1305_SHA256":                  tls.TLS_CHACHA20_POLY1305_SHA256,
}

var tlsConfig *tls.Config

func loadTLS() {
	// Load client cert
	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Fatal(err)
	}

	cipherList := []uint16{}
	for _, c := range strings.Split(*ciphers, ",") {
		c = strings.TrimSpace(c)
		if cv, ok := cipher_map[c]; ok {
			cipherList = append(cipherList, cv)
		} else {
			log.Fatal("Unknown cipher: ", c)
		}
	}

	// Load CA cert
	caCert, err := ioutil.ReadFile(*caFile)
	if err != nil {
		log.Fatal(err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	// Setup HTTPS client
	tlsConfig = &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caCertPool,
		ClientCAs:          caCertPool,
		InsecureSkipVerify: false,
		ClientAuth:         tls.VerifyClientCertIfGiven,

		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites:             cipherList,
	}
	tlsConfig.BuildNameToCertificate()
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	http.DefaultClient = &http.Client{Transport: transport}
	http.DefaultClient = &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
}
