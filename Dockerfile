FROM registry.svc.ci.openshift.org/openshift/release:golang-1.10 AS builder
WORKDIR /go/src/github.com/openshift/builder
COPY . .
RUN hack/build.sh

FROM registry.svc.ci.openshift.org/openshift/origin-v4.0:base
RUN INSTALL_PKGS=" \
      which tar wget hostname sysvinit-tools util-linux \
      socat tree findutils lsof bind-utils \
      git tar bsdtar \
      " && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all
COPY --from=builder /go/src/github.com/openshift/builder/openshift-builder /usr/bin/
COPY imagecontent/policy.json /etc/containers/
COPY imagecontent/registries.conf /etc/containers/
COPY imagecontent/storage.conf /etc/containers/
RUN mkdir /var/cache/blobs

RUN ln -s /usr/bin/openshift-builder /usr/bin/openshift-sti-build && \
    ln -s /usr/bin/openshift-builder /usr/bin/openshift-docker-build && \
    ln -s /usr/bin/openshift-builder /usr/bin/openshift-git-clone && \
    ln -s /usr/bin/openshift-builder /usr/bin/openshift-manage-dockerfile && \
    ln -s /usr/bin/openshift-builder /usr/bin/openshift-extract-image-content
LABEL io.k8s.display-name="OpenShift Builder" \
      io.k8s.description="This is a component of OpenShift and is responsible for executing image builds." \
      io.openshift.tags="openshift,builder"
