/*
 * Copyright 2023 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package standard

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

func TestDial(t *testing.T) {
	const nw = "tcp"
	const addr = "localhost:10104"
	transporter := NewTransporter(&config.Options{
		Addr:    addr,
		Network: nw,
	})

	go transporter.ListenAndServe(func(ctx context.Context, conn interface{}) error {
		return nil
	})
	defer transporter.Close()
	time.Sleep(time.Millisecond * 100)

	dial := NewDialer()
	_, err := dial.DialConnection(nw, addr, time.Second, nil)
	assert.Nil(t, err)

	nConn, err := dial.DialTimeout(nw, addr, time.Second, nil)
	assert.Nil(t, err)
	defer nConn.Close()
}

func TestDialTLS(t *testing.T) {
	const nw = "tcp"
	const addr = "localhost:10105"
	data := []byte("abcdefg")
	listened := make(chan struct{})
	go func() {
		mockTLSServe(nw, addr, func(conn net.Conn) {
			defer conn.Close()
			_, err := conn.Write(data)
			assert.Nil(t, err)
		}, listened)
	}()

	select {
	case <-listened:
	case <-time.After(time.Second * 5):
		t.Fatalf("timeout")
	}

	buf := make([]byte, len(data))

	dial := NewDialer()
	conn, err := dial.DialConnection(nw, addr, time.Second, &tls.Config{
		InsecureSkipVerify: true,
	})
	assert.Nil(t, err)

	_, err = conn.Read(buf)
	assert.Nil(t, err)
	assert.DeepEqual(t, string(data), string(buf))

	conn, err = dial.DialConnection(nw, addr, time.Second, nil)
	assert.Nil(t, err)
	nConn, err := dial.AddTLS(conn, &tls.Config{
		InsecureSkipVerify: true,
	})
	assert.Nil(t, err)

	_, err = nConn.Read(buf)
	assert.Nil(t, err)
	assert.DeepEqual(t, string(data), string(buf))
}

func mockTLSServe(nw, addr string, handle func(conn net.Conn), listened chan struct{}) (err error) {
	certData, keyData, err := generateTestCertificate("")
	if err != nil {
		return
	}

	cert, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		return
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	ln, err := tls.Listen(nw, addr, tlsConfig)
	if err != nil {
		return
	}
	defer ln.Close()

	listened <- struct{}{}
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handle(conn)
	}
}

// generateTestCertificate generates a test certificate and private key based on the given host.
func generateTestCertificate(host string) ([]byte, []byte, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, err
	}

	cert := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"fasthttp test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		SignatureAlgorithm:    x509.SHA256WithRSA,
		DNSNames:              []string{host},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certBytes, err := x509.CreateCertificate(
		rand.Reader, cert, cert, &priv.PublicKey, priv,
	)

	p := pem.EncodeToMemory(
		&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(priv),
		},
	)

	b := pem.EncodeToMemory(
		&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certBytes,
		},
	)

	return b, p, err
}
