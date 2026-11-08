package api

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alfredokang/kang-fiscal/shared/config"
	"github.com/alfredokang/kang-fiscal/shared/database/postgres"
	"github.com/alfredokang/kang-fiscal/shared/logger"
)

// moduleMigrationPaths é a lista de módulos com migrations, na ordem de aplicação.
//
// Ordem explícita e não descoberta por varredura do diretório: a ordem entre módulos é
// uma decisão (emitente antes de nota), e uma listagem alfabética de
// pastas acertaria isso por acaso.
var moduleMigrationPaths = []string{
	// issuer PRIMEIRO: o emitente é de quem são as notas, os certificados e a numeração.
	// Nenhuma FK cruza a fronteira (Art. 2), mas a ordem mantém um banco novo coerente.
	"internal/modules/issuer/infrastructure/persistence/postgres/migrations",
	// certificate: `issuer_ref` é o id do emitente como texto opaco, sem FK (Art. 2).
	"internal/modules/certificate/infrastructure/persistence/postgres/migrations",
}

var migrateModule string

// Migrate aplica as migrations pendentes.
var Migrate = &cobra.Command{
	Use:   "migrate",
	Short: "Aplica as migrations dos módulos",
	Run: func(_ *cobra.Command, _ []string) {
		fatal(config.StartConfig())
		fatal(logger.InitLogger())

		db, err := postgres.GetConnection()
		fatal(err)

		// Registro do que já rodou. Sem ele, reaplicar um CREATE TABLE em produção é
		// questão de tempo — e "IF NOT EXISTS" não protege um ALTER.
		_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS schema_migrations (
				module     VARCHAR(64)  NOT NULL,
				filename   VARCHAR(255) NOT NULL,
				applied_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
				PRIMARY KEY (module, filename)
			)`)
		fatal(err)

		for _, path := range moduleMigrationPaths {
			module := moduleNameFromPath(path)
			if migrateModule != "" && migrateModule != module {
				continue
			}

			files, err := filepath.Glob(filepath.Join(path, "*.sql"))
			fatal(err)
			sort.Strings(files)

			for _, file := range files {
				name := filepath.Base(file)

				var applied bool
				err := db.QueryRow(
					`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE module = $1 AND filename = $2)`,
					module, name,
				).Scan(&applied)
				fatal(err)

				if applied {
					logger.SugaredLogger.Infof("migration já aplicada: %s/%s", module, name)
					continue
				}

				content, err := os.ReadFile(file)
				fatal(err)

				// Uma transação por arquivo: uma migration que falha na metade não deixa
				// meia tabela criada e sem registro.
				tx, err := db.Begin()
				fatal(err)

				if _, err := tx.Exec(string(content)); err != nil {
					_ = tx.Rollback()
					fatal(fmt.Errorf("migration %s/%s falhou: %w", module, name, err))
				}
				if _, err := tx.Exec(
					`INSERT INTO schema_migrations (module, filename) VALUES ($1, $2)`, module, name,
				); err != nil {
					_ = tx.Rollback()
					fatal(err)
				}
				fatal(tx.Commit())

				logger.SugaredLogger.Infof("migration aplicada: %s/%s", module, name)
			}
		}

		logger.SugaredLogger.Info("migrations concluídas")
	},
}

func init() {
	Migrate.Flags().StringVar(&migrateModule, "module", "", "aplica apenas as migrations deste módulo")
}

// moduleNameFromPath extrai "issuer" de "internal/modules/issuer/infrastructure/...".
func moduleNameFromPath(path string) string {
	parts := strings.Split(path, string(filepath.Separator))
	for i, p := range parts {
		if p == "modules" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return path
}
