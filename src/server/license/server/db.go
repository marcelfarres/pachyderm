package server

import (
	"github.com/pachyderm/pachyderm/src/server/pkg/migrations"
	"golang.org/x/net/context"
)

var desiredClusterState migrations.State = migrations.InitialState().
	Apply("create license schema", func(ctx context.Context, env migrations.Env) error {
		_, err := env.Tx.ExecContext(ctx, `CREATE SCHEMA license`)
		return err
	}).
	Apply("clusters table v0", func(ctx context.Context, env migrations.Env) error {
		_, err := env.Tx.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS license.clusters (
	id VARCHAR(4096) PRIMARY KEY,
	address VARCHAR(4096) NOT NULL,
	secret VARCHAR(64) NOT NULL,
	version VARCHAR(64) NOT NULL,
	enabled BOOL NOT NULL,
	auth_enabled BOOL NOT NULL,
	last_heartbeat TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
);
`)
		return err
	})
