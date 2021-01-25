package cmds

import (
	"fmt"

	"github.com/pachyderm/pachyderm/src/client"
	"github.com/pachyderm/pachyderm/src/client/enterprise"
	"github.com/pachyderm/pachyderm/src/client/pkg/errors"
	"github.com/pachyderm/pachyderm/src/server/pkg/cmdutil"

	"github.com/gogo/protobuf/types"
	"github.com/spf13/cobra"
)

// ActivateCmd returns a cobra.Command to activate the enterprise features of
// Pachyderm within a Pachyderm cluster. All repos will go from
// publicly-accessible to accessible only by the owner, who can subsequently add
// users
func ActivateCmd() *cobra.Command {
	var licenseServer string
	activate := &cobra.Command{
		Use: "{{alias}}",
		Short: "Activate the enterprise features of Pachyderm with an activation " +
			"code",
		Long: "Activate the enterprise features of Pachyderm with an activation " +
			"code",
		Run: cmdutil.RunFixedArgs(0, func(args []string) error {
			c, err := client.NewOnUserMachine("user")
			if err != nil {
				return errors.Wrapf(err, "could not connect")
			}
			defer c.Close()
			req := &enterprise.ActivateRequest{}
			req.LicenseServer = licenseServer
			if _, err := c.Enterprise.Activate(c.Ctx(), req); err != nil {
				return err
			}
			fmt.Printf("Activation succeeded.")
			return nil
		}),
	}
	activate.PersistentFlags().StringVar(&licenseServer, "license-server", "l",
		"The address of the license server to activate with.")

	return cmdutil.CreateAlias(activate, "enterprise activate")
}

// DeactivateCmd returns a cobra.Command to disable enterprise features and
// clear the configuration of the enterprise service.
func DeactivateCmd() *cobra.Command {
	activate := &cobra.Command{
		Use:   "{{alias}}",
		Short: "Deactivate the enterprise features of Pachyderm",
		Long:  "Deactivate the enterprise features of Pachyderm",
		Run: cmdutil.RunFixedArgs(0, func(args []string) error {
			c, err := client.NewOnUserMachine("user")
			if err != nil {
				return errors.Wrapf(err, "could not connect")
			}
			defer c.Close()
			if _, err := c.Enterprise.Deactivate(c.Ctx(), &enterprise.DeactivateRequest{}); err != nil {
				return err
			}
			fmt.Printf("Enterprise license deactivated.")
			return nil
		}),
	}

	return cmdutil.CreateAlias(activate, "enterprise deactivate")
}

// GetStateCmd returns a cobra.Command to activate the enterprise features of
// Pachyderm within a Pachyderm cluster. All repos will go from
// publicly-accessible to accessible only by the owner, who can subsequently add
// users
func GetStateCmd() *cobra.Command {
	getState := &cobra.Command{
		Short: "Check whether the Pachyderm cluster has enterprise features " +
			"activated",
		Long: "Check whether the Pachyderm cluster has enterprise features " +
			"activated",
		Run: cmdutil.Run(func(args []string) error {
			c, err := client.NewOnUserMachine("user")
			if err != nil {
				return errors.Wrapf(err, "could not connect")
			}
			defer c.Close()
			resp, err := c.Enterprise.GetState(c.Ctx(), &enterprise.GetStateRequest{})
			if err != nil {
				return err
			}
			if resp.State == enterprise.State_NONE {
				fmt.Println("No Pachyderm Enterprise token was found")
				return nil
			}
			ts, err := types.TimestampFromProto(resp.Info.Expires)
			if err != nil {
				return errors.Wrapf(err, "activation request succeeded, but could not "+
					"convert token expiration time to a timestamp")
			}
			fmt.Printf("Pachyderm Enterprise token state: %s\nExpiration: %s\n",
				resp.State.String(), ts.String())
			return nil
		}),
	}
	return cmdutil.CreateAlias(getState, "enterprise get-state")
}

// Cmds returns pachctl commands related to Pachyderm Enterprise
func Cmds() []*cobra.Command {
	var commands []*cobra.Command

	enterprise := &cobra.Command{
		Short: "Enterprise commands enable Pachyderm Enterprise features",
		Long:  "Enterprise commands enable Pachyderm Enterprise features",
	}
	commands = append(commands, cmdutil.CreateAlias(enterprise, "enterprise"))

	commands = append(commands, ActivateCmd())
	commands = append(commands, DeactivateCmd())
	commands = append(commands, GetStateCmd())

	return commands
}
