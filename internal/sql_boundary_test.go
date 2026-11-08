package internal_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestSQLNaoCruzaModulos é o Artigo 2 aplicado ao SQL.
//
// O guard de arquitetura (`architecture_test.go`) confere IMPORTS — ele pega
// `internal/modules/invoice` importando `internal/modules/issuer`. Mas um `JOIN
// issuer_issuers` escrito dentro do repositório da nota passaria por ele sem ruído:
// não há import nenhum, só uma string. E é justamente a forma mais fácil de furar a
// fronteira sem perceber, porque o SQL "funciona".
//
// Este teste lê cada arquivo `.go` dos módulos, procura nomes de tabela de OUTROS módulos,
// e falha. É o que impede a regra de existir só na Constituição.
//
// **Sem exceção para relatório.** Ler a tabela do vizinho é acoplamento pelo caminho mais
// difícil de enxergar: não aparece em nenhum import, e o dia em que o módulo dono renomear
// uma coluna, quem quebra é o relatório, longe de onde a mudança foi feita.
// tableOwners é cada tabela do sistema e o módulo dono dela. A lista é explícita e não
// descoberta: uma varredura automática acertaria por acaso, e quando errasse ninguém saberia.
// TestTodaTabelaTemDono confere que ela acompanha as migrations.
var tableOwners = map[string]string{
	"certificate_certificates": "certificate",
	"credential_keys":          "credential",
	"dfe_cursors":              "dfe",
	"dfe_documents":            "dfe",
	"invoice_contingency":      "invoice",
	"invoice_events":           "invoice",
	"invoice_invoices":         "invoice",
	"invoice_voids":            "invoice",
	"issuer_issuers":           "issuer",
	"nfse_documents":           "nfse",
	"nfse_events":              "nfse",
	"numbering_reservations":   "numbering",
	"numbering_series":         "numbering",
	"taxrule_profiles":         "taxrule",
	"taxrule_rates":            "taxrule",
	// A tabela de controle das migrations não é de ninguém: quem a usa é o comando
	// `migrate`, fora dos módulos.
	"schema_migrations": "",
}

func TestSQLNaoCruzaModulos(t *testing.T) {
	dono := tableOwners

	// A busca procura o nome da tabela como palavra inteira, para `invoice_items` não casar
	// dentro de `invoice_items_archive`.
	patterns := make(map[string]*regexp.Regexp, len(dono))
	for table := range dono {
		patterns[table] = regexp.MustCompile(`\b` + regexp.QuoteMeta(table) + `\b`)
	}

	raiz := filepath.Join("modules")
	err := filepath.Walk(raiz, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Os testes de integração leem tabelas do próprio módulo para conferir o efeito —
		// e alguns limpam registros de outros para montar o cenário. Isso é código de
		// teste, não caminho de produção.
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		modulo := strings.Split(strings.TrimPrefix(path, raiz+string(os.PathSeparator)), string(os.PathSeparator))[0]

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Os COMENTÁRIOS ficam de fora. Este projeto explica as decisões no código, e
		// vários comentários citam a tabela do vizinho justamente para dizer que NÃO a
		// consultam ("a nota guarda o snapshot do emitente sem nunca consultar issuer_issuers"). Um
		// guard que acusa a frase que documenta a regra vira ruído — e guard ruidoso é
		// guard desligado.
		texto := semComentarios(string(content))

		for table, owner := range dono {
			if owner == "" || owner == modulo {
				continue
			}
			if patterns[table].MatchString(texto) {
				t.Errorf(
					"Art. 2: o módulo `%s` menciona a tabela `%s`, que é do módulo `%s`.\n"+
						"  arquivo: %s\n"+
						"  Um módulo não lê a tabela de outro — nem por JOIN, nem por SELECT direto.\n"+
						"  O caminho é declarar uma porta e implementá-la no composition root.",
					modulo, table, owner, path,
				)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("varrer os módulos: %v", err)
	}
}

// semComentarios remove comentários de linha e de bloco.
//
// Ingênuo de propósito: não trata `//` dentro de string, porque o único efeito seria
// deixar passar uma menção — e uma menção a tabela alheia dentro de uma string é
// exatamente o SQL que este teste procura. Errar para o lado de acusar demais seria pior.
func semComentarios(texto string) string {
	var out strings.Builder
	out.Grow(len(texto))

	emBloco := false
	for _, linha := range strings.Split(texto, "\n") {
		if emBloco {
			if idx := strings.Index(linha, "*/"); idx >= 0 {
				linha = linha[idx+2:]
				emBloco = false
			} else {
				continue
			}
		}
		if idx := strings.Index(linha, "/*"); idx >= 0 {
			if fim := strings.Index(linha[idx:], "*/"); fim >= 0 {
				linha = linha[:idx] + linha[idx+fim+2:]
			} else {
				linha = linha[:idx]
				emBloco = true
			}
		}
		if idx := strings.Index(linha, "//"); idx >= 0 {
			linha = linha[:idx]
		}
		out.WriteString(linha)
		out.WriteByte('\n')
	}
	return out.String()
}

// TestTodaTabelaTemDono garante que o mapa de donos acima não envelhece: toda tabela
// criada numa migration precisa estar nele. Sem isto, uma tabela nova fica fora da
// vigilância do teste acima — e ninguém percebe, porque ele continua passando.
func TestTodaTabelaTemDono(t *testing.T) {
	create := regexp.MustCompile(`(?i)CREATE TABLE(?: IF NOT EXISTS)?\s+([a-z0-9_]+)`)
	files, err := filepath.Glob(filepath.Join("modules", "*", "infrastructure", "persistence", "postgres", "migrations", "*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		module := strings.Split(filepath.ToSlash(file), "/")[1]
		for _, m := range create.FindAllStringSubmatch(string(content), -1) {
			table := m[1]
			if !strings.HasPrefix(table, module+"_") {
				t.Errorf("Art. 4.5: a tabela %s, criada em %s, não leva o prefixo do módulo %q", table, file, module)
			}
			if owner, ok := tableOwners[table]; !ok || owner != module {
				t.Errorf("a tabela %s, criada em %s, não está no mapa de donos de TestSQLNaoCruzaModulos", table, file)
			}
		}
	}
}
