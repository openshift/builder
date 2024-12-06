# OpenShift Builder

The OpenShift Builder container image drives OpenShift's
[Builds using BuildConfig](https://docs.redhat.com/en/documentation/openshift_container_platform/4.17/html/builds_using_buildconfig/index),
leveraging [Buildah](https://github.com/containers/buildah) and
[Source-To-Image](https://github.com/openshift/source-to-image) to turn source code into images
that can be deployed on Kubernetes. The container image also handles the lifecycle of the build,
from obtaining source code to pushing the output image to a container registry.

Much of the work here and in the build controllers for
[openshift-controller-manager](https://github.com/openshift/openshift-controller-manager) inspired
the CNCF [Shipwright](https://shipwright.io) project.

## Getting Started

The OpenShift Builder container image is intended to run exclusively as a container within an
OpenShift build pod. The best way to run the container image is to follow the official documentation
on how to run a build with
[BuildConfigs](https://docs.redhat.com/en/documentation/openshift_container_platform/4.17/html/builds_using_buildconfig/understanding-image-builds).

## Contributing

Glad to have you here!  

Please check the [CONTRIBUTING guide](./CONTRIBUTING.md) for detailed steps on how to develop and
test your code.

## License

Licensed under Apache License, Version 2.0. Refer to [LICENSE](./LICENSE) for full terms and
conditions.
