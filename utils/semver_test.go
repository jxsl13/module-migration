package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSemver(t *testing.T) {
	lines := []string{
		"v0.1.0",
		"v0.1.1",
		"v0.3.1",
		"v0.2.0",
		"v0.2.1",
		"v0.2.2",
		"v0.3.0",
	}

	vs := toSortedSemverList(lines)
	require.NotEmpty(t, vs)

	latest := *vs[len(vs)-1]

	newVersion := latest.IncPatch()
	require.Equal(t, "v0.3.2", newVersion.Original())
}
