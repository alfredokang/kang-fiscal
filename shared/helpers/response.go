package helpers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/alfredokang/kang-fiscal/shared/logger"
)

// ErrorResponse é o formato único de erro da API.
//
// A mensagem é sempre voltada ao cliente: erro de banco, stack trace e nome de coluna
// vão para o log, nunca para a resposta (Constituição, Art. 10).
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details any    `json:"details,omitempty"`
}

// WriteJSON escreve a resposta serializada.
func WriteJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if data == nil {
		w.WriteHeader(statusCode)
		return
	}

	body, err := json.Marshal(data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"erro ao serializar resposta"}`))
		return
	}

	w.WriteHeader(statusCode)
	_, _ = w.Write(body)
}

// WriteError escreve um erro no formato padrão.
func WriteError(w http.ResponseWriter, statusCode int, message string, code string) {
	WriteJSON(w, statusCode, ErrorResponse{Error: message, Code: code})
}

// MaxBodyBytes é o teto do corpo JSON. Uma NF-e tem no máximo 990 itens; 4 MB cobre a
// nota cheia com folga e ainda barra o corpo que só existe para derrubar o processo.
const MaxBodyBytes = 4 << 20

// DecodeJSON lê o corpo da requisição com teto de tamanho e sem campo desconhecido.
//
// O teto evita que um corpo de 2 GB derrube o processo; DisallowUnknownFields faz um
// campo com nome errado falhar alto em vez de ser silenciosamente ignorado — que é como
// "por que a nota saiu sem o desconto?" vira uma tarde de investigação.
func DecodeJSON(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, MaxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

// WriteInternalError registra a causa real e devolve uma mensagem genérica.
//
// O Artigo 10 da Constituição exige as duas metades: o cliente nunca vê mensagem de
// banco, e o detalhe **precisa** chegar ao log. Sem esta função, o branch `default` de um
// writeDomainError vira um buraco negro — foi exatamente o que aconteceu ao subir a API
// pela primeira vez, e o erro só apareceu depois de adicionar o log.
func WriteInternalError(w http.ResponseWriter, r *http.Request, err error) {
	if logger.Logger != nil {
		logger.Logger.Error("erro_interno",
			zap.String("req_id", middleware.GetReqID(r.Context())),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Error(err),
		)
	}
	WriteError(w, http.StatusInternalServerError, "erro interno", "INTERNAL_ERROR")
}
