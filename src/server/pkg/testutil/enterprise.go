package testutil

import (
	"context"
	"os"
	"testing"

	"github.com/pachyderm/pachyderm/src/client"
	"github.com/pachyderm/pachyderm/src/client/enterprise"
	"github.com/pachyderm/pachyderm/src/client/license"
	"github.com/pachyderm/pachyderm/src/server/pkg/backoff"
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

		if _, err := c.License.Activate(context.Background(),
			&license.ActivateRequest{
				ActivationCode: code,
			}); err != nil {
			return err
		}

		client, err := c.License.AddCluster(context.Background(),
			&license.AddClusterRequest{
				Id:      "localhost",
				Secret:  "localhost",
				Address: "grpc://localhost:650",
			})
		if err != nil {
			return err
		}

		_, err = c.Enterprise.Activate(context.Background(),
			&enterprise.ActivateRequest{
				Id:            "localhost",
				Secret:        client.Secret,
				LicenseServer: "grpc://localhost:650",
			})
		return err
	}, backoff.NewTestingBackOff())
}
