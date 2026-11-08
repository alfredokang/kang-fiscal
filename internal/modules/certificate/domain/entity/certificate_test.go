package entity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain"
	"github.com/alfredokang/kang-fiscal/internal/modules/certificate/domain/entity"
	"github.com/alfredokang/kang-fiscal/pkg/certificate"
)

const issuerRef = "01a1bb1a-a6ac-72fd-9d66-8078b42feefd"

func pfx(t *testing.T, cnpj string, notAfter time.Time) []byte {
	t.Helper()
	b, err := certificate.TestPFX("CAFE EXEMPLO LTDA", cnpj, "s", notAfter)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestUploadConfereOEmitente(t *testing.T) {
	now := time.Now()
	year := now.Add(365 * 24 * time.Hour)

	// O A1 da matriz (0001) assina a nota da filial (0002): a raiz é a mesma.
	c, err := entity.FromUpload("default", issuerRef, "11222333000262", pfx(t, "11222333000181", year), "s", now)
	if err != nil {
		t.Fatalf("certificado da matriz para a filial: %v", err)
	}
	if c.CNPJ != "11222333000181" || c.HolderName != "CAFE EXEMPLO LTDA" || c.ExpiringSoon(now) {
		t.Errorf("certificado: %+v", c)
	}

	cases := map[string]struct {
		pfx      []byte
		password string
		issuer   string
		want     error
	}{
		"outra empresa":  {pfx(t, "99888777000166", year), "s", "11222333000181", domain.ErrCNPJMismatch},
		"senha errada":   {pfx(t, "11222333000181", year), "x", "11222333000181", domain.ErrWrongPassword},
		"vencido":        {pfx(t, "11222333000181", now.Add(-time.Minute)), "s", "11222333000181", domain.ErrExpired},
		"arquivo vazio":  {nil, "s", "11222333000181", domain.ErrEmptyFile},
		"grande demais":  {make([]byte, certificate.MaxFileSize+1), "s", "11222333000181", domain.ErrFileTooLarge},
		"não é um .pfx":  {[]byte("texto"), "s", "11222333000181", domain.ErrWrongPassword},
		"alfanumérico ≠": {pfx(t, "12ABC34501DE35", year), "s", "12ABD34501DE35", domain.ErrCNPJMismatch},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := entity.FromUpload("default", issuerRef, c.issuer, c.pfx, c.password, now); !errors.Is(err, c.want) {
				t.Errorf("esperava %v, veio %v", c.want, err)
			}
		})
	}
}

func TestAvisoDeVencimento(t *testing.T) {
	now := time.Now()
	c, err := entity.FromUpload("default", issuerRef, "11222333000181",
		pfx(t, "11222333000181", now.Add(20*24*time.Hour)), "s", now)
	if err != nil {
		t.Fatal(err)
	}
	if !c.ExpiringSoon(now) || c.DaysLeft(now) != 19 {
		t.Errorf("faltam %d dias, aviso=%v", c.DaysLeft(now), c.ExpiringSoon(now))
	}
}
