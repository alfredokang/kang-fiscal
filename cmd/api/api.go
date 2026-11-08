package api

import (
	"github.com/spf13/cobra"

	"github.com/alfredokang/kang-fiscal/shared/config"
	"github.com/alfredokang/kang-fiscal/shared/database/postgres"
	"github.com/alfredokang/kang-fiscal/shared/logger"
	"github.com/alfredokang/kang-fiscal/shared/server"
)

// Api sobe o servidor HTTP.
var Api = &cobra.Command{
	Use:   "api",
	Short: "Sobe a API HTTP",
	Run: func(_ *cobra.Command, _ []string) {
		fatal(config.StartConfig())
		fatal(logger.InitLogger())

		db, err := postgres.GetConnection()
		fatal(err)
		defer postgres.CloseConnection()

		srv, err := server.NewServer(db)
		fatal(err)
		fatal(srv.Start())
	},
}
