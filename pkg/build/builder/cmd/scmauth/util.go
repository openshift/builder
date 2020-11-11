package scmauth

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	buildv1 "github.com/openshift/api/build/v1"
	"github.com/openshift/builder/pkg/build/builder"
	utillog "github.com/openshift/builder/pkg/build/builder/util/log"
)

var log = utillog.ToFile(os.Stderr, 2)

func createGitConfig(includePath string, context SCMAuthContext, gitSource *buildv1.GitBuildSource) error {
	tempDir, err := ioutil.TempDir("", "git")
	if err != nil {
		return err
	}
	gitconfig := filepath.Join(tempDir, ".gitconfig")
	content := ""
	if len(includePath) > 0 {
		content = fmt.Sprintf("[include]\npath = %s\n", includePath)
	}
	if gitSource != nil {
		if gitSource.HTTPProxy != nil && len(*gitSource.HTTPProxy) > 0 {
			content = content + fmt.Sprintf("[http]\nproxy = %s\n", *gitSource.HTTPProxy)
		}
		if gitSource.HTTPSProxy != nil && len(*gitSource.HTTPSProxy) > 0 {
			content = content + fmt.Sprintf("[https]\n.proxy = %s\n", *gitSource.HTTPSProxy)
		}
	}
	if len(content) == 0 {
		return nil
	}
	if err := ioutil.WriteFile(gitconfig, []byte(content), 0600); err != nil {
		return err
	}
	// The GIT_CONFIG variable won't affect regular git operation
	// therefore the HOME variable needs to be set so git can pick up
	// .gitconfig from that location. The GIT_CONFIG variable is still used
	// to track the location of the GIT_CONFIG for multiple SCMAuth objects.
	if err := context.Set("HOME", tempDir); err != nil {
		return err
	}
	if err := context.Set("GIT_CONFIG", gitconfig); err != nil {
		return err
	}
	return nil
}

// EnsureGitConfigIncludes ensures that the OS env var GIT_CONFIG is set and
// that it points to a file that has an include statement for the given path
func EnsureGitConfigIncludes(path string, context SCMAuthContext, gitSource *buildv1.GitBuildSource) error {
	gitconfig, present := context.Get("GIT_CONFIG")
	if !present {
		return createGitConfig(path, context, gitSource)
	}

	lines, err := builder.ReadLines(gitconfig)
	if err != nil {
		return err
	}
	for _, line := range lines {
		// If include already exists, return with no error
		if line == fmt.Sprintf("path = %s", path) {
			return nil
		}
	}

	lines = append(lines, fmt.Sprintf("path = %s", path))
	content := []byte(strings.Join(lines, "\n"))
	return ioutil.WriteFile(gitconfig, content, 0600)
}
