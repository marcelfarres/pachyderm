package testutil

import (
	"context"
	"os"
	"testing"

	"github.com/pachyderm/pachyderm/v2/src/client"
	"github.com/pachyderm/pachyderm/v2/src/enterprise"
	"github.com/pachyderm/pachyderm/v2/src/internal/backoff"
)

// GetTestEnterpriseCode Pulls the enterprise code out of the env var stored in travis
func GetTestEnterpriseCode(t testing.TB) string {
	acode, exists := os.LookupEnv("ENT_ACT_CODE")
	if !exists {
		t.Error("Enterprise Activation code not found in Env Vars")
	}
	return acode

}

// ActivateEnterprise activates enterprise in Pachyderm (if it's not on already.)
func ActivateEnterprise(t testing.TB, c *client.APIClient) error {
	code := GetTestEnterpriseCode(t)

	return backoff.Retry(func() error {
		resp, err := c.Enterprise.GetState(context.Background(),
			&enterprise.GetStateRequest{})
		if err != nil {
			return err
		}
		if resp.State == enterprise.State_ACTIVE {
			return nil
		}
		_, err = c.Enterprise.Activate(context.Background(),
			&enterprise.ActivateRequest{
				ActivationCode: code,
			})
		return err
	}, backoff.NewTestingBackOff())
}
