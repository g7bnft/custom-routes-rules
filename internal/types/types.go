package types

import (
	"custom-rules/internal/config"

	"github.com/v2fly/v2ray-core/v5/app/router/routercommon"
)

type StrategyManager interface {
	Run(baseData map[string][]*routercommon.Domain) map[string][]*routercommon.Domain
}

type BuildContext struct {
	Config   *config.Config
	Strategy StrategyManager
}
