package server

import (
	"testing"
	"time"

	"github.com/gogo/protobuf/types"
	"golang.org/x/net/context"

	"github.com/pachyderm/pachyderm/src/client/enterprise"
	"github.com/pachyderm/pachyderm/src/client/license"
	"github.com/pachyderm/pachyderm/src/client/pkg/require"
	tu "github.com/pachyderm/pachyderm/src/server/pkg/testutil"
)

// TestActivate tests that we can activate the license server
// by providing a valid enterprise activation code. This is exercised
// in a bunch of other tests, but in the interest of being explicit
// this test only focuses on activation.
func TestActivate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tu.DeleteAll(t)
	defer tu.DeleteAll(t)
	client := tu.GetPachClient(t)

	// Activate Enterprise
	tu.ActivateEnterprise(t, client)

	// Confirm we can get the activation code back
	resp, err := client.License.GetActivationCode(client.Ctx(), &license.GetActivationCodeRequest{})
	require.NoError(t, err)
	require.Equal(t, enterprise.State_ACTIVE, resp.State)
	require.Equal(t, tu.GetTestEnterpriseCode(t), resp.ActivationCode)
}

// TestExpired tests that the license server returns the expired state
// if the expiration of the license is in the past.
func TestExpired(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tu.DeleteAll(t)
	defer tu.DeleteAll(t)
	client := tu.GetPachClient(t)

	expires := time.Now().Add(-30 * time.Second)
	expiresProto, err := types.TimestampProto(expires)
	require.NoError(t, err)

	// Activate Enterprise with an expiration in the past
	_, err = client.License.Activate(context.Background(),
		&license.ActivateRequest{
			ActivationCode: tu.GetTestEnterpriseCode(t),
			Expires:        expiresProto,
		})
	require.NoError(t, err)

	// Confirm the license server state is expired
	resp, err := client.License.GetActivationCode(client.Ctx(), &license.GetActivationCodeRequest{})
	require.NoError(t, err)
	require.Equal(t, enterprise.State_EXPIRED, resp.State)
	require.Equal(t, tu.GetTestEnterpriseCode(t), resp.ActivationCode)
}

// TestGetActivationCodeNotAdmin tests that non-admin users cannot retrieve
// the enterprise activation code
func TestGetActivationCodeNotAdmin(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tu.DeleteAll(t)
	defer tu.DeleteAll(t)
	aliceClient := tu.GetAuthenticatedPachClient(t, "robot:alice")
	_, err := aliceClient.License.GetActivationCode(aliceClient.Ctx(), &license.GetActivationCodeRequest{})
	require.YesError(t, err)
	require.Matches(t, "not authorized", err.Error())
}

// TestDeleteAll tests that DeleteAll removes all registered clusters and
// puts the license server in the NONE state.
func TestDeleteAll(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tu.DeleteAll(t)
	defer tu.DeleteAll(t)
	client := tu.GetPachClient(t)

	// Activate Enterprise, which activates a license and adds a "localhost" cluster
	tu.ActivateEnterprise(t, client)

	// Confirm one cluster is registered
	clusters, err := client.License.ListClusters(client.Ctx(), &license.ListClustersRequest{})
	require.NoError(t, err)
	require.Equal(t, 1, len(clusters.Clusters))

	// Call DeleteAll
	_, err = client.License.DeleteAll(client.Ctx(), &license.DeleteAllRequest{})
	require.NoError(t, err)

	// No license is registered
	resp, err := client.License.GetActivationCode(client.Ctx(), &license.GetActivationCodeRequest{})
	require.NoError(t, err)
	require.Equal(t, enterprise.State_NONE, resp.State)

	// Activate Enterprise but don't register any clusters
	_, err = client.License.Activate(context.Background(),
		&license.ActivateRequest{
			ActivationCode: tu.GetTestEnterpriseCode(t),
		})
	require.NoError(t, err)

	// No clusters are registered
	clusters, err = client.License.ListClusters(client.Ctx(), &license.ListClustersRequest{})
	require.NoError(t, err)
	require.Equal(t, 0, len(clusters.Clusters))
}

func TestDeleteAllNotAdmin(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tu.DeleteAll(t)
	defer tu.DeleteAll(t)
	aliceClient := tu.GetAuthenticatedPachClient(t, "robot:alice")
	_, err := aliceClient.License.DeleteAll(aliceClient.Ctx(), &license.DeleteAllRequest{})
	require.YesError(t, err)
	require.Matches(t, "not authorized", err.Error())
}

func TestClusterCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tu.DeleteAll(t)
	defer tu.DeleteAll(t)
	client := tu.GetPachClient(t)

	// Activate enterprise, which will register the localhost cluster
	tu.ActivateEnterprise(t, client)

	clusters, err := client.License.ListClusters(client.Ctx(), &license.ListClustersRequest{})
	require.NoError(t, err)
	require.Equal(t, 1, len(clusters.Clusters))
}

func TestClusterCRUDNotAdmin(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tu.DeleteAll(t)
	defer tu.DeleteAll(t)
	aliceClient := tu.GetAuthenticatedPachClient(t, "robot:alice")

	_, err := aliceClient.License.ListClusters(aliceClient.Ctx(), &license.ListClustersRequest{})
	require.YesError(t, err)
	require.Matches(t, "not authorized", err.Error())

	_, err = aliceClient.License.DeleteCluster(aliceClient.Ctx(), &license.DeleteClusterRequest{
		Id: "localhost",
	})
	require.YesError(t, err)
	require.Matches(t, "not authorized", err.Error())

	_, err = aliceClient.License.UpdateCluster(aliceClient.Ctx(), &license.UpdateClusterRequest{
		Id:   "localhost",
		Name: "shouldntchange",
	})
	require.YesError(t, err)
	require.Matches(t, "not authorized", err.Error())
}

func TestHeartbeat(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}
}
