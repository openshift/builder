package scmauth

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
)

const SSHPrivateKeyMethodName = "ssh-privatekey"
const knownHostsFileName = "known_hosts"

// SSHPrivateKey implements SCMAuth interface for using SSH private keys.
type SSHPrivateKey struct{}

// Setup creates a wrapper script for SSH command to be able to use the provided
// SSH key while accessing private repository. Note that this does _not_ generate a .gitconfig
// file or set the GIT_CONFIG environment variable.
func (SSHPrivateKey) Setup(baseDir string, context SCMAuthContext) (string, error) {
	script, err := ioutil.TempFile("", "gitssh")
	if err != nil {
		return "", err
	}
	defer script.Close()
	if err := script.Chmod(0711); err != nil {
		return "", err
	}
	foundPrivateKey := false
	foundKnownHosts := false
	files, err := ioutil.ReadDir(baseDir)
	if err != nil {
		return "", err
	}
	for _, file := range files {
		switch {
		case file.Name() == knownHostsFileName:
			foundKnownHosts = true
		case file.Name() == SSHPrivateKeyMethodName:
			foundPrivateKey = true
		}
		log.V(5).Infof("source secret dir %s has file %s", baseDir, file.Name())
	}
	if !foundPrivateKey {
		return "", fmt.Errorf("could not find the ssh-privatekey file for the ssh secret stored at %s", baseDir)
	}
	// let's see if known_hosts was included in the secret
	content := "#!/bin/sh\nssh -i " + filepath.Join(baseDir, SSHPrivateKeyMethodName)
	if !foundKnownHosts {
		content = content + " -o StrictHostKeyChecking=false \"$@\"\n"
	} else {
		content = content + " -o UserKnownHostsFile=" + filepath.Join(baseDir, knownHostsFileName) + " \"$@\"\n"
	}
	log.V(5).Infof("Adding Private SSH Auth:\n%s\n", content)

	if _, err := script.WriteString(content); err != nil {
		return "", err
	}
	// set environment variable to tell git to use the SSH wrapper
	if err := context.Set("GIT_SSH", script.Name()); err != nil {
		return "", err
	}
	return "", nil
}

// Name returns the name of this auth method.
func (SSHPrivateKey) Name() string {
	return SSHPrivateKeyMethodName
}

// Handles returns true if the file is an SSH private key
func (SSHPrivateKey) Handles(name string) bool {
	return name == SSHPrivateKeyMethodName
}
