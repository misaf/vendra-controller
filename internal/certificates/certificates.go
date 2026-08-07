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

// CAFileName is the local authority every serving certificate is issued from.
//
// It exists because a self-signed *leaf* cannot be its own trust anchor: adding
// one to a trust store still fails verification with
// DEPTH_ZERO_SELF_SIGNED_CERT, so the storefront's server-side calls to the API
// could never be made to succeed under certificate_mode: self-signed. Clients
// trust this file; the serving key is not a trust anchor and compromising it
// alone does not let anyone issue further certificates.
const CAFileName = "ca.pem"

func Generate(dir, baseDomain string, domains []string) error {
	names := []string{baseDomain, "*." + baseDomain, "*.admin." + baseDomain}
	for _, domain := range domains {
		names = append(names, domain, "www."+domain)
	}
	sort.Strings(names)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	authority, authorityKey, err := loadOrCreateAuthority(dir, baseDomain)
	if err != nil {
		return err
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	serial, err := serialNumber()
	if err != nil {
		return err
	}
	template := x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: baseDomain}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(10, 0, 0), DNSNames: names, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, &template, authority, &key.PublicKey, authorityKey)
	if err != nil {
		return err
	}
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	private := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := filesystem.AtomicWrite(filepath.Join(dir, "local.pem"), cert, 0o644); err != nil {
		return err
	}
	return filesystem.AtomicWrite(filepath.Join(dir, "local-key.pem"), private, 0o600)
}

// loadOrCreateAuthority reuses the authority on disk whenever one is there.
//
// Reissuing it would be the more obvious thing to do, and it is wrong: a new
// property regenerates the serving certificate, and a new authority each time
// would break every storefront already running against the old one until it was
// restarted.
func loadOrCreateAuthority(dir, baseDomain string) (*x509.Certificate, *rsa.PrivateKey, error) {
	certPath := filepath.Join(dir, CAFileName)
	keyPath := filepath.Join(dir, "ca-key.pem")
	certPEM, certErr := os.ReadFile(certPath)
	keyPEM, keyErr := os.ReadFile(keyPath)
	if certErr == nil && keyErr == nil {
		certBlock, _ := pem.Decode(certPEM)
		keyBlock, _ := pem.Decode(keyPEM)
		if certBlock != nil && keyBlock != nil {
			cert, err := x509.ParseCertificate(certBlock.Bytes)
			if err != nil {
				return nil, nil, err
			}
			key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
			if err != nil {
				return nil, nil, err
			}
			return cert, key, nil
		}
	}
	if certErr != nil && !os.IsNotExist(certErr) {
		return nil, nil, certErr
	}
	if keyErr != nil && !os.IsNotExist(keyErr) {
		return nil, nil, keyErr
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	serial, err := serialNumber()
	if err != nil {
		return nil, nil, err
	}
	template := x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "Vendra local authority (" + baseDomain + ")"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(10, 0, 0), KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature, BasicConstraintsValid: true, IsCA: true, MaxPathLen: 0, MaxPathLenZero: true}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}
	if err := filesystem.AtomicWrite(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		return nil, nil, err
	}
	if err := filesystem.AtomicWrite(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0o600); err != nil {
		return nil, nil, err
	}
	return cert, key, nil
}

func serialNumber() (*big.Int, error) {
	return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
}

func ListenAddress(host string) string { return net.JoinHostPort(host, "443") }
