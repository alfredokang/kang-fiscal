// Package entity contém a credencial de API.
package entity

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/credential/domain"
)

// Escopos.
const (
	ScopeSetup    = "setup"     // emitentes, certificados, séries, perfis tributários
	ScopeNFeWrite = "nfe:write" // emitir, cancelar, corrigir, inutilizar
	ScopeNFeRead  = "nfe:read"  // consultar, baixar XML e DANFE, simular
)

var tenantPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,63}$`)

// Key é uma credencial. O segredo nunca fica aqui depois de criado: só o hash.
type Key struct {
	ID         string
	TenantID   string
	Name       string
	Prefix     string // parte pública: kf_ab12cd34
	Hash       string // HMAC-SHA256(pepper, segredo)
	Scopes     []string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

// Has diz se a credencial tem o escopo.
func (k *Key) Has(scope string) bool {
	for _, s := range k.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// NewKey cria a credencial e devolve o token completo — a única vez em que ele existe.
func NewKey(id, tenantID, name string, scopes []string, pepper []byte, now time.Time) (*Key, string, error) {
	if !tenantPattern.MatchString(tenantID) {
		return nil, "", domain.ErrInvalidTenant
	}
	name = strings.Join(strings.Fields(name), " ")
	if name == "" || len(scopes) == 0 {
		return nil, "", domain.ErrRequiredField
	}
	seen := map[string]bool{}
	var clean []string
	for _, s := range scopes {
		if s != ScopeSetup && s != ScopeNFeWrite && s != ScopeNFeRead {
			return nil, "", domain.ErrInvalidScope
		}
		if !seen[s] {
			seen[s] = true
			clean = append(clean, s)
		}
	}
	if len(pepper) == 0 {
		return nil, "", domain.ErrNoPepper
	}
	prefix := "kf_" + random(8)
	secret := random(40)
	return &Key{ID: id, TenantID: tenantID, Name: name, Prefix: prefix, Hash: HashSecret(pepper, secret),
		Scopes: clean, CreatedAt: now}, prefix + "_" + secret, nil
}

// Split separa o token em prefixo e segredo.
func Split(token string) (prefix, secret string, ok bool) {
	if !strings.HasPrefix(token, "kf_") || len(token) != 3+8+1+40 || token[11] != '_' {
		return "", "", false
	}
	return token[:11], token[12:], true
}

// HashSecret é o HMAC do segredo com o pepper do ambiente: quem leva o dump do banco não
// consegue testar chaves sem o pepper.
func HashSecret(pepper []byte, secret string) string {
	m := hmac.New(sha256.New, pepper)
	m.Write([]byte(secret))
	return hex.EncodeToString(m.Sum(nil))
}

// Matches confere o segredo em tempo constante.
func (k *Key) Matches(pepper []byte, secret string) bool {
	return hmac.Equal([]byte(HashSecret(pepper, secret)), []byte(k.Hash))
}

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// random gera n caracteres base62 com crypto/rand: 40 deles são ~238 bits.
func random(n int) string {
	b := make([]byte, n)
	max := big.NewInt(int64(len(alphabet)))
	for i := range b {
		v, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic(err)
		}
		b[i] = alphabet[v.Int64()]
	}
	return string(b)
}
