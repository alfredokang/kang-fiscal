package nfse

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Tipos de evento da NFS-e (tiposEventos v1.01).
const (
	EventCancel           = "101101" // cancelamento pelo prestador
	EventCancelAnalysis   = "101103" // pedido de análise fiscal para cancelar
	EventCancelSubstitute = "105102"
	EventAnalysisGranted  = "105104"
	EventAnalysisDenied   = "105105"
	EventCancelOfficial   = "305101"
)

// ErrAlreadyLinked é o E0840: já há um evento de cancelamento ligado à nota. NÃO prova que
// ela está cancelada — a Sefin devolve o mesmo código quando só existe o pedido de análise
// (medido pelo PyNFSeNacional em homologação). Confira com CancelState.
const ErrAlreadyLinked = "E0840"

// cancelingEvents são os que deixam a nota cancelada.
var cancelingEvents = []string{EventCancel, EventCancelSubstitute, EventAnalysisGranted, EventCancelOfficial}

// Motivos do cancelamento (cMotivo).
const (
	ReasonIssueError      = 1
	ReasonServiceNotGiven = 2
	ReasonOther           = 9
)

// PedRegEvento é o pedido de registro de evento (o elemento assinado é o infPedReg).
type PedRegEvento struct {
	XMLName   xml.Name  `xml:"pedRegEvento"`
	Xmlns     string    `xml:"xmlns,attr"`
	Versao    string    `xml:"versao,attr"`
	InfPedReg InfPedReg `xml:"infPedReg"`
}

// InfPedReg é o bloco assinado.
type InfPedReg struct {
	ID        string       `xml:"Id,attr"`
	TpAmb     int          `xml:"tpAmb"`
	VerAplic  string       `xml:"verAplic"`
	DhEvento  string       `xml:"dhEvento"`
	CNPJAutor string       `xml:"CNPJAutor,omitempty"`
	CPFAutor  string       `xml:"CPFAutor,omitempty"`
	ChNFSe    string       `xml:"chNFSe"`
	E101101   *CancelEvent `xml:"e101101,omitempty"`
	E101103   *CancelEvent `xml:"e101103,omitempty"`
}

// CancelEvent é o corpo do e101101 e do e101103 (o mesmo leiaute).
type CancelEvent struct {
	XDesc   string `xml:"xDesc"`
	CMotivo int    `xml:"cMotivo"`
	XMotivo string `xml:"xMotivo"`
}

// Cancellation é o que se pede ao cancelar.
type Cancellation struct {
	AccessKey     string
	Author        string // CNPJ ou CPF do prestador
	Reason        int    // 1, 2 ou 9
	Justification string // 15 a 255 caracteres
	Environment   int    // 1 produção, 2 homologação
	At            time.Time
	// Analysis pede o cancelamento por análise fiscal (e101103): é o único aceito onde o
	// município não deixa cancelar direto (o direto volta E0823). A nota fica válida até o
	// município decidir.
	Analysis   bool
	AppVersion string
}

// NewCancellation monta o pedido de cancelamento, sem assinatura.
func NewCancellation(c Cancellation) (*PedRegEvento, error) {
	key := strings.TrimPrefix(c.AccessKey, "NFS")
	if len(key) != 50 {
		return nil, fmt.Errorf("%w: chave da NFS-e com 50 posições", ErrInvalid)
	}
	if c.Reason != ReasonIssueError && c.Reason != ReasonServiceNotGiven && c.Reason != ReasonOther {
		return nil, fmt.Errorf("%w: motivo do cancelamento 1 (erro na emissão), 2 (serviço não prestado) ou 9 (outros)", ErrInvalid)
	}
	just := strings.Join(strings.Fields(c.Justification), " ")
	if n := len([]rune(just)); n < 15 || n > 255 {
		return nil, fmt.Errorf("%w: justificativa com 15 a 255 caracteres", ErrInvalid)
	}
	if c.Environment != 1 && c.Environment != 2 {
		return nil, fmt.Errorf("%w: ambiente 1 ou 2", ErrInvalid)
	}
	kind, body := EventCancel, &CancelEvent{XDesc: "Cancelamento de NFS-e", CMotivo: c.Reason, XMotivo: just}
	// A Sefin exige o texto ACENTUADO (sem acento volta E1235).
	inf := InfPedReg{TpAmb: c.Environment, VerAplic: c.AppVersion, DhEvento: Timestamp(c.At), ChNFSe: key}
	if c.Analysis {
		kind = EventCancelAnalysis
		body.XDesc = "Solicitação de Análise Fiscal para Cancelamento de NFS-e"
		inf.E101103 = body
	} else {
		inf.E101101 = body
	}
	if inf.VerAplic == "" {
		inf.VerAplic = "kang-fiscal"
	}
	switch doc := strings.ToUpper(c.Author); len(doc) {
	case 14:
		inf.CNPJAutor = doc
	case 11:
		inf.CPFAutor = doc
	default:
		return nil, fmt.Errorf("%w: autor do evento é um CNPJ ou CPF", ErrInvalid)
	}
	// "PRE" + chave + tipo: 59 posições (TSIdPedRegEvt).
	inf.ID = "PRE" + key + kind
	return &PedRegEvento{InfPedReg: inf}, nil
}

// Justification devolve a justificativa do pedido, já normalizada.
func (i InfPedReg) Justification() string {
	if i.E101103 != nil {
		return i.E101103.XMotivo
	}
	if i.E101101 != nil {
		return i.E101101.XMotivo
	}
	return ""
}

// Kind é o tipo do evento pedido.
func (p *PedRegEvento) Kind() string { return p.InfPedReg.ID[len(p.InfPedReg.ID)-6:] }

// MarshalEvent serializa o pedido, pronto para assinar o infPedReg.
func MarshalEvent(p *PedRegEvento) ([]byte, error) {
	p.Xmlns = Namespace
	if p.Versao == "" {
		p.Versao = "1.01"
	}
	return xml.Marshal(p)
}

// EventResult é a resposta do registro de evento.
type EventResult struct {
	HTTPStatus int
	EventXML   []byte // o evento registrado, como a Sefin devolve
	Errors     []Message
}

// Registered diz se o evento foi registrado.
func (r EventResult) Registered() bool { return len(r.EventXML) > 0 && len(r.Errors) == 0 }

// AlreadyLinked diz se a recusa foi o E0840.
func (r EventResult) AlreadyLinked() bool {
	for _, e := range r.Errors {
		if e.Code == ErrAlreadyLinked {
			return true
		}
	}
	return false
}

// SendEvent envia o pedido assinado: POST /nfse/{chave}/eventos. Um envio só: repetir é
// decisão de quem chama, e só depois de conferir (EventRegistered) — repetido, um
// cancelamento já registrado volta E0840.
func (c *Client) SendEvent(ctx context.Context, key string, signed []byte) (EventResult, error) {
	packed, err := Pack(signed)
	if err != nil {
		return EventResult{}, err
	}
	body, _ := json.Marshal(map[string]string{"pedidoRegistroEventoXmlGZipB64": packed})
	resp, raw, err := c.do(ctx, http.MethodPost, "/nfse/"+key+"/eventos", body)
	if err != nil {
		return EventResult{}, err
	}
	res := EventResult{HTTPStatus: resp.StatusCode}
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		// 5xx/408 sem corpo pode ter sido processado: o gateway desistiu antes da resposta.
		if resp.StatusCode >= 500 || resp.StatusCode == http.StatusRequestTimeout {
			return res, fmt.Errorf("%w: evento HTTP %d sem JSON", ErrUncertain, resp.StatusCode)
		}
		return res, fmt.Errorf("nfse: evento recusado, HTTP %d sem JSON", resp.StatusCode)
	}
	if res.Errors = messages(data, "erros", "erro"); len(res.Errors) > 0 {
		return res, nil
	}
	b64, _ := field(data, "eventoXmlGZipB64").(string)
	if b64 == "" {
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusRequestTimeout {
			return res, fmt.Errorf("nfse: evento recusado, HTTP %d sem o evento", resp.StatusCode)
		}
		return res, fmt.Errorf("%w: evento HTTP %d sem o evento", ErrUncertain, resp.StatusCode)
	}
	if res.EventXML, err = Unpack(b64); err != nil {
		return res, fmt.Errorf("%w: evento ilegível", ErrUncertain)
	}
	return res, nil
}

// EventRegistered consulta se a nota tem o evento: GET /nfse/{chave}/eventos/{tipo}/{seq}.
// 404 com JSON é "não tem"; 404 sem JSON é rota errada e vira erro, não "não tem".
func (c *Client) EventRegistered(ctx context.Context, key, kind string, seq int) (bool, error) {
	resp, raw, err := c.do(ctx, http.MethodGet, "/nfse/"+key+"/eventos/"+kind+"/"+strconv.Itoa(seq), nil)
	if err != nil {
		return false, err
	}
	var data map[string]any
	isJSON := json.Unmarshal(raw, &data) == nil
	if resp.StatusCode == http.StatusNotFound && isJSON {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK || !isJSON {
		return false, fmt.Errorf("%w: consulta do evento %s HTTP %d", ErrUncertain, kind, resp.StatusCode)
	}
	list, ok := field(data, "eventos").([]any)
	if !ok {
		return false, fmt.Errorf("%w: consulta do evento %s sem a lista de eventos", ErrUncertain, kind)
	}
	return len(list) > 0, nil
}

// Estados de cancelamento de uma NFS-e, segundo os eventos dela.
const (
	CancelStateCanceled = "canceled" // há um evento que cancela
	CancelStateAnalysis = "analysis" // só o pedido de análise, sem decisão: a nota segue válida
	CancelStateDenied   = "denied"   // a análise negou: a nota segue válida
	CancelStateNone     = "none"     // nenhum — e isso NÃO prova nada logo depois de um envio:
	//                                   a produção demora a mostrar um cancelamento recém-registrado
)

// CancelState pergunta à Sefin pelos eventos em vez de deduzir de um E-code.
func (c *Client) CancelState(ctx context.Context, key string) (string, error) {
	for _, kind := range cancelingEvents {
		ok, err := c.EventRegistered(ctx, key, kind, 1)
		if err != nil {
			return "", err
		}
		if ok {
			return CancelStateCanceled, nil
		}
	}
	ok, err := c.EventRegistered(ctx, key, EventCancelAnalysis, 1)
	if err != nil || !ok {
		return CancelStateNone, err
	}
	if denied, err := c.EventRegistered(ctx, key, EventAnalysisDenied, 1); err != nil {
		return "", err
	} else if denied {
		return CancelStateDenied, nil
	}
	return CancelStateAnalysis, nil
}
