module github.com/openshift/builder

go 1.16

require (
	github.com/MakeNowJust/heredoc v0.0.0-20170808103936-bb23615498cd
	github.com/Microsoft/hcsshim v0.9.1 // indirect
	github.com/containers/buildah v1.22.3
	github.com/containers/common v0.42.1
	github.com/containers/image/v5 v5.16.1
	github.com/containers/storage v1.37.1-0.20211108123020-ee6db3140c1c
	github.com/docker/distribution v2.7.1+incompatible
	github.com/elazarl/goproxy v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/fsouza/go-dockerclient v1.7.3
	github.com/opencontainers/runc v1.0.2
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417
	github.com/openshift/api v0.0.0-20210730095913-85e1d547cdee
	github.com/openshift/client-go v0.0.0-20210730113412-1811c1b3fc0e
	github.com/openshift/imagebuilder v1.2.2-0.20210415181909-87f3e48c2656
	github.com/openshift/library-go v0.0.0-20210430084706-e555322cb708
	github.com/openshift/source-to-image v1.3.2-0.20210719201059-4f4cb8b11f98
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5 // indirect
	golang.org/x/sys v0.0.0-20211025201205-69cdffdb9359
	k8s.io/api v0.22.0-rc.0
	k8s.io/apimachinery v0.22.0-rc.0
	k8s.io/client-go v0.22.0-rc.0
	k8s.io/component-base v0.22.0-rc.0
	k8s.io/klog/v2 v2.9.0
	k8s.io/kubectl v0.22.0-rc.0
	k8s.io/kubernetes v1.22.0-rc.0
)

replace (
	// temporary replacement for development purposes
	github.com/containerd/containerd => github.com/containerd/containerd v1.5.0
	github.com/docker/docker => github.com/docker/docker v0.0.0-20200911110540-7ca355652fe0
	github.com/docker/libnetwork => github.com/docker/libnetwork v0.0.0-20210525090646-64b7a4574d14
	github.com/godbus/dbus => github.com/godbus/dbus v0.0.0-20181101234600-2ff6f7ffd60f
	// Needed to resolve broken transitive dependencies in buildkit and s2i
	golang.org/x/crypto => golang.org/x/crypto v0.0.0-20200323165209-0ec3e9974c59
	// these are needed since k8s.io/kubernetes cites v0.0.0 for these in its go.mod
	k8s.io/api => k8s.io/api v0.22.0-rc.0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.22.0-rc.0
	k8s.io/apimachinery => k8s.io/apimachinery v0.22.0-rc.0
	k8s.io/apiserver => k8s.io/apiserver v0.22.0-rc.0
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.22.0-rc.0
	k8s.io/client-go => k8s.io/client-go v0.22.0-rc.0
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.22.0-rc.0
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.22.0-rc.0
	k8s.io/code-generator => k8s.io/code-generator v0.22.0-rc.0
	k8s.io/component-base => k8s.io/component-base v0.22.0-rc.0
	k8s.io/component-helpers => k8s.io/component-helpers v0.22.0-rc.0
	k8s.io/controller-manager => k8s.io/controller-manager v0.22.0-rc.0
	k8s.io/cri-api => k8s.io/cri-api v0.22.0-rc.0
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.22.0-rc.0
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.22.0-rc.0
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.22.0-rc.0
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.22.0-rc.0
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.22.0-rc.0
	k8s.io/kubectl => k8s.io/kubectl v0.22.0-rc.0
	k8s.io/kubelet => k8s.io/kubelet v0.22.0-rc.0
	k8s.io/kubernetes => k8s.io/kubernetes v1.22.0-rc.0
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.22.0-rc.0
	k8s.io/metrics => k8s.io/metrics v0.22.0-rc.0
	k8s.io/mount-utils => k8s.io/mount-utils v0.22.0-rc.0
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.22.0-rc.0
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.22.0-rc.0
)
