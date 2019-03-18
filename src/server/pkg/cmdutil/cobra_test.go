package cmdutil

import (
	"errors"
	"testing"

	"github.com/pachyderm/pachyderm/src/client/pkg/require"
	"github.com/spf13/cobra"
)

func TestGetStringFlag(t *testing.T) {
	// Returns a zero-value for an empty flag
	emptyFlag := BatchArgs{[]string{}, map[string][]string{}}
	require.Equal(t, "", emptyFlag.GetStringFlag("foo"))

	// Returns the specified value for a flag given once
	singleFlag := BatchArgs{[]string{}, map[string][]string{"foo": {"bar"}}}
	require.Equal(t, "bar", singleFlag.GetStringFlag("foo"))

	// Returns the last value for a flag given multiple times
	multiFlag := BatchArgs{[]string{}, map[string][]string{"foo": {"bar", "baz"}}}
	require.Equal(t, "baz", multiFlag.GetStringFlag("foo"))
}

func TestPartitionBatchArgs(t *testing.T) {
	var args []string
	var actual, expected [][]string
	var err error

	cmd := &cobra.Command{Use: "cmd do stuff", Short: "do stuff", Run: func(*cobra.Command, []string){}}
	cmd.Flags().StringP("value", "v", "", "A value.")
	cmd.Flags().BoolP("bool", "b", false, "A boolean.")

	// cobra lazily creates this flag when execute is called, but this test
	// doesn't use execute
	cmd.Flags().BoolP("help", "h", false, "")

	// Partitions a simple array of positionals
	args = []string{"foo", "bar", "baz"}
	expected = [][]string{{"foo"}, {"bar"}, {"baz"}}
	actual, err = partitionBatchArgs(args, 1, cmd)
	require.NoError(t, err)
	require.Equal(t, expected, actual)

	// Partitions with value flags
	args = []string{"--value", "val", "foo", "--value", "val2", "bar", "-v", "val3"}
	expected = [][]string{{"--value", "val", "foo", "--value", "val2"}, {"bar", "-v", "val3"}}
	actual, err = partitionBatchArgs(args, 1, cmd)
	require.NoError(t, err)
	require.Equal(t, expected, actual)

	// Handles it ok if there is a trailing value flag with no value
	args = []string{"foo", "--value"}
	expected = [][]string{{"foo", "--value"}}
	actual, err = partitionBatchArgs(args, 1, cmd)
	require.NoError(t, err)
	require.Equal(t, expected, actual)

	args = []string{"foo", "-v"}
	expected = [][]string{{"foo", "-v"}}
	actual, err = partitionBatchArgs(args, 1, cmd)
	require.NoError(t, err)
	require.Equal(t, expected, actual)

	// Partitions with boolean flags
	args = []string{"--bool", "foo", "-b", "bar", "--bool"}
	expected = [][]string{{"--bool", "foo", "-b"}, {"bar", "--bool"}}
	actual, err = partitionBatchArgs(args, 1, cmd)
	require.NoError(t, err)
	require.Equal(t, expected, actual)

	// Partitions larger sets of positionals
	args = []string{"foo", "bar", "baz", "floop", "bloop", "hunter2"}
	expected = [][]string{{"foo", "bar", "baz"}, {"floop", "bloop", "hunter2"}}
	actual, err = partitionBatchArgs(args, 3, cmd)
	require.NoError(t, err)
	require.Equal(t, expected, actual)

	// Errors if the wrong number of positionals was passed in
	args = []string{"foo", "bar", "baz", "floop", "bloop"}
	_, err = partitionBatchArgs(args, 3, cmd)
	require.YesError(t, err)
	require.Matches(t, "must have 3 arguments, but found 2", err.Error())

	args = []string{"foo", "bar", "baz", "floop"}
	_, err = partitionBatchArgs(args, 3, cmd)
	require.YesError(t, err)
	require.Matches(t, "must have 3 arguments, but found 1", err.Error())

	// Errors for an unrecognized long flag
	args = []string{"foo", "bar", "--unknown"}
	_, err = partitionBatchArgs(args, 2, cmd)
	require.YesError(t, err)
	require.Matches(t, "Error: unknown flag: --unknown", err.Error())

	// Errors for an unrecognized shorthand flag
	args = []string{"foo", "bar", "-l"}
	_, err = partitionBatchArgs(args, 2, cmd)
	require.YesError(t, err)
	require.Matches(t, "Error: unknown shorthand flag: 'l' in -l", err.Error())

	// Errors for an unrecognized shorthand flag in a compound argument
	args = []string{"foo", "bar", "-bl5"}
	_, err = partitionBatchArgs(args, 2, cmd)
	require.YesError(t, err)
	require.Matches(t, "Error: unknown shorthand flag: 'l' in -bl5", err.Error())

	// Partitions with mixed flags
	args = []string{"-v=", "foo", "-bv500", "bar", "--bool", "baz", "-bbv", "val", "boz"}
	expected = [][]string{{"-v=", "foo", "-bv500", "bar", "--bool"}, {"baz", "-bbv", "val", "boz"}}
	actual, err = partitionBatchArgs(args, 2, cmd)
	require.NoError(t, err)
	require.Equal(t, expected, actual)

	// Returns an empty error on a help flag
	args = []string{"foo", "--help", "bar"}
	_, err = partitionBatchArgs(args, 2, cmd)
	require.YesError(t, err)
	require.Equal(t, 0, len(err.Error()))

	args = []string{"foo", "bar", "-h"}
	_, err = partitionBatchArgs(args, 2, cmd)
	require.YesError(t, err)
	require.Equal(t, 0, len(err.Error()))
}

// catchExecuteErrors changes a command's behavior to panic instead of `os.Exit`
// when the command fails.  This lets us test error cases without ending the
// test case, using a defer block to recover from the panic.
func catchExecuteErrors(cmd *cobra.Command) (err error) {
	original := internalErrorAndExit
	internalErrorAndExit = func(message string) { panic(message) }
	defer func () {
		internalErrorAndExit = original
	}()
	defer func () {
		if r := recover(); r != nil {
			err = errors.New(r.(string))
		}
	}()

	return cmd.Execute()
}

func TestRunBatchCommand(t *testing.T) {
	persistentBool := false
	persistentString := ""
	nonpersistent := ""
	preRunCalled := false
	postRunCalled := false
	parentCmd := &cobra.Command{
		Run: func(*cobra.Command, []string) {
			t.Fatal("parent command Run should never be called")
		},
		PersistentPreRun: func(*cobra.Command, []string) {
			preRunCalled = true
		},
		PersistentPostRunE: func(*cobra.Command, []string) error {
			postRunCalled = true
			return nil
		},
	}
	parentCmd.PersistentFlags().BoolVarP(&persistentBool, "bool", "b", false, "Parent persistent bool flag.")
	parentCmd.PersistentFlags().StringVarP(&persistentString, "string", "s", "", "Parent persistent string flag.")
	parentCmd.Flags().StringVarP(&nonpersistent, "np", "", "", "Parent local non-persistent string flag.")

	var args []BatchArgs
	cmd := &cobra.Command{
		Use: "subcmd [flags]",
		DisableFlagParsing: true,
		Run: RunBatchCommand(2, func(_args []BatchArgs) error {
			args = _args
			return nil
		}),
	}
	cmd.Flags().StringP("value", "v", "", "A value.")
	parentCmd.AddCommand(cmd)

	// This command does not disable flag parsing and will fail
	invalidCmd := &cobra.Command{
		Use: "invalid [flags]",
		Run: RunBatchCommand(2, func([]BatchArgs) error {
			t.Fatal("invalid command Run should never be called")
			return nil
		}),
	}
	parentCmd.AddCommand(invalidCmd)

	var err error
	var expected []BatchArgs
	reset := func () {
		persistentBool = false
		persistentString = ""
		preRunCalled = false
		postRunCalled = false
		args = []BatchArgs{}
	}

	parentCmd.SetArgs([]string {"subcmd", "foo", "-v5", "bar", "--bool", "--string", "str"})
	expected = []BatchArgs{
		BatchArgs{
			Positionals: []string{"foo", "bar"},
			Flags: map[string][]string{"bool": {"true"}, "string": {"str"}, "value": {"5"}},
		},
	}
	err = parentCmd.Execute()
	require.NoError(t, err)
	require.Equal(t, expected, args)
	// Should call parents' persistent hook functions
	require.Equal(t, true, preRunCalled)
	require.Equal(t, true, postRunCalled)
	// Should parse parents' persistent flags
	require.Equal(t, true, persistentBool)
	require.Equal(t, "str", persistentString)
	reset()

	// Should not recognize parents' local non-persistent flags
	parentCmd.SetArgs([]string {"subcmd", "foo", "bar", "--np", "str"})
	err = catchExecuteErrors(parentCmd)
	require.YesError(t, err)
	require.Matches(t, "Error: unknown flag: --np", err.Error())
	reset()

	// Should guarantee that batch commands disable flag parsing
	parentCmd.SetArgs([]string {"invalid", "foo"})
	err = catchExecuteErrors(parentCmd)
	require.YesError(t, err)
	require.Matches(t, "batch commands must disable flag parsing", err.Error())
	reset()

	// Should partition flags and positionals into BatchArgs
	parentCmd.SetArgs([]string {
		"subcmd",
		"foo", "bar", "--value=4", "--bool",
		"floop", "-bv6", "bloop", "--string", "str", "--string", "str2",
		"baz", "boz", "--value", "abc", "--value", "def",
	})
	expected = []BatchArgs{
		BatchArgs{
			Positionals: []string{"foo", "bar"},
			Flags: map[string][]string{"bool": {"true"}, "value": {"4"}},
		},
		BatchArgs{
			Positionals: []string{"floop", "bloop"},
			Flags: map[string][]string{"bool": {"true"}, "string": {"str", "str2"}, "value": {"6"}},
		},
		BatchArgs{
			Positionals: []string{"baz", "boz"},
			Flags: map[string][]string{"value": {"abc", "def"}},
		},
	}
	err = parentCmd.Execute()
	require.NoError(t, err)
	require.Equal(t, expected, args)
	require.Equal(t, true, persistentBool)
	require.Equal(t, "str2", persistentString)
	reset()
}