#!/bin/bash

set -euo pipefail
INSTALL=$(which install)

install() {
    pushd "$WORKING_DIRECTORY"
    echo "Determining Kubernetes release"
    #RELEASE="$(_curl --location https://dl.k8s.io/release/stable.txt)"
    RELEASE="v1.21.0"
    echo "Downloading Kubernetes binaries"
    _curl --location --remote-name-all https://storage.googleapis.com/kubernetes-release/release/"$RELEASE"/bin/linux/"$ARCH"/{kubeadm,kubelet,kubectl}
    _curl --location https://github.com/kubernetes-sigs/cri-tools/releases/download/"$RELEASE"/crictl-"$RELEASE"-linux-"$ARCH".tar.gz | tar xz
    sudo "$INSTALL" -m 755 kubeadm kubelet kubectl crictl root/usr/bin
    echo "Downloading runc"
    _curl --location https://archlinuxarm.org/"$ARCH_FULL"/community/runc-1.0.1-2-"$ARCH_FULL".pkg.tar.xz | sudo tar xJv -C root usr
    echo "Downloading CRI-O"
    _curl --location https://archlinuxarm.org/"$ARCH_FULL"/community/cri-o-1.21.2-1-"$ARCH_FULL".pkg.tar.xz | sudo tar xJv -C root usr etc
    echo "Downloading conmon"
    _curl --location https://archlinuxarm.org/"$ARCH_FULL"/community/conmon-1:2.0.29-1-"$ARCH_FULL".pkg.tar.xz | sudo tar xJv -C root usr
    #echo "Downloading CNI plugins"
    #_curl --location https://archlinuxarm.org/"$ARCH_FULL"/community/cni-plugins-0.9.1-3-"$ARCH_FULL".pkg.tar.xz | sudo tar xJv -C root usr etc opt
    echo "Downloading containers common"
    _curl --location https://archlinuxarm.org/armv7h/community/containers-common-0.43.2-1-any.pkg.tar.xz | sudo tar xJv -C root usr etc var
    echo "Downloading conntrack"
    _curl --location https://archlinuxarm.org/"$ARCH_FULL"/extra/conntrack-tools-1.4.6-2-"$ARCH_FULL".pkg.tar.xz | sudo tar xJv -C root usr etc
    RELEASE_VERSION="v0.4.0"
    echo "Downloading kubelet systemd configuration"
    _curl --location --remote-name-all "https://raw.githubusercontent.com/kubernetes/release/$RELEASE_VERSION/cmd/kubepkg/templates/latest/deb/kubelet/lib/systemd/system/kubelet.service"
    sudo mkdir -p root/etc/systemd/system/kubelet.service.d
    echo "Downloading kubeadm systemd configuration"
    _curl --location --remote-name-all "https://raw.githubusercontent.com/kubernetes/release/$RELEASE_VERSION/cmd/kubepkg/templates/latest/deb/kubeadm/10-kubeadm.conf"
    sudo "$INSTALL" -D -m 644 10-kubeadm.conf root/etc/systemd/system/kubelet.service.d/10-kubeadm.conf
    cat <<EOF | sudo tee root/usr/lib/systemd/system-preset/50-kubeadm.preset
enable kubelet.service
enable kubeadm-join.path
enable crio.service
EOF
    sudo "$INSTALL" -D -m 644 kubelet.service root/etc/systemd/system/kubelet.service
    cat <<'EOF' | sudo tee root/etc/systemd/system/kubeadm-join.service
[Unit]
Description=Join a Kubernetes cluster with kubeadm
Wants=kubelet.service
After=kubelet.service
ConditionPathExists=!/var/lib/kubeadm/join.done

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/usr/bin/kubeadm join --config /var/lib/kubeadm/config.yaml
ExecStartPost=/usr/bin/touch /var/lib/kubeadm/join.done

[Install]
WantedBy=multi-user.target
EOF
    cat <<EOF | sudo tee root/etc/systemd/system/kubeadm-join.path
[Unit]
Description=Trigger the kubeadm-join unit when the configuration file changes

[Path]
PathChanged=/var/lib/kubeadm
MakeDirectory=yes

[Install]
WantedBy=multi-user.target
EOF
    cat <<EOF | sudo tee root/etc/modules-load.d/cri-o.conf
br_netfilter
overlay
EOF
    cat <<EOF | sudo tee root/etc/sysctl.d/cri-o.conf
net.bridge.bridge-nf-call-ip6tables = 1
net.bridge.bridge-nf-call-iptables = 1
net.ipv4.ip_forward = 1
EOF
    cat <<EOF | sudo tee root/etc/crio/crio.conf.d/10-onboard.conf
[crio.network]
plugin_dirs = [
	"/usr/lib/cni/",
	"/opt/cni/bin/",
]
EOF
    sudo mkdir -p root/etc/onboard
    cat <<EOF | sudo tee root/etc/onboard/10-kubeadm.yaml
checks:
- name: dns
  dns:
    value: api
- name: cluster
  systemd:
    unit: kubeadm-join.service
    description: Joining Cluster
values:
- name: api
  description: k8s API Server Endpoint
- name: ca_cert_hash
  description: k8s CA Certificate Hash
- name: token
  description: k8s Bootstrap Token
- name: registry
  description: Unqualified Container Registry
actions:
- name: containers-registries
  file:
    path: /etc/containers/registries.conf
    template: |
      unqualified-search-registries = ["{{.registry}}"]
- name: restart-crio
  systemd:
    unit: crio.service
    command: restart
- name: kubeadm-config
  file:
    path: /var/lib/kubeadm/config.yaml
    template: |
      apiVersion: kubeadm.k8s.io/v1beta2
      kind: JoinConfiguration
      discovery:
        bootstrapToken:
          apiServerEndpoint: {{.api}}
          token: {{.token}}
          caCertHashes:
          - {{.ca_cert_hash}}
      nodeRegistration:
        name: {{.hostname}}
      ---
      apiVersion: kubelet.config.k8s.io/v1beta1
      kind: KubeletConfiguration
      cgroupDriver: systemd
EOF
}

done-file() {
    echo /var/lib/kubeadm/join.done
}

kernel-command-line() {
    echo cgroup_enable=cpuset cgroup_enable=memory cgroup_memory=1
}
