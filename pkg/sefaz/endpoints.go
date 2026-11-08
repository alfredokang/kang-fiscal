// Package sefaz conversa com os web services da NF-e e da NFC-e (leiaute 4.00): status do
// serviço, autorização, consulta pela chave e o catálogo de códigos de retorno.
//
// SOAP 1.2 sobre TLS com autenticação mútua, sem biblioteca de SOAP: o envelope é montado e
// lido aqui, para os bytes enviados serem exatamente os assinados.
//
// Os endereços vêm de endpoints.json, gerado a partir do sped-nfe (MIT) por
// scripts/sefaz-tables.py. Eles mudam algumas vezes por ano: confira a página "Serviços
// Web" do Portal da NF-e antes de produção e sobreponha o que divergir com Table.Override.
package sefaz

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Environment é o tpAmb.
type Environment int

const (
	Production   Environment = 1
	Homologation Environment = 2
)

func (e Environment) key() string {
	if e == Production {
		return "production"
	}
	return "homologation"
}

// Model é o modelo do documento.
type Model int

const (
	NFe  Model = 55
	NFCe Model = 65
)

// Service é o web service.
type Service string

const (
	ServiceStatus              Service = "status"
	ServiceAuthorization       Service = "authorization"
	ServiceAuthorizationResult Service = "authorization_result"
	ServiceProtocolQuery       Service = "protocol_query"
	ServiceVoidRange           Service = "void_range"
	ServiceEvent               Service = "event"
	ServiceRegistryQuery       Service = "registry_query"
)

// Contingency é a SEFAZ virtual de contingência (tpEmis 6 = SVC-AN, 7 = SVC-RS).
type Contingency string

const (
	NoContingency Contingency = ""
	SVCAN         Contingency = "SVCAN"
	SVCRS         Contingency = "SVCRS"
)

// contingencyOf é a SVC de cada UF (NT 2013.007 e atualizações, conforme o sped-nfe).
var contingencyOf = map[string]Contingency{
	"AM": SVCRS, "BA": SVCRS, "GO": SVCRS, "MA": SVCRS, "MS": SVCRS, "MT": SVCRS, "PE": SVCRS, "PR": SVCRS,
	"AC": SVCAN, "AL": SVCAN, "AP": SVCAN, "CE": SVCAN, "DF": SVCAN, "ES": SVCAN, "MG": SVCAN, "PA": SVCAN,
	"PB": SVCAN, "PI": SVCAN, "RJ": SVCAN, "RN": SVCAN, "RO": SVCAN, "RR": SVCAN, "RS": SVCAN, "SC": SVCAN,
	"SE": SVCAN, "SP": SVCAN, "TO": SVCAN,
}

// ContingencyFor devolve a SVC da UF.
func ContingencyFor(uf string) Contingency { return contingencyOf[strings.ToUpper(uf)] }

// ufCodes é o cUF de cada UF.
var ufCodes = map[string]int{
	"RO": 11, "AC": 12, "AM": 13, "RR": 14, "PA": 15, "AP": 16, "TO": 17,
	"MA": 21, "PI": 22, "CE": 23, "RN": 24, "PB": 25, "PE": 26, "AL": 27, "SE": 28, "BA": 29,
	"MG": 31, "ES": 32, "RJ": 33, "SP": 35, "PR": 41, "SC": 42, "RS": 43,
	"MS": 50, "MT": 51, "GO": 52, "DF": 53,
}

// UFCode devolve o cUF da sigla, ou 0.
func UFCode(uf string) int { return ufCodes[strings.ToUpper(uf)] }

// Endpoint é um web service resolvido.
type Endpoint struct {
	URL       string `json:"url"`
	Operation string `json:"operation"` // NFeStatusServico4: compõe o namespace do nfeDadosMsg
	Method    string `json:"method"`    // nfeStatusServicoNF: compõe o SOAPAction
	Version   string `json:"version"`
}

// Namespace é o namespace do nfeDadosMsg.
func (e Endpoint) Namespace() string { return "http://www.portalfiscal.inf.br/nfe/wsdl/" + e.Operation }

// Action é o SOAPAction do cabeçalho.
func (e Endpoint) Action() string { return e.Namespace() + "/" + e.Method }

//go:embed endpoints.json
var endpointsJSON []byte

// Table resolve UF, modelo, ambiente e serviço em endereço.
type Table struct {
	authorizers map[string]map[string]string                         // modelo → UF → autorizador
	services    map[string]map[string]map[string]map[string]Endpoint // modelo → autorizador → ambiente → serviço
}

// ErrNoEndpoint: a combinação não existe (serviço que a UF não oferece, por exemplo).
var ErrNoEndpoint = errors.New("sefaz: serviço não disponível para esta UF, modelo e ambiente")

// DefaultTable é a tabela embutida.
func DefaultTable() (*Table, error) {
	var raw struct {
		Authorizers map[string]map[string]string                         `json:"authorizers"`
		Services    map[string]map[string]map[string]map[string]Endpoint `json:"services"`
	}
	if err := json.Unmarshal(endpointsJSON, &raw); err != nil {
		return nil, fmt.Errorf("sefaz: endpoints.json ilegível: %w", err)
	}
	return &Table{authorizers: raw.Authorizers, services: raw.Services}, nil
}

// Authorizer devolve o autorizador da UF no modelo (SP, MG, SVRS, SVAN...).
func (t *Table) Authorizer(model Model, uf string) string {
	return t.authorizers[fmt.Sprint(int(model))][strings.ToUpper(uf)]
}

// Resolve devolve o endereço do serviço. Em contingência, o autorizador é a SVC da UF.
func (t *Table) Resolve(model Model, uf string, env Environment, service Service, cont Contingency) (Endpoint, error) {
	auth := t.Authorizer(model, uf)
	if cont != NoContingency {
		auth = string(cont)
	}
	ep, ok := t.services[fmt.Sprint(int(model))][auth][env.key()][string(service)]
	if !ok || ep.URL == "" {
		return Endpoint{}, fmt.Errorf("%w: modelo %d, %s (%s), %s, %s", ErrNoEndpoint, model, uf, auth, env.key(), service)
	}
	return ep, nil
}

// Override troca um endereço sem esperar uma versão nova do Kang Fiscal — a SEFAZ muda
// endereço com aviso de dias.
func (t *Table) Override(model Model, authorizer string, env Environment, service Service, ep Endpoint) {
	m := fmt.Sprint(int(model))
	if t.services[m] == nil {
		t.services[m] = map[string]map[string]map[string]Endpoint{}
	}
	if t.services[m][authorizer] == nil {
		t.services[m][authorizer] = map[string]map[string]Endpoint{}
	}
	if t.services[m][authorizer][env.key()] == nil {
		t.services[m][authorizer][env.key()] = map[string]Endpoint{}
	}
	t.services[m][authorizer][env.key()][string(service)] = ep
}
