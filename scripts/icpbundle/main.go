// Comando icpbundle gera pkg/sefaz/icpbrasil.pem: as raízes da ICP-Brasil que servem ao
// TLS e os intermediários que comprovadamente descendem delas.
//
//	go run ./scripts/icpbundle <diretório com .crt/.cer> > pkg/sefaz/icpbrasil.pem
//
// Por que existe: 11 de 13 SEFAZ testadas em 2026-11-08 apresentam certificado ICP-Brasil,
// que nenhum sistema operacional traz de fábrica, e várias não enviam os intermediários no
// handshake. Sem este pacote, a saída fácil seria desligar a verificação de TLS — o que a
// Constituição proíbe (Art. 9.2).
//
// Como a confiança é estabelecida: as raízes entram só se o SHA-256 bater com os valores
// fixados abaixo, conferidos no repositório de raízes da Microsoft (CCADB) e em cópias
// independentes. Os intermediários podem vir de qualquer fonte — inclusive uma adulterada —
// porque só entram se a assinatura fechar até uma raiz fixada.
package main

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// pinnedRoots são as raízes aceitas, pelo SHA-256 do DER.
//
// v5 e v10: iguais no repositório da Microsoft (CCADB, 2026-03-28) e em quatro projetos
// independentes. v12 (emitida em 2024, ainda fora do CCADB): igual em três projetos
// independentes. A v13 (2025) ficou de fora: só uma fonte a confirmava.
var pinnedRoots = map[string]string{
	"caa53fc6091c6951887c976e378f6ef89aa6377c55d97b6475422b71ed7e9b17": "Autoridade Certificadora Raiz Brasileira v5",
	"6e0bff069a26994c15de2c4888cc54af84882e5495b7fbf66be9ccffec7489f6": "Autoridade Certificadora Raiz Brasileira v10",
	"d8478e37ce19c690cf657381e68fe600e4e1a042536830f06847e03e554c4b01": "Autoridade Certificadora Raiz Brasileira v12",
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "uso: go run ./scripts/icpbundle <diretório>")
		os.Exit(2)
	}
	certs, err := readAll(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	roots := x509.NewCertPool()
	var rootCerts []*x509.Certificate
	seen := map[string]bool{}
	for _, c := range certs {
		fp := fingerprint(c)
		if _, ok := pinnedRoots[fp]; ok && !seen[fp] {
			roots.AddCert(c)
			rootCerts = append(rootCerts, c)
			seen[fp] = true
		}
	}
	if len(rootCerts) != len(pinnedRoots) {
		fmt.Fprintf(os.Stderr, "faltam raízes: achei %d de %d\n", len(rootCerts), len(pinnedRoots))
		os.Exit(1)
	}

	// Intermediários: só CA, ainda válidos, e cuja cadeia fecha numa raiz fixada. A
	// verificação roda em rodadas porque há AC de segundo nível (AC → AC → raiz).
	now := time.Now()
	inter := x509.NewCertPool()
	var accepted []*x509.Certificate
	for changed := true; changed; {
		changed = false
		for _, c := range certs {
			fp := fingerprint(c)
			if seen[fp] || !c.IsCA || now.After(c.NotAfter) {
				continue
			}
			if _, err := c.Verify(x509.VerifyOptions{Roots: roots, Intermediates: inter,
				CurrentTime: now, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}}); err != nil {
				continue
			}
			inter.AddCert(c)
			accepted = append(accepted, c)
			seen[fp] = true
			changed = true
		}
	}

	sort.Slice(accepted, func(i, j int) bool { return accepted[i].Subject.CommonName < accepted[j].Subject.CommonName })
	fmt.Printf("# Cadeia ICP-Brasil para o TLS com a SEFAZ — gerado por scripts/icpbundle em %s.\n", now.UTC().Format("2006-01-02"))
	fmt.Printf("# %d raízes fixadas por SHA-256 e %d intermediários verificados até elas.\n", len(rootCerts), len(accepted))
	for _, c := range append(rootCerts, accepted...) {
		fmt.Printf("\n# %s\n# SHA-256 %s, válido até %s\n", c.Subject.CommonName, fingerprint(c), c.NotAfter.Format("2006-01-02"))
		_ = pem.Encode(os.Stdout, &pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})
	}
	fmt.Fprintf(os.Stderr, "%d raízes, %d intermediários aceitos de %d arquivos\n", len(rootCerts), len(accepted), len(certs))
}

func readAll(dir string) ([]*x509.Certificate, error) {
	var out []*x509.Certificate
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".crt" && ext != ".cer" && ext != ".pem" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		found := false
		for rest := data; ; {
			block, r := pem.Decode(rest)
			if block == nil {
				break
			}
			if c, err := x509.ParseCertificate(block.Bytes); err == nil {
				out = append(out, c)
				found = true
			}
			rest = r
		}
		if found {
			return nil
		}
		if c, err := x509.ParseCertificate(data); err == nil {
			out = append(out, c)
		}
		return nil
	})
	return out, err
}

func fingerprint(c *x509.Certificate) string {
	sum := sha256.Sum256(c.Raw)
	return hex.EncodeToString(sum[:])
}
