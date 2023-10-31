package release

import "errors"

type ReleaseConfig struct {
	RemoteName string `koanf:"remote" short:"r" description:"name of the remote url"`
	Push       bool   `koanf:"push" short:"p" description:"push tags to remote repo"`
}

func (c *ReleaseConfig) Validate() error {
	if c.RemoteName == "" {
		return errors.New("remote name is empty")
	}

	return nil
}
