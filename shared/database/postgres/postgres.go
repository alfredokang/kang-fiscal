package postgres

import (
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // driver PostgreSQL

	"github.com/alfredokang/kang-fiscal/shared/config"
)

var (
	instance *sql.DB
	once     sync.Once
	connErr  error
)

// GetConnection devolve a conexão singleton com o Postgres.
//
// Singleton porque *sql.DB já é um pool com lock interno: abrir um por módulo
// multiplicaria os pools e estouraria max_connections do servidor sem que nenhum
// módulo isolado parecesse culpado.
//
// TimeZone=UTC é deliberado — a data de emissão (dhEmi) vai no XML com o fuso do emitente,
// e o prazo de cancelamento conta horas a partir dela. Timestamp lido num fuso e escrito
// noutro é a nota que a SEFAZ diz estar fora do prazo.
func GetConnection() (*sql.DB, error) {
	once.Do(func() {
		connStr := ConnectionString(
			config.Env.PostgresUser, config.Env.PostgresPassword,
			config.Env.PostgresHost, config.Env.PostgresPort,
			config.Env.PostgresDb, config.Env.PostgresSSLMode,
		)

		instance, connErr = sql.Open("pgx", connStr)
		if connErr != nil {
			return
		}

		if connErr = instance.Ping(); connErr != nil {
			return
		}
		if connErr = RequireSupportedVersion(instance); connErr != nil {
			return
		}

		// 20 abertas e 5 paradas por processo: no deploy com atualização sem parada sobem
		// duas APIs e o migrate ao mesmo tempo, e o Postgres de um servidor pequeno vem com
		// max_connections=100. A emissão espera a SEFAZ, não o banco — mais conexões não
		// emitem mais rápido.
		maxConns := config.Env.PostgresMaxConns
		if maxConns < 2 {
			maxConns = 20
		}
		instance.SetMaxOpenConns(maxConns)
		instance.SetMaxIdleConns(min(5, maxConns))
		instance.SetConnMaxLifetime(30 * time.Minute)
	})

	return instance, connErr
}

// SupportedMajor é a única versão do Postgres suportada (Constituição, Art. 4.6): a mesma
// em desenvolvimento, nos testes e em produção. As migrations usam recursos dela, e um
// banco de outra versão falharia no meio, com um erro que não aponta a causa.
const SupportedMajor = 18

// RequireSupportedVersion recusa, logo na conexão, um Postgres que não seja o 18.
func RequireSupportedVersion(db *sql.DB) error {
	var num int
	if err := db.QueryRow(`SELECT current_setting('server_version_num')::int`).Scan(&num); err != nil {
		return fmt.Errorf("ler a versão do Postgres: %w", err)
	}
	return CheckVersion(num)
}

// CheckVersion confere o server_version_num (180001 = 18.1).
func CheckVersion(versionNum int) error {
	if major := versionNum / 10000; major != SupportedMajor {
		return fmt.Errorf("o Kang Fiscal só funciona com Postgres %d, e este banco é o Postgres %d", SupportedMajor, major)
	}
	return nil
}

// ConnectionString monta a URL de conexão com usuário e senha ESCAPADOS.
//
// A versão anterior colava a senha crua na URL. Uma senha com `@`, `/`, `#`, `?` ou `%` —
// o que `openssl rand -base64` produz com frequência — quebrava a URL, e o erro falava de
// host ou porta inválidos, longe da causa. `url.UserPassword` escapa o que for preciso, e
// a senha chega ao Postgres exatamente como foi configurada no ambiente.
func ConnectionString(user, password, host, port, db, sslMode string) string {
	u := url.URL{
		Scheme:   "postgresql",
		User:     url.UserPassword(user, password),
		Host:     net.JoinHostPort(host, port),
		Path:     "/" + db,
		RawQuery: url.Values{"sslmode": {sslMode}, "TimeZone": {"UTC"}}.Encode(),
	}
	return u.String()
}

// CloseConnection encerra o pool (shutdown graceful e testes).
func CloseConnection() error {
	if instance != nil {
		return instance.Close()
	}
	return nil
}
