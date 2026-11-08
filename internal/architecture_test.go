package internal_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestModuleBoundaries é o guarda automático do Artigo 1 da Constituição.
//
// A regra "nenhum módulo importa outro módulo" não sobrevive à boa vontade no code
// review: a primeira violação é sempre razoável e urgente. Este teste transforma a regra
// numa falha de build, que é a única forma de disciplina que não depende de quem está de
// plantão numa sexta-feira..
func TestModuleBoundaries(t *testing.T) {
	root := modulesRoot(t)

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("não foi possível ler %s: %v", root, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		module := entry.Name()

		err := filepath.WalkDir(filepath.Join(root, module), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}

			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}

			for _, imp := range file.Imports {
				importPath := strings.Trim(imp.Path.Value, `"`)

				const prefix = "github.com/alfredokang/kang-fiscal/internal/modules/"
				if !strings.HasPrefix(importPath, prefix) {
					continue
				}

				imported := strings.SplitN(strings.TrimPrefix(importPath, prefix), "/", 2)[0]
				if imported != module {
					rel, _ := filepath.Rel(root, path)
					t.Errorf(
						"violação do Art. 1 da Constituição:\n  %s\n  importa o módulo %q.\n"+
							"  Declare uma porta em modules/%s/application/port/output/ e implemente\n"+
							"  um adapter em shared/server/. Ver CONSTITUTION.md, Artigo 1.",
						rel, imported, module,
					)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("erro ao varrer o módulo %s: %v", module, err)
		}
	}
}

// TestDomainPurity garante o Artigo 3.1: o domínio não conhece infraestrutura.
//
// Um domínio que importa database/sql ou net/http deixa de ser testável sem banco e sem
// servidor — e, quando isso acontece, os testes de regra de negócio param de ser escritos.
func TestDomainPurity(t *testing.T) {
	root := modulesRoot(t)

	forbidden := []string{
		"database/sql",
		"net/http",
		"github.com/go-chi",
		"github.com/redis",
		"github.com/jackc",
		"github.com/alfredokang/kang-fiscal/shared/",
	}

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		if !strings.Contains(filepath.ToSlash(path), "/domain/") {
			return nil
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}

		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbidden {
				if strings.HasPrefix(importPath, bad) {
					rel, _ := filepath.Rel(root, path)
					t.Errorf(
						"violação do Art. 3.1 da Constituição:\n  domain/%s importa %q.\n"+
							"  O domínio é puro: o que precisa de infraestrutura vira porta de saída.",
						rel, importPath,
					)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("erro ao varrer o domínio: %v", err)
	}
}

func modulesRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join("modules")
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("diretório de módulos não encontrado: %v", err)
	}
	return root
}

// TestPkgIsStandalone garante o Artigo 3.2: o `pkg/` é a biblioteca que outro projeto Go
// importa sem subir a API. Se ele importar `internal/` — que o compilador proíbe fora deste
// repositório — ou `shared/`, quem fizer `go get` leva junto banco, log e HTTP que não pediu.
func TestPkgIsStandalone(t *testing.T) {
	const module = "github.com/alfredokang/kang-fiscal/"
	root := filepath.Join("..", "pkg")

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(importPath, module+"internal/") || strings.HasPrefix(importPath, module+"shared/") {
				rel, _ := filepath.Rel(root, path)
				t.Errorf("violação do Art. 3.2 da Constituição:\n  pkg/%s importa %q.\n"+
					"  O pkg/ não conhece banco, HTTP nem tenant: quem precisa disso é o módulo.", rel, importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("erro ao varrer o pkg/: %v", err)
	}
}
