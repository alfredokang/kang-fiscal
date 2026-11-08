package entity_test

import (
	"testing"
	"time"

	"github.com/alfredokang/kang-fiscal/internal/modules/dfe/domain/entity"
)

func TestCursorRespeitaAEsperaDaSefaz(t *testing.T) {
	now := time.Now()
	c := &entity.Cursor{}
	if !c.After(138, 50, 120, now) || c.LastNSU != 50 || !c.NextQueryAt.IsZero() {
		t.Errorf("lote com mais por vir: %+v", c)
	}
	if c.After(138, 120, 120, now) || c.NextQueryAt != now.Add(time.Hour) {
		t.Errorf("último lote: %+v", c)
	}
	c2 := &entity.Cursor{LastNSU: 120}
	if c2.After(137, 120, 120, now) || c2.NextQueryAt != now.Add(time.Hour) {
		t.Errorf("nenhum documento: %+v", c2)
	}
	c3 := &entity.Cursor{LastNSU: 120}
	if c3.After(656, 0, 0, now) || c3.LastNSU != 120 || c3.NextQueryAt != now.Add(time.Hour) {
		t.Errorf("consumo indevido não pode mexer no NSU: %+v", c3)
	}
}
