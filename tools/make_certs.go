package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"
)

// CreateCA creates a new CA with a one year expiration
func CreateCA() error {
	ca := &x509.Certificate{
		SerialNumber:          big.NewInt(2022),
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	// create a 4096 bit private key
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return fmt.Errorf("error generating CA private key: %v", err)
	}
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return fmt.Errorf("error generating CA cert: %v", err)
	}
	// create a ca.pem and pem encode the caBytes above in to it
	caPemFile, err := os.Create("ca.pem")
	if err != nil {
		return fmt.Errorf("error creating ca.pem: %v", err)
	}
	pem.Encode(caPemFile, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	// create and pem encode the CA key
	caKeyFile, err := os.Create("ca.key")
	if err != nil {
		return fmt.Errorf("error creating ca.key: %v", err)
	}
	if err = os.Chmod("ca.key", 0600); err != nil {
		return fmt.Errorf("error chmoding ca.key: %v", err)
	}
	if err = pem.Encode(caKeyFile, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	}); err != nil {
		return fmt.Errorf("error encoding ca.key: %v", err)
	}

	return nil
}

// CertSetup creates a certificate using the CA created above and returns it
// It takes the common name (cn) of the requested cert as an input and uses the
// organization (o) as a role for authorizing the access of this certificate to run methods
func CreateCert(cn, role string) error {
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2022),
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: []string{role},
		},
		DNSNames:    []string{"localhost"},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(0, 0, 30),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}

	caCertPem, err := os.ReadFile("ca.pem")
	if err != nil {
		return err
	}
	caKeyPem, err := os.ReadFile("ca.key")
	if err != nil {
		return err
	}
	// Decode and parse the CA PEM and key so we can use it to sign our certs
	block, _ := pem.Decode(caCertPem)
	ca, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("error parsing ca cert: %v", err)
	}
	block, _ = pem.Decode(caKeyPem)
	caKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("error parsing ca key: %v", err)
	}
	// generate a new RSA key for our cert
	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return fmt.Errorf("error generating key: %v", err)
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("error creating cert for %v: %v", cert.Subject.CommonName, err)
	}

	certPemFile, err := os.Create(cn + ".pem")
	if err != nil {
		return fmt.Errorf("error creating cert.pem: %v", err)
	}
	pem.Encode(certPemFile, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	certPrivKeyFile, err := os.Create(cn + ".key")
	if err != nil {
		return fmt.Errorf("error creating ca.pem: %v", err)
	}
	if err = os.Chmod(cn+".key", 0600); err != nil {
		return fmt.Errorf("error chmoding ca.key: %v", err)
	}
	pem.Encode(certPrivKeyFile, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})

	return nil
}

func main() {
	if err := CreateCA(); err != nil {
		log.Fatalf("error creating CA: %v", err)
	}

	type cert struct {
		name, role string
	}

	certs := []cert{
		{"server", "admin"},
		{"client_user", "user"},
		{"client_admin", "admin"},
	}

	for _, cert := range certs {
		if err := CreateCert(cert.name, cert.role); err != nil {
			log.Fatalf("error creating server cert: %v", err)
		}
	}
}
