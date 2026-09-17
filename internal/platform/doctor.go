package platform

import (
	"context"
	"fmt"
	"io"

	assetslocal "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets/localstorage"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/infrastructure/postgres"
)

func Doctor(ctx context.Context, cfg Config, out io.Writer) error {
	assetStorage, err := assetslocal.New(cfg.AssetStoragePath)
	if err != nil {
		return err
	}
	defer func() { _ = assetStorage.Close() }()
	if _, err := fmt.Fprintln(out, "Asset storage: ready"); err != nil {
		return err
	}
	pool, err := postgres.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if _, err := fmt.Fprintln(out, "PostgreSQL: ready"); err != nil {
		return err
	}
	if err := postgres.SchemaCheck(ctx, pool); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, "Schema: compatible")
	return err
}
