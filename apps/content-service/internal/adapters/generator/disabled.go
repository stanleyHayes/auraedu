package generator

import (
	"context"

	"github.com/auraedu/content-service/internal/domain"
	"github.com/auraedu/content-service/internal/ports"
)

type Disabled struct{}

func (Disabled) Generate(context.Context, ports.GenerateInput) (ports.GenerateOutput, error) {
	return ports.GenerateOutput{}, domain.ErrUnavailable
}
