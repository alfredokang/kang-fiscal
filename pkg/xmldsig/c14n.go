package xmldsig

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Limites do XML de entrada. XML de fora é hostil (Constituição, Art. 9.5): um documento
// fiscal real tem poucas centenas de KB e profundidade menor que 20.
const (
	MaxDocumentSize = 16 << 20
	maxDepth        = 128
)

var (
	// ErrMalformed: o documento não é XML bem formado.
	ErrMalformed = errors.New("xmldsig: XML malformado")
	// ErrForbidden: DOCTYPE, entidade ou documento grande/profundo demais.
	ErrForbidden = errors.New("xmldsig: XML com construção proibida (DOCTYPE, tamanho ou profundidade)")
)

type nodeKind int

const (
	elementNode nodeKind = iota
	textNode
	piNode
)

// node é a árvore mínima para a canonicalização: guarda os nomes como escritos (com
// prefixo) e os deslocamentos no documento original, para a assinatura ser inserida sem
// reserializar nada.
type node struct {
	kind     nodeKind
	prefix   string
	local    string
	attrs    []xml.Attr // atributos comuns, como escritos
	nsDecl   map[string]string
	text     string // texto já sem escape, ou o conteúdo da instrução de processamento
	target   string // alvo da instrução de processamento
	parent   *node
	children []*node
	// endTagStart é o deslocamento do "</" da tag de fechamento; -1 na forma <a/>.
	endTagStart int64
}

func parse(doc []byte) (*node, error) {
	if len(doc) > MaxDocumentSize {
		return nil, ErrForbidden
	}
	d := xml.NewDecoder(bytes.NewReader(doc))
	d.Strict = true
	// Nada de entidade além das cinco do XML: entidade declarada é o caminho do XXE e da
	// bomba de expansão.
	d.Entity = nil

	root := &node{kind: elementNode, endTagStart: -1}
	cur := root
	depth := 0
	var lastOffset int64
	var lastStart *node

	for {
		before := d.InputOffset()
		tok, err := d.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
		}
		after := d.InputOffset()

		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth > maxDepth {
				return nil, ErrForbidden
			}
			n := &node{kind: elementNode, prefix: t.Name.Space, local: t.Name.Local, parent: cur,
				nsDecl: map[string]string{}, endTagStart: -1}
			for _, a := range t.Attr {
				switch {
				case a.Name.Space == "" && a.Name.Local == "xmlns":
					n.nsDecl[""] = a.Value
				case a.Name.Space == "xmlns":
					n.nsDecl[a.Name.Local] = a.Value
				default:
					n.attrs = append(n.attrs, a)
				}
			}
			cur.children = append(cur.children, n)
			cur = n
			lastStart, lastOffset = n, after
		case xml.EndElement:
			depth--
			// Na forma <a/>, o decodificador devolve um fechamento sintético sem avançar.
			if !(cur == lastStart && before == lastOffset && after == lastOffset) {
				cur.endTagStart = before
			}
			cur = cur.parent
			if cur == nil {
				return nil, ErrMalformed
			}
		case xml.CharData:
			if cur != root {
				cur.children = append(cur.children, &node{kind: textNode, text: string(t), parent: cur})
			}
		case xml.ProcInst:
			if cur != root {
				cur.children = append(cur.children, &node{kind: piNode, target: t.Target, text: string(t.Inst), parent: cur})
			}
		case xml.Directive:
			return nil, ErrForbidden
		case xml.Comment:
			// C14N sem comentários: eles não entram na assinatura.
		}
	}
	if cur != root || len(root.elements()) != 1 {
		return nil, ErrMalformed
	}
	return root, nil
}

func (n *node) elements() []*node {
	var out []*node
	for _, c := range n.children {
		if c.kind == elementNode {
			out = append(out, c)
		}
	}
	return out
}

// scope são as declarações de namespace em vigor no elemento (prefixo → URI).
func (n *node) scope() map[string]string {
	var chain []*node
	for p := n; p != nil; p = p.parent {
		chain = append(chain, p)
	}
	s := map[string]string{}
	for i := len(chain) - 1; i >= 0; i-- {
		for k, v := range chain[i].nsDecl {
			s[k] = v
		}
	}
	return s
}

func (n *node) namespace() string { return n.scope()[n.prefix] }

func (n *node) attr(name string) (string, bool) {
	for _, a := range n.attrs {
		if a.Name.Space == "" && a.Name.Local == name {
			return a.Value, true
		}
	}
	return "", false
}

// find devolve o primeiro elemento com o nome local dado, em ordem de documento.
func (n *node) find(local, ns string) *node {
	for _, c := range n.children {
		if c.kind != elementNode {
			continue
		}
		if c.local == local && (ns == "" || c.namespace() == ns) {
			return c
		}
		if f := c.find(local, ns); f != nil {
			return f
		}
	}
	return nil
}

func (n *node) findAll(local, ns string, out *[]*node) {
	for _, c := range n.children {
		if c.kind != elementNode {
			continue
		}
		if c.local == local && (ns == "" || c.namespace() == ns) {
			*out = append(*out, c)
		}
		c.findAll(local, ns, out)
	}
}

func (n *node) byID(id string) *node {
	for _, c := range n.children {
		if c.kind != elementNode {
			continue
		}
		if v, ok := c.attr("Id"); ok && v == id {
			return c
		}
		if f := c.byID(id); f != nil {
			return f
		}
	}
	return nil
}

func (n *node) child(local string) *node {
	for _, c := range n.children {
		if c.kind == elementNode && c.local == local {
			return c
		}
	}
	return nil
}

func (n *node) textContent() string {
	var b strings.Builder
	for _, c := range n.children {
		if c.kind == textNode {
			b.WriteString(c.text)
		} else if c.kind == elementNode {
			b.WriteString(c.textContent())
		}
	}
	return b.String()
}

// canonicalize escreve a subárvore em Canonical XML 1.0 (inclusivo, sem comentários) —
// http://www.w3.org/TR/2001/REC-xml-c14n-20010315.
//
// O ápice recebe todas as declarações de namespace em vigor, herdadas dos ancestrais:
// é por isso que o infNFe assinado carrega o xmlns do NFe, mesmo sem declará-lo. Com
// skipSignatures, os elementos Signature do XMLDSig ficam de fora (transformação
// enveloped-signature).
func canonicalize(n *node, skipSignatures bool) []byte {
	var b bytes.Buffer
	writeCanonical(&b, n, map[string]string{}, n.scope(), skipSignatures)
	return b.Bytes()
}

func writeCanonical(b *bytes.Buffer, n *node, rendered, scope map[string]string, skipSig bool) {
	switch n.kind {
	case textNode:
		b.WriteString(escapeText(n.text))
		return
	case piNode:
		b.WriteString("<?" + n.target)
		if n.text != "" {
			b.WriteString(" " + n.text)
		}
		b.WriteString("?>")
		return
	}

	qname := n.local
	if n.prefix != "" {
		qname = n.prefix + ":" + n.local
	}
	b.WriteString("<" + qname)

	// Declarações: só as que diferem do que o ancestral de saída já declarou. xmlns=""
	// só aparece para desfazer um default declarado acima.
	prefixes := make([]string, 0, len(scope))
	for p := range scope {
		prefixes = append(prefixes, p)
	}
	sort.Strings(prefixes) // "" (o default) vem primeiro
	next := make(map[string]string, len(rendered))
	for k, v := range rendered {
		next[k] = v
	}
	for _, p := range prefixes {
		uri := scope[p]
		prev, had := rendered[p]
		if (had && prev == uri) || (!had && uri == "") {
			continue
		}
		next[p] = uri
		if p == "" {
			b.WriteString(` xmlns="` + escapeAttr(uri) + `"`)
		} else {
			b.WriteString(" xmlns:" + p + `="` + escapeAttr(uri) + `"`)
		}
	}

	// Atributos ordenados pelo URI do namespace e depois pelo nome local; os sem prefixo
	// não têm namespace e vêm primeiro.
	attrs := append([]xml.Attr(nil), n.attrs...)
	uriOf := func(a xml.Attr) string {
		if a.Name.Space == "" {
			return ""
		}
		return scope[a.Name.Space]
	}
	sort.SliceStable(attrs, func(i, j int) bool {
		ui, uj := uriOf(attrs[i]), uriOf(attrs[j])
		if ui != uj {
			return ui < uj
		}
		return attrs[i].Name.Local < attrs[j].Name.Local
	})
	for _, a := range attrs {
		name := a.Name.Local
		if a.Name.Space != "" {
			name = a.Name.Space + ":" + a.Name.Local
		}
		b.WriteString(" " + name + `="` + escapeAttr(a.Value) + `"`)
	}
	b.WriteString(">")

	for _, c := range n.children {
		if c.kind == elementNode && skipSig && c.local == "Signature" && c.namespace() == NamespaceDSig {
			continue
		}
		childScope := scope
		if c.kind == elementNode && len(c.nsDecl) > 0 {
			childScope = make(map[string]string, len(scope)+len(c.nsDecl))
			for k, v := range scope {
				childScope[k] = v
			}
			for k, v := range c.nsDecl {
				childScope[k] = v
			}
		}
		writeCanonical(b, c, next, childScope, skipSig)
	}
	b.WriteString("</" + qname + ">")
}

var textEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\r", "&#xD;")

var attrEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", `"`, "&quot;",
	"\t", "&#x9;", "\n", "&#xA;", "\r", "&#xD;")

func escapeText(s string) string { return textEscaper.Replace(s) }
func escapeAttr(s string) string { return attrEscaper.Replace(s) }
