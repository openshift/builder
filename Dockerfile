#
# This is the image that executes a Docker build inside Origin. It expects the
# following environment variables:
#
#   BUILD - JSON string containing the openshift build object
#
# This image expects to have the Docker socket bind-mounted into the container.
# If "/root/.dockercfg" is bind mounted in, it will use that as authorization to a
# Docker registry. It depends on bsdtar for extraction of binaries over STDIN.
#
# The standard name for this image is openshift/origin-docker-builder
#
FROM openshift/origin-release:golang-1.10
COPY . /go/src/github.com/openshift/builder
RUN cd /go/src/github.com/openshift/builder && \
    hack/build.sh

FROM docker.io/library/centos:7
LABEL io.k8s.display-name="OpenShift Origin Builder" \
      io.k8s.description="This is a component of OpenShift Origin and is responsible for executing image builds." \
      io.openshift.tags="openshift,builder"

RUN INSTALL_PKGS=" \
      which tar wget hostname sysvinit-tools util-linux \
      socat tree findutils lsof bind-utils \
      git tar bsdtar \
      " && \
    yum install -y ${INSTALL_PKGS} && \
    rpm -V ${INSTALL_PKGS} && \
    yum clean all

COPY --from=0 /go/src/github.com/openshift/builder/openshift-builder /usr/bin
RUN ln -s /usr/bin/openshift-builder /usr/bin/openshift-sti-build && \
    ln -s /usr/bin/openshift-builder /usr/bin/openshift-docker-build && \
    ln -s /usr/bin/openshift-builder /usr/bin/openshift-git-clone && \
    ln -s /usr/bin/openshift-builder /usr/bin/openshift-manage-dockerfile && \
    ln -s /usr/bin/openshift-builder /usr/bin/openshift-extract-image-content
