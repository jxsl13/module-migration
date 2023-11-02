package defaults

import "path/filepath"

var (
	Include = []string{
		`\.go$`,
		`Dockerfile$`,
		`Jenkinsfile$`,
		`\.yaml$`,
		`\.yml$`,
		`\.md$`,
		`\.MD$`,
	}

	Exclude = []string{
		`\.git$`,
	}
)

const (
	ListSeparator     = ","
	FilePathSeparator = string(filepath.Separator)
)
