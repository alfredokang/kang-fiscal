package nfe_test

import (
	"errors"
	"testing"

	"github.com/alfredokang/kang-fiscal/pkg/nfe"
	"github.com/alfredokang/kang-fiscal/pkg/nfe/nfetest"
	"github.com/alfredokang/kang-fiscal/pkg/xmldsig"
)

func TestInutilizacaoPassaNoXSD(t *testing.T) {
	v, err := nfe.NewVoidRange(nfe.VoidRange{TpAmb: 2, CUF: 35, Model: 55, Series: 1, Year: 2026, CNPJ: nfetest.CNPJ,
		From: 5, To: 7, Justification: "Numeros pulados por falha na emissao"})
	if err != nil {
		t.Fatal(err)
	}
	if v.InfInut.ID != "ID3526"+nfetest.CNPJ+"55001000000005000000007" {
		t.Errorf("Id: %s", v.InfInut.ID)
	}
	raw, _ := nfe.MarshalVoid(v)
	pfxSigned := signVoid(t, raw)
	if _, err := xmldsig.Verify(pfxSigned); err != nil {
		t.Fatal(err)
	}
	nfetest.MustValidateSchema(t, pfxSigned, "inutNFe_v4.00.xsd")

	if _, err := nfe.NewVoidRange(nfe.VoidRange{From: 7, To: 5, Justification: "Numeros pulados por falha"}); !errors.Is(err, nfe.ErrVoidRange) {
		t.Errorf("faixa invertida: %v", err)
	}
}

func signVoid(t *testing.T, raw []byte) []byte {
	t.Helper()
	return nfetest.SignTag(t, raw, "infInut")
}
