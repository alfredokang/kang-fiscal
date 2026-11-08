// Package config carrega o contrato de configuração do serviço a partir do ambiente.
package config

import (
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// EnvConfig é o contrato de configuração do serviço.
//
// Tudo que varia entre instalações mora aqui. Quem baixa a imagem do Docker Hub configura o
// Kang Fiscal só por variável de ambiente — nenhum arquivo precisa ser editado dentro do
// contêiner.
type EnvConfig struct {
	// HTTP
	ApiPort     string `envconfig:"API_PORT" default:"8080"`
	Environment string `envconfig:"ENVIRONMENT" default:"development"`

	// AdminToken protege as rotas de administração (cadastro de emitente, enquanto as
	// credenciais por tenant não existem). Vazio desliga essas rotas — instalação recém-
	// -baixada não pode nascer com o cadastro aberto para a internet.
	AdminToken string `envconfig:"ADMIN_TOKEN"`

	// Postgres
	PostgresUser     string `envconfig:"POSTGRES_USER"`
	PostgresPassword string `envconfig:"POSTGRES_PASSWORD"`
	PostgresDb       string `envconfig:"POSTGRES_DB"`
	PostgresHost     string `envconfig:"POSTGRES_HOST" default:"localhost"`
	PostgresPort     string `envconfig:"POSTGRES_PORT" default:"5432"`
	PostgresSSLMode  string `envconfig:"POSTGRES_SSL_MODE" default:"disable"`

	// SecretsEncryptionKey cifra a chave privada do certificado A1, a senha dele e o CSC
	// (Constituição, Art. 9.1). 32 bytes em base64: `openssl rand -base64 32`. Fica SÓ
	// aqui — a cópia do banco não pode levar a chave junto. Trocar a chave torna ilegíveis
	// os certificados já gravados: é preciso enviá-los de novo.
	SecretsEncryptionKey string `envconfig:"SECRETS_ENCRYPTION_KEY"`

	// Inutilização automática: o worker varre a cada VOID_SWEEP_MINUTES os números
	// reservados há mais de VOID_AFTER_HOURS sem nota válida. 24 horas dá tempo de alguém
	// olhar uma rejeição, e o prazo legal é o dia 10 do mês seguinte.
	VoidSweepMinutes int `envconfig:"VOID_SWEEP_MINUTES" default:"60"`
	VoidAfterHours   int `envconfig:"VOID_AFTER_HOURS" default:"24"`

	// Responsável técnico (infRespTec): a empresa que mantém o sistema emissor. Várias UFs
	// exigem; quem instala o Kang Fiscal para clientes informa os próprios dados.
	RespTecCNPJ    string `envconfig:"RESP_TEC_CNPJ"`
	RespTecContact string `envconfig:"RESP_TEC_CONTACT"`
	RespTecEmail   string `envconfig:"RESP_TEC_EMAIL"`
	RespTecPhone   string `envconfig:"RESP_TEC_PHONE"`
}

// Env é a configuração carregada no boot.
var Env EnvConfig

// StartConfig lê o .env (se existir) e o ambiente.
//
// O .env é opcional de propósito: no contêiner a configuração vem do ambiente, e exigir o
// arquivo obrigaria quem roda `docker run` a montar um volume só para isso.
func StartConfig() error {
	_ = godotenv.Load()
	return envconfig.Process("", &Env)
}

// IsProduction diz se o serviço roda em produção.
func IsProduction() bool {
	return Env.Environment == "production"
}
