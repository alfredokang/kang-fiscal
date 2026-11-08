package sefaz

import (
	"crypto/sha256"
	"crypto/x509"
	_ "embed"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"time"
)

//go:embed icpbrasil.pem
var icpBrasilPEM []byte

// pinnedRoots: as raízes ICP-Brasil aceitas, pelo SHA-256 do DER. Ver scripts/icpbundle,
// que documenta de onde cada valor foi conferido.
var pinnedRoots = map[string]bool{
	"caa53fc6091c6951887c976e378f6ef89aa6377c55d97b6475422b71ed7e9b17": true, // v5
	"6e0bff069a26994c15de2c4888cc54af84882e5495b7fbf66be9ccffec7489f6": true, // v10
	"d8478e37ce19c690cf657381e68fe600e4e1a042536830f06847e03e554c4b01": true, // v12
}

// TrustPool devolve as raízes do sistema mais a cadeia ICP-Brasil.
//
// O sistema entra porque algumas SEFAZ (GO, SVAN) usam certificado de AC pública. A
// ICP-Brasil entra porque as outras usam certificado ICP, que nenhum sistema traz.
//
// O arquivo embutido é reverificado a cada carga: raiz só entra se o SHA-256 estiver
// fixado, e intermediário só entra se a assinatura fechar numa raiz fixada. Um
// icpbrasil.pem editado à mão não consegue introduzir âncora falsa.
func TrustPool() (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	roots, inters, err := loadICP(icpBrasilPEM, time.Now())
	if err != nil {
		return nil, err
	}
	for _, c := range roots {
		pool.AddCert(c)
	}
	for _, c := range inters {
		pool.AddCert(c)
	}
	return pool, nil
}

func loadICP(bundle []byte, now time.Time) (roots, inters []*x509.Certificate, err error) {
	var all []*x509.Certificate
	for rest := bundle; ; {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, nil, err
		}
		all = append(all, c)
	}
	rootPool := x509.NewCertPool()
	for _, c := range all {
		sum := sha256.Sum256(c.Raw)
		if pinnedRoots[hex.EncodeToString(sum[:])] {
			rootPool.AddCert(c)
			roots = append(roots, c)
		}
	}
	if len(roots) != len(pinnedRoots) {
		return nil, nil, errors.New("sefaz: icpbrasil.pem sem todas as raízes fixadas")
	}
	interPool := x509.NewCertPool()
	for changed := true; changed; {
		changed = false
		for _, c := range all {
			if !c.IsCA || contains(roots, c) || contains(inters, c) || now.After(c.NotAfter) {
				continue
			}
			if _, err := c.Verify(x509.VerifyOptions{Roots: rootPool, Intermediates: interPool,
				CurrentTime: now, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}}); err == nil {
				interPool.AddCert(c)
				inters = append(inters, c)
				changed = true
			}
		}
	}
	return roots, inters, nil
}

func contains(list []*x509.Certificate, c *x509.Certificate) bool {
	for _, x := range list {
		if x.Equal(c) {
			return true
		}
	}
	return false
}
