// Package nfe é o leiaute 4.00 da NF-e (modelo 55) e da NFC-e (modelo 65), com os grupos
// da reforma tributária (NT 2025.002 — IBS, CBS e IS) convivendo com os tributos atuais
// (ICMS, ICMS-ST, DIFAL, FCP, IPI, PIS e COFINS).
//
// As structs seguem o XSD do pacote PL_010_V1.30 campo a campo e NA MESMA ORDEM: o
// encoding/xml escreve na ordem dos campos, e elemento fora de ordem é rejeição de schema.
// Os testes validam o XML gerado contra o XSD oficial (testdata/schemas).
//
// Nomes: os campos levam o nome do elemento do leiaute (VProd, CClassTrib), e não uma
// tradução. É a exceção à regra de identificadores em inglês, e é deliberada: quem confere
// a nota contra o Manual de Orientação do Contribuinte procura "vProd", não "ProductValue".
//
// Números são decimal.Fixed — nunca float (Constituição, Art. 5). A escala de cada valor é
// responsabilidade de quem monta a nota; Validate confere as do leiaute.
package nfe

import (
	"encoding/xml"

	"github.com/alfredokang/kang-fiscal/pkg/decimal"
)

// Namespace é o namespace da NF-e.
const Namespace = "http://www.portalfiscal.inf.br/nfe"

// Dec é o número do leiaute.
type Dec = decimal.Fixed

// NFe é a nota (raiz do documento assinado).
type NFe struct {
	XMLName xml.Name `xml:"NFe"`
	Xmlns   string   `xml:"xmlns,attr"`
	InfNFe  InfNFe   `xml:"infNFe"`
}

// InfNFe é o bloco assinado.
type InfNFe struct {
	ID         string      `xml:"Id,attr"`
	Versao     string      `xml:"versao,attr"`
	Ide        Ide         `xml:"ide"`
	Emit       Emit        `xml:"emit"`
	Dest       *Dest       `xml:"dest,omitempty"`
	Det        []Det       `xml:"det"`
	Total      Total       `xml:"total"`
	Transp     Transp      `xml:"transp"`
	Pag        Pag         `xml:"pag"`
	InfAdic    *InfAdic    `xml:"infAdic,omitempty"`
	InfRespTec *InfRespTec `xml:"infRespTec,omitempty"`
}

// Ide é a identificação da nota.
type Ide struct {
	CUF      int    `xml:"cUF"`
	CNF      string `xml:"cNF"`
	NatOp    string `xml:"natOp"`
	Mod      int    `xml:"mod"`
	Serie    int    `xml:"serie"`
	NNF      int    `xml:"nNF"`
	DhEmi    string `xml:"dhEmi"` // AAAA-MM-DDThh:mm:ssTZD, no fuso do emitente
	DhSaiEnt string `xml:"dhSaiEnt,omitempty"`
	TpNF     int    `xml:"tpNF"`   // 0 entrada, 1 saída
	IdDest   int    `xml:"idDest"` // 1 interna, 2 interestadual, 3 exterior
	CMunFG   string `xml:"cMunFG"`
	// CMunFGIBS é o município do fato gerador do IBS, quando difere do cMunFG (NT 2025.002).
	CMunFGIBS string `xml:"cMunFGIBS,omitempty"`
	TpImp     int    `xml:"tpImp"`  // 1 DANFE retrato, 4 DANFCE...
	TpEmis    int    `xml:"tpEmis"` // 1 normal, 6 SVC-AN, 7 SVC-RS, 9 offline NFC-e
	CDV       int    `xml:"cDV"`
	TpAmb     int    `xml:"tpAmb"`
	FinNFe    int    `xml:"finNFe"` // 1 normal, 2 complementar, 3 ajuste, 4 devolução...
	// TpNFDebito e TpNFCredito são as notas de débito e crédito da reforma (finNFe 5 e 6).
	TpNFDebito  string  `xml:"tpNFDebito,omitempty"`
	TpNFCredito string  `xml:"tpNFCredito,omitempty"`
	IndFinal    int     `xml:"indFinal"` // 1 consumidor final
	IndPres     int     `xml:"indPres"`  // 1 presencial, 2 internet, 9 outros...
	IndIntermed *int    `xml:"indIntermed,omitempty"`
	ProcEmi     int     `xml:"procEmi"` // 0 aplicativo do contribuinte
	VerProc     string  `xml:"verProc"`
	DhCont      string  `xml:"dhCont,omitempty"` // entrada em contingência
	XJust       string  `xml:"xJust,omitempty"`
	NFref       []NFref `xml:"NFref,omitempty"`
}

// NFref referencia outra nota (devolução, complementar).
type NFref struct {
	RefNFe string `xml:"refNFe,omitempty"`
}

// Emit é o emitente.
type Emit struct {
	CNPJ      string   `xml:"CNPJ,omitempty"`
	CPF       string   `xml:"CPF,omitempty"`
	XNome     string   `xml:"xNome"`
	XFant     string   `xml:"xFant,omitempty"`
	EnderEmit Endereco `xml:"enderEmit"`
	IE        string   `xml:"IE,omitempty"`
	IEST      string   `xml:"IEST,omitempty"`
	IM        string   `xml:"IM,omitempty"`
	CNAE      string   `xml:"CNAE,omitempty"` // só com IM
	CRT       int      `xml:"CRT"`
}

// Endereco é o endereço do emitente ou do destinatário.
type Endereco struct {
	XLgr    string `xml:"xLgr"`
	Nro     string `xml:"nro"`
	XCpl    string `xml:"xCpl,omitempty"`
	XBairro string `xml:"xBairro"`
	CMun    string `xml:"cMun"`
	XMun    string `xml:"xMun"`
	UF      string `xml:"UF"`
	CEP     string `xml:"CEP,omitempty"`
	CPais   string `xml:"cPais,omitempty"`
	XPais   string `xml:"xPais,omitempty"`
	Fone    string `xml:"fone,omitempty"`
}

// Dest é o destinatário. Na NFC-e de consumidor não identificado, fica nil.
type Dest struct {
	CNPJ          string    `xml:"CNPJ,omitempty"`
	CPF           string    `xml:"CPF,omitempty"`
	IdEstrangeiro string    `xml:"idEstrangeiro,omitempty"`
	XNome         string    `xml:"xNome,omitempty"`
	EnderDest     *Endereco `xml:"enderDest,omitempty"`
	IndIEDest     int       `xml:"indIEDest"` // 1 contribuinte, 2 isento, 9 não contribuinte
	IE            string    `xml:"IE,omitempty"`
	ISUF          string    `xml:"ISUF,omitempty"`
	IM            string    `xml:"IM,omitempty"`
	Email         string    `xml:"email,omitempty"`
}

// Det é um item.
type Det struct {
	NItem     int     `xml:"nItem,attr"`
	Prod      Prod    `xml:"prod"`
	Imposto   Imposto `xml:"imposto"`
	InfAdProd string  `xml:"infAdProd,omitempty"`
	// VItem é o valor total do item com os tributos da reforma (NT 2025.002); opcional.
	VItem *Dec `xml:"vItem,omitempty"`
}

// Prod é o produto do item.
type Prod struct {
	CProd    string `xml:"cProd"`
	CEAN     string `xml:"cEAN"` // GTIN ou "SEM GTIN"
	XProd    string `xml:"xProd"`
	NCM      string `xml:"NCM"`
	CEST     string `xml:"CEST,omitempty"`
	CBenef   string `xml:"cBenef,omitempty"`
	EXTIPI   string `xml:"EXTIPI,omitempty"`
	CFOP     string `xml:"CFOP"`
	UCom     string `xml:"uCom"`
	QCom     Dec    `xml:"qCom"`   // até 4 casas
	VUnCom   Dec    `xml:"vUnCom"` // até 10 casas
	VProd    Dec    `xml:"vProd"`  // 2 casas
	CEANTrib string `xml:"cEANTrib"`
	UTrib    string `xml:"uTrib"`
	QTrib    Dec    `xml:"qTrib"`
	VUnTrib  Dec    `xml:"vUnTrib"`
	VFrete   *Dec   `xml:"vFrete,omitempty"`
	VSeg     *Dec   `xml:"vSeg,omitempty"`
	VDesc    *Dec   `xml:"vDesc,omitempty"`
	VOutro   *Dec   `xml:"vOutro,omitempty"`
	IndTot   int    `xml:"indTot"` // 1 o item compõe o vNF
	XPed     string `xml:"xPed,omitempty"`
	NItemPed string `xml:"nItemPed,omitempty"`
}

// Imposto são os tributos do item: os atuais e os da reforma, lado a lado na transição.
type Imposto struct {
	VTotTrib   *Dec        `xml:"vTotTrib,omitempty"` // Lei da Transparência (12.741/2012)
	ICMS       *ICMS       `xml:"ICMS,omitempty"`
	IPI        *IPI        `xml:"IPI,omitempty"`
	PIS        *PIS        `xml:"PIS,omitempty"`
	COFINS     *COFINS     `xml:"COFINS,omitempty"`
	ICMSUFDest *ICMSUFDest `xml:"ICMSUFDest,omitempty"`
	IS         *IS         `xml:"IS,omitempty"`
	IBSCBS     *IBSCBS     `xml:"IBSCBS,omitempty"`
}

// ICMS tem um grupo por CST (regime normal) ou CSOSN (Simples). Preencha exatamente um.
type ICMS struct {
	ICMS00    *ICMS00    `xml:"ICMS00,omitempty"`
	ICMS20    *ICMS20    `xml:"ICMS20,omitempty"`
	ICMS40    *ICMS40    `xml:"ICMS40,omitempty"`
	ICMS60    *ICMS60    `xml:"ICMS60,omitempty"`
	ICMS90    *ICMS90    `xml:"ICMS90,omitempty"`
	ICMSSN101 *ICMSSN101 `xml:"ICMSSN101,omitempty"`
	ICMSSN102 *ICMSSN102 `xml:"ICMSSN102,omitempty"`
	ICMSSN500 *ICMSSN500 `xml:"ICMSSN500,omitempty"`
	ICMSSN900 *ICMSSN900 `xml:"ICMSSN900,omitempty"`
}

// ICMS00 é a tributação integral.
type ICMS00 struct {
	Orig  int    `xml:"orig"`
	CST   string `xml:"CST"`
	ModBC int    `xml:"modBC"` // 3 = valor da operação
	VBC   Dec    `xml:"vBC"`
	PICMS Dec    `xml:"pICMS"`
	VICMS Dec    `xml:"vICMS"`
	PFCP  *Dec   `xml:"pFCP,omitempty"`
	VFCP  *Dec   `xml:"vFCP,omitempty"`
}

// ICMS20 é a tributação com redução de base.
type ICMS20 struct {
	Orig       int    `xml:"orig"`
	CST        string `xml:"CST"`
	ModBC      int    `xml:"modBC"`
	PRedBC     Dec    `xml:"pRedBC"`
	VBC        Dec    `xml:"vBC"`
	PICMS      Dec    `xml:"pICMS"`
	VICMS      Dec    `xml:"vICMS"`
	VBCFCP     *Dec   `xml:"vBCFCP,omitempty"`
	PFCP       *Dec   `xml:"pFCP,omitempty"`
	VFCP       *Dec   `xml:"vFCP,omitempty"`
	VICMSDeson *Dec   `xml:"vICMSDeson,omitempty"`
	MotDesICMS string `xml:"motDesICMS,omitempty"`
}

// ICMS40 é isenta (40), não tributada (41) ou com suspensão (50).
type ICMS40 struct {
	Orig       int    `xml:"orig"`
	CST        string `xml:"CST"`
	VICMSDeson *Dec   `xml:"vICMSDeson,omitempty"`
	MotDesICMS string `xml:"motDesICMS,omitempty"`
}

// ICMS60 é o ICMS cobrado anteriormente por substituição tributária.
type ICMS60 struct {
	Orig       int    `xml:"orig"`
	CST        string `xml:"CST"`
	VBCSTRet   *Dec   `xml:"vBCSTRet,omitempty"`
	PST        *Dec   `xml:"pST,omitempty"`
	VICMSSubst *Dec   `xml:"vICMSSubstituto,omitempty"`
	VICMSSTRet *Dec   `xml:"vICMSSTRet,omitempty"`
}

// ICMS90 é "outros". Sem os campos de ST, que entram com o grupo de substituição.
type ICMS90 struct {
	Orig   int    `xml:"orig"`
	CST    string `xml:"CST"`
	ModBC  *int   `xml:"modBC,omitempty"`
	VBC    *Dec   `xml:"vBC,omitempty"`
	PRedBC *Dec   `xml:"pRedBC,omitempty"`
	PICMS  *Dec   `xml:"pICMS,omitempty"`
	VICMS  *Dec   `xml:"vICMS,omitempty"`
}

// ICMSSN101 é o Simples com permissão de crédito.
type ICMSSN101 struct {
	Orig        int    `xml:"orig"`
	CSOSN       string `xml:"CSOSN"`
	PCredSN     Dec    `xml:"pCredSN"`
	VCredICMSSN Dec    `xml:"vCredICMSSN"`
}

// ICMSSN102 é o Simples sem crédito (102), isento por faixa (103), imune (300) e não
// tributado (400). É o grupo da maioria das vendas do Simples.
type ICMSSN102 struct {
	Orig  int    `xml:"orig"`
	CSOSN string `xml:"CSOSN"`
}

// ICMSSN500 é o ICMS já cobrado por substituição (Simples).
type ICMSSN500 struct {
	Orig       int    `xml:"orig"`
	CSOSN      string `xml:"CSOSN"`
	VBCSTRet   *Dec   `xml:"vBCSTRet,omitempty"`
	PST        *Dec   `xml:"pST,omitempty"`
	VICMSSubst *Dec   `xml:"vICMSSubstituto,omitempty"`
	VICMSSTRet *Dec   `xml:"vICMSSTRet,omitempty"`
}

// ICMSSN900 é "outros" do Simples. Sem os campos de ST.
type ICMSSN900 struct {
	Orig        int    `xml:"orig"`
	CSOSN       string `xml:"CSOSN"`
	ModBC       *int   `xml:"modBC,omitempty"`
	VBC         *Dec   `xml:"vBC,omitempty"`
	PRedBC      *Dec   `xml:"pRedBC,omitempty"`
	PICMS       *Dec   `xml:"pICMS,omitempty"`
	VICMS       *Dec   `xml:"vICMS,omitempty"`
	PCredSN     *Dec   `xml:"pCredSN,omitempty"`
	VCredICMSSN *Dec   `xml:"vCredICMSSN,omitempty"`
}

// IPI é o imposto sobre produtos industrializados.
type IPI struct {
	CNPJProd string   `xml:"CNPJProd,omitempty"`
	CEnq     string   `xml:"cEnq"` // enquadramento; 999 quando não há
	IPITrib  *IPITrib `xml:"IPITrib,omitempty"`
	IPINT    *IPINT   `xml:"IPINT,omitempty"`
}

// IPITrib é o IPI tributado por alíquota.
type IPITrib struct {
	CST  string `xml:"CST"`
	VBC  Dec    `xml:"vBC"`
	PIPI Dec    `xml:"pIPI"`
	VIPI Dec    `xml:"vIPI"`
}

// IPINT é o IPI não tributado.
type IPINT struct {
	CST string `xml:"CST"`
}

// PIS tem um grupo por CST. Preencha exatamente um.
type PIS struct {
	PISAliq *PISAliq `xml:"PISAliq,omitempty"`
	PISNT   *PISNT   `xml:"PISNT,omitempty"`
	PISOutr *PISOutr `xml:"PISOutr,omitempty"`
}

// PISAliq é o PIS por alíquota (CST 01 e 02).
type PISAliq struct {
	CST  string `xml:"CST"`
	VBC  Dec    `xml:"vBC"`
	PPIS Dec    `xml:"pPIS"`
	VPIS Dec    `xml:"vPIS"`
}

// PISNT é o PIS não tributado (CST 04 a 09).
type PISNT struct {
	CST string `xml:"CST"`
}

// PISOutr são as outras operações (CST 49 a 99), por alíquota.
type PISOutr struct {
	CST  string `xml:"CST"`
	VBC  Dec    `xml:"vBC"`
	PPIS Dec    `xml:"pPIS"`
	VPIS Dec    `xml:"vPIS"`
}

// COFINS tem um grupo por CST. Preencha exatamente um.
type COFINS struct {
	COFINSAliq *COFINSAliq `xml:"COFINSAliq,omitempty"`
	COFINSNT   *COFINSNT   `xml:"COFINSNT,omitempty"`
	COFINSOutr *COFINSOutr `xml:"COFINSOutr,omitempty"`
}

// COFINSAliq é a COFINS por alíquota.
type COFINSAliq struct {
	CST     string `xml:"CST"`
	VBC     Dec    `xml:"vBC"`
	PCOFINS Dec    `xml:"pCOFINS"`
	VCOFINS Dec    `xml:"vCOFINS"`
}

// COFINSNT é a COFINS não tributada.
type COFINSNT struct {
	CST string `xml:"CST"`
}

// COFINSOutr são as outras operações, por alíquota.
type COFINSOutr struct {
	CST     string `xml:"CST"`
	VBC     Dec    `xml:"vBC"`
	PCOFINS Dec    `xml:"pCOFINS"`
	VCOFINS Dec    `xml:"vCOFINS"`
}

// ICMSUFDest é a partilha do ICMS (DIFAL) na venda interestadual a consumidor final não
// contribuinte.
type ICMSUFDest struct {
	VBCUFDest      Dec  `xml:"vBCUFDest"`
	VBCFCPUFDest   *Dec `xml:"vBCFCPUFDest,omitempty"`
	PFCPUFDest     *Dec `xml:"pFCPUFDest,omitempty"`
	PICMSUFDest    Dec  `xml:"pICMSUFDest"`
	PICMSInter     Dec  `xml:"pICMSInter"` // 4.00, 7.00 ou 12.00
	PICMSInterPart Dec  `xml:"pICMSInterPart"`
	VFCPUFDest     *Dec `xml:"vFCPUFDest,omitempty"`
	VICMSUFDest    Dec  `xml:"vICMSUFDest"`
	VICMSUFRemet   Dec  `xml:"vICMSUFRemet"`
}

// IS é o Imposto Seletivo (a partir de 2027).
type IS struct {
	CSTIS        string `xml:"CSTIS"`
	CClassTribIS string `xml:"cClassTribIS"`
	VBCIS        *Dec   `xml:"vBCIS,omitempty"`
	PIS          *Dec   `xml:"pIS,omitempty"`
	AdRemIS      *Dec   `xml:"adRemIS,omitempty"` // alíquota específica (ad rem)
	UTrib        string `xml:"uTrib,omitempty"`
	QTrib        *Dec   `xml:"qTrib,omitempty"`
	VIS          *Dec   `xml:"vIS,omitempty"`
}

// IBSCBS são o IBS e a CBS do item.
type IBSCBS struct {
	CST        string   `xml:"CST"`        // 3 dígitos
	CClassTrib string   `xml:"cClassTrib"` // 6 dígitos, tabela oficial de classificação
	IndDoacao  string   `xml:"indDoacao,omitempty"`
	GIBSCBS    *GIBSCBS `xml:"gIBSCBS,omitempty"`
}

// GIBSCBS é a base e o cálculo do IBS (UF e município) e da CBS.
type GIBSCBS struct {
	VBC          Dec           `xml:"vBC"`
	GIBSUF       GIBSUF        `xml:"gIBSUF"`
	GIBSMun      GIBSMun       `xml:"gIBSMun"`
	VIBS         Dec           `xml:"vIBS"` // vIBSUF + vIBSMun
	GCBS         GCBS          `xml:"gCBS"`
	GTribRegular *GTribRegular `xml:"gTribRegular,omitempty"`
}

// GIBSUF é a parcela estadual do IBS.
type GIBSUF struct {
	PIBSUF   Dec       `xml:"pIBSUF"`
	GDif     *GDif     `xml:"gDif,omitempty"`
	GDevTrib *GDevTrib `xml:"gDevTrib,omitempty"`
	GRed     *GRed     `xml:"gRed,omitempty"`
	VIBSUF   Dec       `xml:"vIBSUF"`
}

// GIBSMun é a parcela municipal do IBS.
type GIBSMun struct {
	PIBSMun  Dec       `xml:"pIBSMun"`
	GDif     *GDif     `xml:"gDif,omitempty"`
	GDevTrib *GDevTrib `xml:"gDevTrib,omitempty"`
	GRed     *GRed     `xml:"gRed,omitempty"`
	VIBSMun  Dec       `xml:"vIBSMun"`
}

// GCBS é a CBS.
type GCBS struct {
	PCBS     Dec       `xml:"pCBS"`
	GDif     *GDif     `xml:"gDif,omitempty"`
	GDevTrib *GDevTrib `xml:"gDevTrib,omitempty"`
	GRed     *GRed     `xml:"gRed,omitempty"`
	VCBS     Dec       `xml:"vCBS"`
}

// GDif é o diferimento.
type GDif struct {
	PDif Dec `xml:"pDif"`
	VDif Dec `xml:"vDif"`
}

// GDevTrib é a devolução de tributo (cashback).
type GDevTrib struct {
	PDevTrib *Dec `xml:"pDevTrib,omitempty"`
	VDevTrib Dec  `xml:"vDevTrib"`
}

// GRed é a redução de alíquota (regimes diferenciados da LC 214/2025).
type GRed struct {
	PRedAliq  Dec `xml:"pRedAliq"`
	PAliqEfet Dec `xml:"pAliqEfet"`
}

// GTribRegular é a tributação que valeria no regime regular, informada quando a operação
// usa regime diferenciado.
type GTribRegular struct {
	CSTReg             string `xml:"CSTReg"`
	CClassTribReg      string `xml:"cClassTribReg"`
	PAliqEfetRegIBSUF  Dec    `xml:"pAliqEfetRegIBSUF"`
	VTribRegIBSUF      Dec    `xml:"vTribRegIBSUF"`
	PAliqEfetRegIBSMun Dec    `xml:"pAliqEfetRegIBSMun"`
	VTribRegIBSMun     Dec    `xml:"vTribRegIBSMun"`
	PAliqEfetRegCBS    Dec    `xml:"pAliqEfetRegCBS"`
	VTribRegCBS        Dec    `xml:"vTribRegCBS"`
}

// Total são os totais da nota.
type Total struct {
	ICMSTot   ICMSTot    `xml:"ICMSTot"`
	ISTot     *ISTot     `xml:"ISTot,omitempty"`
	IBSCBSTot *IBSCBSTot `xml:"IBSCBSTot,omitempty"`
	// VNFTot é o total com IBS, CBS e IS. Opcional; ver ComputeTotals.
	VNFTot *Dec `xml:"vNFTot,omitempty"`
}

// ICMSTot são os totais dos tributos atuais e o valor da nota.
type ICMSTot struct {
	VBC          Dec  `xml:"vBC"`
	VICMS        Dec  `xml:"vICMS"`
	VICMSDeson   Dec  `xml:"vICMSDeson"`
	VFCPUFDest   *Dec `xml:"vFCPUFDest,omitempty"`
	VICMSUFDest  *Dec `xml:"vICMSUFDest,omitempty"`
	VICMSUFRemet *Dec `xml:"vICMSUFRemet,omitempty"`
	VFCP         Dec  `xml:"vFCP"`
	VBCST        Dec  `xml:"vBCST"`
	VST          Dec  `xml:"vST"`
	VFCPST       Dec  `xml:"vFCPST"`
	VFCPSTRet    Dec  `xml:"vFCPSTRet"`
	VProd        Dec  `xml:"vProd"`
	VFrete       Dec  `xml:"vFrete"`
	VSeg         Dec  `xml:"vSeg"`
	VDesc        Dec  `xml:"vDesc"`
	VII          Dec  `xml:"vII"`
	VIPI         Dec  `xml:"vIPI"`
	VIPIDevol    Dec  `xml:"vIPIDevol"`
	VPIS         Dec  `xml:"vPIS"`
	VCOFINS      Dec  `xml:"vCOFINS"`
	VOutro       Dec  `xml:"vOutro"`
	VNF          Dec  `xml:"vNF"`
	VTotTrib     *Dec `xml:"vTotTrib,omitempty"`
}

// ISTot é o total do Imposto Seletivo.
type ISTot struct {
	VIS Dec `xml:"vIS"`
}

// IBSCBSTot são os totais do IBS e da CBS.
type IBSCBSTot struct {
	VBCIBSCBS Dec      `xml:"vBCIBSCBS"`
	GIBS      *GIBSTot `xml:"gIBS,omitempty"`
	GCBS      *GCBSTot `xml:"gCBS,omitempty"`
}

// GIBSTot é o total do IBS.
type GIBSTot struct {
	GIBSUF           GIBSUFTot  `xml:"gIBSUF"`
	GIBSMun          GIBSMunTot `xml:"gIBSMun"`
	VIBS             Dec        `xml:"vIBS"`
	VCredPres        Dec        `xml:"vCredPres"`
	VCredPresCondSus Dec        `xml:"vCredPresCondSus"`
}

// GIBSUFTot é o total da parcela estadual.
type GIBSUFTot struct {
	VDif     Dec `xml:"vDif"`
	VDevTrib Dec `xml:"vDevTrib"`
	VIBSUF   Dec `xml:"vIBSUF"`
}

// GIBSMunTot é o total da parcela municipal.
type GIBSMunTot struct {
	VDif     Dec `xml:"vDif"`
	VDevTrib Dec `xml:"vDevTrib"`
	VIBSMun  Dec `xml:"vIBSMun"`
}

// GCBSTot é o total da CBS.
type GCBSTot struct {
	VDif             Dec `xml:"vDif"`
	VDevTrib         Dec `xml:"vDevTrib"`
	VCBS             Dec `xml:"vCBS"`
	VCredPres        Dec `xml:"vCredPres"`
	VCredPresCondSus Dec `xml:"vCredPresCondSus"`
}

// Transp é o transporte.
type Transp struct {
	ModFrete   int         `xml:"modFrete"` // 0 emitente, 1 destinatário, 9 sem frete...
	Transporta *Transporta `xml:"transporta,omitempty"`
	Vol        []Vol       `xml:"vol,omitempty"`
}

// Transporta é a transportadora.
type Transporta struct {
	CNPJ   string `xml:"CNPJ,omitempty"`
	CPF    string `xml:"CPF,omitempty"`
	XNome  string `xml:"xNome,omitempty"`
	IE     string `xml:"IE,omitempty"`
	XEnder string `xml:"xEnder,omitempty"`
	XMun   string `xml:"xMun,omitempty"`
	UF     string `xml:"UF,omitempty"`
}

// Vol é um volume transportado.
type Vol struct {
	QVol  *int   `xml:"qVol,omitempty"`
	Esp   string `xml:"esp,omitempty"`
	Marca string `xml:"marca,omitempty"`
	NVol  string `xml:"nVol,omitempty"`
	PesoL *Dec   `xml:"pesoL,omitempty"` // 3 casas
	PesoB *Dec   `xml:"pesoB,omitempty"`
}

// Pag são os pagamentos.
type Pag struct {
	DetPag []DetPag `xml:"detPag"`
	VTroco *Dec     `xml:"vTroco,omitempty"`
}

// DetPag é uma forma de pagamento.
type DetPag struct {
	IndPag *int   `xml:"indPag,omitempty"` // 0 à vista, 1 a prazo
	TPag   string `xml:"tPag"`             // 01 dinheiro, 03 crédito, 17 PIX, 90 sem pagamento...
	XPag   string `xml:"xPag,omitempty"`
	VPag   Dec    `xml:"vPag"`
	Card   *Card  `xml:"card,omitempty"`
}

// Card é a operação com cartão.
type Card struct {
	TpIntegra int    `xml:"tpIntegra"` // 1 integrado ao sistema, 2 não integrado (POS)
	CNPJ      string `xml:"CNPJ,omitempty"`
	TBand     string `xml:"tBand,omitempty"`
	CAut      string `xml:"cAut,omitempty"`
}

// InfAdic são as informações adicionais.
type InfAdic struct {
	InfAdFisco string `xml:"infAdFisco,omitempty"`
	InfCpl     string `xml:"infCpl,omitempty"`
}

// InfRespTec é o responsável técnico pelo sistema emissor.
type InfRespTec struct {
	CNPJ     string `xml:"CNPJ"`
	XContato string `xml:"xContato"`
	Email    string `xml:"email"`
	Fone     string `xml:"fone"`
}

// Marshal serializa a nota sem declaração XML e sem indentação — a forma que vai para a
// assinatura e para o lote.
func Marshal(n *NFe) ([]byte, error) {
	n.Xmlns = Namespace
	if n.InfNFe.Versao == "" {
		n.InfNFe.Versao = "4.00"
	}
	return xml.Marshal(n)
}
