module github.com/openshift/builder

go 1.12

require (
	github.com/MakeNowJust/heredoc v0.0.0-20170808103936-bb23615498cd
	github.com/certifi/gocertifi v0.0.0-20180905225744-ee1a9a0726d2 // indirect
	github.com/containerd/continuity v0.0.0-20190426062206-aaeac12a7ffc // indirect
	github.com/containers/buildah v1.11.3
	github.com/containers/image v3.0.2+incompatible // indirect
	github.com/containers/image/v4 v4.0.1
	github.com/containers/storage v1.13.4
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/spdystream v0.0.0-20160310174837-449fdfce4d96 // indirect
	github.com/evanphx/json-patch v4.5.0+incompatible // indirect
	github.com/exponent-io/jsonpath v0.0.0-20151013193312-d6023ce2651d // indirect
	github.com/fsouza/go-dockerclient v1.4.4
	github.com/getsentry/raven-go v0.0.0-20190513200303-c977f96e1095 // indirect
	github.com/go-openapi/spec v0.19.3 // indirect
	github.com/google/btree v1.0.0 // indirect
	github.com/googleapis/gnostic v0.3.1 // indirect
	github.com/gregjones/httpcache v0.0.0-20170728041850-787624de3eb7 // indirect
	github.com/klauspost/compress v1.8.2 // indirect
	github.com/mitchellh/go-wordwrap v0.0.0-20150314170334-ad45545899c7 // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/opencontainers/runc v1.0.0-rc8.0.20190827142921-dd075602f158
	github.com/opencontainers/runtime-spec v1.0.0
	github.com/openshift/api v3.9.1-0.20190810003144-27fb16909b15+incompatible
	github.com/openshift/client-go v0.0.0-20190813201236-5a5508328169
	github.com/openshift/imagebuilder v1.1.0
	github.com/openshift/library-go v0.0.0-20190904120025-7d4acc018c61
	github.com/openshift/source-to-image v1.2.0
	github.com/pborman/uuid v1.2.0 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pkg/errors v0.8.1
	github.com/pkg/profile v1.3.0 // indirect
	github.com/seccomp/containers-golang v0.0.0-20190710130535-8d1ed6a872a5 // indirect
	github.com/shurcooL/sanitized_anchor_name v0.0.0-20151028001915-10ef21a441db // indirect
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45 // indirect
	golang.org/x/sys v0.0.0-20190902133755-9109b7679e13
	google.golang.org/genproto v0.0.0-20180831171423-11092d34479b // indirect
	gopkg.in/inf.v0 v0.9.0 // indirect
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/cli-runtime v0.0.0-00010101000000-000000000000 // indirect
	k8s.io/client-go v0.0.0
	k8s.io/component-base v0.0.0
	k8s.io/klog v0.4.0
	k8s.io/kube-openapi v0.0.0-20190918143330-0270cf2f1c1d // indirect
	k8s.io/kubernetes v0.0.0
	k8s.io/utils v0.0.0-20190923111123-69764acb6e8e // indirect
	sigs.k8s.io/kustomize v2.0.3+incompatible // indirect
)

replace (
	github.com/containers/image/v4 => github.com/containers/image/v4 v4.0.0-20191004175620-9da78d1f50d2
	github.com/docker/docker => github.com/openshift/moby-moby v1.4.2-0.20190308215630-da810a85109d
	github.com/opencontainers/runtime-tools => github.com/opencontainers/runtime-tools v0.8.0
	github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.4.2
	k8s.io/api => github.com/openshift/kubernetes-api v0.0.0-20190918181048-13e46d2d750b
	k8s.io/apiextensions-apiserver => github.com/openshift/kubernetes-apiextensions-apiserver v0.0.0-20190907075317-b45b4fd943f2
	k8s.io/apimachinery => github.com/openshift/kubernetes-apimachinery v0.0.0-20190822093434-ef0be50b30d5
	k8s.io/apiserver => github.com/openshift/kubernetes-api v0.0.0-20190918181048-13e46d2d750b
	k8s.io/cli-runtime => github.com/openshift/kubernetes-cli-runtime v0.0.0-20190529092519-a9444fab0824
	k8s.io/client-go => github.com/openshift/kubernetes-client-go v11.0.1-0.20190909040624-51edc5bf3d27+incompatible
	k8s.io/cloud-provider => github.com/openshift/kubernetes-cloud-provider v0.0.0-20190820070159-cc69e8c1a0fb
	k8s.io/cluster-bootstrap => github.com/openshift/kubernetes-cluster-bootstrap v0.0.0-20190409023614-027c502bb854
	k8s.io/code-generator => github.com/openshift/kubernetes-code-generator v0.0.0-20171023130718-1000a095f043
	k8s.io/component-base => github.com/openshift/kubernetes-component-base v0.0.0-20190820062816-3297a9ba437a
	k8s.io/cri-api => github.com/openshift/kubernetes-cri-api v0.0.0-20190326082326-5c2568eea0b8
	k8s.io/csi-translation-lib => github.com/openshift/kubernetes-csi-translation-lib v0.0.0-20190820102622-9cac5f72ab0a
	k8s.io/kube-aggregator => github.com/openshift/kubernetes-kube-aggregator v0.0.0-20190827130650-530023b8fd15
	k8s.io/kube-controller-manager => github.com/openshift/kubernetes-kube-controller-manager v0.0.0-20190820102450-04a282e1ef9e
	k8s.io/kube-proxy => github.com/openshift/kube-proxy v0.0.0-20190820102304-3f9be70440ef
	k8s.io/kube-scheduler => github.com/openshift/kubernetes-kube-scheduler v0.0.0-20190820102418-07021c6f4c62
	k8s.io/kubelet => github.com/openshift/kubernetes-kubelet v0.0.0-20190802155351-eb626078f47b
	k8s.io/kubernetes => github.com/openshift/kubernetes v1.14.7-0.20190920214232-910ed3779497
	k8s.io/legacy-cloud-providers => github.com/openshift/kubernetes-legacy-cloud-providers v0.0.0-20190416155406-4c85c9b0ae06
	k8s.io/metrics => github.com/openshift/kubernetes-metrics v0.0.0-20190820064753-ea52c2989504
	k8s.io/sample-apiserver => github.com/openshift/kubernetes-sample-apiserver v0.0.0-20190820064015-ee4ed4ef2815
)
