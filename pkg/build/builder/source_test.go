package builder

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	buildapiv1 "github.com/openshift/api/build/v1"
	"github.com/openshift/library-go/pkg/git"

	"github.com/openshift/builder/pkg/build/builder/timing"
)

func TestCheckRemoteGit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	gitRepo := git.NewRepositoryWithEnv([]string{"GIT_ASKPASS=true", fmt.Sprintf("HOME=%s", os.TempDir())})

	var err error
	err = checkRemoteGit(gitRepo, server.URL, 10*time.Second)
	switch v := err.(type) {
	case gitAuthError:
	default:
		t.Errorf("expected gitAuthError, got %q", v)
	}

	err = checkRemoteGit(gitRepo, "https://github.com/openshift/origin", 10*time.Second)
	if err != nil {
		t.Errorf("unexpected error %q", err)
	}
}

type testGitRepo struct {
	Name      string
	Path      string
	Files     []string
	Submodule *testGitRepo
}

func initializeTestGitRepo(name string) (*testGitRepo, error) {
	repo := &testGitRepo{Name: name}
	dir, err := ioutil.TempDir("", "test-"+repo.Name)
	if err != nil {
		return repo, err
	}
	repo.Path = dir
	tmpfn := filepath.Join(dir, "initial-file")
	if err := ioutil.WriteFile(tmpfn, []byte("test"), 0666); err != nil {
		return repo, fmt.Errorf("unable to create temporary file")
	}
	repo.Files = append(repo.Files, tmpfn)
	initCmd := exec.Command("git", "init")
	initCmd.Dir = dir
	if out, err := initCmd.CombinedOutput(); err != nil {
		return repo, fmt.Errorf("unable to initialize repository: %q", out)
	}

	configEmailCmd := exec.Command("git", "config", "user.email", "me@example.com")
	configEmailCmd.Dir = dir
	if out, err := configEmailCmd.CombinedOutput(); err != nil {
		return repo, fmt.Errorf("unable to set git email prefs: %q", out)
	}
	configNameCmd := exec.Command("git", "config", "user.name", "Me Myself")
	configNameCmd.Dir = dir
	if out, err := configNameCmd.CombinedOutput(); err != nil {
		return repo, fmt.Errorf("unable to set git name prefs: %q", out)
	}

	return repo, nil
}

func (r *testGitRepo) addSubmodule() error {
	subRepo, err := initializeTestGitRepo("submodule")
	if err != nil {
		return err
	}
	if err := subRepo.addCommit(); err != nil {
		return err
	}

	relPathSubRepo, err := filepath.Rel(r.Path, subRepo.Path)
	if err != nil {
		return err
	}

	subCmd := exec.Command("git", "submodule", "add", relPathSubRepo, "sub")
	subCmd.Dir = r.Path
	if out, err := subCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("unable to add submodule: %q", out)
	}
	r.Submodule = subRepo
	return nil
}

// getRef returns the sha256 of the commit specified by the negative offset.
// The '0' is the current HEAD.
func (r *testGitRepo) getRef(offset int) (string, error) {
	q := ""
	for i := offset; i != 0; i++ {
		q += "^"
	}
	refCmd := exec.Command("git", "rev-parse", "HEAD"+q)
	refCmd.Dir = r.Path
	if out, err := refCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("unable to checkout %d offset: %q", offset, out)
	} else {
		return strings.TrimSpace(string(out)), nil
	}
}

func (r *testGitRepo) createBranch(name string) error {
	refCmd := exec.Command("git", "checkout", "-b", name)
	refCmd.Dir = r.Path
	if out, err := refCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("unable to checkout new branch: %q", out)
	}
	return nil
}

func (r *testGitRepo) switchBranch(name string) error {
	refCmd := exec.Command("git", "checkout", name)
	refCmd.Dir = r.Path
	if out, err := refCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("unable to checkout branch: %q", out)
	}
	return nil
}

func (r *testGitRepo) cleanup() {
	os.RemoveAll(r.Path)
	if r.Submodule != nil {
		os.RemoveAll(r.Submodule.Path)
	}
}

func (r *testGitRepo) addCommit() error {
	f, err := ioutil.TempFile(r.Path, "")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(f.Name(), []byte("test"), 0666); err != nil {
		return fmt.Errorf("unable to create temporary file %q", f.Name())
	}
	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = r.Path
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("unable to add files to repo: %q", out)
	}
	commitCmd := exec.Command("git", "commit", "-a", "-m", "test commit")
	commitCmd.Dir = r.Path
	out, err := commitCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to commit: %q", out)
	}
	r.Files = append(r.Files, f.Name())
	return nil
}

func TestUnqualifiedClone(t *testing.T) {
	repo, err := initializeTestGitRepo("unqualified")
	defer repo.cleanup()
	if err != nil {
		t.Errorf("%v", err)
	}
	if err := repo.addSubmodule(); err != nil {
		t.Errorf("%v", err)
	}
	// add two commits to check that shallow clone take account
	if err := repo.addCommit(); err != nil {
		t.Errorf("unable to add commit: %v", err)
	}
	if err := repo.addCommit(); err != nil {
		t.Errorf("unable to add commit: %v", err)
	}
	destDir, err := ioutil.TempDir("", "clone-dest-")
	defer os.RemoveAll(destDir)
	client := git.NewRepositoryWithEnv([]string{})
	source := &buildapiv1.GitBuildSource{URI: "file://" + repo.Path}
	revision := buildapiv1.SourceRevision{Git: &buildapiv1.GitSourceRevision{}}
	ctx := timing.NewContext(context.Background())
	if _, err = extractGitSource(ctx, client, source, &revision, destDir, 10*time.Second); err != nil {
		t.Errorf("%v", err)
	}
	for _, f := range repo.Files {
		if _, err := os.Stat(filepath.Join(destDir, path.Base(f))); os.IsNotExist(err) {
			t.Errorf("unable to find repository file %q", path.Base(f))
		}
	}
	if _, err := os.Stat(filepath.Join(destDir, "sub")); os.IsNotExist(err) {
		t.Errorf("unable to find submodule dir")
	}
	for _, f := range repo.Submodule.Files {
		if _, err := os.Stat(filepath.Join(destDir, "sub/"+path.Base(f))); os.IsNotExist(err) {
			t.Errorf("unable to find submodule repository file %q", path.Base(f))
		}
	}
}

func TestCloneFromRef(t *testing.T) {
	repo, err := initializeTestGitRepo("commit")
	defer repo.cleanup()
	if err != nil {
		t.Errorf("%v", err)
	}
	if err := repo.addSubmodule(); err != nil {
		t.Errorf("%v", err)
	}
	// add two commits to check that shallow clone take account
	if err := repo.addCommit(); err != nil {
		t.Errorf("unable to add commit: %v", err)
	}
	if err := repo.addCommit(); err != nil {
		t.Errorf("unable to add commit: %v", err)
	}
	destDir, err := ioutil.TempDir("", "commit-dest-")
	defer os.RemoveAll(destDir)
	client := git.NewRepositoryWithEnv([]string{})
	firstCommitRef, err := repo.getRef(-1)
	if err != nil {
		t.Errorf("%v", err)
	}
	source := &buildapiv1.GitBuildSource{
		URI: "file://" + repo.Path,
		Ref: firstCommitRef,
	}
	revision := buildapiv1.SourceRevision{Git: &buildapiv1.GitSourceRevision{}}
	ctx := timing.NewContext(context.Background())
	if _, err = extractGitSource(ctx, client, source, &revision, destDir, 10*time.Second); err != nil {
		t.Errorf("%v", err)
	}
	for _, f := range repo.Files[:len(repo.Files)-1] {
		if _, err := os.Stat(filepath.Join(destDir, path.Base(f))); os.IsNotExist(err) {
			t.Errorf("unable to find repository file %q", path.Base(f))
		}
	}
	if _, err := os.Stat(filepath.Join(destDir, path.Base(repo.Files[len(repo.Files)-1]))); !os.IsNotExist(err) {
		t.Errorf("last file should not exists in this checkout")
	}
	if _, err := os.Stat(filepath.Join(destDir, "sub")); os.IsNotExist(err) {
		t.Errorf("unable to find submodule dir")
	}
	for _, f := range repo.Submodule.Files {
		if _, err := os.Stat(filepath.Join(destDir, "sub/"+path.Base(f))); os.IsNotExist(err) {
			t.Errorf("unable to find submodule repository file %q", path.Base(f))
		}
	}
}

func TestCloneFromBranch(t *testing.T) {
	repo, err := initializeTestGitRepo("branch")
	defer repo.cleanup()
	if err != nil {
		t.Errorf("%v", err)
	}
	if err := repo.addSubmodule(); err != nil {
		t.Errorf("%v", err)
	}
	// add two commits to check that shallow clone take account
	if err := repo.addCommit(); err != nil {
		t.Errorf("unable to add commit: %v", err)
	}
	if err := repo.createBranch("test"); err != nil {
		t.Errorf("%v", err)
	}
	if err := repo.addCommit(); err != nil {
		t.Errorf("unable to add commit: %v", err)
	}
	if err := repo.switchBranch("master"); err != nil {
		t.Errorf("%v", err)
	}
	if err := repo.addCommit(); err != nil {
		t.Errorf("unable to add commit: %v", err)
	}
	destDir, err := ioutil.TempDir("", "branch-dest-")
	defer os.RemoveAll(destDir)
	client := git.NewRepositoryWithEnv([]string{})
	source := &buildapiv1.GitBuildSource{
		URI: "file://" + repo.Path,
		Ref: "test",
	}
	revision := buildapiv1.SourceRevision{Git: &buildapiv1.GitSourceRevision{}}
	ctx := timing.NewContext(context.Background())
	if _, err = extractGitSource(ctx, client, source, &revision, destDir, 10*time.Second); err != nil {
		t.Errorf("%v", err)
	}
	for _, f := range repo.Files[:len(repo.Files)-1] {
		if _, err := os.Stat(filepath.Join(destDir, path.Base(f))); os.IsNotExist(err) {
			t.Errorf("file %q should not exists in the test branch", f)
		}
	}
	if _, err := os.Stat(filepath.Join(destDir, path.Base(repo.Files[len(repo.Files)-1]))); !os.IsNotExist(err) {
		t.Errorf("last file should not exists in the test branch")
	}
	if _, err := os.Stat(filepath.Join(destDir, "sub")); os.IsNotExist(err) {
		t.Errorf("unable to find submodule dir")
	}
	for _, f := range repo.Submodule.Files {
		if _, err := os.Stat(filepath.Join(destDir, "sub/"+path.Base(f))); os.IsNotExist(err) {
			t.Errorf("unable to find submodule repository file %q", path.Base(f))
		}
	}
}

func TestCopyImageSourceFromFilesystem(t *testing.T) {

	testCases := []struct {
		name        string
		testFiles   map[string]string
		testLinks   map[string]string
		copyPath    string
		destination string
		verifyFiles map[string]string
		verifyLinks map[string]string
	}{
		{
			name: "single file",
			testFiles: map[string]string{
				"hello.txt": "Hello world!",
			},
			copyPath:    "hello.txt",
			destination: "dst",
			verifyFiles: map[string]string{
				"dst/hello.txt": "Hello world!",
			},
		},
		{
			name:        "single symlink",
			destination: "dst",
			testFiles: map[string]string{
				"src/hello.txt": "Hello world!",
			},
			testLinks: map[string]string{
				"link/hello.txt": "../hello.txt",
			},
			copyPath: "link/hello.txt",
			verifyLinks: map[string]string{
				"dst/hello.txt": "../hello.txt",
			},
		},
		{
			name:        "path preserving parent directory",
			copyPath:    "src",
			destination: "dst",
			testFiles: map[string]string{
				"src/foo/hello.txt": "Hello world!",
				"src/foo/foo.txt":   "bar",
			},
			testLinks: map[string]string{
				"src/bar/link.txt": "../hello.txt",
			},
			verifyFiles: map[string]string{
				"dst/src/foo/hello.txt": "Hello world!",
				"dst/src/foo/foo.txt":   "bar",
			},
			verifyLinks: map[string]string{
				"dst/src/bar/link.txt": "../hello.txt",
			},
		},
		{
			name:        "path removing parent directory",
			copyPath:    "src/.",
			destination: "dst",
			testFiles: map[string]string{
				"src/foo/hello.txt": "Hello world!",
				"src/foo/foo.txt":   "bar",
			},
			testLinks: map[string]string{
				"src/bar/link.txt": "../hello.txt",
			},
			verifyFiles: map[string]string{
				"dst/foo/hello.txt": "Hello world!",
				"dst/foo/foo.txt":   "bar",
			},
			verifyLinks: map[string]string{
				"dst/bar/link.txt": "../hello.txt",
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testDir, err := ioutil.TempDir("", "copy-src-from-fs")
			if err != nil {
				t.Fatalf("failed to create test directory: %v", err)
			}
			defer os.RemoveAll(testDir)
			for fileName, data := range tc.testFiles {
				err = createTestFile(testDir, fileName, data)
				if err != nil {
					t.Fatalf("failed to create test file %s: %v", fileName, err)
				}
			}
			for linkName, linkSrc := range tc.testLinks {
				err = createTestSymlink(testDir, linkName, linkSrc)
				if err != nil {
					t.Fatalf("failed to create test symlink %s: %v", linkName, err)
				}
			}
			dstDir := filepath.Join(testDir, tc.destination)
			copySrc := fmt.Sprintf("%s/%s", testDir, tc.copyPath)
			t.Logf("copying %s to %s", copySrc, dstDir)
			err = copyImageSourceFromFilesytem(copySrc, dstDir)
			if err != nil {
				t.Errorf("unexpected error occurred: %v", err)
			}
			for f, d := range tc.verifyFiles {
				fileName := filepath.Join(testDir, f)
				verifyFile(fileName, d, t)
			}
			for l, src := range tc.verifyLinks {
				linkName := filepath.Join(testDir, l)
				verifyLink(linkName, src, t)
			}
		})
	}
}

func createTestFile(testDir string, filename string, content string) error {
	file := filepath.Join(testDir, filename)
	fileDir := filepath.Dir(file)
	if _, err := os.Stat(fileDir); err != nil {
		err = os.MkdirAll(fileDir, 0777)
		if err != nil {
			return err
		}
	}
	return ioutil.WriteFile(file, []byte(content), 0644)
}

func createTestSymlink(testDir string, linkname string, source string) error {
	file := filepath.Join(testDir, linkname)
	fileDir := filepath.Dir(file)
	if _, err := os.Stat(fileDir); err != nil {
		err = os.MkdirAll(fileDir, 0777)
		if err != nil {
			return err
		}
	}
	return os.Symlink(source, file)
}

func verifyFile(filename string, expectedContent string, t *testing.T) {
	info, err := os.Lstat(filename)
	if err != nil {
		t.Fatalf("failed to lstat %s: %v", filename, err)
	}
	if info.IsDir() {
		t.Errorf("expected regular file for %s, got directory", filename)
	}
	switch mode := info.Mode(); {
	case mode&os.ModeSymlink != 0:
		t.Errorf("expected regular file for %s, got symlink", filename)
	case mode.IsRegular():
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			t.Errorf("could not read %s: %v", filename, err)
		}
		if string(data) != expectedContent {
			t.Errorf("expected file content %q, got %q", string(expectedContent), string(data))
		}
	default:
		t.Errorf("file %s is not a regular file, mode is: %v", filename, mode)
	}
}

func verifyLink(filename string, expectedSrc string, t *testing.T) {
	info, err := os.Lstat(filename)
	if err != nil {
		t.Fatalf("failed to lstat %s: %v", filename, err)
	}
	if info.IsDir() {
		t.Errorf("expected symlink for %s, got directory", filename)
	}
	switch mode := info.Mode(); {
	case mode&os.ModeSymlink != 0:
		linkSrc, err := os.Readlink(filename)
		if err != nil {
			// Should be able to read the symlink
			t.Errorf("failed to read symlink for %s: %v", filename, err)
		}
		if linkSrc != expectedSrc {
			t.Errorf("expected link source for %s to be %q, got %q", filename, expectedSrc, linkSrc)
		}
	case mode.IsRegular():
		t.Errorf("expected symlink for %s, got regular file", filename)
	default:
		t.Errorf("file %s is not a symlink, mode is: %v", filename, mode)
	}
}
