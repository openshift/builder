package util

import (
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	buildapiv1 "github.com/openshift/api/build/v1"
	buildscheme "github.com/openshift/client-go/build/clientset/versioned/scheme"
)

var (
	buildCodecFactory = serializer.NewCodecFactory(buildscheme.Scheme)
	buildJSONCodec    runtime.Codec
)

func init() {
	buildJSONCodec = buildCodecFactory.LegacyCodec(buildapiv1.SchemeGroupVersion)
}

func GetBuildFromEnv(build *buildapiv1.Build) error {
	buildStr := os.Getenv("BUILD")

	obj, _, err := buildJSONCodec.Decode([]byte(buildStr), nil, build)
	if err != nil {
		return fmt.Errorf("unable to parse build string: %v", err)
	}
	_, ok := obj.(*buildapiv1.Build)
	if !ok {
		return fmt.Errorf("build string %s is not a build: %#v", buildStr, obj)
	}

	return nil
}
