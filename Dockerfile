FROM registry.svc.ci.openshift.org/openshift/release:golang-1.12 AS builder
WORKDIR /go/src/github.com/openshift/builder
COPY . .
RUN hack/build.sh

FROM registry.svc.ci.openshift.org/openshift/origin-v4.0:base
# TODO: Add fuse-overlayfs once we build off of RHEL-8 UBI

ENV BASH_ENV=/usr/local/bin/scl_enable \
    ENV=/usr/local/bin/scl_enable \
    PROMPT_COMMAND=". /usr/local/bin/scl_enable" \
    PATH=/opt/rh/rh-git29/root/usr/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

RUN INSTALL_PKGS=" \
      bind-utils bsdtar findutils rh-git29 hostname lsof socat \
      sysvinit-tools tar tree util-linux wget which \
      " && \
    yum install -y --setopt=skip_missing_names_on_install=False $INSTALL_PKGS && \
    yum clean all
COPY --from=builder /go/src/github.com/openshift/builder/openshift-builder /usr/bin/
COPY imagecontent/policy.json /etc/containers/
COPY imagecontent/registries.conf /etc/containers/
COPY imagecontent/storage.conf /etc/containers/
COPY imagecontent/scl_enable /usr/local/bin/scl_enable

RUN mkdir -p /var/cache/blobs \
    /var/lib/shared/overlay-images \
    /var/lib/shared/overlay-layers && \
    touch /var/lib/shared/overlay-images/images.lock \
    /var/lib/shared/overlay-layers/layers.lock

RUN ln -s /usr/bin/openshift-builder /usr/bin/openshift-sti-build && \
    ln -s /usr/bin/openshift-builder /usr/bin/openshift-docker-build && \
    ln -s /usr/bin/openshift-builder /usr/bin/openshift-git-clone && \
    ln -s /usr/bin/openshift-builder /usr/bin/openshift-manage-dockerfile && \
    ln -s /usr/bin/openshift-builder /usr/bin/openshift-extract-image-content
LABEL io.k8s.display-name="OpenShift Builder" \
      io.k8s.description="This is a component of OpenShift and is responsible for executing image builds." \
      io.openshift.tags="openshift,builder"
