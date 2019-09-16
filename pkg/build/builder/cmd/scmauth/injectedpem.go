package scmauth

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	s2igit "github.com/openshift/source-to-image/pkg/scm/git"
)

const (
	PEMName = "tls-ca-bundle.pem"
)

// InjectedPem implements SCMAuth interface for pem files injected by global CA support
type InjectedPem struct {
	SourceURL s2igit.URL
}

// Setup creates a .gitconfig fragment that points to the given ca.crt
func (s InjectedPem) Setup(baseDir string, context SCMAuthContext) error {
	if !(s.SourceURL.Type == s2igit.URLTypeURL && s.SourceURL.URL.Scheme == "https" && s.SourceURL.URL.Opaque == "") {
		return nil
	}
	gitconfig, err := ioutil.TempFile("", "ca.pem.")
	if err != nil {
		return err
	}
	defer gitconfig.Close()
	content := fmt.Sprintf(CACertConfig, filepath.Join(baseDir, PEMName))
	log.V(5).Infof("Adding CACert Auth to %s:\n%s\n", gitconfig.Name(), content)
	gitconfig.WriteString(content)

	return ensureGitConfigIncludes(gitconfig.Name(), context)
}

// Name returns the name of this auth method.
func (_ InjectedPem) Name() string {
	return PEMName
}

// Handles returns true if the secret is a CA certificate
func (_ InjectedPem) Handles(name string) bool {
	return name == PEMName
}
