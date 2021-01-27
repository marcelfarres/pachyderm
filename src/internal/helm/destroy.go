package helm

import (
	"github.com/pachyderm/pachyderm/v2/src/internal/config"
	"github.com/pachyderm/pachyderm/v2/src/internal/errors"

	"helm.sh/helm/v3/pkg/action"
)

// Destroy uninstalls a helm chart. Note that it does not remove the
// kubernetes resources.
func Destroy(context *config.Context, installName, overrideNamespace string) error {
	_, actionConfig, err := configureHelm(context, overrideNamespace)
	if err != nil {
		return err
	}

	uninstall := action.NewUninstall(actionConfig)
	_, err = uninstall.Run(installName)
	if err != nil {
		return errors.Wrapf(err, "failed to uninstall helm package")
	}

	return nil
}
