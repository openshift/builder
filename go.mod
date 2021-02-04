module github.com/openshift/builder

go 1.12

require (
	github.com/MakeNowJust/heredoc v0.0.0-20170808103936-bb23615498cd
	github.com/certifi/gocertifi v0.0.0-20180905225744-ee1a9a0726d2 // indirect
	github.com/containers/buildah v1.14.11
	github.com/containers/common v0.8.4
	github.com/containers/image/v5 v5.4.3
	github.com/containers/storage v1.18.2
	github.com/docker/distribution v2.7.1+incompatible
	github.com/elazarl/goproxy v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/evanphx/json-patch v4.5.0+incompatible // indirect
	github.com/fsouza/go-dockerclient v1.6.3
	github.com/getsentry/raven-go v0.0.0-20190513200303-c977f96e1095 // indirect
	github.com/googleapis/gnostic v0.3.1 // indirect
	github.com/opencontainers/runc v1.0.0-rc9
	github.com/opencontainers/runtime-spec v1.0.1
	github.com/openshift/api v0.0.0-20200618202633-7192180f496a
	github.com/openshift/client-go v0.0.0-20191022152013-2823239d2298
	github.com/openshift/imagebuilder v1.1.4
	github.com/openshift/library-go v0.0.0-20190904120025-7d4acc018c61
	github.com/openshift/source-to-image v1.3.0
	github.com/pkg/errors v0.9.1
	github.com/pkg/profile v1.3.0 // indirect
	github.com/seccomp/containers-golang v0.0.0-20190710130535-8d1ed6a872a5 // indirect
	github.com/spf13/cobra v0.0.7
	github.com/spf13/pflag v1.0.5
	golang.org/x/sys v0.0.0-20200327173247-9dae0f8f5775
	k8s.io/api v0.17.13
	k8s.io/apimachinery v0.17.13
	k8s.io/client-go v0.0.0
	k8s.io/component-base v0.0.0
	k8s.io/klog v1.0.0
	k8s.io/kubectl v0.0.0
	k8s.io/kubernetes v1.13.0
)

replace (
	github.com/containerd/containerd => github.com/containerd/containerd v1.3.6
	github.com/docker/docker => github.com/docker/docker v0.0.0-20190404075923-dbe4a30928d4
	github.com/docker/libnetwork => github.com/docker/libnetwork v0.8.0-dev.2.0.20171107005402-dcf79f8c7fcb
	github.com/godbus/dbus => github.com/godbus/dbus v0.0.0-20181101234600-2ff6f7ffd60f
	github.com/opencontainers/runtime-tools => github.com/opencontainers/runtime-tools v0.8.0
	// CVE-2020-9283 fix
	golang.org/x/crypto => golang.org/x/crypto v0.0.0-20200220183623-bac4c82f6975
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20190708153700-3bdd9d9f5532
	k8s.io/api => k8s.io/api v0.0.0-20191122220107-b5267f2975e0
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20191122222427-64482ea217ff
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20191121175448-79c2a76c473a
	k8s.io/apiserver => k8s.io/apiserver v0.0.0-20191122221311-9d521947b1e1
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20191122222818-9150eb3ded31
	k8s.io/client-go => k8s.io/client-go v0.0.0-20191122220542-ed16ecbdf3a0
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20191122224014-59d76ad41876
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.0.0-20191122223827-289de4a64c1c
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20191121175249-e95606b614f0
	k8s.io/component-base => k8s.io/component-base v0.0.0-20191122220729-2684fb322cb9
	k8s.io/cri-api => k8s.io/cri-api v0.0.0-20191121183020-775aa3c1cf73
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.0.0-20191122224157-518651485193
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20191122221605-1e8d331e4dcc
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.0.0-20191122223648-5cfd5067047c
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.0.0-20191122223145-16f2c0c680a0
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.0.0-20191122223507-ac656df73673
	k8s.io/kubectl => k8s.io/kubectl v0.0.0-20191122225023-1e3c8b70f494
	k8s.io/kubelet => k8s.io/kubelet v0.0.0-20191122223325-9316382755ad
	k8s.io/kubernetes => k8s.io/kubernetes v1.17.0-rc.2
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20191122224431-860df69ff5cc
	k8s.io/metrics => k8s.io/metrics v0.0.0-20191122222628-19ed227de2b6
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.0.0-20191122221846-294c70c3d5d4
)
