package api

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/alfredokang/kang-fiscal/shared/config"
)

// Healthcheck é o HEALTHCHECK da imagem.
//
// Existe porque a imagem final não tem shell, wget nem curl (Constituição, Art. 9.8): o
// próprio binário consulta o /health.
var Healthcheck = &cobra.Command{
	Use:   "healthcheck",
	Short: "Confere se a API local responde (usado pelo HEALTHCHECK do Docker)",
	Run: func(_ *cobra.Command, _ []string) {
		fatal(config.StartConfig())
		client := http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get("http://127.0.0.1:" + config.Env.ApiPort + "/health")
		if err != nil || resp.StatusCode != http.StatusOK {
			fmt.Fprintln(os.Stderr, "API não respondeu")
			os.Exit(1)
		}
		_ = resp.Body.Close()
	},
}
