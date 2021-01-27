package server

import (
	"path"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pachyderm/pachyderm/v2/src/internal/backoff"
	"github.com/pachyderm/pachyderm/v2/src/internal/dlock"

	"github.com/pachyderm/pachyderm/v2/src/internal/serviceenv"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

const (
	masterLockPath = "pfs-master-lock"
)

func (d *driver) master(env *serviceenv.ServiceEnv, db *sqlx.DB) {
	ctx := context.Background()
	masterLock := dlock.NewDLock(d.etcdClient, path.Join(d.prefix, masterLockPath))
	err := backoff.RetryNotify(func() error {
		masterCtx, err := masterLock.Lock(ctx)
		if err != nil {
			return err
		}
		defer masterLock.Unlock(masterCtx)
		return d.storage.GC(masterCtx)
	}, backoff.NewInfiniteBackOff(), func(err error, _ time.Duration) error {
		log.Errorf("error in pfs master: %v", err)
		return nil
	})
	panic(err)
}
