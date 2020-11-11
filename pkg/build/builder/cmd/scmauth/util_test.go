package scmauth

import (
	buildv1 "github.com/openshift/api/build/v1"
	"os"
	"testing"
)

func TestEnsureGitConfigIncludesProxyConfig_BothSet(t *testing.T) {
	context := NewDefaultSCMContext()
	httpProxy := "http://proxy.io"
	httpsProxy := "https://proxy.io"
	gitSource := &buildv1.GitBuildSource{
		ProxyConfig: buildv1.ProxyConfig{
			HTTPSProxy: &httpsProxy,
			HTTPProxy:  &httpProxy,
		},
	}
	err := EnsureGitConfigIncludes("", context, gitSource)
	if err != nil {
		t.Fatalf(err.Error())
	}
	tempDir, exists := context.Get("HOME")
	if !exists {
		t.Fatalf("temp dir not registered in context: %#v", context)
	}
	defer os.RemoveAll(tempDir)
	gitConfigFile, exists := context.Get("GIT_CONFIG")
	if !exists {
		t.Fatalf("git config file not registered in context: %#v", context)
	}
	validateConfigContent(t, gitConfigFile, "[http]")
	validateConfigContent(t, gitConfigFile, "[https]")
	validateConfigContent(t, gitConfigFile, "proxy = http://proxy.io")
	validateConfigContent(t, gitConfigFile, "proxy = https://proxy.io")
}

func TestEnsureGitConfigIncludesProxyConfig_BothSetAndSourceSecret(t *testing.T) {
	context := NewDefaultSCMContext()
	httpProxy := "http://proxy.io"
	httpsProxy := "https://proxy.io"
	gitSource := &buildv1.GitBuildSource{
		ProxyConfig: buildv1.ProxyConfig{
			HTTPSProxy: &httpsProxy,
			HTTPProxy:  &httpProxy,
		},
	}
	err := EnsureGitConfigIncludes("secret.file.txt", context, gitSource)
	if err != nil {
		t.Fatalf(err.Error())
	}
	tempDir, exists := context.Get("HOME")
	if !exists {
		t.Fatalf("temp dir not registered in context: %#v", context)
	}
	defer os.RemoveAll(tempDir)
	gitConfigFile, exists := context.Get("GIT_CONFIG")
	if !exists {
		t.Fatalf("git config file not registered in context: %#v", context)
	}
	validateConfigContent(t, gitConfigFile, "[http]")
	validateConfigContent(t, gitConfigFile, "[https]")
	validateConfigContent(t, gitConfigFile, "proxy = http://proxy.io")
	validateConfigContent(t, gitConfigFile, "proxy = https://proxy.io")
	validateConfigContent(t, gitConfigFile, "secret.file.txt")
}

func TestEnsureGitConfigIncludesProxyConfig_HTTPSet(t *testing.T) {
	context := NewDefaultSCMContext()
	httpProxy := "http://proxy.io"
	gitSource := &buildv1.GitBuildSource{
		ProxyConfig: buildv1.ProxyConfig{
			HTTPProxy: &httpProxy,
		},
	}
	err := EnsureGitConfigIncludes("", context, gitSource)
	if err != nil {
		t.Fatalf(err.Error())
	}
	tempDir, exists := context.Get("HOME")
	if !exists {
		t.Fatalf("temp dir not registered in context: %#v", context)
	}
	defer os.RemoveAll(tempDir)
	gitConfigFile, exists := context.Get("GIT_CONFIG")
	if !exists {
		t.Fatalf("git config file not registered in context: %#v", context)
	}
	validateConfigContent(t, gitConfigFile, "[http]")
	validateConfigContent(t, gitConfigFile, "proxy = http://proxy.io")
	validateConfigContentDoesNotExist(t, gitConfigFile, "https")
}

func TestEnsureGitConfigIncludesProxyConfig_HTTPSSet(t *testing.T) {
	context := NewDefaultSCMContext()
	httpsProxy := "https://proxy.io"
	gitSource := &buildv1.GitBuildSource{
		ProxyConfig: buildv1.ProxyConfig{
			HTTPSProxy: &httpsProxy,
		},
	}
	err := EnsureGitConfigIncludes("", context, gitSource)
	if err != nil {
		t.Fatalf(err.Error())
	}
	tempDir, exists := context.Get("HOME")
	if !exists {
		t.Fatalf("temp dir not registered in context: %#v", context)
	}
	defer os.RemoveAll(tempDir)
	gitConfigFile, exists := context.Get("GIT_CONFIG")
	if !exists {
		t.Fatalf("git config file not registered in context: %#v", context)
	}
	validateConfigContent(t, gitConfigFile, "[https]")
	validateConfigContent(t, gitConfigFile, "proxy = https://proxy.io")
	validateConfigContentDoesNotExist(t, gitConfigFile, "[http]")
}

func TestEnsureGitConfigIncludesProxyConfig_NonSet(t *testing.T) {
	context := NewDefaultSCMContext()
	gitSource := &buildv1.GitBuildSource{}
	err := EnsureGitConfigIncludes("", context, gitSource)
	if err != nil {
		t.Fatalf(err.Error())
	}
	_, exists := context.Get("HOME")
	if exists {
		t.Fatalf("home registered in context: %#v", context)
	}
	_, exists = context.Get("GIT_CONFIG")
	if exists {
		t.Fatalf("git config file registered in context: %#v", context)
	}
}
