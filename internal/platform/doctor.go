package platform

import (
	"context"
	"fmt"
	"io"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/infrastructure/postgres"
)

func Doctor(ctx context.Context, cfg Config, out io.Writer) error {
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
