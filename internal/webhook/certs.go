package webhook

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

type Certs struct {
	CACert     []byte
	ServerCert []byte
	ServerKey  []byte
}

func GenerateCerts(serviceName, namespace string) (*Certs, error) {
	caEntry := &x509.Certificate{
		SerialNumber:          big.NewInt(2025),
		Subject:               pkix.Name{Organization: []string{"Readiness Controller CA"}},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, fmt.Errorf("failed to generate CA private key: %v", err)
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, caEntry, caEntry, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create CA certificate: %v", err)
	}

	dnsNames := []string{
		serviceName,
		fmt.Sprintf("%s.%s", serviceName, namespace),
		fmt.Sprintf("%s.%s.svc", serviceName, namespace),
	}

	serverEntry := &x509.Certificate{
		SerialNumber: big.NewInt(2026),
		Subject: pkix.Name{
			CommonName:   fmt.Sprintf("%s.%s.svc", serviceName, namespace),
			Organization: []string{"Readiness Controller"},
		},
		DNSNames:     dnsNames,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	serverPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, fmt.Errorf("failed to generate server private key: %v", err)
	}

	serverBytes, err := x509.CreateCertificate(rand.Reader, serverEntry, caEntry, &serverPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create server certificate: %v", err)
	}

	return &Certs{
		CACert:     encodePEM("CERTIFICATE", caBytes),
		ServerCert: encodePEM("CERTIFICATE", serverBytes),
		ServerKey:  encodePEM("RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(serverPrivKey)),
	}, nil
}

func encodePEM(typeStr string, content []byte) []byte {
	b := &pem.Block{
		Type:  typeStr,
		Bytes: content,
	}

	var out bytes.Buffer
	pem.Encode(&out, b)
	return out.Bytes()
}
