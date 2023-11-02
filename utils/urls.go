package utils

import (
	"fmt"
	"strings"

	giturls "github.com/whilp/git-urls"
)

func ToModuleUrl(gitUrl string) (string, error) {
	u, err := giturls.Parse(gitUrl)
	if err != nil {
		return "", fmt.Errorf("invalid git url: %s: %w", gitUrl, err)
	}
	u.Scheme = ""
	u.User = nil

	return strings.TrimSuffix(strings.TrimLeft(u.String(), "/"), ".git"), nil
}
