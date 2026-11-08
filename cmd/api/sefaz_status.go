package api

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/alfredokang/kang-fiscal/pkg/certificate"
	"github.com/alfredokang/kang-fiscal/pkg/sefaz"
)

var (
	statusPFX        string
	statusUF         string
	statusModel      int
	statusProduction bool
)

// SefazStatus é o primeiro teste de quem acabou de receber o certificado: se a SEFAZ
// responde "107 — Serviço em Operação", o certificado, a cadeia ICP-Brasil e o endereço
// estão certos, e a emissão vai funcionar.
//
// A senha vem de KANG_PFX_PASSWORD, e não de uma flag: flag fica no histórico do shell e
// aparece no `ps` de quem estiver na mesma máquina.
var SefazStatus = &cobra.Command{
	Use:   "sefaz-status",
	Short: "Testa certificado A1 e conexão com a SEFAZ (KANG_PFX_PASSWORD=... kang-fiscal sefaz-status --pfx a1.pfx --uf SP)",
	Run: func(_ *cobra.Command, _ []string) {
		data, err := os.ReadFile(statusPFX)
		fatal(err)
		cert, err := certificate.Load(data, os.Getenv("KANG_PFX_PASSWORD"))
		fatal(err)
		fmt.Printf("Certificado: %s (CNPJ %s), emitido por %s, válido até %s\n",
			cert.HolderName(), cert.CNPJ(), cert.IssuerName(), cert.NotAfter().Format("02/01/2006"))
		if err := cert.ValidAt(time.Now()); err != nil {
			fatal(err)
		}

		env := sefaz.Homologation
		if statusProduction {
			env = sefaz.Production
		}
		client, err := sefaz.NewClient(sefaz.Config{Certificate: cert.TLS()})
		fatal(err)
		target := sefaz.Target{Model: sefaz.Model(statusModel), UF: strings.ToUpper(statusUF), Environment: env}
		ep, err := client.Table().Resolve(target.Model, target.UF, env, sefaz.ServiceStatus, "")
		fatal(err)
		fmt.Printf("Autorizador: %s → %s\n", client.Table().Authorizer(target.Model, target.UF), ep.URL)

		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		defer cancel()
		st, err := client.Status(ctx, target)
		fatal(err)
		fmt.Printf("SEFAZ: %d — %s (versão %s, tempo médio %ds)\n", st.CStat, st.Motive, st.AppVersion, st.AvgSeconds)
		if !st.Online() {
			if c, ok := sefaz.Describe(st.CStat); ok && c.Hint != "" {
				fmt.Println("Dica:", c.Hint)
			}
			os.Exit(1)
		}
	},
}

func init() {
	SefazStatus.Flags().StringVar(&statusPFX, "pfx", "", "arquivo .pfx do certificado A1")
	SefazStatus.Flags().StringVar(&statusUF, "uf", "SP", "UF do emitente")
	SefazStatus.Flags().IntVar(&statusModel, "model", 55, "55 (NF-e) ou 65 (NFC-e)")
	SefazStatus.Flags().BoolVar(&statusProduction, "production", false, "consulta o ambiente de produção (o padrão é homologação)")
	_ = SefazStatus.MarkFlagRequired("pfx")
}
