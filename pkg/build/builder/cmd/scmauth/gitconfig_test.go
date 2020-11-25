package scmauth

import (
	"os"
	"testing"
)

func TestGitConfigHandles(t *testing.T) {
	caCert := &GitConfig{}
	if !caCert.Handles(".gitconfig") {
		t.Errorf("should handle .gitconfig")
	}
	if caCert.Handles("username") {
		t.Errorf("should not handle username")
	}
	if caCert.Handles("gitconfig") {
		t.Errorf("should not handle gitconfig")
	}
}

func TestGitConfigSetup(t *testing.T) {
	context := NewDefaultSCMContext()
	gitConfig := &GitConfig{}
	secretDir := secretDir(t, ".gitconfig")
	defer os.RemoveAll(secretDir)

	configFile, err := gitConfig.Setup(secretDir, context)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	envConfig, _ := context.Get("GIT_CONFIG")
	if configFile != envConfig {
		t.Errorf("expected .gitconfig from Setup %s to match GIT_CONFIG value %s", configFile, envConfig)
	}
	defer cleanupConfig(envConfig)
	validateConfig(t, envConfig, "test")
}
