package fake

import (
	"io"
	"time"

	"github.com/openshift/library-go/pkg/git"
)

func NewFakeGitRepository() *FakeGitRepository {
	return &FakeGitRepository{
		Configs:       make(map[string]string),
		LocalConfigs:  make(map[string]string),
		GlobalConfigs: make(map[string]string),
	}
}

type FakeGitRepository struct {
	Configs       map[string]string
	LocalConfigs  map[string]string
	GlobalConfigs map[string]string
}

func (g *FakeGitRepository) GetRootDir(dir string) (string, error) {
	return "", nil
}

func (g *FakeGitRepository) GetOriginURL(dir string) (string, bool, error) {
	return "", false, nil
}
func (g *FakeGitRepository) GetRef(dir string) string {
	return ""
}

func (g *FakeGitRepository) Clone(dir string, url string) error {
	return nil
}
func (g *FakeGitRepository) CloneWithOptions(dir string, url string, args ...string) error {
	return nil
}

func (g *FakeGitRepository) CloneBare(dir string, url string) error {
	return nil
}

func (g *FakeGitRepository) CloneMirror(dir string, url string) error {
	return nil
}

func (g *FakeGitRepository) Fetch(dir string, url string, ref string) error {
	return nil
}

func (g *FakeGitRepository) Checkout(dir string, ref string) error {
	return nil
}

func (g *FakeGitRepository) PotentialPRRetryAsFetch(dir string, url string, ref string, err error) error {
	return nil
}

func (g *FakeGitRepository) SubmoduleUpdate(dir string, init, recursive bool) error {
	return nil
}
func (g *FakeGitRepository) Archive(dir, ref, format string, w io.Writer) error {
	return nil
}

func (g *FakeGitRepository) Init(dir string, bare bool) error {
	return nil
}

func (g *FakeGitRepository) Add(dir string, spec string) error {
	return nil
}

func (g *FakeGitRepository) Commit(dir string, message string) error {
	return nil
}
func (g *FakeGitRepository) AddRemote(dir string, name, url string) error {
	return nil
}

func (g *FakeGitRepository) AddConfig(dir, name, value string) error {
	g.Configs[name] = value
	return nil
}

func (g *FakeGitRepository) AddLocalConfig(dir, name, value string) error {
	g.LocalConfigs[name] = value
	return nil
}

func (g *FakeGitRepository) AddGlobalConfig(name, value string) error {
	g.GlobalConfigs[name] = value
	return nil
}
func (g *FakeGitRepository) ShowFormat(dir, commit, format string) (string, error) {
	return "", nil
}

func (g *FakeGitRepository) ListRemote(url string, args ...string) (string, string, error) {
	return "", "", nil
}
func (g *FakeGitRepository) TimedListRemote(timeout time.Duration, url string, args ...string) (string, string, error) {
	return "", "", nil
}

func (g *FakeGitRepository) GetInfo(location string) (*git.SourceInfo, []error) {
	return nil, nil
}
