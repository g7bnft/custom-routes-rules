package pipeline

import (
	"custom-rules/internal/types"
)


type Stage interface {
	Name() string
	Execute(ctx *types.BuildContext) error
}