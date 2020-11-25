package scmauth

import (
	"path/filepath"
)

const GitConfigName = ".gitconfig"

// GitConfig implements SCMAuth interface for using a custom .gitconfig file
type GitConfig struct{}

// Setup adds the secret .gitconfig as an include to the .gitconfig file to be used in the build.
// Returns the path to the generated .gitconfig and error if raised
func (GitConfig) Setup(baseDir string, context SCMAuthContext) (string, error) {
	log.V(4).Infof("Adding user-provided gitconfig %s to build gitconfig", filepath.Join(baseDir, GitConfigName))
	return ensureGitConfigIncludes(filepath.Join(baseDir, GitConfigName), context)
}

// Name returns the name of this auth method.
func (GitConfig) Name() string {
	return GitConfigName
}

// Handles returns true if the secret file is a gitconfig
func (GitConfig) Handles(name string) bool {
	return name == GitConfigName
}
