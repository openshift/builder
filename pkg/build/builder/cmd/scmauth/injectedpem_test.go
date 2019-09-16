package scmauth

import (
	"os"
	"testing"

	"github.com/openshift/source-to-image/pkg/scm/git"
)

func TestPemHandles(t *testing.T) {
	pem := &InjectedPem{}
	if !pem.Handles("tls-ca-bundle.pem") {
		t.Errorf("should handle tls-ca-bundle.pem")
	}
	if pem.Handles("username") {
		t.Errorf("should not handle username")
	}
}

func TestPemSetup(t *testing.T) {
	context := NewDefaultSCMContext()
	pem := &InjectedPem{
		SourceURL: *git.MustParse("https://my.host/git/repo"),
	}
	secretDir := secretDir(t, "tls-ca-bundle.pem")
	defer os.RemoveAll(secretDir)

	err := pem.Setup(secretDir, context)
	gitConfig, _ := context.Get("GIT_CONFIG")
	defer cleanupConfig(gitConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	validateConfig(t, gitConfig, "sslCAInfo")
}

func TestPemSetupNoSSL(t *testing.T) {
	context := NewDefaultSCMContext()
	pem := &InjectedPem{
		SourceURL: *git.MustParse("http://my.host/git/repo"),
	}
	secretDir := secretDir(t, "tls-ca-bundle.pem")
	defer os.RemoveAll(secretDir)

	err := pem.Setup(secretDir, context)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, gitConfigPresent := context.Get("GIT_CONFIG")
	if gitConfigPresent {
		t.Fatalf("git config not expected")
	}
}
