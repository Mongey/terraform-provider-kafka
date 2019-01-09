package kafka

import "testing"

func Test_versionFromString(t *testing.T) {
	version := versionFromString("1.1.0")
	if version.String() != "1.1.0" {
		t.Error(version.String())
	}

	defaultVersion := versionFromString("banana")
	if defaultVersion.String() != "1.0.0" {
		t.Errorf("Default version has changed %s", defaultVersion)
	}
}
