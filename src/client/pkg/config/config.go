package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/satori/go.uuid"
)

const configEnvVar = "PACH_CONFIG"

var defaultConfigDir = filepath.Join(os.Getenv("HOME"), ".pachyderm")
var defaultConfigPath = filepath.Join(defaultConfigDir, "config.json")

func configPath() string {
	if env, ok := os.LookupEnv(configEnvVar); ok {
		return env
	}
	return defaultConfigPath
}

// ActiveContext gets the active context in the config
func (c *Config) ActiveContext(initialize bool) *Context {
	if c.V2.ActiveContext == "" {
		if initialize {
			newContext := &Context{
				Source: ContextSource_NONE,
			}
			c.V2.ActiveContext = "default"
			c.V2.Contexts["default"] = newContext
			return newContext
		}
		return nil
	}
	context, ok := c.V2.Contexts[c.V2.ActiveContext]
	if !ok && initialize {
		newContext := &Context{}
		c.V2.Contexts[c.V2.ActiveContext] = newContext
		return newContext
	}
	return context
}

// Read loads the Pachyderm config on this machine.
// If an existing configuration cannot be found, it sets up the defaults. Read
// returns a nil Config if and only if it returns a non-nil error.
func Read() (*Config, error) {
	var c *Config

	// Read json file
	p := configPath()
	if raw, err := ioutil.ReadFile(p); err == nil {
		err = json.Unmarshal(raw, &c)
		if err != nil {
			return nil, err
		}
	} else if os.IsNotExist(err) {
		// File doesn't exist, so create a new config
		fmt.Fprintf(os.Stderr, "No config detected at %q. Generating new config...\n", p)
		c = &Config{}
	} else {
		return nil, fmt.Errorf("fatal: could not read config at %q: %v", p, err)
	}
	if c.UserID == "" {
		fmt.Fprintf(os.Stderr, "No UserID present in config. Generating new UserID and "+
			"updating config at %s\n", p)
		uuid, err := uuid.NewV4()
		if err != nil {
			return nil, err
		}
		c.UserID = uuid.String()
		if err := c.Write(); err != nil {
			return nil, err
		}
	}
	if c.V2 == nil {
		c.V2 = &ConfigV2{
			ActiveContext: "",
			Contexts:      map[string]*Context{},
		}
	}
	if c.V1 != nil {
		if _, ok := c.V2.Contexts["default"]; ok {
			return nil, fmt.Errorf("Attempting to migrate to config V2, but there's already a default context")
		}
		c.V2.ActiveContext = "default"
		c.V2.Contexts["default"] = &Context{
			Source:            ContextSource_CONFIG_V1,
			PachdAddress:      c.V1.PachdAddress,
			ServerCAs:         c.V1.ServerCAs,
			SessionToken:      c.V1.SessionToken,
			ActiveTransaction: c.V1.ActiveTransaction,
		}

		c.V1 = nil
	}
	return c, nil
}

// Write writes the configuration in 'c' to this machine's Pachyderm config
// file.
func (c *Config) Write() error {
	if c.V1 != nil {
		panic("v1 config included, implying a bug")
	}

	rawConfig, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	// If we're not using a custom config path, create the default config path
	p := configPath()
	if _, ok := os.LookupEnv(configEnvVar); ok {
		// using overridden config path -- just make sure the parent dir exists
		d := filepath.Dir(p)
		if _, err := os.Stat(d); err != nil {
			return fmt.Errorf("cannot use config at %s: could not stat parent directory (%v)", p, err)
		}
	} else {
		// using the default config path, create the config directory
		err = os.MkdirAll(defaultConfigDir, 0755)
		if err != nil {
			return err
		}
	}
	return ioutil.WriteFile(p, rawConfig, 0644)
}
