# Contributing to the Openshift Builder

The OpenShift Builder image drives [OpenShift Builds](https://docs.okd.io/latest/dev_guide/builds/index.html),
leveraging [Buildah](https://github.com/containers/buildah) and [Source-To-Image](https://github.com/openshift/source-to-image)
to turn source code into images that can be deployed on Kubernetes.

## Getting the code

To get started, [fork](https://help.github.com/articles/fork-a-repo) the [openshift/builder](https://github.com/openshift/builder) repo.

## Developing

### Testing on an OpenShift Cluster

The easiest way to test your changes is to launch an OpenShift 4.x cluster.
First, go to [try.openshift.com](https://try.openshift.com) to obtain a pull secret and download the installer.
Follow the instructions to launch a cluster on AWS.

If you want the latest `openshift-install` and `oc` clients, go to the [openshift-release](https://openshift-release.svc.ci.openshift.org/) 
page, select the channel and build you wish to install, and download the respective `oc` and `openshift-installer` binaries.
There are three types of channels you can obtain the installer from:

1. `4-stable` - these are stable releases of OpenShift 4, corresponding to GA or beta releases.
2. `4.x.0-nightly` - nightly development releases, with payloads published to quay.io.
3. `4.x.0-ci` - bleeding-edge releases published to the OpenShift CI imagestreams.

**Note**: Installs from the `4.x.0-ci` channel require a pull secret to `registry.ci.openshift.org`, which is only available to Red Hat OpenShift developers. For detailed steps, refer [Obtain Pull Secret for Internal registry](https://github.com/openshift/builder/blob/master/CONTRIBUTING.md#obtaining-pull-secret-for-internal-registry).

After your cluster is installed, you will need to do the following:

1. Patch the cluster version so that you can launch your own builder image:

```
$ oc patch clusterversion/version --patch '{"spec":{"overrides":[{"group":"v1", "kind":"ConfigMap", "namespace":"openshift-controller-manager-operator", "name":"openshift-controller-manager-images", "unmanaged":true}]}}' --type=merge
```

or, scale down the CVO pod replica:

```
$ oc scale --replicas 0 -n openshift-cluster-version deployments/cluster-version-operator
```

2. Make your code changes and build the binary with `make build`.
> While running `make build` if you come across any error something like
` OS_GIT_MAJOR: unbound variable`, make sure that you have pulled all the tags from the [openshift/builder](https://github.com/openshift/builder) repo.
 You can use `git fetch upstream --tags` to pull all the tags.

3. Build the image using the `Dockerfile.dev` file, giving it a unique tag:

```
$ make build-devel-image IMAGE=<MYREPO>/<MYIMAGE> TAG=<MYTAG> 
```

or if you are using `buildah`:

```
$ buildah bud -t <MYREPO>/<MYIMAGE>:<MYTAG> -f Dockerfile.dev .
```

4. Push the image to a registry accessible from the cluster (e.g. your repository on quay.io).
5. Patch the ConfigMap in the override above to instruct the cluster to use your builder image:

```
$ oc patch configmap openshift-controller-manager-images -n openshift-controller-manager-operator --patch '{"data":{"builderImage":"<MYREPO>/<MYIMAGE>:<MYTAG>"}}' --type=merge
```

6. Watch the openshift controller manager pods rollout (this can take a few minutes):

```
$ oc get deployment controller-manager -n openshift-controller-manager -w
```

7. Trigger an OpenShift build via `oc start-build`. You can use one of the templates suggested in `oc new-app` to populate your project with a build.

8. To set your cluster back to its original state, either remove the `overrides` section added in step 1, or, scale up the CVO pods back to previous count:

```
$ oc scale --replicas 1 -n openshift-cluster-version deployments/cluster-version-operator

```

### Obtaining Pull Secret for Internal registry

1. Connect to the VPN, and log in using Internal SSO to the [OpenShift Cluster](https://console-openshift-console.apps.ci.l2s4.p1.openshiftapps.com/).

2. Use `Copy Login Command` to login to the cluster using the `token`:

```
$ oc login --token=<xxx> --server=<server_url>:<port>
```

3. Once logged in to the cluster successfully, login to the [registry](registry.ci.openshift.org):

```
$ oc registry login
```

4. Now, you should be able to pull the images from `registry.ci.openshift.org`.
5. You can also build the image now using `Dockerfile.rhel8` in `step 3` of [Testing on an OpenShift Cluster](https://github.com/openshift/builder/blob/master/CONTRIBUTING.md#testing-on-an-openshift-cluster).

## Submitting a Pull Request

Once you are satisfied with your code changes, you may submit a pull request to the [openshift/builder](https://github.com/openshift/builder) repo.
A member of the OpenShift Developer Experience team will evaluate your change for approval.

## End-to-End Testing

The OpenShift builder is considered a core component of the OpenShift Container Platform.
The e2e test suite run against each pull request is located in the [openshift/origin](https://github.com/openshift/origin) repo.
Tests whose description contains `[Feature:Builds]` are run as part of the e2e suite, and are generally placed in the 
`test/extended/builds` directory.
Refer to the extended tests [README](https://github.com/openshift/origin/blob/master/test/extended/README.md) 
for instructions on how to develop and run these tests against your own cluster.
