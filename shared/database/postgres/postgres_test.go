package postgres_test

import (
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/alfredokang/kang-fiscal/shared/database/postgres"
)

// TestSenhaComCaractereEspecialChegaInteira: a senha gerada com `@`, `/`, `#`, `?`, `%`,
// `:` e `+` precisa chegar ao Postgres exatamente como foi configurada no ambiente.
func TestSenhaComCaractereEspecialChegaInteira(t *testing.T) {
	senha := `p@ss/w#rd?x%2F:y+z=`
	dsn := postgres.ConnectionString("kangfiscal", senha, "postgres_kangfiscal", "5432", "kangfiscal", "disable")

	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("a URL não abre: %v", err)
	}
	if cfg.Password != senha {
		t.Errorf("senha: esperava %q, chegou %q", senha, cfg.Password)
	}
	if cfg.User != "kangfiscal" || cfg.Host != "postgres_kangfiscal" || cfg.Port != 5432 || cfg.Database != "kangfiscal" {
		t.Errorf("conexão inesperada: %s@%s:%d/%s", cfg.User, cfg.Host, cfg.Port, cfg.Database)
	}
	if tz := cfg.RuntimeParams["TimeZone"]; tz != "UTC" {
		t.Errorf("fuso: esperava UTC, veio %q (%v)", tz, cfg.RuntimeParams)
	}
}

func TestSoPostgres18(t *testing.T) {
	for num, ok := range map[int]bool{180000: true, 180003: true, 170006: false, 160010: false, 190000: false} {
		if err := postgres.CheckVersion(num); (err == nil) != ok {
			t.Errorf("server_version_num %d: %v", num, err)
		}
	}
}
