# Contributing Guide

## Prerequisites

`openshift/builder` is designed to run exclusively on Linux systems. If you are developing on a
Windows or Mac machine, we recommend setting up a virtual environment that runs the latest Fedora,
CentOS Stream, or Red Hat Enterprise Linux distribution.

You will also need to install the following dependencies on your system:

- Go SDK - refer to [go.mod](./go.mod) for the minimum required version.
- The following packages:
  - `btrfs-progs-devel`
  - `device-mapper-devel`
  - `gpgme-devel`

## Compiling the Code

To get started, [fork](https://help.github.com/articles/fork-a-repo) the
[openshift/builder](https://github.com/openshift/builder) repo, then clone it to a local directory:

```sh
$ mkdir -p ~/go/src/github.com/openshift
$ cd ~/go/src/github.com/openshift
$ git clone https://github.com/openshift/builder.git
$ cd builder
```

Once in the source code directory, run the `build` Makefile target to compile the binary:

```sh
$ make build
```

If the build succeeds, you are ready to start making code changes!

## Submitting a Pull Request

Before submitting a pull request, ensure all unit tests are passing by running `make test`. You are
encouraged to add your own unit tests alongside code contributions. This repository uses the
standard Go test framework for unit tests.

Once you are satisfied with your code changes (and unit tests are passing), submit a pull request
to the [openshift/builder](https://github.com/openshift/builder) repo. A member of the OpenShift
team will evaluate your change for approval.

## Testing

The OpenShift Builder image is intended to run exclusively as a container within an OpenShift
build. The best way to experiment with new code changes is to build the image, push to a container
registry, and then configure OpenShift to deploy your "test" image when running builds.

Red Hat employees can also use `cluster-bot` (available on the internal Slack as an app) to launch
a cluster that includes code changes submitted in a pull request. Refer to the
[ci-chat-bot](https://github.com/openshift/ci-chat-bot) source for more information.

### Building the image

This repository can build two flavors of images:

1. A "developer" image, which uses publicly available base images.
2. The "product" image, which uses base images only available on Red Hat's CI infrastructure.

The two images are intended to be similar, but may have some skews due to differences in how
OpenShift's build system assembles the entire platform (aka "the payload").

#### Developer Image

Run the `build-devel-image` Makefile target, giving it an appropriate host, repository, and image tag:

```
$ make build-devel-image IMAGE=<MYREPO>/<MYIMAGE> TAG=<MYTAG> 
```

#### Product Image

This procedure will only succeed if you are a Red Hat employee and have been granted permission to
pull images from the OpenShift CI system. Refer to the
[Openshift CI docs](https://docs.ci.openshift.org/docs/how-tos/use-registries-in-build-farm/#how-do-i-gain-access-to-qci)
for instructions on how to request access to the CI container registries.

1. First, log into the [api.ci](https://console-openshift-console.apps.ci.l2s4.p1.openshiftapps.com/)
   cluster using your Red Hat SSO credentials.
2. Use the `Copy Login Command` to get command line access to the cluster. Note that you must have
   the `oc` command line installed on your system:

   ```sh
   $ oc login --token=<xxx> --server=<server_url>:<port>
   ```

3. Once logged in to the cluster successfully, use `oc` to authenticate with the cluster's
   container registry (`registry.ci.openshift.org`):

   ```sh
   $ oc registry login
   ```

4. Finally, build the container image using the `build-image` Makefile target:

   ```sh
   $ make build IMAGE=<MYREPO>/<MYIMAGE> TAG=<MYTAG> 
   ```

### Testing on an OpenShift Cluster

#### Cluster-bot

For Red Hat employees, the easiest way to test code changes is to submit a pull request (add "WIP"
to the title if your code is not ready to merge), then create a cluster with `cluster-bot`
referencing your PR number. Refer to the
[cluster-bot FAQ](https://github.com/openshift/ci-chat-bot/blob/main/docs/FAQ.md#frequently-asked-questions)
for an example of how to do this.

#### Self-Managed OpenShift

For the general public, you must first gain access to a _self-managed_ OpenShift cluster. The
builder image cannot be tested on managed clusters, such as Red Hat OpenShift Service on AWS
(ROSA), because the procedure for testing brings your cluster into an unsupported state. Options
include self-managed installation paths on [console.redhat.com](https://console.redhat.com) or
running [OpenShift Local](https://developers.redhat.com/products/openshift-local/overview) on your
machine.

You may also deploy OpenShift from a CI payload - refer to the appropriate
[release status page](https://docs.ci.openshift.org/docs/getting-started/useful-links/#services)
for instructions on how to deploy a cluster with a specific channel and payload.

After your cluster is installed, you will need to do the following:

1. Patch the cluster version so that you can launch your own builder image:

   ```sh
   $ oc patch clusterversion/version --patch '{"spec":{"overrides":[{"group":"v1", "kind":"ConfigMap", "namespace":"openshift-controller-manager-operator", "name":"openshift-controller-manager-images", "unmanaged":true}]}}' --type=merge
   ```

2. Alternatively, scale the Cluster Version Operator (CVO) deployment down to zero:

   ```sh
   $ oc scale --replicas 0 -n openshift-cluster-version deployments/cluster-version-operator
   ```

3. Patch the ConfigMap in the override above to instruct the cluster to use your builder image:

   ```sh
   $ oc patch configmap openshift-controller-manager-images -n openshift-controller-manager-operator --patch '{"data":{"builderImage":"<MYREPO>/<MYIMAGE>:<MYTAG>"}}' --type=merge
   ```

4. Wait for the openshift controller manager deployment to roll out the update (this can take a few
   minutes):

   ```sh
   $ oc get deployment controller-manager -n openshift-controller-manager -w
   ```

5. To test your changes end to end, trigger an OpenShift build via `oc start-build`. You can use
   one of the templates suggested in `oc new-app` to populate your project with a build.

6. To set your cluster back to its original state, either remove the `overrides` section added in
   step 1, or scale the CVO deployment back to previous count:

   ```sh
   $ oc scale --replicas 1 -n openshift-cluster-version deployments/cluster-version-operator
   ```

### E2E Test Suite

The OpenShift builder is considered a core component of the OpenShift Container Platform. The e2e
test suite run against each pull request is located in the
[openshift/origin](https://github.com/openshift/origin) repo. Tests whose description contains
`[Feature:Builds]` are run as part of the e2e suite, and are generally placed in the
`test/extended/builds` directory. Refer to the extended tests
[README](https://github.com/openshift/origin/blob/master/test/extended/README.md) for instructions
on how to develop and run these tests against your own cluster.
