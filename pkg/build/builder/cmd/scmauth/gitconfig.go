package scmauth

import (
	buildv1 "github.com/openshift/api/build/v1"
	"path/filepath"
)

const GitConfigName = ".gitconfig"

// GitConfig implements SCMAuth interface for using a custom .gitconfig file
type GitConfig struct{}

// Setup adds the secret .gitconfig as an include to the .gitconfig file to be used in the build
func (_ GitConfig) Setup(baseDir string, context SCMAuthContext, gitSource *buildv1.GitBuildSource) error {
	log.V(4).Infof("Adding user-provided gitconfig %s to build gitconfig", filepath.Join(baseDir, GitConfigName))
	return EnsureGitConfigIncludes(filepath.Join(baseDir, GitConfigName), context, gitSource)
}

// Name returns the name of this auth method.
func (_ GitConfig) Name() string {
	return GitConfigName
}

// Handles returns true if the secret file is a gitconfig
func (_ GitConfig) Handles(name string) bool {
	return name == GitConfigName
}
