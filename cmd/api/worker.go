package api

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/alfredokang/kang-fiscal/shared/config"
	"github.com/alfredokang/kang-fiscal/shared/database/postgres"
	"github.com/alfredokang/kang-fiscal/shared/logger"
	"github.com/alfredokang/kang-fiscal/shared/server"
)

// workerLock é a chave do advisory lock do Postgres: com várias réplicas do worker, só uma
// varre por vez — duas inutilizando a mesma faixa ao mesmo tempo dariam rejeição na segunda.
const workerLock = 727_001

// Worker roda as tarefas periódicas: a sonda que encerra a contingência quando a SEFAZ da
// UF volta, e a varredura que inutiliza os buracos da numeração (e concilia as notas
// incertas no caminho).
var Worker = &cobra.Command{
	Use:   "worker",
	Short: "Roda as tarefas periódicas (inutilização dos números sem nota)",
	Run: func(_ *cobra.Command, _ []string) {
		fatal(config.StartConfig())
		fatal(logger.InitLogger())
		db, err := postgres.GetConnection()
		fatal(err)
		defer postgres.CloseConnection()
		srv, err := server.NewServer(db)
		fatal(err)

		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		every := time.Duration(config.Env.VoidSweepMinutes) * time.Minute
		age := time.Duration(config.Env.VoidAfterHours) * time.Hour
		logger.SugaredLogger.Infof("worker: varredura a cada %s, números abertos há mais de %s", every, age)

		run := func() {
			conn, err := db.Conn(ctx)
			if err != nil {
				logger.SugaredLogger.Error(err)
				return
			}
			defer conn.Close()
			var got bool
			if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, workerLock).Scan(&got); err != nil || !got {
				return
			}
			defer conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, workerLock) //nolint:errcheck
			// Primeiro a sonda: se a UF voltou, a varredura já concilia as incertas pela UF.
			if restored := srv.Invoices().ProbeContingencies(ctx); len(restored) > 0 {
				logger.SugaredLogger.Infow("contingência encerrada: autorizadora de volta", "ufs", restored)
			}
			res := srv.Invoices().SweepPending(ctx, age, 500)
			if res.Checked > 0 || len(res.Errors) > 0 {
				logger.SugaredLogger.Infow("varredura de inutilização", "conferidos", res.Checked,
					"inutilizados", res.Voided, "pulados", res.Skipped, "erros", res.Errors)
			}
		}
		run()
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				run()
			}
		}
	},
}
