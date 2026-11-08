package nfe

import "bytes"

// Proc monta o nfeProc — a nota assinada com o protocolo de autorização. É o arquivo de
// distribuição: o que vai para o destinatário, para o contador e para os 5 anos de guarda.
func Proc(signedNFe, protNFe []byte) []byte {
	var b bytes.Buffer
	b.WriteString(`<nfeProc xmlns="` + Namespace + `" versao="4.00">`)
	b.Write(stripDeclaration(signedNFe))
	b.Write(stripDeclaration(protNFe))
	b.WriteString(`</nfeProc>`)
	return b.Bytes()
}

func stripDeclaration(doc []byte) []byte {
	doc = bytes.TrimSpace(doc)
	if bytes.HasPrefix(doc, []byte("<?xml")) {
		if i := bytes.Index(doc, []byte("?>")); i > 0 {
			return bytes.TrimSpace(doc[i+2:])
		}
	}
	return doc
}
