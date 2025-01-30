module github.com/openshift/builder

go 1.22.0

require (
	github.com/MakeNowJust/heredoc v1.0.0
	github.com/containers/buildah v1.35.4
	github.com/containers/common v0.58.3
	github.com/containers/image/v5 v5.30.1
	github.com/containers/storage v1.53.0
	github.com/docker/distribution v2.8.3+incompatible
	github.com/fsouza/go-dockerclient v1.10.1
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/opencontainers/runc v1.1.12
	github.com/opencontainers/runtime-spec v1.2.0
	github.com/openshift/api v0.0.0-20240522145529-93d6bda14341
	github.com/openshift/client-go v0.0.0-20231018150822-6e226e2825a6
	github.com/openshift/imagebuilder v1.2.6
	github.com/openshift/library-go v0.0.0-20231017173800-126f85ed0cc7
	github.com/openshift/source-to-image v1.4.0
	github.com/pkg/errors v0.9.1 // indirect
	github.com/spf13/cobra v1.8.0
	github.com/spf13/pflag v1.0.5
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635
	golang.org/x/sys v0.28.0
	k8s.io/api v0.30.2
	k8s.io/apimachinery v0.30.2
	k8s.io/client-go v0.30.2
	k8s.io/component-base v0.30.2
	k8s.io/klog/v2 v2.120.1
	k8s.io/kubectl v0.30.2
	k8s.io/kubernetes v1.30.2
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/BurntSushi/toml v1.3.2 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/Microsoft/hcsshim v0.12.0-rc.3 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/chai2010/gettext-go v1.0.2 // indirect
	github.com/chzyer/readline v1.5.1 // indirect
	github.com/containerd/containerd v1.7.13 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.15.1 // indirect
	github.com/containernetworking/cni v1.1.2 // indirect
	github.com/containers/libtrust v0.0.0-20230121012942-c1716e8a8d01 // indirect
	github.com/containers/ocicrypt v1.1.10 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/cyphar/filepath-securejoin v0.2.4 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/docker v25.0.3+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.8.1 // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/evanphx/json-patch v4.12.0+incompatible // indirect
	github.com/exponent-io/jsonpath v0.0.0-20151013193312-d6023ce2651d // indirect
	github.com/go-errors/errors v1.4.2 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.10 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/go-intervals v0.0.2 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.7 // indirect
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/manifoldco/promptui v0.9.0 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/mattn/go-shellwords v1.0.12 // indirect
	github.com/miekg/pkcs11 v1.1.1 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/moby/sys/mountinfo v0.7.1 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0 // indirect
	github.com/opencontainers/runtime-tools v0.9.1-0.20230914150019-408c51e934dc // indirect
	github.com/opencontainers/selinux v1.11.0 // indirect
	github.com/ostreedev/ostree-go v0.0.0-20210805093236-719684c64e4f // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pkg/profile v1.5.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/proglottis/gpgme v0.1.3 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/seccomp/libseccomp-golang v0.10.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/stefanberger/go-pkcs11uri v0.0.0-20201008174630-78d3cae3a980 // indirect
	github.com/stretchr/testify v1.9.0
	github.com/sylabs/sif/v2 v2.15.1 // indirect
	github.com/ulikunitz/xz v0.5.11 // indirect
	github.com/vbatts/tar-split v0.11.5 // indirect
	github.com/xlab/treeprint v1.2.0 // indirect
	go.mozilla.org/pkcs7 v0.0.0-20210826202110-33d05740a352 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.starlark.net v0.0.0-20230525235612-a134d8f9ddca // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/oauth2 v0.18.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/term v0.27.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/grpc v1.61.0 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiserver v0.30.2 // indirect
	k8s.io/cli-runtime v0.30.2 // indirect
	k8s.io/kube-openapi v0.0.0-20240228011516-70dd3763d340 // indirect
	k8s.io/utils v0.0.0-20230726121419-3b25d923346b // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/kustomize/api v0.13.5-0.20230601165947-6ce0bf390ce3 // indirect
	sigs.k8s.io/kustomize/kyaml v0.14.3-0.20230601165947-6ce0bf390ce3 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

require github.com/go-imports-organizer/goio v1.3.3

require (
	dario.cat/mergo v1.0.0 // indirect
	github.com/aead/serpent v0.0.0-20160714141033-fba169763ea6 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cilium/ebpf v0.9.1 // indirect
	github.com/containerd/cgroups/v3 v3.0.2 // indirect
	github.com/containerd/errdefs v0.1.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/typeurl/v2 v2.1.1 // indirect
	github.com/containernetworking/plugins v1.4.0 // indirect
	github.com/containers/luksy v0.0.0-20240212203526-ceb12d4fd50c // indirect
	github.com/cyberphone/json-canonicalization v0.0.0-20231217050601-ba74d44ecf5f // indirect
	github.com/disiqueira/gotree/v3 v3.0.2 // indirect
	github.com/distribution/reference v0.5.0 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-jose/go-jose/v3 v3.0.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/analysis v0.21.4 // indirect
	github.com/go-openapi/errors v0.21.1 // indirect
	github.com/go-openapi/loads v0.21.2 // indirect
	github.com/go-openapi/runtime v0.26.0 // indirect
	github.com/go-openapi/spec v0.20.9 // indirect
	github.com/go-openapi/strfmt v0.22.2 // indirect
	github.com/go-openapi/validate v0.22.1 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-containerregistry v0.19.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/jinzhu/copier v0.4.0 // indirect
	github.com/letsencrypt/boulder v0.0.0-20230907030200-6d76a0f91e1e // indirect
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mistifyio/go-zfs/v3 v3.0.1 // indirect
	github.com/moby/buildkit v0.12.5 // indirect
	github.com/moby/patternmatcher v0.6.0 // indirect
	github.com/moby/sys/sequential v0.5.0 // indirect
	github.com/moby/sys/user v0.1.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/prometheus/client_golang v1.17.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.44.0 // indirect
	github.com/prometheus/procfs v0.11.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.8.0 // indirect
	github.com/sigstore/fulcio v1.4.3 // indirect
	github.com/sigstore/rekor v1.2.2 // indirect
	github.com/sigstore/sigstore v1.8.2 // indirect
	github.com/tchap/go-patricia/v2 v2.3.1 // indirect
	github.com/titanous/rocacheck v0.0.0-20171023193734-afe73141d399 // indirect
	github.com/vbauerster/mpb/v8 v8.7.2 // indirect
	github.com/vishvananda/netlink v1.2.1-beta.2 // indirect
	github.com/vishvananda/netns v0.0.4 // indirect
	go.etcd.io/bbolt v1.3.9 // indirect
	go.mongodb.org/mongo-driver v1.14.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.45.0 // indirect
	go.opentelemetry.io/otel v1.22.0 // indirect
	go.opentelemetry.io/otel/metric v1.22.0 // indirect
	go.opentelemetry.io/otel/trace v1.22.0 // indirect
	golang.org/x/exp v0.0.0-20240222234643-814bf88cf225 // indirect
	golang.org/x/tools v0.21.1-0.20240508182429-e35e4ccd0d2d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231106174013-bbf56f31fb17 // indirect
	gopkg.in/go-jose/go-jose.v2 v2.6.3 // indirect
	k8s.io/apiextensions-apiserver v0.28.2 // indirect
	tags.cncf.io/container-device-interface v0.6.2 // indirect
)

replace (
	// these are needed since k8s.io/kubernetes cites v0.0.0 for these in its go.mod
	k8s.io/api => k8s.io/api v0.30.2
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.30.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.30.2
	k8s.io/apiserver => k8s.io/apiserver v0.30.2
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.30.2
	k8s.io/client-go => k8s.io/client-go v0.30.2
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.30.2
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.30.2
	k8s.io/code-generator => k8s.io/code-generator v0.30.2
	k8s.io/component-base => k8s.io/component-base v0.30.2
	k8s.io/component-helpers => k8s.io/component-helpers v0.30.2
	k8s.io/controller-manager => k8s.io/controller-manager v0.30.2
	k8s.io/cri-api => k8s.io/cri-api v0.30.2
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.30.2
	// Fix "go: k8s.io/dynamic-resource-allocation@v0.0.0: invalid version: unknown revision v0.0.0"
	k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.30.2
	k8s.io/endpointslice => k8s.io/endpointslice v0.30.2
	k8s.io/kms => k8s.io/kms v0.30.2
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.30.2
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.30.2
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.30.2
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.30.2
	k8s.io/kubectl => k8s.io/kubectl v0.30.2
	k8s.io/kubelet => k8s.io/kubelet v0.30.2
	k8s.io/kubernetes => k8s.io/kubernetes v1.30.2
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.30.2
	k8s.io/metrics => k8s.io/metrics v0.30.2
	k8s.io/mount-utils => k8s.io/mount-utils v0.30.2
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.30.2
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.30.2
)
