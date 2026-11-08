// Adaptado de github.com/mschunke/gonfe/internal/pdf — Copyright (c) 2026 mschunke, licença
// MIT. Mudanças: identificadores em inglês.

// Package pdf escreve arquivos PDF simples, o suficiente para os documentos
// auxiliares dos documentos fiscais: texto, linhas, retângulos e código de
// barras.
//
// O gerador usa apenas as fontes base-14, que todo leitor de PDF já possui, e
// por isso não precisa embutir arquivo de fonte nem depender de biblioteca
// gráfica. O módulo inteiro continua sem dependências além da biblioteca
// padrão.
//
// As coordenadas são em milímetros e a origem fica no canto superior esquerdo,
// com o eixo Y crescendo para baixo — a convenção natural para diagramar um
// formulário. A conversão para o sistema do PDF, que tem origem embaixo e mede
// em pontos, acontece internamente.
package pdf

import (
	"bytes"
	"fmt"
	"strings"
)

// pointsPerMM converte milímetros em pontos tipográficos, a unidade do PDF.
const pointsPerMM = 72.0 / 25.4

// Dimensões usuais de página, em milímetros.
const (
	A4Width  = 210.0
	A4Height = 297.0
)

// Font identifica uma das fontes base-14 usadas pelo gerador.
type Font int

const (
	// Regular é a Helvetica.
	Regular Font = iota
	// Bold é a Helvetica-Bold.
	Bold
	// Mono é a Courier, útil para colunas numéricas alinhadas.
	Mono
)

func (f Font) resource() string {
	switch f {
	case Bold:
		return "F2"
	case Mono:
		return "F3"
	default:
		return "F1"
	}
}

func (f Font) baseName() string {
	switch f {
	case Bold:
		return "Helvetica-Bold"
	case Mono:
		return "Courier"
	default:
		return "Helvetica"
	}
}

// Style descreve como um texto é desenhado.
type Style struct {
	// Font a usar; o padrão é [Regular].
	Font Font
	// Size em pontos; zero equivale a 6, o corpo usual do DANFE.
	Size float64
	// Gray é o tom do texto, de 0 (preto) a 1 (branco).
	Gray float64
}

func (e Style) size() float64 {
	if e.Size <= 0 {
		return 6
	}
	return e.Size
}

// PDF acumula as páginas de um documento.
type PDF struct {
	pages []*Page
}

// New cria um documento vazio.
func New() *PDF { return &PDF{} }

// Page é uma página em construção.
type Page struct {
	width, height float64 // em milímetros
	content       bytes.Buffer
}

// AddPage acrescenta uma página com as dimensões informadas, em milímetros.
func (p *PDF) AddPage(width, height float64) *Page {
	pg := &Page{width: width, height: height}
	p.pages = append(p.pages, pg)
	return pg
}

// AddPageA4 acrescenta uma página A4 em retrato.
func (p *PDF) AddPageA4() *Page { return p.AddPage(A4Width, A4Height) }

// Pages devolve quantas páginas o documento tem.
func (p *PDF) Pages() int { return len(p.pages) }

// Width devolve a width da página em milímetros.
func (pg *Page) Width() float64 { return pg.width }

// Height devolve a height da página em milímetros.
func (pg *Page) Height() float64 { return pg.height }

// y converte a coordenada vertical do sistema de cima para baixo, em
// milímetros, para o sistema do PDF, de baixo para cima, em pontos.
func (pg *Page) y(mm float64) float64 { return (pg.height - mm) * pointsPerMM }

func x(mm float64) float64 { return mm * pointsPerMM }

// Text desenha o texto com o canto superior esquerdo em (posX, posY).
func (pg *Page) Text(posX, posY float64, texto string, e Style) {
	if texto == "" {
		return
	}
	size := e.size()
	// A linha de base fica abaixo do topo do texto; 0,8 do corpo é uma
	// aproximação boa o suficiente para as fontes base-14.
	base := pg.y(posY) - size*0.8

	fmt.Fprintf(&pg.content, "BT\n")
	if e.Gray > 0 {
		fmt.Fprintf(&pg.content, "%.3f g\n", e.Gray)
	}
	fmt.Fprintf(&pg.content, "/%s %.2f Tf\n", e.Font.resource(), size)
	fmt.Fprintf(&pg.content, "%.2f %.2f Td\n", x(posX), base)
	fmt.Fprintf(&pg.content, "(%s) Tj\n", escape(toWinAnsi(texto)))
	fmt.Fprintf(&pg.content, "ET\n")
	if e.Gray > 0 {
		pg.content.WriteString("0 g\n")
	}
}

// TextCenter desenha o texto centralizado na faixa horizontal que começa
// em posX e tem a width informada.
func (pg *Page) TextCenter(posX, width, posY float64, texto string, e Style) {
	sobra := width - Measure(texto, e.Font, e.size())
	if sobra < 0 {
		sobra = 0
	}
	pg.Text(posX+sobra/2, posY, texto, e)
}

// TextRight desenha o texto alinhado à direita, terminando em posX mais a
// width informada.
func (pg *Page) TextRight(posX, width, posY float64, texto string, e Style) {
	sobra := width - Measure(texto, e.Font, e.size())
	if sobra < 0 {
		sobra = 0
	}
	pg.Text(posX+sobra, posY, texto, e)
}

// Line desenha um segmento de reta.
func (pg *Page) Line(x1, y1, x2, y2, espessura float64) {
	fmt.Fprintf(&pg.content, "%.2f w\n%.2f %.2f m\n%.2f %.2f l\nS\n",
		espessura*pointsPerMM, x(x1), pg.y(y1), x(x2), pg.y(y2))
}

// Rect desenha o contorno de um retângulo com o canto superior esquerdo em
// (posX, posY).
func (pg *Page) Rect(posX, posY, width, height, espessura float64) {
	fmt.Fprintf(&pg.content, "%.2f w\n%.2f %.2f %.2f %.2f re\nS\n",
		espessura*pointsPerMM, x(posX), pg.y(posY+height), x(width), x(height))
}

// FillRect desenha um retângulo cheio no tom de cinza informado, de
// 0 (preto) a 1 (branco).
func (pg *Page) FillRect(posX, posY, width, height, cinza float64) {
	fmt.Fprintf(&pg.content, "%.3f g\n%.2f %.2f %.2f %.2f re\nf\n0 g\n",
		cinza, x(posX), pg.y(posY+height), x(width), x(height))
}

// Measure devolve a width que o texto ocupará, em milímetros.
func Measure(texto string, f Font, size float64) float64 {
	var milesimos float64
	for _, r := range texto {
		milesimos += float64(charWidth(r, f))
	}
	return milesimos / 1000 * size / pointsPerMM
}

// Truncate corta o texto até caber na width informada, acrescentando
// reticências quando houver corte.
func Truncate(texto string, f Font, size, width float64) string {
	if Measure(texto, f, size) <= width {
		return texto
	}
	const reticencias = "…"
	disponivel := width - Measure(reticencias, f, size)
	if disponivel <= 0 {
		return ""
	}
	runas := []rune(texto)
	for i := len(runas); i > 0; i-- {
		if Measure(string(runas[:i]), f, size) <= disponivel {
			return strings.TrimRight(string(runas[:i]), " ") + reticencias
		}
	}
	return ""
}

// Wrap divide o texto em linhas que cabem na width informada, quebrando
// entre palavras sempre que possível.
func Wrap(texto string, f Font, size, width float64) []string {
	var linhas []string
	for _, paragrafo := range strings.Split(texto, "\n") {
		palavras := strings.Fields(paragrafo)
		if len(palavras) == 0 {
			linhas = append(linhas, "")
			continue
		}
		atual := palavras[0]
		for _, palavra := range palavras[1:] {
			tentativa := atual + " " + palavra
			if Measure(tentativa, f, size) <= width {
				atual = tentativa
				continue
			}
			linhas = append(linhas, atual)
			atual = palavra
		}
		linhas = append(linhas, atual)
	}
	return linhas
}

// TextWrapped desenha o texto em várias linhas dentro da width informada,
// respeitando o limite de linhas quando ele for maior que zero. Devolve a
// height ocupada, em milímetros.
func (pg *Page) TextWrapped(posX, posY, width float64, texto string, e Style, maxLinhas int) float64 {
	alturaLinha := e.size() / pointsPerMM * 1.15
	linhas := Wrap(texto, e.Font, e.size(), width)
	if maxLinhas > 0 && len(linhas) > maxLinhas {
		linhas = linhas[:maxLinhas]
		ultima := linhas[len(linhas)-1]
		linhas[len(linhas)-1] = Truncate(ultima+" …", e.Font, e.size(), width)
	}
	for i, linha := range linhas {
		pg.Text(posX, posY+float64(i)*alturaLinha, linha, e)
	}
	return float64(len(linhas)) * alturaLinha
}

// Bytes serializa o documento.
func (p *PDF) Bytes() ([]byte, error) {
	if len(p.pages) == 0 {
		return nil, fmt.Errorf("pdf: documento sem páginas")
	}

	var saida bytes.Buffer
	var deslocamentos []int
	objeto := func(corpo string) {
		deslocamentos = append(deslocamentos, saida.Len())
		fmt.Fprintf(&saida, "%d 0 obj\n%s\nendobj\n", len(deslocamentos), corpo)
	}

	saida.WriteString("%PDF-1.4\n")
	// O comentário binário evita que ferramentas tratem o arquivo como texto.
	saida.WriteString("%\xE2\xE3\xCF\xD3\n")

	// Numeração: 1 catálogo, 2 árvore de páginas, 3–5 fontes, e a partir de 6
	// os pares página/conteúdo.
	const primeiraPagina = 6
	var refPaginas []string
	for i := range p.pages {
		refPaginas = append(refPaginas, fmt.Sprintf("%d 0 R", primeiraPagina+i*2))
	}

	objeto("<< /Type /Catalog /Pages 2 0 R >>")
	objeto(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>",
		strings.Join(refPaginas, " "), len(p.pages)))
	for _, f := range []Font{Regular, Bold, Mono} {
		objeto(fmt.Sprintf(
			"<< /Type /Font /Subtype /Type1 /BaseFont /%s /Encoding /WinAnsiEncoding >>",
			f.baseName()))
	}

	for i, pg := range p.pages {
		conteudoRef := primeiraPagina + i*2 + 1
		objeto(fmt.Sprintf(
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] "+
				"/Resources << /Font << /F1 3 0 R /F2 4 0 R /F3 5 0 R >> >> /Contents %d 0 R >>",
			x(pg.width), x(pg.height), conteudoRef))

		fluxo := pg.content.String()
		objeto(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(fluxo), fluxo))
	}

	inicioXref := saida.Len()
	fmt.Fprintf(&saida, "xref\n0 %d\n0000000000 65535 f \n", len(deslocamentos)+1)
	for _, d := range deslocamentos {
		fmt.Fprintf(&saida, "%010d 00000 n \n", d)
	}
	fmt.Fprintf(&saida, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(deslocamentos)+1, inicioXref)

	return saida.Bytes(), nil
}

// escape protege os caracteres que têm significado em uma string PDF.
func escape(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '(', ')', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case '\r':
			b.WriteString(`\r`)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
