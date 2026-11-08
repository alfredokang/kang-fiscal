// Package response são os corpos HTTP devolvidos pela emissão.
package response

import "time"

// Invoice é a nota.
type Invoice struct {
	ID          string `json:"id"`
	AccessKey   string `json:"access_key"`
	Model       int    `json:"model"`
	Series      int    `json:"series"`
	Number      int    `json:"number"`
	Environment string `json:"environment"`
	Status      string `json:"status"`
	CStat       int    `json:"cstat,omitempty"`
	// Motive é o xMotivo como a SEFAZ devolveu — é o que o contador pesquisa.
	Motive string `json:"motive,omitempty"`
	// Hint é a dica de correção do catálogo de códigos, quando há.
	Hint         string     `json:"hint,omitempty"`
	Protocol     string     `json:"protocol,omitempty"`
	Total        string     `json:"total"`
	XMLHash      string     `json:"xml_sha256,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	AuthorizedAt *time.Time `json:"authorized_at,omitempty"`
}

// Event é um evento da nota.
type Event struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // cancellation | correction
	Code      string    `json:"code"` // 110111 | 110110
	Seq       int       `json:"seq"`
	Status    string    `json:"status"`
	CStat     int       `json:"cstat,omitempty"`
	Motive    string    `json:"motive,omitempty"`
	Hint      string    `json:"hint,omitempty"`
	Protocol  string    `json:"protocol,omitempty"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}
