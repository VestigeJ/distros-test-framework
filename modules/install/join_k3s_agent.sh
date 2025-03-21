#!/bin/bash

PS4='+(${LINENO}): '
set -e
trap 'echo "Error on line $LINENO: $BASH_COMMAND"' ERR

# Script args
node_os=${1}
server_ip=${2}
token=${3}
public_ip=${4}
private_ip=${5}
ipv6_ip=${6}
install_mode=${7}
version=${8}
channel=${9}
worker_flags=${10}
rhel_username=${12}
rhel_password=${13}

create_config() {
  hostname=$(hostname -f)
  mkdir -p /etc/rancher/k3s
  cat <<EOF >>/etc/rancher/k3s/config.yaml
token:  "$token"
node-label:
 - role-worker=true
EOF
}

update_config() {
  if [[ -n "$worker_flags" ]] && [[ "$worker_flags" == *":"* ]]; then
    echo -e "$worker_flags" >>/etc/rancher/k3s/config.yaml
  fi

  if [[ "$worker_flags" != *"cloud-provider-name"* ]] || [[ -z "$worker_flags" ]]; then
    if [ -n "$ipv6_ip" ] && [ -n "$public_ip" ] && [ -n "$private_ip" ]; then
      echo -e "node-external-ip: $public_ip,$ipv6_ip" >>/etc/rancher/k3s/config.yaml
      echo -e "node-ip: $private_ip,$ipv6_ip" >>/etc/rancher/k3s/config.yaml
    elif [ -n "$ipv6_ip" ]; then
      echo -e "node-external-ip: $ipv6_ip" >>/etc/rancher/k3s/config.yaml
      server_ip="[$server_ip]"
      hostname="$hostname-ag$RANDOM"
    else
      echo -e "node-external-ip: $public_ip" >>/etc/rancher/k3s/config.yaml
      echo -e "node-ip: $private_ip" >>/etc/rancher/k3s/config.yaml
    fi
  fi
  echo -e server: https://${server_ip}:6443 >>/etc/rancher/k3s/config.yaml
  echo -e node-name: "${hostname}" >>/etc/rancher/k3s/config.yaml
  cat /etc/rancher/k3s/config.yaml

  if [[ -n "$worker_flags" ]] && [[ "$worker_flags" == *"protect-kernel-defaults"* ]]; then
    cat /tmp/cis_worker_config.yaml >>/etc/rancher/k3s/config.yaml
    printf "%s\n" "vm.panic_on_oom=0" "vm.overcommit_memory=1" "kernel.panic=10" "kernel.panic_on_oops=1" "kernel.keys.root_maxbytes=25000000" >>/etc/sysctl.d/90-kubelet.conf
    sysctl -p /etc/sysctl.d/90-kubelet.conf
    systemctl restart systemd-sysctl
  fi
}

subscription_manager() {
  if [ "$node_os" = "rhel" ]; then
    subscription-manager register --auto-attach --username="$rhel_username" --password="$rhel_password" || echo "Failed to register or attach subscription."
    subscription-manager repos --enable=rhel-7-server-extras-rpms || echo "Failed to enable repositories on this Os."
  fi
}

disable_cloud_setup() {
  if [[ "$node_os" = *"rhel"* ]] || [[ "$node_os" = "centos8" ]] || [[ "$node_os" = *"oracle"* ]]; then
    if systemctl is-enabled --quiet nm-cloud-setup.service 2>/dev/null; then
      systemctl disable nm-cloud-setup.service
    else
      echo "nm-cloud-setup.service not found or not enabled"
    fi

    if systemctl is-enabled --quiet nm-cloud-setup.timer 2>/dev/null; then
      systemctl disable nm-cloud-setup.timer
    else
      echo "nm-cloud-setup.timer not found or not enabled"
    fi
  fi
}

install_k3s() {
  url="https://get.k3s.io"
  params="$install_mode=$version"
  if [[ -n "$channel" ]]; then
    params="$params INSTALL_K3S_CHANNEL=$channel"
  fi

  install_cmd="curl -sfL $url | $params sh -s - agent"
  
  if ! eval "$install_cmd"; then
    echo "Failed to install k3s-agent on node: $public_ip"
    exit 1
  fi

}

check_service() {
  if systemctl is-active --quiet k3s-agent; then
    echo "k3s-agent is running on node: $public_ip" 
  else
    echo "k3s-agent failed to start on node: $public_ip while joining server: $server_ip" 
    sudo journalctl -xeu k3s-agent.service | grep -i "error\|failed\|fatal"
    exit 1
  fi
}

install() {
  install_k3s
  sleep 10
  check_service
}

main() {
  create_config
  update_config
  subscription_manager
  disable_cloud_setup
  install
}
main "$@"
