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
		`go.mod$`,
		`go.sum$`,
	}

	Exclude = []string{
		`\.git$`,
		`go\.sum$`, // causes checksum mismatch
	}
)

const (
	ListSeparator     = ","
	FilePathSeparator = string(filepath.Separator)
)
