package scmauth

import (
	"os"
	"testing"

	"github.com/openshift/source-to-image/pkg/scm/git"
)

func TestCACertHandles(t *testing.T) {
	caCert := &CACert{}
	if !caCert.Handles("ca.crt") {
		t.Errorf("should handle ca.crt")
	}
	if caCert.Handles("username") {
		t.Errorf("should not handle username")
	}
}

func TestCACertSetup(t *testing.T) {
	context := NewDefaultSCMContext()
	caCert := &CACert{
		SourceURL: *git.MustParse("https://my.host/git/repo"),
	}
	secretDir := secretDir(t, "ca.crt")
	defer os.RemoveAll(secretDir)

	configFile, err := caCert.Setup(secretDir, context)
	gitConfig, _ := context.Get("GIT_CONFIG")
	if configFile != gitConfig {
		t.Errorf("expected .gitconfig from Setup %s to match GIT_CONFIG value %s", configFile, gitConfig)
	}
	defer cleanupConfig(gitConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	validateConfig(t, gitConfig, "sslCAInfo")
}

func TestCACertSetupNoSSL(t *testing.T) {
	context := NewDefaultSCMContext()
	caCert := &CACert{
		SourceURL: *git.MustParse("http://my.host/git/repo"),
	}
	secretDir := secretDir(t, "ca.crt")
	defer os.RemoveAll(secretDir)

	configFile, err := caCert.Setup(secretDir, context)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(configFile) > 0 {
		t.Errorf("expected .gitconfig from Setup to be empty, got %s", configFile)
	}
	value, gitConfigPresent := context.Get("GIT_CONFIG")
	if gitConfigPresent {
		t.Errorf("expected GIT_CONFIG to be unset, got %s", value)
	}
}
