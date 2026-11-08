package sefaz

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

func TestCadeiaICPBrasil(t *testing.T) {
	roots, inters, err := loadICP(icpBrasilPEM, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 3 || len(inters) < 100 {
		t.Errorf("raízes=%d intermediários=%d", len(roots), len(inters))
	}
}

// Uma AC forjada colada no arquivo não pode virar âncora: ela não descende de nenhuma raiz
// fixada.
func TestACForjadaNaoEntra(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "AC Forjada SSL"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	forged := append(append([]byte{}, icpBrasilPEM...), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})...)

	_, inters, err := loadICP(forged, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range inters {
		if c.Subject.CommonName == "AC Forjada SSL" {
			t.Fatal("AC forjada aceita como intermediária")
		}
	}
}
