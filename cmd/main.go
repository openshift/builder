package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/containers/storage/pkg/reexec"
	"github.com/spf13/cobra"

	"k8s.io/component-base/logs"

	"github.com/openshift/builder/pkg/build/builder"
	"github.com/openshift/builder/pkg/version"
	"github.com/openshift/library-go/pkg/serviceability"
	s2ifs "github.com/openshift/source-to-image/pkg/util/fs"
)

func main() {
	if reexec.Init() {
		return
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)
	go func() {
		<-sigs
		fmt.Println("Error: received unexpected terminate signal")
		os.Exit(1)
	}()

	logs.InitLogs()
	defer logs.FlushLogs()
	defer serviceability.BehaviorOnPanic(os.Getenv("OPENSHIFT_ON_PANIC"), version.Get())()
	defer serviceability.Profile(os.Getenv("OPENSHIFT_PROFILE")).Stop()

	rand.Seed(time.Now().UTC().UnixNano())
	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	const tlsCertRoot = "/etc/pki/tls/certs"
	const runtimeCertRoot = "/etc/docker/certs.d"

	clusterCASrc := fmt.Sprintf("%s/ca.crt", builder.SecretCertsMountPath)
	clusterCADst := fmt.Sprintf("%s/cluster.crt", tlsCertRoot)
	fs := s2ifs.NewFileSystem()
	err := fs.Copy(clusterCASrc, clusterCADst, map[string]string{})
	if err != nil {
		fmt.Printf("Error setting up cluster CA cert: %v", err)
		os.Exit(1)
	}

	runtimeCASrc := fmt.Sprintf("%s/certs.d", builder.ConfigMapCertsMountPath)
	err = fs.CopyContents(runtimeCASrc, runtimeCertRoot, map[string]string{})
	if err != nil {
		fmt.Printf("Error setting up service CA cert: %v", err)
		os.Exit(1)
	}

	basename := filepath.Base(os.Args[0])
	command := CommandFor(basename)
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}

// CommandFor returns the appropriate command for this base name,
// or the OpenShift CLI command.
func CommandFor(basename string) *cobra.Command {
	var cmd *cobra.Command

	switch basename {
	case "openshift-sti-build":
		cmd = NewCommandS2IBuilder(basename)
	case "openshift-docker-build":
		cmd = NewCommandDockerBuilder(basename)
	case "openshift-git-clone":
		cmd = NewCommandGitClone(basename)
	case "openshift-manage-dockerfile":
		cmd = NewCommandManageDockerfile(basename)
	case "openshift-extract-image-content":
		cmd = NewCommandExtractImageContent(basename)
	default:
		fmt.Printf("unknown command name: %s\n", basename)
		os.Exit(1)
	}

	GLog(cmd.PersistentFlags())

	return cmd
}
