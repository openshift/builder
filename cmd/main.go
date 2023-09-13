package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/reexec"
	"github.com/spf13/cobra"

	"k8s.io/component-base/cli"
	klog "k8s.io/klog/v2"
	kcmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/openshift/library-go/pkg/serviceability"
	s2ifs "github.com/openshift/source-to-image/pkg/util/fs"

	"github.com/openshift/builder/pkg/build/builder"
	"github.com/openshift/builder/pkg/version"
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

	defer serviceability.BehaviorOnPanic(os.Getenv("OPENSHIFT_ON_PANIC"), version.Get())()
	defer serviceability.Profile(os.Getenv("OPENSHIFT_PROFILE")).Stop()

	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	const tlsCertRoot = "/etc/pki/tls/certs"
	const runtimeCertRoot = "/etc/docker/certs.d"

	clusterCASrc := fmt.Sprintf("%s/ca.crt", builder.SecretCertsMountPath)
	clusterCADst := fmt.Sprintf("%s/cluster.crt", tlsCertRoot)
	fs := s2ifs.NewFileSystem()
	err := fs.Copy(clusterCASrc, clusterCADst, func(path string) bool { return false })
	if err != nil {
		fmt.Printf("Error setting up cluster CA cert: %v\n", err)
		os.Exit(1)
	}

	runtimeCASrc := fmt.Sprintf("%s/certs.d", builder.ConfigMapCertsMountPath)
	err = fs.CopyContents(runtimeCASrc, runtimeCertRoot, func(path string) bool { return false })
	if err != nil {
		fmt.Printf("Error setting up service CA cert: %v\n", err)
		os.Exit(1)
	}

	basename := filepath.Base(os.Args[0])
	command := CommandFor(basename)

	flags := flag.NewFlagSet(basename, flag.ExitOnError)
	klog.InitFlags(flags)
	pflags := command.Flags()
	var uidmap, gidmap string
	var useNewuidmap, useNewgidmap bool
	flags.StringVar(&uidmap, "uidmap", "", "re-exec in a user namespace using the specified UID map")
	flags.StringVar(&gidmap, "gidmap", "", "re-exec in a user namespace using the specified GID map")
	flags.BoolVar(&useNewuidmap, "use-newuidmap", os.Geteuid() != 0, "use newuidmap to set up UID mappings")
	flags.BoolVar(&useNewgidmap, "use-newgidmap", os.Geteuid() != 0, "use newgidmap to set up GID mappings")
	vflag := flags.Lookup("v")
	flags.Var(vflag.Value, "loglevel", "logging verbosity")
	pflags.AddGoFlagSet(flags)
	wrapped := command.Run
	command.Run = func(c *cobra.Command, args []string) {
		switch basename {
		case "openshift-sti-build", "openshift-docker-build", "openshift-extract-image-content":
			storeOptions, err := storage.DefaultStoreOptions(false, 0)
			kcmdutil.CheckErr(err)
			os.MkdirAll(storeOptions.GraphRoot, 0775)
			os.MkdirAll(storeOptions.RunRoot, 0775)
			maybeReexecUsingUserNamespace(uidmap, useNewuidmap, gidmap, useNewgidmap)
			wrapped(c, args)
		default:
			wrapped(c, args)
		}
	}

	code := cli.Run(command)
	os.Exit(code)
}

// CommandFor returns the appropriate command for this base name,
// or the OpenShift CLI command.
func CommandFor(basename string) *cobra.Command {
	var cmd *cobra.Command

	switch basename {
	case "openshift-sti-build", "openshift-sti-build-in-a-user-namespace":
		cmd = NewCommandS2IBuilder(basename)
	case "openshift-docker-build", "openshift-docker-build-in-a-user-namespace":
		cmd = NewCommandDockerBuilder(basename)
	case "openshift-git-clone":
		cmd = NewCommandGitClone(basename)
	case "openshift-manage-dockerfile":
		cmd = NewCommandManageDockerfile(basename)
	case "openshift-extract-image-content", "openshift-extract-image-content-in-a-user-namespace":
		cmd = NewCommandExtractImageContent(basename)
	default:
		fmt.Printf("unknown command name: %s\n", basename)
		os.Exit(1)
	}

	return cmd
}
