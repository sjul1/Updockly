package certs

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CertManager handles generation and storage of self-signed certificates
type CertManager struct {
	CertPath string
	KeyPath  string
	CAPath   string
}

func NewCertManager(certPath, keyPath, caPath string) *CertManager {
	return &CertManager{
		CertPath: certPath,
		KeyPath:  keyPath,
		CAPath:   caPath,
	}
}

func (cm *CertManager) EnsureCertificates() error {
	if fileExists(cm.CertPath) && fileExists(cm.KeyPath) && fileExists(cm.CAPath) {
		return nil
	}

	return cm.generateSelfSigned()
}

func (cm *CertManager) GetCACert() ([]byte, error) {
	return os.ReadFile(cm.CAPath)
}

func (cm *CertManager) generateSelfSigned() error {
	// Ensure destination directory exists and is private
	if err := os.MkdirAll(filepath.Dir(cm.CertPath), 0o700); err != nil {
		return err
	}

	// 1. Generate CA
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2024),
		Subject: pkix.Name{
			Organization: []string{"Updockly Self-Signed CA"},
			CommonName:   "Updockly Root CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return err
	}

	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	// 2. Generate Server Cert
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2025),
		Subject: pkix.Name{
			Organization: []string{"Updockly Server"},
			CommonName:   "updockly-server",
		},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
		DNSNames:     []string{"localhost", "backend", "updockly-backend"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	// Add extra SANs from environment variables
	if extraIPs := os.Getenv("SERVER_SAN_IPS"); extraIPs != "" {
		for _, ipStr := range strings.Split(extraIPs, ",") {
			if ip := net.ParseIP(strings.TrimSpace(ipStr)); ip != nil {
				cert.IPAddresses = append(cert.IPAddresses, ip)
			} else {
				fmt.Printf("Warning: ignoring invalid SERVER_SAN_IPS entry: %s\n", ipStr)
			}
		}
	}
	if extraDomains := os.Getenv("SERVER_SAN_DOMAINS"); extraDomains != "" {
		for _, domain := range strings.Split(extraDomains, ",") {
			if d := strings.TrimSpace(domain); d != "" {
				if strings.ContainsAny(d, " *") {
					fmt.Printf("Warning: ignoring invalid SERVER_SAN_DOMAINS entry: %s\n", d)
					continue
				}
				cert.DNSNames = append(cert.DNSNames, d)
			}
		}
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return err
	}

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	certPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})

	// Save files
	if err := os.WriteFile(cm.CAPath, caPEM.Bytes(), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(cm.CertPath, certPEM.Bytes(), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(cm.KeyPath, certPrivKeyPEM.Bytes(), 0600); err != nil {
		return err
	}

	return nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
