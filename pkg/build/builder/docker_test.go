package builder

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MakeNowJust/heredoc"
	docker "github.com/fsouza/go-dockerclient"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildapiv1 "github.com/openshift/api/build/v1"
	buildfake "github.com/openshift/client-go/build/clientset/versioned/fake"
	"github.com/openshift/imagebuilder"
	"github.com/openshift/library-go/pkg/git"

	"github.com/openshift/builder/pkg/build/builder/util/dockerfile"
)

func TestInsertEnvAfterFrom(t *testing.T) {
	tests := map[string]struct {
		original string
		env      []corev1.EnvVar
		want     string
	}{
		"no FROM instruction": {
			original: `RUN echo "invalid Dockerfile"
`,
			env: []corev1.EnvVar{
				{Name: "PATH", Value: "/bin"},
			},
			want: `RUN echo "invalid Dockerfile"
`},
		"empty env": {
			original: `FROM busybox
`,
			env: []corev1.EnvVar{},
			want: `FROM busybox
`},
		"single FROM instruction": {
			original: `FROM busybox
RUN echo "hello world"
`,
			env: []corev1.EnvVar{
				{Name: "PATH", Value: "/bin"},
			},
			want: `FROM busybox
ENV "PATH"="/bin"
RUN echo "hello world"
`},
		"multiple FROM instructions": {
			original: `FROM scratch
FROM busybox
RUN echo "hello world"
`,
			env: []corev1.EnvVar{
				{Name: "PATH", Value: "/bin"},
				{Name: "GOPATH", Value: "/go"},
				{Name: "PATH", Value: "/go/bin:$PATH"},
			},
			want: `FROM scratch
ENV "PATH"="/bin" "GOPATH"="/go" "PATH"="/go/bin:$PATH"
FROM busybox
ENV "PATH"="/bin" "GOPATH"="/go" "PATH"="/go/bin:$PATH"
RUN echo "hello world"`},
	}
	for name, test := range tests {
		got, err := dockerfile.Parse(strings.NewReader(test.original))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		want, err := dockerfile.Parse(strings.NewReader(test.want))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		insertEnvAfterFrom(got, test.env)
		if !bytes.Equal(dockerfile.Write(got), dockerfile.Write(want)) {
			t.Errorf("%s: insertEnvAfterFrom(node, %+v) = %+v; want %+v", name, test.env, got, want)
			t.Logf("resulting Dockerfile:\n%s", dockerfile.Write(got))
		}
	}
}

func TestReplaceLastFrom(t *testing.T) {
	tests := []struct {
		original string
		image    string
		want     string
	}{
		{
			original: `# no FROM instruction`,
			image:    "centos",
			want:     ``,
		},
		{
			original: `FROM scratch
# FROM busybox
RUN echo "hello world"
`,
			image: "centos",
			want: `FROM centos
RUN echo "hello world"
`,
		},
		{
			original: `FROM scratch
FROM busybox
RUN echo "hello world"
`,
			image: "centos",
			want: `FROM scratch
FROM centos
RUN echo "hello world"
`,
		},
	}
	for i, test := range tests {
		got, err := dockerfile.Parse(strings.NewReader(test.original))
		if err != nil {
			t.Errorf("test[%d]: %v", i, err)
			continue
		}
		want, err := dockerfile.Parse(strings.NewReader(test.want))
		if err != nil {
			t.Errorf("test[%d]: %v", i, err)
			continue
		}
		replaceLastFrom(got, test.image, "")
		if !bytes.Equal(dockerfile.Write(got), dockerfile.Write(want)) {
			t.Errorf("test[%d]: replaceLastFrom(node, %+v) = %+v; want %+v", i, test.image, got, want)
			t.Logf("resulting Dockerfile:\n%s", dockerfile.Write(got))
		}
	}
}

func TestAppendPostCommit(t *testing.T) {
	type want struct {
		Err bool
		Out string
	}
	tests := []struct {
		description string
		original    string
		postCommit  buildapiv1.BuildPostCommitSpec
		from        *corev1.ObjectReference
		build       []buildapiv1.ImageSource
		want        want
	}{
		{
			description: "basic multi-part bash command",
			original: heredoc.Doc(`
				FROM busybox
				RUN echo "hello world"
				`),
			postCommit: buildapiv1.BuildPostCommitSpec{
				Command: []string{"echo", "hello"},
			},
			want: want{
				Out: heredoc.Doc(`
				FROM busybox as <alias>
				RUN echo "hello world"
				FROM <alias>
				RUN echo hello
				FROM <alias>
				`),
			},
		},
		{
			description: "basic bash command with args",
			original: heredoc.Doc(`
				FROM busybox
				RUN touch /tmp/hello
				`),
			postCommit: buildapiv1.BuildPostCommitSpec{
				Command: []string{"ls"},
				Args:    []string{"-l", "/tmp/hello"}},
			want: want{
				Out: heredoc.Doc(`
				FROM busybox as <alias>
				RUN touch /tmp/hello
				FROM <alias>
				RUN ls -l /tmp/hello
				FROM <alias>
				`),
			},
		},
		{
			description: "basic bash script",
			original: heredoc.Doc(`
				FROM busybox
				RUN echo "hello world"
				`),
			postCommit: buildapiv1.BuildPostCommitSpec{
				Script: "echo hello $1 world",
			},
			want: want{
				Out: heredoc.Doc(`
				FROM busybox as <alias>
				RUN echo "hello world"
				FROM <alias>
				RUN /bin/sh -ic 'echo hello $1 world'
				FROM <alias>
				`),
			},
		},
		{
			description: "basic bash script with args",
			original: heredoc.Doc(`
				FROM busybox
				RUN echo "hello world"
				`),
			postCommit: buildapiv1.BuildPostCommitSpec{
				Script: "echo",
				Args:   []string{"hello", "$1", "world"},
			},
			want: want{
				Out: heredoc.Doc(`
				FROM busybox as <alias>
				RUN echo "hello world"
				FROM <alias>
				RUN /bin/sh -ic 'echo hello $1 world'
				FROM <alias>
				`),
			},
		},
		{
			description: "multi-stage basic multi-part bash command",
			original: heredoc.Doc(`
				FROM busybox as appimage
				RUN echo "hello world"
				FROM appimage
				RUN touch /tmp/hello
				`),
			postCommit: buildapiv1.BuildPostCommitSpec{
				Command: []string{"echo", "hello", "$1", "world"},
			},
			want: want{
				Out: heredoc.Doc(`
				FROM busybox as appimage
				RUN echo "hello world"
				FROM appimage as <alias>
				RUN touch /tmp/hello
				FROM <alias>
				RUN echo hello $1 world
				FROM <alias>
				`),
			},
		},
		{
			description: "multi-stage basic bash command with args and alias",
			original: heredoc.Doc(`
				FROM busybox as appimage
				RUN echo "hello world"
				FROM appimage as myimage
				RUN touch /tmp/hello
				`),
			postCommit: buildapiv1.BuildPostCommitSpec{
				Command: []string{"echo"},
				Args:    []string{"hello", "$1", "world"},
			},
			want: want{
				Out: heredoc.Doc(`
				FROM busybox as appimage
				RUN echo "hello world"
				FROM appimage as myimage
				RUN touch /tmp/hello
				FROM myimage
				RUN echo hello $1 world
				FROM myimage
				`),
			},
		},
		{
			description: "multi-stage basic bash script with aliases",
			original: heredoc.Doc(`
				FROM busybox as appimage
				RUN echo "hello world"
				FROM appimage as appimage2
				RUN touch /tmp/hello
				FROM appimage2 as appimage3
				RUN touch /tmp/hello2
				FROM appimage3 as appimage4
				RUN touch /tmp/hello3
				`),
			postCommit: buildapiv1.BuildPostCommitSpec{
				Script: "echo hello $1 world",
			},
			want: want{
				Out: heredoc.Doc(`
				FROM busybox as appimage
				RUN echo "hello world"
				FROM appimage as appimage2
				RUN touch /tmp/hello
				FROM appimage2 as appimage3
				RUN touch /tmp/hello2
				FROM appimage3 as appimage4
				RUN touch /tmp/hello3
				FROM appimage4
				RUN /bin/sh -ic 'echo hello $1 world'
				FROM appimage4
				`),
			},
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			node, err := imagebuilder.ParseDockerfile(strings.NewReader(test.original))
			if err != nil {
				t.Fatal(err)
			}
			if err := appendPostCommit(node, buildPostCommit(test.postCommit)); err != nil {
				t.Errorf("appendPostCommit error: %#v", err)
			}
			wantNode, err := imagebuilder.ParseDockerfile(strings.NewReader(strings.Replace(test.want.Out, "<alias>", postCommitAlias, -1)))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(dockerfile.Write(node), dockerfile.Write(wantNode)) {
				t.Errorf("FAILED!")
				t.Logf("wanted:\n%s", dockerfile.Write(wantNode))
				t.Logf("got:\n%s", dockerfile.Write(node))
			}
		})
	}
}

// TestDockerfilePath validates that we can use a Dockerfile with a custom name, and in a sub-directory
func TestDockerfilePath(t *testing.T) {
	tests := []struct {
		contextDir     string
		dockerfilePath string
		dockerStrategy *buildapiv1.DockerBuildStrategy
	}{
		// default Dockerfile path
		{
			dockerfilePath: "Dockerfile",
			dockerStrategy: &buildapiv1.DockerBuildStrategy{},
		},
		// custom Dockerfile path in the root context
		{
			dockerfilePath: "mydockerfile",
			dockerStrategy: &buildapiv1.DockerBuildStrategy{
				DockerfilePath: "mydockerfile",
			},
		},
		// custom Dockerfile path in a sub directory
		{
			dockerfilePath: "dockerfiles/mydockerfile",
			dockerStrategy: &buildapiv1.DockerBuildStrategy{
				DockerfilePath: "dockerfiles/mydockerfile",
			},
		},
		// custom Dockerfile path in a sub directory
		// with a contextDir
		{
			contextDir:     "somedir",
			dockerfilePath: "dockerfiles/mydockerfile",
			dockerStrategy: &buildapiv1.DockerBuildStrategy{
				DockerfilePath: "dockerfiles/mydockerfile",
			},
		},
	}

	from := "FROM openshift/origin-base"
	expected := []string{
		from,
		// expected env variables
		"\"OPENSHIFT_BUILD_NAME\"=\"name\"",
		"\"OPENSHIFT_BUILD_NAMESPACE\"=\"namespace\"",
		"\"OPENSHIFT_BUILD_SOURCE\"=\"http://github.com/openshift/origin.git\"",
		"\"OPENSHIFT_BUILD_COMMIT\"=\"commitid\"",
		// expected labels
		"\"io.openshift.build.commit.author\"=\"test user <test@email.com>\"",
		"\"io.openshift.build.commit.date\"=\"date\"",
		"\"io.openshift.build.commit.id\"=\"commitid\"",
		"\"io.openshift.build.commit.ref\"=\"ref\"",
		"\"io.openshift.build.commit.message\"=\"message\"",
		"\"io.openshift.build.name\"=\"name\"",
		"\"io.openshift.build.namespace\"=\"namespace\"",
	}

	for _, test := range tests {
		buildDir, err := ioutil.TempDir("", "dockerfile-path")
		if err != nil {
			t.Errorf("failed to create tmpdir: %v", err)
			continue
		}
		defer func() {
			if err := os.RemoveAll(buildDir); err != nil {
				t.Fatal(err)
			}
		}()

		absoluteDockerfilePath := filepath.Join(buildDir, test.contextDir, test.dockerfilePath)
		if err = os.MkdirAll(filepath.Dir(absoluteDockerfilePath), os.FileMode(0750)); err != nil {
			t.Errorf("failed to create directory %s: %v", filepath.Dir(absoluteDockerfilePath), err)
			continue
		}
		if err = ioutil.WriteFile(absoluteDockerfilePath, []byte(from), os.FileMode(0644)); err != nil {
			t.Errorf("failed to write dockerfile to %s: %v", absoluteDockerfilePath, err)
			continue
		}

		build := &buildapiv1.Build{
			Spec: buildapiv1.BuildSpec{
				CommonSpec: buildapiv1.CommonSpec{
					Source: buildapiv1.BuildSource{
						Git: &buildapiv1.GitBuildSource{
							URI: "http://github.com/openshift/origin.git",
						},
						ContextDir: test.contextDir,
					},
					Strategy: buildapiv1.BuildStrategy{
						DockerStrategy: test.dockerStrategy,
					},
					Output: buildapiv1.BuildOutput{
						To: &corev1.ObjectReference{
							Kind: "DockerImage",
							Name: "test/test-result:latest",
						},
					},
				},
			},
		}
		build.Name = "name"
		build.Namespace = "namespace"

		sourceInfo := &git.SourceInfo{}
		sourceInfo.AuthorName = "test user"
		sourceInfo.AuthorEmail = "test@email.com"
		sourceInfo.Date = "date"
		sourceInfo.CommitID = "commitid"
		sourceInfo.Ref = "ref"
		sourceInfo.Message = "message"
		dockerClient := &FakeDocker{
			buildImageFunc: func(opts docker.BuildImageOptions) error {
				if opts.Dockerfile != test.dockerfilePath {
					t.Errorf("Unexpected dockerfile path: %s (expected: %s)", opts.Dockerfile, test.dockerfilePath)
				}
				return nil
			},
		}

		dockerBuilder := &DockerBuilder{
			dockerClient: dockerClient,
			build:        build,
		}

		// this will validate that the Dockerfile is readable
		// and append some labels to the Dockerfile
		if err = addBuildParameters(buildDir, build, sourceInfo); err != nil {
			t.Errorf("failed to add build parameters: %v", err)
			continue
		}

		// check that our Dockerfile has been modified
		dockerfileData, err := ioutil.ReadFile(absoluteDockerfilePath)
		if err != nil {
			t.Errorf("failed to read dockerfile %s: %v", absoluteDockerfilePath, err)
			continue
		}
		for _, value := range expected {
			if !strings.Contains(string(dockerfileData), value) {
				t.Errorf("Updated Dockerfile content does not contain expected value:\n%s\n\nUpdated content:\n%s\n", value, string(dockerfileData))

			}
		}

		// check that the docker client is called with the right Dockerfile parameter
		if err = dockerBuilder.dockerBuild(context.TODO(), buildDir, ""); err != nil {
			t.Errorf("failed to build: %v", err)
			continue
		}
		os.RemoveAll(buildDir)
	}
}

func TestEmptySource(t *testing.T) {
	build := &buildapiv1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "buildid",
			Namespace: "default",
		},
		Spec: buildapiv1.BuildSpec{
			CommonSpec: buildapiv1.CommonSpec{
				Source: buildapiv1.BuildSource{},
				Strategy: buildapiv1.BuildStrategy{
					DockerStrategy: &buildapiv1.DockerBuildStrategy{},
				},
				Output: buildapiv1.BuildOutput{
					To: &corev1.ObjectReference{
						Kind: "DockerImage",
						Name: "test/test-result:latest",
					},
				},
			},
		},
	}

	client := buildfake.Clientset{}

	dockerBuilder := &DockerBuilder{
		client: client.BuildV1().Builds(""),
		build:  build,
	}

	if err := dockerBuilder.Build(); err == nil {
		t.Error("Should have received error on docker build")
	} else {
		if !strings.Contains(err.Error(), "must provide a value for at least one of source, binary, images, or dockerfile") {
			t.Errorf("Did not receive correct error: %v", err)
		}
	}
}

// We should not be able to try to pull from scratch
func TestDockerfileFromScratch(t *testing.T) {
	dockerFile := `FROM scratch
USER 1001`

	dockerClient := &FakeDocker{
		buildImageFunc: func(opts docker.BuildImageOptions) error {
			return nil
		},
		pullImageFunc: func(opts docker.PullImageOptions, searchPaths []string) error {
			if opts.Repository == "scratch" && opts.Registry == "" {
				return fmt.Errorf("cannot pull scratch")
			}
			return nil
		},
	}

	build := &buildapiv1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "buildid",
			Namespace: "default",
		},
		Spec: buildapiv1.BuildSpec{
			CommonSpec: buildapiv1.CommonSpec{
				Source: buildapiv1.BuildSource{
					ContextDir: "",
					Dockerfile: &dockerFile,
				},
				Strategy: buildapiv1.BuildStrategy{
					DockerStrategy: &buildapiv1.DockerBuildStrategy{
						DockerfilePath: "",
						From: &corev1.ObjectReference{
							Kind: "DockerImage",
							Name: "scratch",
						},
					},
				},
				Output: buildapiv1.BuildOutput{
					To: &corev1.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "scratch",
					},
				},
			},
		},
	}

	client := buildfake.Clientset{}

	buildDir, err := ioutil.TempDir("", "dockerfile-path")
	if err != nil {
		t.Errorf("failed to create tmpdir: %v", err)
	}

	dockerBuilder := &DockerBuilder{
		client:       client.BuildV1().Builds(""),
		build:        build,
		dockerClient: dockerClient,
		inputDir:     buildDir,
	}
	if err := ManageDockerfile(buildDir, build); err != nil {
		t.Errorf("failed to manage the dockerfile: %v", err)
	}
	if err := dockerBuilder.Build(); err != nil {
		if strings.Contains(err.Error(), "cannot pull scratch") {
			t.Errorf("Docker build should not have attempted to pull from scratch")
		} else {
			t.Errorf("Received unexpected error: %v", err)
		}
	}
}

// TestCopyLocalObject verifies that we are able to copy mounted Kubernetes Secret or ConfigMap
// data to the build directory. The build directory is typically where git source code is cloned,
// though other sources of code may be used as well.
func TestCopyLocalObject(t *testing.T) {
	// Test scenario consists of a file tree that simulates the git clone environment
	// /tmp/source-code-xxxx
	// ├── git-data
	// │   ├── Dockerfile
	// │   └── hello.txt
	// │   └── linkSrc
	// │       └── link -> ../linkDst
	// |   └── linkDst
	// ├── secrets
	// │   └── test
	// │       └── secret.txt
	// TODO: Update the structure of the `secrets` directory so it more closely matches what
	// Kubernetes does when it mounts secrets/configmaps.
	buildDir, err := os.MkdirTemp("", "source-code")
	t.Logf("buildDir: %s", buildDir)
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(buildDir); err != nil {
			t.Fatalf("failed to clean up temp dir: %v", err)
		}
	}()
	// Set up secrets/test directory, with "secret.txt" file
	secretRoot := filepath.Join(buildDir, "secrets")
	testSecretFile := filepath.Join(secretRoot, "test", "sourceSecret.txt")
	err = fsCopyFile("testdata/fakesecret/sourceSecret.txt", testSecretFile)
	if err != nil {
		t.Fatalf("failed to copy secret data: %v", err)
	}
	if _, err := os.Stat(testSecretFile); err != nil {
		t.Fatalf("failed to stat %q: %v", testSecretFile, err)
	}
	// Set up "git-data" directory as fake "git source"
	gitRoot := filepath.Join(buildDir, "git-data")

	err = filepath.Walk("testdata/fakesource", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		dstBase := filepath.Base(path)
		return fsCopyFile(path, filepath.Join(gitRoot, dstBase))
	})
	if err != nil {
		t.Fatalf("failed to copy test data: %v", err)
	}
	// Git source may contain symlinks. This is OK as long as the symlink is relative and remains
	// within the git source tree.
	linkSrc := filepath.Join(gitRoot, "linkSrc")
	err = os.MkdirAll(linkSrc, os.ModePerm)
	if err != nil {
		t.Fatalf("failed to create test directory %q: %v", linkSrc, err)
	}
	linkDst := filepath.Join(gitRoot, "linkDst")
	err = os.MkdirAll(linkDst, os.ModePerm)
	if err != nil {
		t.Fatalf("failed to create test directory %q: %v", linkDst, err)
	}
	// Create symlink from linkSrc/link to linkDst (directory)
	err = os.Symlink("../linkDst", filepath.Join(linkSrc, "link"))
	if err != nil {
		t.Fatalf("failed to create symlink to linkDst directory: %v", err)
	}
	dockerBuilder := &DockerBuilder{
		inputDir: buildDir,
	}
	localObj := buildapiv1.SecretBuildSource{
		Secret: corev1.LocalObjectReference{
			Name: "test",
		},
		DestinationDir: "secrets",
	}
	err = dockerBuilder.copyLocalObject(secretSource(localObj), secretRoot, gitRoot)
	if err != nil {
		t.Errorf("unexpected error copying local secret: %v", err)
	}
	// sourceSecret.txt should be copied to git-data/secrets/sourceSecret.txt
	expectedFile := filepath.Join(gitRoot, "secrets", "sourceSecret.txt")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Errorf("failed to stat %q: %v", expectedFile, err)
	}
	// Try again, this time copying to a destination that happens to be a symlink to another
	// directory
	localObj.DestinationDir = filepath.Join("linkSrc", "link")
	err = dockerBuilder.copyLocalObject(secretSource(localObj), secretRoot, gitRoot)
	if err != nil {
		t.Errorf("unexpected error copying local secret: %v", err)
	}
	// sourceSecret.txt should be copied to git-data/linkSrc/link/sourceSecret.txt
	expectedFile = filepath.Join(gitRoot, "linkSrc", "link", "sourceSecret.txt")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Errorf("failed to stat %q: %v", expectedFile, err)
	}
	// sourceSecret.txt should also show up in git-data/linkDst/sourceSecret.txt
	expectedFile = filepath.Join(gitRoot, "linkDst", "sourceSecret.txt")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Errorf("failed to stat %q: %v", expectedFile, err)
	}

}

// TestCopyLocalObjectZipSlip verifies that any copied Secret or ConfigMap is not able to be placed
// outside of the target directory. A well-crafted symbolic link can otherwise allow secret data
// to be placed in a location outside of our trusted target dirctory.
func TestCopyLocalObjectZipSlip(t *testing.T) {
	// Test scenario consists of a file tree that simulates the git clone environment
	// /tmp/source-code-xxxx
	// ├── git-data
	// │   ├── Dockerfile
	// │   └── hello.txt
	// │   └── pwn-link -> /tmp/source-code-xxxx/pwned
	// ├── secrets
	// │   └── test
	// │       └── pwned.txt
	// └── pwned
	//
	// If data ends up in the "pwned" directory, we have a "zip-slip" vulnerability.
	buildDir, err := os.MkdirTemp("", "source-code")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(buildDir); err != nil {
			t.Fatalf("failed to clean up temp dir: %v", err)
		}
	}()
	// Set up secrets/test directory, with "pwned.txt" file
	secretRoot := filepath.Join(buildDir, "secrets")
	testPwnedFile := filepath.Join(secretRoot, "test", "pwned.txt")
	err = fsCopyFile("testdata/fakesecret/pwned.txt", testPwnedFile)
	if err != nil {
		t.Fatalf("failed to copy secret data: %v", err)
	}
	if _, err := os.Stat(testPwnedFile); err != nil {
		t.Fatalf("failed to stat pwned.txt: %v", err)
	}
	pwnedPath := filepath.Join(buildDir, "pwned")
	err = os.MkdirAll(pwnedPath, os.ModePerm)
	if err != nil {
		t.Fatalf("failed to create test directory %q: %v", pwnedPath, err)
	}
	// Set up "git-data" directory as fake "git source"
	gitRoot := filepath.Join(buildDir, "git-data")
	err = filepath.Walk("testdata/fakesource", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		dstBase := filepath.Base(path)
		return fsCopyFile(path, filepath.Join(gitRoot, dstBase))
	})
	if err != nil {
		t.Fatalf("failed to copy test data: %v", err)
	}
	// Add symlink to "pwned" directory in the git root
	err = os.Symlink(pwnedPath, filepath.Join(gitRoot, "pwn-link"))
	if err != nil {
		t.Fatalf("failed to create symlink to pwned directory: %v", err)
	}
	dockerBuilder := &DockerBuilder{
		inputDir: buildDir,
	}
	localObj := buildapiv1.SecretBuildSource{
		Secret: corev1.LocalObjectReference{
			Name: "test",
		},
		DestinationDir: "pwn-link",
	}
	err = dockerBuilder.copyLocalObject(secretSource(localObj), secretRoot, gitRoot)
	if err == nil {
		t.Error("expected copyLocalObject to fail if the secret tries to write its contents outside the destination directory")
	}
	if err != nil && !strings.Contains(err.Error(), "outside of the target directory") {
		t.Errorf("expected error message to contain %q, got: %v", "outside of the target directory", err)
	}
	// Can we stat the pwned.txt file?
	badPwnedFile := filepath.Join(pwnedPath, "pwned.txt")
	if _, err := os.Stat(badPwnedFile); err == nil {
		t.Errorf("secret file %q was copied outside of the trusted build directory to %q", testPwnedFile, badPwnedFile)
	}
}

func fsCopyFile(srcPath, dstPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	parentDir := filepath.Dir(dstPath)
	if err := os.MkdirAll(parentDir, os.ModePerm); err != nil {
		return err
	}
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	return nil
}
