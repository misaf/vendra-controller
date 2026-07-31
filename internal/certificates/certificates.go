package certificates

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/misaf/vendra-controller/internal/filesystem"
)

func Generate(dir, baseDomain string, domains []string) error {
	names := []string{baseDomain, "*." + baseDomain, "*.admin." + baseDomain}
	for _, domain := range domains {
		names = append(names, domain, "www."+domain)
	}
	sort.Strings(names)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	template := x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: baseDomain}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(10, 0, 0), DNSNames: names, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return err
	}
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	private := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := filesystem.AtomicWrite(filepath.Join(dir, "local.pem"), cert, 0o644); err != nil {
		return err
	}
	return filesystem.AtomicWrite(filepath.Join(dir, "local-key.pem"), private, 0o600)
}

func ListenAddress(host string) string { return net.JoinHostPort(host, "443") }
