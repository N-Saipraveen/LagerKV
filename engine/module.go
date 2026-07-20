package engine

import (
	"github.com/N-Saipraveen/LagerKV/store"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Options(
		store.Module,
		fx.Provide(NewDB),
	)
}
