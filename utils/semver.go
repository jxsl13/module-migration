package utils

import "github.com/Masterminds/semver/v3"

func NewVersion(v string) (version semver.Version, err error) {
	vp, err := semver.NewVersion(v)
	if err != nil {
		return version, err
	}

	return *vp, nil
}

func NewUpdatedVersion(versionStr string, major, minor, patch bool) (semver.Version, error) {
	vp, err := semver.NewVersion(versionStr)
	if err != nil {
		return semver.Version{}, err
	}
	v := *vp
	if major {
		v = v.IncMajor()
	}
	if minor {
		v = v.IncMinor()
	}
	if patch {
		v = v.IncPatch()
	}

	return v, nil
}
