// Package secretbox cifra os segredos que ficam no banco (a chave privada do certificado A1, a
// senha dele, o CSC da NFC-e): AES-256-GCM, com a chave SÓ no ambiente do servidor.
//
// Por que cifrar, se o banco já é protegido: cópia de banco circula — backup, dump para
// testar, DataGrip na máquina de alguém. Sem a chave, o que está lá é ruído. E por que a
// chave fica fora do banco: a mesma cópia que leva a credencial não pode levar a chave.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
)

// ErrNoKey: o servidor subiu sem SECRETS_ENCRYPTION_KEY. Gravar credencial fica recusado
// em vez de cair em texto puro.
var ErrNoKey = errors.New("o servidor está sem a chave de criptografia (SECRETS_ENCRYPTION_KEY)")

// ErrInvalidKey: a chave não é base64 de 32 bytes.
var ErrInvalidKey = errors.New("SECRETS_ENCRYPTION_KEY inválida: use 32 bytes em base64 (openssl rand -base64 32)")

// ErrCorrupted: o texto cifrado não abre com esta chave — trocaram a chave, ou o valor foi
// mexido à mão no banco.
var ErrCorrupted = errors.New("credencial cifrada ilegível: a chave mudou ou o valor foi alterado")

// prefixo marca a versão do formato, para trocar o algoritmo um dia sem ambiguidade.
const prefixo = "v1:"

// Box cifra e decifra com uma chave.
type Box struct {
	aead cipher.AEAD
}

// New monta a caixa a partir da chave em base64. Chave vazia devolve uma caixa que recusa
// tudo com ErrNoKey — o servidor sobe, e só a gravação de credencial fica bloqueada.
func New(keyBase64 string) (*Box, error) {
	keyBase64 = strings.TrimSpace(keyBase64)
	if keyBase64 == "" {
		return &Box{}, nil
	}
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil || len(key) != 32 {
		return nil, ErrInvalidKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrInvalidKey
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrInvalidKey
	}
	return &Box{aead: aead}, nil
}

// Ready diz se há chave.
func (b *Box) Ready() bool { return b != nil && b.aead != nil }

// Seal cifra. O nonce é aleatório a cada chamada: o mesmo segredo cifrado duas vezes sai
// diferente, e ninguém descobre que dois emitentes usam a mesma senha.
func (b *Box) Seal(plain string) (string, error) {
	if !b.Ready() {
		return "", ErrNoKey
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := b.aead.Seal(nonce, nonce, []byte(plain), nil)
	return prefixo + base64.StdEncoding.EncodeToString(out), nil
}

// Open decifra.
func (b *Box) Open(sealed string) (string, error) {
	if !b.Ready() {
		return "", ErrNoKey
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(sealed, prefixo))
	if err != nil || !strings.HasPrefix(sealed, prefixo) || len(raw) < b.aead.NonceSize() {
		return "", ErrCorrupted
	}
	nonce, data := raw[:b.aead.NonceSize()], raw[b.aead.NonceSize():]
	plain, err := b.aead.Open(nil, nonce, data, nil)
	if err != nil {
		return "", ErrCorrupted
	}
	return string(plain), nil
}
