// Package api é a CLI do serviço.
package api

import (
	"log"

	"github.com/spf13/cobra"
)

// Version é sobrescrita no build da imagem (-ldflags "-X ...api.Version=v0.1.0").
var Version = "dev"

// Execute monta e roda o comando raiz.
//
// CLI em vez de um main que só sobe o HTTP porque o mesmo binário roda a API, as
// migrations e o healthcheck da imagem — um binário por função multiplicaria build e
// deploy sem ganho nenhum.
func Execute() {
	root := &cobra.Command{
		Use:     "kang-fiscal",
		Short:   "Kang Fiscal — emissor de NF-e, NFC-e e NFS-e open source, feito por Alfredo Kang",
		Version: Version,
	}

	root.AddCommand(Api, Migrate, Healthcheck, SefazStatus)

	if err := root.Execute(); err != nil {
		log.Fatal(err)
	}
}

// fatal encerra o processo quando o boot falha. Serviço que sobe com dependência quebrada
// aceita tráfego e erra tudo — pior de diagnosticar que um serviço que não sobe.
func fatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
