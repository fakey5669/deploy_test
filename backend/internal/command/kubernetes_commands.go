package command

import (
	"fmt"
	"strings"
	"time"
)

// 쿠버네티스 관련 액션 상수 정의
const (
	// 클러스터 관리
	ActionInstallLoadBalancer = "installLoadBalancer"
	ActionGetNodeStatus       = "getNodeStatus"
	ActionInstallFirstMaster  = "installFirstMaster"
	ActionJoinMaster          = "joinMaster"
	ActionJoinWorker          = "joinWorker"
	// ActionRemoveNode          = "removeNode"
	ActionDeleteWorker             = "deleteWorker"
	ActionDeleteMaster             = "deleteMaster"
	ActionGetNamespaceAndPodStatus = "getNamespaceAndPodStatus" // 액션 추가

	// // 인프라 CRUD
	// ActionGetInfras    = "getInfras"
	// ActionGetInfraById = "getInfraById"
	// ActionCreateInfra  = "createInfra"
	// ActionUpdateInfra  = "updateInfra"
	// ActionDeleteInfra  = "deleteInfra"

	// // 서버 CRUD
	// ActionGetServers    = "getServers"
	// ActionGetServerById = "getServerById"
	// ActionCreateServer  = "createServer"
	// ActionUpdateServer  = "updateServer"
	// ActionDeleteServer  = "deleteServer"
	// ActionRestartServer = "restartServer"
	// ActionStartServer   = "startServer"
	// ActionStopServer    = "stopServer"
)

// RegisterKubernetesCommands는 쿠버네티스 관련 명령어 템플릿을 등록합니다
func RegisterKubernetesCommands(manager *CommandManager) {
	// LoadBalancer 설치 명령어
	manager.RegisterCommand(ActionInstallLoadBalancer, CommandTemplate{
		ValidateFunc: validateLoadBalancerParams,
		PrepareFunc:  prepareLoadBalancerCommands,
	})

	// 노드 상태 확인 명령어
	manager.RegisterCommand(ActionGetNodeStatus, CommandTemplate{
		PrepareFunc: prepareNodeStatusCommands,
	})

	// 첫번째 마스터 노드 설치 명령어
	manager.RegisterCommand(ActionInstallFirstMaster, CommandTemplate{
		ValidateFunc: validateFirstMasterParams,
		PrepareFunc:  prepareFirstMasterCommands,
	})

	// 마스터 노드 조인 명령어
	manager.RegisterCommand(ActionJoinMaster, CommandTemplate{
		ValidateFunc: validateJoinMasterParams,
		PrepareFunc:  prepareJoinMasterCommandsWrapper,
	})

	// 워커 노드 조인 명령어
	manager.RegisterCommand(ActionJoinWorker, CommandTemplate{
		ValidateFunc: validateJoinWorkerParams,
		PrepareFunc:  prepareJoinWorkerCommands,
	})

	// 워커 노드 조인 명령어
	manager.RegisterCommand(ActionDeleteWorker, CommandTemplate{
		ValidateFunc: validateDeleteWorkerParams,
		PrepareFunc:  prepareDeleteWorkerCommands,
	})

	// 마스터 노드 삭제 명령어 등록
	manager.RegisterCommand(ActionDeleteMaster, CommandTemplate{
		ValidateFunc: validateDeleteMasterParams,
		PrepareFunc:  prepareDeleteMasterCommands,
	})

	// HAProxy 설정 업데이트 명령어
	manager.RegisterCommand("updateHAProxy", CommandTemplate{
		PrepareFunc: prepareHAProxyUpdateCommands,
	})

	// 네임스페이스 및 파드 상태 확인 명령어 등록
	manager.RegisterCommand(ActionGetNamespaceAndPodStatus, CommandTemplate{
		PrepareFunc: prepareGetNamespaceAndPodStatusCommands,
	})
}

// LoadBalancer 관련 함수들
func validateLoadBalancerParams(params map[string]interface{}) error {
	// 필요한 파라미터 검증
	if _, exists := params["server_id"]; !exists {
		return fmt.Errorf("server_id 파라미터가 필요합니다")
	}
	return nil
}

// getStringParameter는 인터페이스 타입의 파라미터에서 문자열 값을 추출합니다
func getStringParameter(param interface{}) string {
	if param == nil {
		return ""
	}
	if str, ok := param.(string); ok {
		return str
	}
	return ""
}

func prepareLoadBalancerCommands(params map[string]interface{}) ([]string, error) {
	password := getStringParameter(params["password"])

	// HAProxy 설정
	haproxyConfig := `
global
    log /dev/log    local0
    log /dev/log    local1 notice
    chroot /var/lib/haproxy
    stats socket /run/haproxy/admin.sock mode 660 level admin expose-fd listeners
    stats timeout 30s
    user haproxy
    group haproxy
    daemon

defaults
    log     global
    mode    tcp
    option  tcplog
    option  dontlognull
    timeout connect 5000
    timeout client  50000
    timeout server  50000

frontend kubernetes-frontend
    bind *:6444
    mode tcp
    default_backend kubernetes-backend

backend kubernetes-backend
    mode tcp
    balance roundrobin
    option tcp-check
`

	// 설치 명령어들을 개별 문자열로 분리
	installCommands := []string{
		fmt.Sprintf("echo '%s' | sudo -S apt-get update > /tmp/haproxy_install.log 2>&1", password),
		fmt.Sprintf("echo '%s' | sudo -S apt-get install -y haproxy >> /tmp/haproxy_install.log 2>&1", password),
		fmt.Sprintf("echo '%s' | sudo -S touch /etc/haproxy/haproxy.cfg >> /tmp/haproxy_install.log 2>&1", password),
		fmt.Sprintf("echo '%s' | sudo -S cp /etc/haproxy/haproxy.cfg /etc/haproxy/haproxy.cfg.bak >> /tmp/haproxy_install.log 2>&1", password),
		fmt.Sprintf("echo '%s' | sudo -S bash -c 'echo \"%s\" > /etc/haproxy/haproxy.cfg' >> /tmp/haproxy_install.log 2>&1", password, haproxyConfig),
		fmt.Sprintf("echo '%s' | sudo -S systemctl restart haproxy || echo '%s' | sudo -S service haproxy restart >> /tmp/haproxy_install.log 2>&1", password, password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl enable haproxy || echo '%s' | sudo -S service haproxy enable >> /tmp/haproxy_install.log 2>&1", password, password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl status haproxy || echo '%s' | sudo -S service haproxy status >> /tmp/haproxy_install.log 2>&1", password, password),
		"echo '로드 밸런서 설치 완료' >> /tmp/haproxy_install.log",
		"local_ip=$(hostname -I | awk '{print $1}') && echo \"로드 밸런서 IP: $local_ip\" >> /tmp/haproxy_install.log",
		fmt.Sprintf("echo '%s' | sudo -S bash -c 'local_ip=$(hostname -I | cut -d\" \" -f1); echo \"LOAD_BALANCER_IP=$local_ip\" > /tmp/load_balancer_info'", password),
	}

	// 로그 확인 명령어들을 개별 문자열로 분리
	logCheckCommands := []string{
		fmt.Sprintf("echo '%s' | sudo -S ls -la /tmp/haproxy_install.log 2>/dev/null && echo 'LOG_EXISTS=true' || echo 'LOG_EXISTS=false'", password),
		fmt.Sprintf("echo '%s' | sudo -S grep -q '로드 밸런서 설치 완료' /tmp/haproxy_install.log 2>/dev/null && echo 'INSTALL_COMPLETE=true' || echo 'INSTALL_COMPLETE=false'", password),
		fmt.Sprintf("echo '%s' | sudo -S grep -i 'error\\|failed\\|실패' /tmp/haproxy_install.log 2>/dev/null && echo 'HAS_ERRORS=true' || echo 'HAS_ERRORS=false'", password),
		fmt.Sprintf("echo '%s' | sudo -S cat /tmp/haproxy_install.log 2>/dev/null || echo '로그 파일을 읽을 수 없습니다.'", password),
	}

	// 모든 명령어를 하나의 슬라이스로 합치기
	allCommands := installCommands
	allCommands = append(allCommands, logCheckCommands...)

	return allCommands, nil
}

// 노드 상태 확인 관련 함수
func prepareNodeStatusCommands(params map[string]interface{}) ([]string, error) {
	// 노드 타입 가져오기
	nodeType := ""
	if typeVal, ok := params["type"].(string); ok {
		nodeType = strings.ToLower(typeVal)
	}

	// 서버 이름 가져오기
	serverName := ""
	if nameVal, ok := params["server_name"].(string); ok && nameVal != "" {
		serverName = nameVal
	}

	// 비밀번호 가져오기
	password := ""
	if pwdVal, ok := params["password"].(string); ok && pwdVal != "" {
		password = pwdVal
	}

	// 단일 통합 명령어 생성
	var cmd string

	switch nodeType {
	case "ha":
		cmd = "echo '===START==='; " +
			"if dpkg -l | grep -q haproxy; then echo 'INSTALLED=true'; else echo 'INSTALLED=false'; fi; " +
			"if systemctl status haproxy | grep -q 'Active: active (running)'; then echo 'RUNNING=true'; else echo 'RUNNING=false'; fi; " +
			"echo '===END==='"
	case "master":
		// 서버 이름이 제공되지 않은 경우 현재 호스트 이름 사용
		hostVar := serverName
		if hostVar == "" {
			hostVar = "$(hostname)"
		}

		cmd = "echo '===START==='; " +
			"if command -v kubectl >/dev/null 2>&1 && command -v kubelet >/dev/null 2>&1; then echo 'INSTALLED=true'; else echo 'INSTALLED=false'; fi; " +
			"if systemctl status kubelet 2>/dev/null | grep -q 'Active: active (running)'; then echo 'KUBELET_RUNNING=true'; else echo 'KUBELET_RUNNING=false'; fi; " +
			fmt.Sprintf("echo \"hostname=%s\"; ", hostVar)

		// 비밀번호가 제공된 경우 사용
		if password != "" {
			cmd += fmt.Sprintf("if echo '%s' | sudo -S kubectl get nodes --no-headers 2>/dev/null | grep -E \"%s.*control-plane|%s.*master\"; then echo 'IS_MASTER=true'; else echo 'IS_MASTER=false'; fi; ",
				password, hostVar, hostVar) +
				fmt.Sprintf("if echo '%s' | sudo -S kubectl get nodes --no-headers 2>/dev/null | grep \"%s\" | grep -vE \"control-plane|master\"; then echo 'IS_WORKER=true'; else echo 'IS_WORKER=false'; fi; ",
					password, hostVar) +
				fmt.Sprintf("if echo '%s' | sudo -S kubectl get nodes --no-headers 2>/dev/null | grep -q \"%s\"; then echo 'NODE_REGISTERED=true'; else echo 'NODE_REGISTERED=false'; fi; ",
					password, hostVar) +
				fmt.Sprintf("echo '%s' | sudo -S kubectl get nodes -o wide 2>/dev/null | grep \"%s\" || echo 'NODE_STATUS=NotFound'; ",
					password, hostVar)
		} else {
			cmd += fmt.Sprintf("if kubectl get nodes --no-headers 2>/dev/null | grep -E \"%s.*control-plane|%s.*master\"; then echo 'IS_MASTER=true'; else echo 'IS_MASTER=false'; fi; ",
				hostVar, hostVar) +
				fmt.Sprintf("if kubectl get nodes --no-headers 2>/dev/null | grep \"%s\" | grep -vE \"control-plane|master\"; then echo 'IS_WORKER=true'; else echo 'IS_WORKER=false'; fi; ",
					hostVar) +
				fmt.Sprintf("if kubectl get nodes --no-headers 2>/dev/null | grep -q \"%s\"; then echo 'NODE_REGISTERED=true'; else echo 'NODE_REGISTERED=false'; fi; ",
					hostVar) +
				fmt.Sprintf("kubectl get nodes -o wide 2>/dev/null | grep \"%s\" || echo 'NODE_STATUS=NotFound'; ",
					hostVar)
		}
		cmd += "echo '===END==='"

	case "worker":
		// 서버 이름이 제공되지 않은 경우 현재 호스트 이름 사용
		hostVar := serverName
		if hostVar == "" {
			hostVar = "$(hostname)"
		}

		cmd = "echo '===START==='; " +
			"if command -v kubectl >/dev/null 2>&1 && command -v kubelet >/dev/null 2>&1; then echo 'INSTALLED=true'; else echo 'INSTALLED=false'; fi; " +
			"if systemctl status kubelet 2>/dev/null | grep -q 'Active: active (running)'; then echo 'KUBELET_RUNNING=true'; else echo 'KUBELET_RUNNING=false'; fi; " +
			fmt.Sprintf("echo \"hostname=%s\"; ", hostVar) +
			"echo '===END==='"
	default:
		return nil, fmt.Errorf("지원하지 않는 노드 타입: %s", nodeType)
	}

	return []string{cmd}, nil
}

// 첫번째 마스터 노드 설치 관련 함수
func validateFirstMasterParams(params map[string]interface{}) error {
	// 필요한 파라미터 검증
	if _, exists := params["password"]; !exists {
		return fmt.Errorf("password 파라미터가 필요합니다")
	}
	return nil
}

func prepareFirstMasterCommands(params map[string]interface{}) ([]string, error) {
	password := getStringParameter(params["password"])
	lbIP := getStringParameter(params["lb_ip"])
	serverName := getStringParameter(params["server_name"])

	// 포트 기본값 설정 (기본값: 6443)
	port := "6443"

	// POD CIDR 설정 (기본값: 10.10.0.0/16)
	podCIDR := getStringParameter(params["pod_network_cidr"])
	if podCIDR == "" {
		podCIDR = "10.10.0.0/16"
	}

	// 2. 쿠버네티스 마스터 노드 설치 스크립트
	installScript := fmt.Sprintf(`#!/bin/bash

set -euxo pipefail

# 포트 설정
PORT="%s"
echo "사용할 포트: $PORT"

# 현재 IP 주소 감지
local_ip=$(ip -4 addr show | awk '/inet / && $2 ~ /^192/ {print $2}' | cut -d/ -f1 | head -n 1)
echo "감지된 로컬 IP 주소: $local_ip"

# IP 감지 실패 시 대체 방법 사용
if [ -z "$local_ip" ]; then
  echo "첫 번째 방법으로 IP 감지 실패, 대체 방법 시도..."
  local_ip=$(hostname -I | awk '{print $1}')
  echo "대체 방법으로 감지된 IP: $local_ip"
fi

# 로드 밸런서 IP 설정
LB_IP="%s"
if [ -z "$LB_IP" ]; then
  LB_IP=$local_ip
  echo "로드 밸런서 IP가 제공되지 않아 로컬 IP를 사용합니다."
fi
echo "로드 밸런서 IP: $LB_IP"

# 서버 이름 설정 (DB에서 가져온 값)
SERVER_NAME="%s"
echo "DB에서 가져온 서버 이름: $SERVER_NAME"

# 현재 사용자 확인
CURRENT_USER=$(whoami)
USER_HOME=$(eval echo ~$CURRENT_USER)
echo "현재 사용자: $CURRENT_USER"
echo "사용자 홈 디렉토리: $USER_HOME"

# 현재 우분투 버전 감지
UBUNTU_VERSION=$(lsb_release -rs)
echo "감지된 우분투 버전: $UBUNTU_VERSION"

# 우분투 버전에 따라 쿠버네티스 버전 선택
if [ "$(echo "$UBUNTU_VERSION >= 22.04" | bc)" -eq 1 ]; then
  # Ubuntu 22.04 이상은 쿠버네티스 1.28 이상 지원
  K8S_VERSION="1.30"
elif [ "$(echo "$UBUNTU_VERSION >= 20.04" | bc)" -eq 1 ]; then
  # Ubuntu 20.04는 쿠버네티스 1.19 ~ 1.29 지원
  K8S_VERSION="1.29"
elif [ "$(echo "$UBUNTU_VERSION >= 18.04" | bc)" -eq 1 ]; then
  # Ubuntu 18.04는 쿠버네티스 1.17 ~ 1.24 지원
  K8S_VERSION="1.24"
else
  # 이전 버전은 쿠버네티스 1.19 사용
  K8S_VERSION="1.19"
fi

echo "선택된 쿠버네티스 버전: $K8S_VERSION"

# 새로운 쿠버네티스 설치 시작
echo "새로운 쿠버네티스 설치 시작..."

sudo swapoff -a

(crontab -l 2>/dev/null; echo "@reboot /sbin/swapoff -a") | crontab - || true

sudo apt-get update -y

# 필수 패키지 설치
sudo apt-get install -y curl apt-transport-https ca-certificates gnupg lsb-release

# 쿠버네티스 버전 설정
VERSION="$K8S_VERSION"

cat <<EOF | sudo tee /etc/modules-load.d/containerd.conf
overlay
br_netfilter
EOF

sudo modprobe overlay
sudo modprobe br_netfilter

cat <<EOF | sudo tee /etc/sysctl.d/99-kubernetes-cri.conf
net.bridge.bridge-nf-call-iptables  = 1
net.ipv4.ip_forward                 = 1
net.bridge.bridge-nf-call-ip6tables = 1
EOF

sudo sysctl --system

# containerd 설치
sudo apt-get update
sudo apt-get install -y containerd

# containerd 설정
sudo mkdir -p /etc/containerd
containerd config default | sudo tee /etc/containerd/config.toml > /dev/null
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml
sudo systemctl restart containerd
sudo systemctl enable containerd

echo "Container runtime (containerd) installed successfully"

sudo apt-get update
sudo apt-get install -y apt-transport-https ca-certificates curl etcd-client

# 문제가 될 수 있는 저장소가 있는 경우 제거 (Ubuntu 20.04의 경우)
if [ "$(echo "$UBUNTU_VERSION >= 20.04" | bc)" -eq 1 ] && [ "$(echo "$UBUNTU_VERSION < 22.04" | bc)" -eq 1 ]; then
  echo "Ubuntu 20.04에서 불필요한 OpenSUSE 저장소 제거 중..."
  sudo rm -f /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list 2>/dev/null || true
  sudo rm -f /etc/apt/sources.list.d/devel:kubic:libcontainers:stable:cri-o:*.list 2>/dev/null || true
  
  sudo apt-get update
fi

# 쿠버네티스 저장소 설정 (우분투 버전에 따라 다른 방법 사용)
sudo mkdir -p /etc/apt/keyrings

# 직접 키 다운로드 및 설치 (--batch 및 --yes 옵션 추가)
curl -fsSL https://pkgs.k8s.io/core:/stable:/v$VERSION/deb/Release.key | sudo gpg --batch --yes --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
sudo chmod a+r /etc/apt/keyrings/kubernetes-apt-keyring.gpg

# 저장소 추가
echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v$VERSION/deb/ /" | sudo tee /etc/apt/sources.list.d/kubernetes.list

# 대체 방법: 직접 키 다운로드 및 설치
if [ ! -s /etc/apt/keyrings/kubernetes-apt-keyring.gpg ]; then
  # 첫 번째 방법이 실패한 경우 대체 방법 시도
  echo "첫 번째 저장소 설정 방법이 실패했습니다. 대체 방법을 시도합니다..."
  
  # 우분투 버전에 따라 다른 방법 사용
  sudo mkdir -p /etc/apt/keyrings
  curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo gpg --batch --yes --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
  sudo chmod a+r /etc/apt/keyrings/kubernetes-apt-keyring.gpg
  echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://apt.kubernetes.io/ kubernetes-xenial main" | sudo tee /etc/apt/sources.list.d/kubernetes.list
  
  # 만약 위 방법이 실패하고 Ubuntu 20.04 이하라면 예전 방식 시도
  if [ $? -ne 0 ] && [ "$(echo "$UBUNTU_VERSION < 22.04" | bc)" -eq 1 ]; then
    echo "새로운 방식이 실패했습니다. Ubuntu $UBUNTU_VERSION에서 예전 방식을 시도합니다..."
    curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key add -
    echo "deb https://apt.kubernetes.io/ kubernetes-xenial main" | sudo tee /etc/apt/sources.list.d/kubernetes.list
  fi
fi

sudo apt-get update -y || true

# 자동 응답을 위한 설정 (대화형 프롬프트 비활성화)
# DEBIAN_FRONTEND 환경 변수 설정
export DEBIAN_FRONTEND=noninteractive

# 특정 버전 설치
if [ "$K8S_VERSION" = "1.19" ]; then
  if ! sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" kubelet=1.19.16-00 kubeadm=1.19.16-00 kubectl=1.19.16-00; then
    echo "특정 버전 설치 실패, 와일드카드로 시도합니다..."
    sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" kubelet=1.19\* kubeadm=1.19\* kubectl=1.19\*
  fi
elif [ "$K8S_VERSION" = "1.24" ]; then
  if ! sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" kubelet=1.24.17-00 kubeadm=1.24.17-00 kubectl=1.24.17-00; then
    echo "특정 버전 설치 실패, 와일드카드로 시도합니다..."
    sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" kubelet=1.24\* kubeadm=1.24\* kubectl=1.24\*
  fi
elif [ "$K8S_VERSION" = "1.29" ]; then
  if ! sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" kubelet=1.29.0-00 kubeadm=1.29.0-00 kubectl=1.29.0-00; then
    echo "특정 버전 설치 실패, 와일드카드로 시도합니다..."
    sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" kubelet=1.29\* kubeadm=1.29\* kubectl=1.29\*
  fi
else
  # 최신 버전 설치
  sudo apt-get install -y kubelet kubeadm kubectl
fi

sudo apt-mark hold kubelet kubeadm kubectl

sudo apt-get update -y
sudo apt-get install -y jq bc

# kubelet 서비스 활성화
sudo systemctl enable kubelet

cat > /tmp/kubelet_config << EOF
KUBELET_EXTRA_ARGS=--node-ip=$local_ip
EOF
sudo mv /tmp/kubelet_config /etc/default/kubelet

########

MASTER_IP=$local_ip
IP_NO_DOT=$(echo "$local_ip" | sed "s/\./-/g")
POD_CIDR="10.10.0.0/16"

# 포트 상태 확인
echo "포트 $PORT 상태 확인 중..."
if netstat -tuln | grep ":$PORT " > /dev/null; then
  echo "경고: 포트 $PORT가 이미 사용 중입니다."
  # 포트가 사용 중인 프로세스 확인
  echo "포트 $PORT를 사용 중인 프로세스:"
  sudo lsof -i :$PORT || true
  sudo netstat -tulnp | grep ":$PORT " || true
fi

echo "쿠버네티스 마스터 노드 설치 시작 (포트: $PORT)..."
sudo kubeadm config images pull 

echo "Preflight Check Passed: Downloaded All Required Images"

# 포트가 이미 사용 중인 경우 무시하고 진행
sudo kubeadm init --pod-network-cidr=$POD_CIDR --node-name "$SERVER_NAME" --control-plane-endpoint "$LB_IP:6444"  --upload-certs

# Kubernetes config 디렉토리 생성
mkdir -p $HOME/.kube
# kubeconfig 파일 복사
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
# 사용자에게 권한 부여
sudo chown $(id -u):$(id -g) $HOME/.kube/config

# 환경 변수 설정
export KUBECONFIG=$USER_HOME/.kube/config

# 현재 사용자의 .bashrc 파일에 환경 변수 추가
echo 'export KUBECONFIG=$HOME/.kube/config' >> $USER_HOME/.bashrc

# 컨트롤 플레인 노드의 테인트 제거
echo "컨트롤 플레인 노드의 테인트 제거 중..."
kubectl taint nodes --all node-role.kubernetes.io/control-plane- || true
kubectl taint nodes --all node-role.kubernetes.io/master- || true

# API 서버가 완전히 시작될 때까지 대기
echo "API 서버가 시작될 때까지 대기 중..."
sleep 30

# Calico 네트워크 플러그인 설치
curl https://raw.githubusercontent.com/projectcalico/calico/v3.25.0/manifests/calico.yaml -O
kubectl apply -f calico.yaml

# 인그레스 컨트롤러 설치
echo "인그레스 컨트롤러 매니페스트 다운로드 및 수정 중..."
curl -o ingress-nginx.yaml https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
kubectl apply -f ingress-nginx.yaml

# 노드에 레이블 추가
sudo kubectl label node $SERVER_NAME ingress-ready=true

# 모든 파드가 실행될 때까지 대기
echo "모든 파드가 실행될 때까지 대기 중..."
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=180s || true

# 설치 완료 후 현재 사용자를 위한 환경 변수 설정 안내
echo "쿠버네티스 마스터 노드 설치가 완료되었습니다."
echo ""
echo "$CURRENT_USER 사용자로 kubectl을 사용하려면 다음 명령을 실행하세요:"
echo "export KUBECONFIG=$USER_HOME/.kube/config"
echo ""
echo "또는 다음 명령을 ~/.bashrc 파일에 추가하세요:"
echo "echo 'export KUBECONFIG=$USER_HOME/.kube/config' >> $USER_HOME/.bashrc"
echo "설치 완료"
`, port, lbIP, serverName)

	// 마스터 노드 설치 명령어 배열 생성
	installCommands := []string{
		// 1. 스크립트를 파일로 저장
		fmt.Sprintf("cat > /tmp/install_k8s.sh << 'EOL'\n%s\nEOL", installScript),
		// 2. 실행 권한 부여
		"chmod +x /tmp/install_k8s.sh",
		// 3. 로그 파일에 출력 저장하면서 스크립트 실행 (sudo 권한으로)
		fmt.Sprintf("echo '%s' | sudo -S bash /tmp/install_k8s.sh > /tmp/k8s_install.log 2>&1 & echo $! > /tmp/k8s_install.pid", password),
		// 4. 설치 시작 확인
		"echo '쿠버네티스 설치가 백그라운드에서 시작되었습니다. 로그 파일: /tmp/k8s_install.log, PID: '$(cat /tmp/k8s_install.pid)",
		// 5. 백그라운드에서 로그 모니터링 및 join 명령어 추출 스크립트 실행
		fmt.Sprintf(`nohup bash -c "
			# 설치가 완료될 때까지 대기 (최대 30분)
			max_wait=1800
			elapsed=0
			while [ \$elapsed -lt \$max_wait ]; do
				if grep -q "설치 완료" /tmp/k8s_install.log 2>/dev/null; then
					echo "쿠버네티스 설치가 완료되었습니다. join 명령어를 추출합니다."
					
					# 로그 파일에서 주요 부분 추출
					echo "로그 파일에서 worker 노드 join 지침 찾기..."
					
					# 여러 가지 방법으로 worker 노드용 join 명령어 추출 시도
					# 방법 1: "Then you can join any number of worker nodes" 이후 부분에서 multi-line 텍스트 추출
					worker_join_cmd=$(grep -A 15 "Then you can join any number of worker nodes" /tmp/k8s_install.log | grep -A 3 "kubeadm join" | grep -v "You can now join" | sed -e '/^$/,\$d' | tr '\n' ' ' | sed -e 's/\\//g' | sed -e 's/^[[:space:]]*//' | sed -e 's/[[:space:]]*\$//')
					
					# 방법 2: 'kubeadm join'으로 시작하는 두 줄 추출해서 공백 제거 후 합치기
					if [ -z "$worker_join_cmd" ]; then
						echo "방법 1 실패, 대체 방법 시도 중..."
						worker_join_start=$(grep -n "kubeadm join" /tmp/k8s_install.log | grep -v "control-plane" | tail -1 | cut -d: -f1)
						if [ ! -z "$worker_join_start" ]; then
							worker_join_cmd=$(sed -n "${worker_join_start},$(($worker_join_start+1))p" /tmp/k8s_install.log | tr -d '\\' | tr '\n' ' ' | sed -e 's/^[[:space:]]*//' | sed -e 's/[[:space:]]*\$//')
							echo "방법 2로 추출: $worker_join_cmd"
						fi
					fi
					
					# 방법 3: 전체 로그에서 kubeadm join 명령어 추출 (멀티라인 고려)
					if [ -z "$worker_join_cmd" ] || ! echo "$worker_join_cmd" | grep -q "discovery-token-ca-cert-hash"; then
						echo "이전 방법 실패 또는 불완전한 명령어, 최종 방법 시도 중..."
						# 로그에서 kubeadm join이 있는 행과 그 다음 행을 함께 추출
						worker_join_cmd=$(grep -n "kubeadm join" /tmp/k8s_install.log | grep -v "control-plane" | tail -1)
						line_num=$(echo "$worker_join_cmd" | cut -d':' -f1)
						if [ ! -z "$line_num" ]; then
							worker_join_cmd=$(sed -n "${line_num},$(($line_num+1))p" /tmp/k8s_install.log | tr -d '\\' | tr '\n' ' ' | sed -e 's/^[[:space:]]*//' | sed -e 's/[[:space:]]*\$//')
							echo "방법 3으로 추출: $worker_join_cmd"
						fi
					fi
					
					# 결과 정리 - discovery-token-ca-cert-hash가 포함되어 있는지 확인
					if [ ! -z "$worker_join_cmd" ] && ! echo "$worker_join_cmd" | grep -q "discovery-token-ca-cert-hash"; then
						echo "불완전한 명령어 (discovery-token-ca-cert-hash 없음), 추가 추출 시도..."
						# 10줄 더 체크하여 discovery-token-ca-cert-hash 부분 찾기
						token_part=$(echo "$worker_join_cmd" | grep -o "kubeadm join [^ ]* --token [^ ]*" || echo "")
						hash_part=$(grep -A 10 "$token_part" /tmp/k8s_install.log | grep -o "discovery-token-ca-cert-hash sha256:[a-f0-9]*" | head -1 || echo "")
						
						if [ ! -z "$token_part" ] && [ ! -z "$hash_part" ]; then
							worker_join_cmd="$token_part --$hash_part"
							echo "재구성된 명령어: $worker_join_cmd"
						fi
					fi
					
					# 불필요한 명령어(mkdir, cp 등) 제거
					if [ ! -z "$worker_join_cmd" ]; then
						# 'kubeadm join'으로 시작하고 'discovery-token-ca-cert-hash'로 끝나는 부분만 추출
						clean_join_cmd=$(echo "$worker_join_cmd" | grep -o "kubeadm join.*discovery-token-ca-cert-hash sha256:[a-f0-9]*" || echo "")
						if [ ! -z "$clean_join_cmd" ]; then
							worker_join_cmd="$clean_join_cmd"
							echo "정리된 명령어: $worker_join_cmd"
						else
							# 대체 방법: + 또는 mkdir 등이 있으면 그 전까지만 사용
							clean_join_cmd=$(echo "$worker_join_cmd" | sed 's/\\s\\+\\(mkdir\\|cp\\|sudo\\|[+&|]\\).*//' || echo "")
							if [ ! -z "$clean_join_cmd" ] && [ "$clean_join_cmd" != "$worker_join_cmd" ]; then
								worker_join_cmd="$clean_join_cmd"
								echo "정리된 명령어 (대체 방법): $worker_join_cmd"
							fi
						fi
					fi
					
					# control-plane 인증서 키 추출
					control_plane_cert=$(grep -A 10 "You can now join any number of the control-plane node" /tmp/k8s_install.log | grep -o "--control-plane --certificate-key [a-zA-Z0-9]+" | head -1)
					
					# 인증서 키를 찾지 못한 경우 대체 방법
					if [ -z "$control_plane_cert" ]; then
						echo "기본 방법으로 인증서 키를 찾지 못했습니다. 대체 방법 시도 중..."
						control_plane_cert=$(grep -o "--control-plane --certificate-key [a-zA-Z0-9]+" /tmp/k8s_install.log | head -1)
					fi
					
					# 결과 저장
					if [ ! -z "$worker_join_cmd" ]; then
						echo "$worker_join_cmd" > /tmp/k8s_join_command.txt
						echo "worker 노드용 join 명령어가 성공적으로 추출되었습니다."
						echo "추출된 join 명령어: $worker_join_cmd"
					else
						# 강제로 kubeadm join 명령어 찾기 시도
						echo "모든 방법이 실패했습니다. 로그 파일에서 kubeadm join이 포함된 모든 줄을 확인합니다."
						grep "kubeadm join" /tmp/k8s_install.log > /tmp/k8s_all_join_commands.txt
						echo "join 명령어 추출 실패, 모든 join 명령어를 /tmp/k8s_all_join_commands.txt에 저장했습니다."
					fi
					
					if [ ! -z "$control_plane_cert" ]; then
						echo "$control_plane_cert" > /tmp/k8s_certificate_key.txt
						echo "인증서 키가 성공적으로 추출되었습니다."
						echo "추출된 인증서 키: $control_plane_cert"
					else
						echo "인증서 키 추출 실패" > /tmp/k8s_certificate_key_error.txt
						echo "로그 파일에서 인증서 키를 찾을 수 없습니다."
					fi
					
					# 디버깅을 위해 로그 파일의 관련 부분 저장
					grep -A 20 -B 5 "You can now join any number" /tmp/k8s_install.log > /tmp/k8s_join_section.log
					grep -A 20 -B 5 "worker nodes by running" /tmp/k8s_install.log >> /tmp/k8s_join_section.log
					
					# 임시 파일 정리 (k8s_install.log는 유지)
					rm -f /tmp/extract_join_cmd.log
					
					# 한 번만 실행하고 종료
					exit 0
				fi
				sleep 10
				elapsed=$(($elapsed + 10))
			done
			
			echo "최대 대기 시간에 도달했습니다. 설치 완료 메시지를 찾지 못했습니다."
			# 실패 시에도 임시 파일 정리
			rm -f /tmp/k8s_certificate_key.txt /tmp/k8s_join_error.txt /tmp/extract_join_cmd.log
			exit 1
		" > /tmp/extract_join_cmd.log 2>&1 &`),
	}

	return installCommands, nil
}

// 마스터 노드 조인 관련 함수
func validateJoinMasterParams(params map[string]interface{}) error {
	// 필요한 파라미터 검증
	if _, exists := params["server_id"]; !exists {
		return fmt.Errorf("server_id 파라미터가 필요합니다")
	}
	if _, exists := params["main_id"]; !exists {
		return fmt.Errorf("main_id 파라미터가 필요합니다")
	}
	if _, exists := params["password"]; !exists {
		return fmt.Errorf("password 파라미터가 필요합니다")
	}
	if _, exists := params["lb_password"]; !exists {
		return fmt.Errorf("lb_password 파라미터가 필요합니다")
	}
	return nil
}

// PrepareJoinMasterCommands는 마스터 노드 조인에 필요한 명령어들을 준비합니다
func PrepareJoinMasterCommands(params map[string]interface{}) (map[string][]string, error) {
	// 변수들을 실제 사용하는 경우만 남기고 제거
	// serverID, _ := params["server_id"].(float64)
	// mainID, _ := params["main_id"].(float64)
	// password := getStringParameter(params["password"])
	// lbPassword := getStringParameter(params["lb_password"])

	// lbIP := getStringParameter(params["lb_ip"])
	serverName := getStringParameter(params["server_name"])
	masterIP := getStringParameter(params["master_ip"])
	joinCommand := getStringParameter(params["join_command"])
	certificateKey := getStringParameter(params["certificate_key"])
	password := getStringParameter(params["password"])
	lbPassword := getStringParameter(params["lb_password"])

	// 포트 기본값 설정 (기본값: 6443)
	port := getStringParameter(params["port"])
	if port == "" {
		port = "6443"
	}

	// 로드밸런서 IP
	lbIP := getStringParameter(params["lb_ip"])

	// HAProxy 설정 업데이트 - 스크립트 파일 사용
	haproxyUpdateCmd := []string{
		// 1. 스크립트 파일 생성
		fmt.Sprintf(`cat > /tmp/update_haproxy.sh << 'EOF'
#!/bin/bash
set -e

# 백업 생성
cp /etc/haproxy/haproxy.cfg /etc/haproxy/haproxy.cfg.bak_$(date +%%Y%%m%%d%%H%%M%%S)

# frontend 섹션에서 모든 server 라인 제거
sed -i '/^frontend/,/^backend/ {/server/d}' /etc/haproxy/haproxy.cfg

# backend 섹션에 서버가 이미 있는지 확인
if grep -q 'server %s %s:%s check' /etc/haproxy/haproxy.cfg; then
    echo '서버 라인이 이미 존재합니다.'
else
    # backend 섹션 찾기
    backend_line=$(grep -n '^backend kubernetes-backend' /etc/haproxy/haproxy.cfg | cut -d: -f1)
    if [ -n "$backend_line" ]; then
        # backend 섹션 마지막에 서버 추가
        sed -i "${backend_line}a\\    server %s %s:%s check" /etc/haproxy/haproxy.cfg
    else
        echo "backend 섹션을 찾을 수 없습니다."
        exit 1
    fi
fi

# 설정 파일 문법 검사
if ! haproxy -c -f /etc/haproxy/haproxy.cfg; then
    echo "HAProxy 설정 파일 문법 오류가 있습니다."
    # 백업에서 복원
    cp /etc/haproxy/haproxy.cfg.bak_$(ls -t /etc/haproxy/haproxy.cfg.bak_* | head -1 | cut -d'_' -f2) /etc/haproxy/haproxy.cfg
    exit 1
fi

# HAProxy 재시작
if ! systemctl restart haproxy && ! service haproxy restart; then
    echo "HAProxy 재시작에 실패했습니다."
    # 백업에서 복원
    cp /etc/haproxy/haproxy.cfg.bak_$(ls -t /etc/haproxy/haproxy.cfg.bak_* | head -1 | cut -d'_' -f2) /etc/haproxy/haproxy.cfg
    exit 1
fi

echo "HAProxy 설정이 성공적으로 업데이트되었습니다."
EOF`, serverName, masterIP, port, serverName, masterIP, port),

		// 2. 스크립트 실행 권한 부여
		"chmod +x /tmp/update_haproxy.sh",

		// 3. sudo로 스크립트 실행
		fmt.Sprintf("echo '%s' | sudo -S bash /tmp/update_haproxy.sh", lbPassword),

		// 4. 임시 스크립트 파일 삭제
		"rm -f /tmp/update_haproxy.sh",
	}

	// 조인 스크립트 생성 및 실행을 위한 명령어
	joinScriptCommands := []string{
		// 1. 마스터 노드 조인 스크립트 생성
		fmt.Sprintf(`cat > /tmp/join_k8s.sh << 'EOL'
#!/bin/bash

set -euxo pipefail

# 포트 설정
PORT="%s"
echo "사용할 포트: $PORT"

# 현재 IP 주소 감지
local_ip=$(ip -4 addr show | awk '/inet / && $2 ~ /^192/ {print $2}' | cut -d/ -f1 | head -n 1)
echo "감지된 로컬 IP 주소: $local_ip"

# IP 감지 실패 시 대체 방법 사용
if [ -z "$local_ip" ]; then
  echo "첫 번째 방법으로 IP 감지 실패, 대체 방법 시도..."
  local_ip=$(hostname -I | awk '{print $1}')
  echo "대체 방법으로 감지된 IP: $local_ip"
fi

# 로드 밸런서 IP 설정
LB_IP="%s"
if [ -z "$LB_IP" ]; then
  LB_IP=$local_ip
  echo "로드 밸런서 IP가 제공되지 않아 로컬 IP를 사용합니다."
fi
echo "로드 밸런서 IP: $LB_IP"

# 서버 이름 설정 (DB에서 가져온 값)
SERVER_NAME="%s"
echo "DB에서 가져온 서버 이름: $SERVER_NAME"

# 현재 사용자 확인
CURRENT_USER=$(whoami)
USER_HOME=$(eval echo ~$CURRENT_USER)
echo "현재 사용자: $CURRENT_USER"
echo "사용자 홈 디렉토리: $USER_HOME"

# 현재 우분투 버전 감지
UBUNTU_VERSION=$(lsb_release -rs)
echo "감지된 우분투 버전: $UBUNTU_VERSION"

# 우분투 버전에 따라 쿠버네티스 버전 선택
if [ "$(echo "$UBUNTU_VERSION >= 22.04" | bc)" -eq 1 ]; then
  # Ubuntu 22.04 이상은 쿠버네티스 1.28 이상 지원
  K8S_VERSION="1.30"
elif [ "$(echo "$UBUNTU_VERSION >= 20.04" | bc)" -eq 1 ]; then
  # Ubuntu 20.04는 쿠버네티스 1.19 ~ 1.29 지원
  K8S_VERSION="1.29"
elif [ "$(echo "$UBUNTU_VERSION >= 18.04" | bc)" -eq 1 ]; then
  # Ubuntu 18.04는 쿠버네티스 1.17 ~ 1.24 지원
  K8S_VERSION="1.24"
else
  # 이전 버전은 쿠버네티스 1.19 사용
  K8S_VERSION="1.19"
fi

echo "선택된 쿠버네티스 버전: $K8S_VERSION"

# 새로운 쿠버네티스 설치 시작
echo "새로운 쿠버네티스 설치 시작..."

sudo swapoff -a

(crontab -l 2>/dev/null; echo "@reboot /sbin/swapoff -a") | crontab - || true

sudo apt-get update -y

# 필수 패키지 설치
sudo apt-get install -y curl apt-transport-https ca-certificates gnupg lsb-release

# 쿠버네티스 버전 설정
VERSION="$K8S_VERSION"

cat <<EOF | sudo tee /etc/modules-load.d/containerd.conf
overlay
br_netfilter
EOF

sudo modprobe overlay
sudo modprobe br_netfilter

cat <<EOF | sudo tee /etc/sysctl.d/99-kubernetes-cri.conf
net.bridge.bridge-nf-call-iptables  = 1
net.ipv4.ip_forward                 = 1
net.bridge.bridge-nf-call-ip6tables = 1
EOF

sudo sysctl --system

# containerd 설치
sudo apt-get update
sudo apt-get install -y containerd

# containerd 설정
sudo mkdir -p /etc/containerd
containerd config default | sudo tee /etc/containerd/config.toml > /dev/null
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml
sudo systemctl restart containerd
sudo systemctl enable containerd

echo "Container runtime (containerd) installed successfully"

sudo apt-get update
sudo apt-get install -y apt-transport-https ca-certificates curl etcd-client

# 문제가 될 수 있는 저장소가 있는 경우 제거 (Ubuntu 20.04의 경우)
if [ "$(echo "$UBUNTU_VERSION >= 20.04" | bc)" -eq 1 ] && [ "$(echo "$UBUNTU_VERSION < 22.04" | bc)" -eq 1 ]; then
  echo "Ubuntu 20.04에서 불필요한 OpenSUSE 저장소 제거 중..."
  sudo rm -f /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list 2>/dev/null || true
  sudo rm -f /etc/apt/sources.list.d/devel:kubic:libcontainers:stable:cri-o:*.list 2>/dev/null || true
  
  sudo apt-get update
fi

# 쿠버네티스 저장소 설정 (우분투 버전에 따라 다른 방법 사용)
sudo mkdir -p /etc/apt/keyrings

# 직접 키 다운로드 및 설치 (--batch 및 --yes 옵션 추가)
curl -fsSL https://pkgs.k8s.io/core:/stable:/v$VERSION/deb/Release.key | sudo gpg --batch --yes --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
sudo chmod a+r /etc/apt/keyrings/kubernetes-apt-keyring.gpg

# 저장소 추가
echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v$VERSION/deb/ /" | sudo tee /etc/apt/sources.list.d/kubernetes.list

# 대체 방법: 직접 키 다운로드 및 설치
if [ ! -s /etc/apt/keyrings/kubernetes-apt-keyring.gpg ]; then
  # 첫 번째 방법이 실패한 경우 대체 방법 시도
  echo "첫 번째 저장소 설정 방법이 실패했습니다. 대체 방법을 시도합니다..."
  
  # 우분투 버전에 따라 다른 방법 사용
  sudo mkdir -p /etc/apt/keyrings
  curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo gpg --batch --yes --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
  sudo chmod a+r /etc/apt/keyrings/kubernetes-apt-keyring.gpg
  echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://apt.kubernetes.io/ kubernetes-xenial main" | sudo tee /etc/apt/sources.list.d/kubernetes.list
  
  # 만약 위 방법이 실패하고 Ubuntu 20.04 이하라면 예전 방식 시도
  if [ $? -ne 0 ] && [ "$(echo "$UBUNTU_VERSION < 22.04" | bc)" -eq 1 ]; then
    echo "새로운 방식이 실패했습니다. Ubuntu $UBUNTU_VERSION에서 예전 방식을 시도합니다..."
    curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key add -
    echo "deb https://apt.kubernetes.io/ kubernetes-xenial main" | sudo tee /etc/apt/sources.list.d/kubernetes.list
  fi
fi

sudo apt-get update -y || true

# 자동 응답을 위한 설정 (대화형 프롬프트 비활성화)
# DEBIAN_FRONTEND 환경 변수 설정
export DEBIAN_FRONTEND=noninteractive

# 특정 버전 설치
if [ "$K8S_VERSION" = "1.19" ]; then
  if ! sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" kubelet=1.19.16-00 kubeadm=1.19.16-00 kubectl=1.19.16-00; then
    echo "특정 버전 설치 실패, 와일드카드로 시도합니다..."
    sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" kubelet=1.19\* kubeadm=1.19\* kubectl=1.19\*
  fi
elif [ "$K8S_VERSION" = "1.24" ]; then
  if ! sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" kubelet=1.24.17-00 kubeadm=1.24.17-00 kubectl=1.24.17-00; then
    echo "특정 버전 설치 실패, 와일드카드로 시도합니다..."
    sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" kubelet=1.24\* kubeadm=1.24\* kubectl=1.24\*
  fi
elif [ "$K8S_VERSION" = "1.29" ]; then
  if ! sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" kubelet=1.29.0-00 kubeadm=1.29.0-00 kubectl=1.29.0-00; then
    echo "특정 버전 설치 실패, 와일드카드로 시도합니다..."
    sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" kubelet=1.29\* kubeadm=1.29\* kubectl=1.29\*
  fi
else
  # 최신 버전 설치
  sudo apt-get install -y kubelet kubeadm kubectl
fi

sudo apt-mark hold kubelet kubeadm kubectl

sudo apt-get update -y
sudo apt-get install -y jq bc

# kubelet 서비스 활성화
sudo systemctl enable kubelet

cat > /tmp/kubelet_config << EOF
KUBELET_EXTRA_ARGS=--node-ip=$local_ip
EOF
sudo mv /tmp/kubelet_config /etc/default/kubelet

########

MASTER_IP=$local_ip
IP_NO_DOT=$(echo "$local_ip" | sed "s/\./-/g")
POD_CIDR="10.10.0.0/16"

# 포트 상태 확인
echo "포트 $PORT 상태 확인 중..."
if netstat -tuln | grep ":$PORT " > /dev/null; then
  echo "경고: 포트 $PORT가 이미 사용 중입니다."
  # 포트가 사용 중인 프로세스 확인
  echo "포트 $PORT를 사용 중인 프로세스:"
  sudo lsof -i :$PORT || true
  sudo netstat -tulnp | grep ":$PORT " || true
fi

echo "kubeadm 이미지 다운로드 중..."
sudo kubeadm config images pull

echo "Preflight Check Passed: Downloaded All Required Images"

# DB에서 가져온 join 명령어와 인증서 키로 마스터 노드 조인
echo "Joining kubernetes control plane with the following command:"
JOIN_CMD="%s"
CERT_KEY="%s"

JOIN_CMD="$JOIN_CMD $CERT_KEY"

# 노드 이름 추가 (이미 있는지 확인 후)
if ! echo "$JOIN_CMD" | grep -q -- "--node-name"; then
  JOIN_CMD="$JOIN_CMD --node-name=$SERVER_NAME"
fi
JOIN_CMD=$(echo "$JOIN_CMD" | sed 's/ \\//g')
echo "실행할 조인 명령어: $JOIN_CMD"
sudo $JOIN_CMD

# Kubernetes config 디렉토리 생성
mkdir -p $HOME/.kube
# kubeconfig 파일 복사
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
# 사용자에게 권한 부여
sudo chown $(id -u):$(id -g) $HOME/.kube/config

# 환경 변수 설정
export KUBECONFIG=$USER_HOME/.kube/config

# 현재 사용자의 .bashrc 파일에 환경 변수 추가
echo 'export KUBECONFIG=$HOME/.kube/config' >> $USER_HOME/.bashrc

# 컨트롤 플레인 노드의 테인트 제거
echo "컨트롤 플레인 노드의 테인트 제거 중..."
kubectl taint nodes --all node-role.kubernetes.io/control-plane- || true
kubectl taint nodes --all node-role.kubernetes.io/master- || true

# API 서버가 완전히 시작될 때까지 대기
echo "API 서버가 시작될 때까지 대기 중..."
sleep 30

# Calico 네트워크 플러그인 설치
curl https://raw.githubusercontent.com/projectcalico/calico/v3.25.0/manifests/calico.yaml -O
kubectl apply -f calico.yaml

# 인그레스 컨트롤러 설치
echo "인그레스 컨트롤러 매니페스트 다운로드 및 수정 중..."
curl -o ingress-nginx.yaml https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
kubectl apply -f ingress-nginx.yaml

# 노드에 레이블 추가
sudo kubectl label node $SERVER_NAME ingress-ready=true

# 모든 파드가 실행될 때까지 대기
echo "모든 파드가 실행될 때까지 대기 중..."
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=180s || true

# 설치 완료 후 현재 사용자를 위한 환경 변수 설정 안내
echo "쿠버네티스 마스터 노드 설치가 완료되었습니다."
echo ""
echo "$CURRENT_USER 사용자로 kubectl을 사용하려면 다음 명령을 실행하세요:"
echo "export KUBECONFIG=$USER_HOME/.kube/config"
echo ""
echo "또는 다음 명령을 ~/.bashrc 파일에 추가하세요:"
echo "echo 'export KUBECONFIG=$USER_HOME/.kube/config' >> $USER_HOME/.bashrc"
echo "설치 완료"
EOL`, port, lbIP, serverName, joinCommand, certificateKey),

		// 2. 스크립트 실행 권한 부여
		"chmod +x /tmp/join_k8s.sh",

		// 3. 백그라운드로 스크립트 실행 (sudo 권한 포함)
		fmt.Sprintf("echo '%s' | sudo -S bash /tmp/join_k8s.sh > /tmp/k8s_join.log 2>&1 & echo $! > /tmp/k8s_join.pid", password),

		// 4. 설치 시작 확인
		"echo '쿠버네티스 마스터 노드 조인이 백그라운드에서 시작되었습니다. 로그 파일: /tmp/k8s_join.log, PID: '$(cat /tmp/k8s_join.pid)",
	}

	// 조인 완료 확인 명령어
	checkJoinCommand := []string{
		"while ! grep -q '마스터 노드 조인 완료' /tmp/k8s_join.log 2>/dev/null; do sleep 10; done; echo '조인이 완료되었습니다.'",
	}

	// 명령어 맵 반환
	return map[string][]string{
		"haproxyUpdate": haproxyUpdateCmd,
		"joinScript":    joinScriptCommands,
		"checkJoin":     checkJoinCommand,
	}, nil
}

// prepareJoinMasterCommandsWrapper는 CommandTemplate 인터페이스와 호환되도록 하는 래퍼 함수입니다
func prepareJoinMasterCommandsWrapper(params map[string]interface{}) ([]string, error) {
	// 하프록시 업데이트 명령만 반환 (joinScript, checkJoin은 API 핸들러에서 직접 사용)
	commandSets, err := PrepareJoinMasterCommands(params)
	if err != nil {
		return nil, err
	}
	return commandSets["haproxyUpdate"], nil
}

// prepareHAProxyUpdateCommands HAProxy 백엔드 설정을 업데이트하는 명령어를 준비합니다.
func prepareHAProxyUpdateCommands(params map[string]interface{}) ([]string, error) {
	password := getStringParameter(params["lb_password"])
	masterIP := getStringParameter(params["master_ip"])
	serverName := getStringParameter(params["server_name"])

	// 포트 기본값 설정 (기본값: 6443)
	port := getStringParameter(params["port"])
	if port == "" {
		port = "6443"
	}

	// HAProxy 업데이트 스크립트 생성
	scriptContent := fmt.Sprintf(`#!/bin/bash
set -euo pipefail

# 백업 생성
cp /etc/haproxy/haproxy.cfg /etc/haproxy/haproxy.cfg.bak_$(date +%%Y%%m%%d%%H%%M%%S)

# frontend 섹션에서 모든 server 라인 제거
sed -i '/^frontend/,/^backend/ {/server/d}' /etc/haproxy/haproxy.cfg

# backend 섹션에 서버가 이미 있는지 확인
if grep -q 'server %s %s:%s check' /etc/haproxy/haproxy.cfg; then
    echo '서버 라인이 이미 존재합니다.'
else
    # backend 섹션 찾기
    backend_line=$(grep -n '^backend kubernetes-backend' /etc/haproxy/haproxy.cfg | cut -d: -f1)
    if [ -n "$backend_line" ]; then
        # backend 섹션 마지막에 서버 추가
        sed -i "${backend_line}a\\    server %s %s:%s check" /etc/haproxy/haproxy.cfg
    else
        echo "backend 섹션을 찾을 수 없습니다."
        exit 1
    fi
fi

# 설정 파일 문법 검사
if ! haproxy -c -f /etc/haproxy/haproxy.cfg; then
    echo "HAProxy 설정 파일 문법 오류가 있습니다."
    # 백업에서 복원
    cp /etc/haproxy/haproxy.cfg.bak_$(ls -t /etc/haproxy/haproxy.cfg.bak_* | head -1 | cut -d'_' -f2) /etc/haproxy/haproxy.cfg
    exit 1
fi

# HAProxy 재시작
if ! systemctl restart haproxy && ! service haproxy restart; then
    echo "HAProxy 재시작에 실패했습니다."
    # 백업에서 복원
    cp /etc/haproxy/haproxy.cfg.bak_$(ls -t /etc/haproxy/haproxy.cfg.bak_* | head -1 | cut -d'_' -f2) /etc/haproxy/haproxy.cfg
    exit 1
fi

echo "HAProxy 설정이 성공적으로 업데이트되었습니다."
EOF`, serverName, masterIP, port, serverName, masterIP, port)

	// 명령어 배열 생성
	commands := []string{
		// 1. 스크립트 파일 생성
		fmt.Sprintf("cat > /tmp/update_haproxy.sh << 'EOL'\n%s\nEOL", scriptContent),
		// 2. 실행 권한 부여
		"chmod +x /tmp/update_haproxy.sh",
		// 3. sudo로 스크립트 실행
		fmt.Sprintf("echo '%s' | sudo -S /tmp/update_haproxy.sh", password),
		// 4. 임시 스크립트 파일 삭제
		"rm -f /tmp/update_haproxy.sh",
	}

	return commands, nil
}

// 워커 노드 조인 관련 함수
func validateJoinWorkerParams(params map[string]interface{}) error {
	// 필수 매개변수 확인
	requiredParams := []string{"server_name", "join_command", "password"}
	for _, param := range requiredParams {
		if _, ok := params[param]; !ok || params[param] == "" {
			return fmt.Errorf("'%s' 매개변수가 필요합니다", param)
		}
	}
	return nil
}

func prepareJoinWorkerCommands(params map[string]interface{}) ([]string, error) {
	// 파라미터 추출
	serverName := getStringParameter(params["server_name"])
	joinCommand := getStringParameter(params["join_command"])
	password := getStringParameter(params["password"])

	// 워커 노드 설치 스크립트 생성
	installScript := fmt.Sprintf(`#!/bin/bash

set -euxo pipefail

# 현재 사용자 홈 디렉토리
USER_HOME=$HOME
# 서버 이름 설정
SERVER_NAME="%s"
echo "서버 이름: $SERVER_NAME"

# 현재 사용자 확인
CURRENT_USER=$(whoami)
USER_HOME=$(eval echo ~$CURRENT_USER)
echo "현재 사용자: $CURRENT_USER"
echo "사용자 홈 디렉토리: $USER_HOME"

# 현재 우분투 버전 감지
UBUNTU_VERSION=$(lsb_release -rs)
echo "감지된 우분투 버전: $UBUNTU_VERSION"

# 우분투 버전에 따라 쿠버네티스 버전 선택
if [ "$(echo "$UBUNTU_VERSION >= 22.04" | bc)" -eq 1 ]; then
  # Ubuntu 22.04 이상은 쿠버네티스 1.28 이상 지원
  K8S_VERSION="1.30"
elif [ "$(echo "$UBUNTU_VERSION >= 20.04" | bc)" -eq 1 ]; then
  # Ubuntu 20.04는 쿠버네티스 1.19 ~ 1.29 지원
  K8S_VERSION="1.29"
elif [ "$(echo "$UBUNTU_VERSION >= 18.04" | bc)" -eq 1 ]; then
  # Ubuntu 18.04는 쿠버네티스 1.17 ~ 1.24 지원
  K8S_VERSION="1.24"
else
  # 이전 버전은 쿠버네티스 1.19 사용
  K8S_VERSION="1.19"
fi

echo "선택된 쿠버네티스 버전: $K8S_VERSION"

# 새로운 쿠버네티스 설치 시작
echo "새로운 쿠버네티스 설치 시작..."

sudo swapoff -a

(crontab -l 2>/dev/null; echo "@reboot /sbin/swapoff -a") | crontab - || true

sudo apt-get update -y

# 필수 패키지 설치
sudo apt-get install -y curl apt-transport-https ca-certificates gnupg lsb-release

# 쿠버네티스 버전 설정
VERSION="$K8S_VERSION"

cat <<EOF | sudo tee /etc/modules-load.d/containerd.conf
overlay
br_netfilter
EOF

sudo modprobe overlay
sudo modprobe br_netfilter

cat <<EOF | sudo tee /etc/sysctl.d/99-kubernetes-cri.conf
net.bridge.bridge-nf-call-iptables  = 1
net.ipv4.ip_forward                 = 1
net.bridge.bridge-nf-call-ip6tables = 1
EOF

sudo sysctl --system

# containerd 설치
sudo apt-get update
sudo apt-get install -y containerd

# containerd 설정
sudo mkdir -p /etc/containerd
containerd config default | sudo tee /etc/containerd/config.toml > /dev/null
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/g' /etc/containerd/config.toml
sudo systemctl restart containerd
sudo systemctl enable containerd

echo "Container runtime (containerd) installed successfully"

sudo apt-get update
sudo apt-get install -y apt-transport-https ca-certificates curl etcd-client

# 문제가 될 수 있는 저장소가 있는 경우 제거 (Ubuntu 20.04의 경우)
if [ "$(echo "$UBUNTU_VERSION >= 20.04" | bc)" -eq 1 ] && [ "$(echo "$UBUNTU_VERSION < 22.04" | bc)" -eq 1 ]; then
  echo "Ubuntu 20.04에서 불필요한 OpenSUSE 저장소 제거 중..."
  sudo rm -f /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list 2>/dev/null || true
  sudo rm -f /etc/apt/sources.list.d/devel:kubic:libcontainers:stable:cri-o:*.list 2>/dev/null || true
  
  sudo apt-get update
fi

# 쿠버네티스 저장소 설정 (우분투 버전에 따라 다른 방법 사용)
sudo mkdir -p /etc/apt/keyrings

# 직접 키 다운로드 및 설치 (--batch 및 --yes 옵션 추가)
curl -fsSL https://pkgs.k8s.io/core:/stable:/v$VERSION/deb/Release.key | sudo gpg --batch --yes --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
sudo chmod a+r /etc/apt/keyrings/kubernetes-apt-keyring.gpg

# 저장소 추가
echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v$VERSION/deb/ /" | sudo tee /etc/apt/sources.list.d/kubernetes.list

# 대체 방법: 직접 키 다운로드 및 설치
if [ ! -s /etc/apt/keyrings/kubernetes-apt-keyring.gpg ]; then
  # 첫 번째 방법이 실패한 경우 대체 방법 시도
  echo "첫 번째 저장소 설정 방법이 실패했습니다. 대체 방법을 시도합니다..."
  
  # 우분투 버전에 따라 다른 방법 사용
  sudo mkdir -p /etc/apt/keyrings
  curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo gpg --batch --yes --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
  sudo chmod a+r /etc/apt/keyrings/kubernetes-apt-keyring.gpg
  echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://apt.kubernetes.io/ kubernetes-xenial main" | sudo tee /etc/apt/sources.list.d/kubernetes.list
  
  # 만약 위 방법이 실패하고 Ubuntu 20.04 이하라면 예전 방식 시도
  if [ $? -ne 0 ] && [ "$(echo "$UBUNTU_VERSION < 22.04" | bc)" -eq 1 ]; then
    echo "새로운 방식이 실패했습니다. Ubuntu $UBUNTU_VERSION에서 예전 방식을 시도합니다..."
    curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key add -
    echo "deb https://apt.kubernetes.io/ kubernetes-xenial main" | sudo tee /etc/apt/sources.list.d/kubernetes.list
  fi
fi

sudo apt-get update -y || true

# 자동 응답을 위한 설정 (대화형 프롬프트 비활성화)
# DEBIAN_FRONTEND 환경 변수 설정
export DEBIAN_FRONTEND=noninteractive

# 특정 버전 설치 
if [ "$K8S_VERSION" = "1.19" ]; then
  if ! sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" kubelet=1.19.16-00 kubeadm=1.19.16-00 kubectl=1.19.16-00; then
    echo "특정 버전 설치 실패, 와일드카드로 시도합니다..."
    sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" kubelet=1.19\* kubeadm=1.19\* kubectl=1.19\*
  fi
elif [ "$K8S_VERSION" = "1.24" ]; then
  if ! sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" kubelet=1.24.17-00 kubeadm=1.24.17-00 kubectl=1.24.17-00; then
    echo "특정 버전 설치 실패, 와일드카드로 시도합니다..."
    sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" kubelet=1.24\* kubeadm=1.24\* kubectl=1.24\*
  fi
elif [ "$K8S_VERSION" = "1.29" ]; then
  if ! sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" kubelet=1.29.0-00 kubeadm=1.29.0-00 kubectl=1.29.0-00; then
    echo "특정 버전 설치 실패, 와일드카드로 시도합니다..."
    sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" kubelet=1.29\* kubeadm=1.29\* kubectl=1.29\*
  fi
else
  # 최신 버전 설치
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" kubelet kubeadm kubectl
fi

sudo apt-mark hold kubelet kubeadm kubectl

sudo apt-get update -y
sudo apt-get install -y jq bc

# kubelet 서비스 활성화
sudo systemctl enable kubelet

# 로컬 IP 주소 가져오기
LOCAL_IP=$(ip -4 addr show | awk '/inet / && $2 ~ /^192/ {print $2}' | cut -d/ -f1 | head -n 1)
echo "로컬 IP 주소: $LOCAL_IP"

cat > /tmp/kubelet_config << EOF
KUBELET_EXTRA_ARGS=--node-ip=$LOCAL_IP
EOF
sudo mv /tmp/kubelet_config /etc/default/kubelet

# 포트 상태 확인
echo "포트 상태 확인 중..."
if netstat -tuln | grep ":6443 " > /dev/null; then
  echo "경고: 포트 6443이 이미 사용 중입니다."
  # 포트가 사용 중인 프로세스 확인
  echo "포트 6443을 사용 중인 프로세스:"
  sudo lsof -i :6443 || true
  sudo netstat -tulnp | grep ":6443 " || true
fi

echo "kubeadm 이미지 다운로드 중..."
sudo kubeadm config images pull

echo "Preflight Check Passed: Downloaded All Required Images"

# DB에서 가져온 join 명령어로 워커 노드 조인
echo "Joining kubernetes as worker node with the following command:"
JOIN_CMD="%s"

# 노드 이름 추가 (이미 있는지 확인 후)
if ! echo "$JOIN_CMD" | grep -q -- "--node-name"; then
  JOIN_CMD="$JOIN_CMD --node-name=$SERVER_NAME"
fi
JOIN_CMD=$(echo "$JOIN_CMD" | sed 's/ \\//g')
echo "실행할 조인 명령어: $JOIN_CMD"
sudo $JOIN_CMD

# Kubernetes config 디렉토리 생성
mkdir -p $HOME/.kube
# 환경 변수 설정
export KUBECONFIG=$USER_HOME/.kube/config

# 현재 사용자의 .bashrc 파일에 환경 변수 추가
echo 'export KUBECONFIG=$HOME/.kube/config' >> $USER_HOME/.bashrc

echo "워커 노드 조인 완료"`, serverName, joinCommand)

	// 워커 노드 설치 명령어 준비
	installCommands := []string{
		// 1. 스크립트를 파일로 저장
		fmt.Sprintf("cat > /tmp/join_k8s.sh << 'EOL'\n%s\nEOL", installScript),
		// 2. 실행 권한 부여
		"chmod +x /tmp/join_k8s.sh",
		// 3. 로그 파일에 출력 저장하면서 스크립트 실행 (sudo 권한으로)
		fmt.Sprintf("echo '%s' | sudo -S bash /tmp/join_k8s.sh > /tmp/k8s_join.log 2>&1 & echo $! > /tmp/k8s_join.pid", password),
		// 4. 설치 시작 확인
		"echo '쿠버네티스 워커 노드 조인이 백그라운드에서 시작되었습니다. 로그 파일: /tmp/k8s_join.log, PID: '$(cat /tmp/k8s_join.pid)",
	}

	return installCommands, nil
}

// 워커 노드 조인 관련 함수
func validateDeleteWorkerParams(params map[string]interface{}) error {
	// 필수 매개변수 확인
	requiredParams := []string{"server_name", "main_password", "password"}
	for _, param := range requiredParams {
		if _, ok := params[param]; !ok || params[param] == "" {
			return fmt.Errorf("'%s' 매개변수가 필요합니다", param)
		}
	}
	return nil
}

func prepareDeleteWorkerCommands(params map[string]interface{}) ([]string, error) {
	// 파라미터 추출
	serverName := getStringParameter(params["server_name"])
	mainPassword := getStringParameter(params["main_password"])
	password := getStringParameter(params["password"])

	// 마스터 노드에서 실행할 명령어 (cordon, drain, delete)
	masterCommands := []string{
		// 1. 노드 cordon (새로운 파드 스케줄링 방지)
		fmt.Sprintf("echo '%s' | sudo -S kubectl cordon %s", mainPassword, serverName),

		// 2. 노드 drain (기존 파드 안전하게 제거)
		fmt.Sprintf("echo '%s' | sudo -S kubectl drain %s --ignore-daemonsets --delete-emptydir-data --force", mainPassword, serverName),

		// 3. 노드 삭제
		fmt.Sprintf("echo '%s' | sudo -S kubectl delete node %s", mainPassword, serverName),
	}

	// 워커 노드에서 실행할 명령어 (쿠버네티스 관련 패키지 제거)
	workerCommands := []string{
		// 1. kubeadm reset 실행
		fmt.Sprintf("echo '%s' | sudo -S kubeadm reset -f", password),

		// 2. CNI 설정 파일 제거
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /etc/cni/net.d/*", password),

		// 3. iptables 규칙 정리
		fmt.Sprintf("echo '%s' | sudo -S iptables -F", password),
		fmt.Sprintf("echo '%s' | sudo -S iptables -t nat -F", password),
		fmt.Sprintf("echo '%s' | sudo -S iptables -t mangle -F", password),
		fmt.Sprintf("echo '%s' | sudo -S iptables -X", password),

		// 4. IPVS 테이블 정리 (클러스터가 IPVS를 사용한 경우)
		fmt.Sprintf("echo '%s' | sudo -S ipvsadm --clear 2>/dev/null || true", password),

		// 5. kubeconfig 파일 정리
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /root/.kube", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /etc/kubernetes/admin.conf", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /etc/kubernetes/kubelet.conf", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf ~/.kube", password),

		// 6. 서비스 중지
		fmt.Sprintf("echo '%s' | sudo -S systemctl stop kubelet", password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl stop containerd", password),

		// 7. 프로세스 강제 종료
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 kube-apiserver 2>/dev/null || true", password),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 kube-scheduler 2>/dev/null || true", password),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 kube-controller-manager 2>/dev/null || true", password),

		// 8. 마운트 해제 및 파드 디렉토리 정리
		fmt.Sprintf("echo '%s' | sudo -S umount -l /var/lib/kubelet/pods/* 2>/dev/null || true", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /var/lib/kubelet/pods/*", password),

		// 9. 쿠버네티스 관련 디렉토리 강제 정리
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /var/lib/kubelet", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /var/lib/etcd", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /etc/kubernetes", password),

		// 10. systemd 서비스 비활성화
		fmt.Sprintf("echo '%s' | sudo -S systemctl disable kubelet", password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl disable containerd", password),

		// 11. 패키지 제거 (대화형 프롬프트 비활성화)
		fmt.Sprintf("echo '%s' | sudo -S DEBIAN_FRONTEND=noninteractive apt-get remove --allow-change-held-packages -y kubeadm kubectl kubelet kubernetes-cni", password),
		fmt.Sprintf("echo '%s' | sudo -S DEBIAN_FRONTEND=noninteractive apt-get purge -y kubeadm kubectl kubelet kubernetes-cni", password),

		// 12. 관련 디렉토리 정리
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /opt/cni", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /usr/bin/kubectl", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /usr/bin/kubeadm", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /usr/bin/kubelet", password),

		// 13. 패키지 캐시 정리
		fmt.Sprintf("echo '%s' | sudo -S apt-get clean", password),
		fmt.Sprintf("echo '%s' | sudo -S DEBIAN_FRONTEND=noninteractive apt-get autoremove -y", password),

		// 14. 완료 메시지
		"echo '쿠버네티스 노드 정리 완료'",
	}

	// 모든 명령어를 하나의 슬라이스로 합치기
	allCommands := append(masterCommands, workerCommands...)

	return allCommands, nil
}

// 마스터 노드 삭제 관련 함수
func validateDeleteMasterParams(params map[string]interface{}) error {
	// 필수 매개변수 확인
	requiredParams := []string{"server_name", "password"}
	for _, param := range requiredParams {
		if _, ok := params[param]; !ok {
			return fmt.Errorf("필수 파라미터가 누락되었습니다: %s", param)
		}
		if params[param] == "" {
			return fmt.Errorf("필수 파라미터가 비어 있습니다: %s", param)
		}
	}
	return nil
}

// prepareDeleteMasterCommands는 통합 명령어를 준비합니다 (모든 명령어를 한번에 실행할 경우 사용)
func prepareDeleteMasterCommands(params map[string]interface{}) ([]string, error) {
	// 파라미터 검증
	if err := validateDeleteMasterParams(params); err != nil {
		return nil, err
	}

	serverName := getStringParameter(params["server_name"])
	password := getStringParameter(params["password"])
	mainPassword := getStringParameter(params["main_password"])
	lbPassword := getStringParameter(params["lb_password"])

	// 모든 명령어를 저장할 슬라이스
	var allCommands []string

	// 1. 로그 파일 시작 설정 명령어
	logStartCommands := []string{
		fmt.Sprintf("echo '===== 마스터 노드 %s 삭제 작업 시작 =====' > /tmp/master_delete.log", serverName),
		fmt.Sprintf("echo '시작 시간: %s' >> /tmp/master_delete.log", time.Now().Format(time.RFC3339)),
	}
	allCommands = append(allCommands, logStartCommands...)

	// 2. 로드 밸런서에서 마스터 노드 제거 (lb_password가 제공된 경우에만)
	if lbPassword != "" {
		haproxyUpdateCmd := []string{
			// 스크립트 파일 생성
			fmt.Sprintf(`cat > /tmp/remove_server.sh << 'EOF'
#!/bin/bash
set -e

# 백업 생성
cp /etc/haproxy/haproxy.cfg /etc/haproxy/haproxy.cfg.bak.$(date +%%Y%%m%%d%%H%%M%%S)

# 서버 이름에 해당하는 라인 제거
sed -i '/server %s /d' /etc/haproxy/haproxy.cfg

# 설정 파일 문법 검사
if ! haproxy -c -f /etc/haproxy/haproxy.cfg; then
    echo "HAProxy 설정 파일 문법 오류가 있습니다."
    # 백업에서 복원
    cp /etc/haproxy/haproxy.cfg.bak.$(date +%%Y%%m%%d%%H%%M%%S) /etc/haproxy/haproxy.cfg
    exit 1
fi

# HAProxy 재시작
if ! systemctl restart haproxy && ! service haproxy restart; then
    echo "HAProxy 재시작에 실패했습니다."
    # 백업에서 복원
    cp /etc/haproxy/haproxy.cfg.bak.$(date +%%Y%%m%%d%%H%%M%%S) /etc/haproxy/haproxy.cfg
    exit 1
fi

echo "HAProxy 설정에서 서버 %s가 성공적으로 제거되었습니다."
EOF`, serverName, serverName),

			// 스크립트 실행 권한 부여
			"chmod +x /tmp/remove_server.sh",

			// sudo로 스크립트 실행
			fmt.Sprintf("echo '%s' | sudo -S bash /tmp/remove_server.sh", lbPassword),

			// 임시 스크립트 파일 삭제
			"rm -f /tmp/remove_server.sh",
		}
		allCommands = append(allCommands, haproxyUpdateCmd...)
	}

	// 3. 메인 마스터에서 노드 제거 명령어 (main_password가 제공된 경우에만)
	if mainPassword != "" {
		// 3.1 노드 드레인 및 삭제
		mainNodeCommands := []string{
			// 노드 드레인 (파드 제거)
			fmt.Sprintf("echo '%s' | sudo -S kubectl drain %s --delete-emptydir-data --force --ignore-daemonsets", mainPassword, serverName),

			// 노드 삭제
			fmt.Sprintf("echo '%s' | sudo -S kubectl delete node %s", mainPassword, serverName),
		}
		allCommands = append(allCommands, mainNodeCommands...)

		// 3.2 etcd 멤버 제거
		etcdCommands := []string{
			// etcd 멤버 리스트 조회
			fmt.Sprintf("echo '%s' | sudo -S ETCDCTL_API=3 etcdctl --endpoints=localhost:2379 --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/server.crt --key=/etc/kubernetes/pki/etcd/server.key member list", mainPassword),

			// etcd 멤버 ID 추출 및 제거
			fmt.Sprintf("echo '%s' | sudo -S bash -c \"ETCDCTL_API=3 etcdctl --endpoints=localhost:2379 --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/server.crt --key=/etc/kubernetes/pki/etcd/server.key member list | grep %s | cut -d',' -f1 | xargs -I {} sudo ETCDCTL_API=3 etcdctl --endpoints=localhost:2379 --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/server.crt --key=/etc/kubernetes/pki/etcd/server.key member remove {}\"", mainPassword, serverName),

			// 노드 목록 확인
			fmt.Sprintf("echo '%s' | sudo -S kubectl get nodes", mainPassword),
		}
		allCommands = append(allCommands, etcdCommands...)
	}

	// 4. 대상 노드 서비스 중지 명령어
	stopServicesCmd := []string{
		fmt.Sprintf("echo '%s' | sudo -S systemctl stop kubelet || true", password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl stop etcd || true", password),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 etcd || true", password),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 kube-apiserver || true", password),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 kube-scheduler || true", password),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 kube-controller-manager || true", password),
	}
	allCommands = append(allCommands, stopServicesCmd...)

	// 5. 노드 cordon 및 drain 명령어
	nodeCommands := []string{
		fmt.Sprintf("echo '%s' | sudo -S kubectl cordon %s || true", password, serverName),
		fmt.Sprintf("echo '%s' | sudo -S kubectl drain %s --delete-emptydir-data --force --ignore-daemonsets || true", password, serverName),
	}
	allCommands = append(allCommands, nodeCommands...)

	// 6. etcd 멤버 제거 명령어 (로컬 노드에서)
	etcdLocalRemoveCmd := []string{
		fmt.Sprintf("echo '%s' | sudo -S bash -c \"ETCDCTL_API=3 etcdctl --endpoints=localhost:2379 --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/server.crt --key=/etc/kubernetes/pki/etcd/server.key member list | grep %s | cut -d',' -f1 | xargs -I {} sudo ETCDCTL_API=3 etcdctl --endpoints=localhost:2379 --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/server.crt --key=/etc/kubernetes/pki/etcd/server.key member remove {}\" || true", password, serverName),
	}
	allCommands = append(allCommands, etcdLocalRemoveCmd...)

	// 7. kubeadm reset 및 정리 명령어
	cleanupCommands := []string{
		fmt.Sprintf("echo '%s' | sudo -S kubeadm reset -f", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /etc/cni/net.d/*", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /var/lib/etcd", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /var/lib/kubelet", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /etc/kubernetes", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf $HOME/.kube", password),
	}
	allCommands = append(allCommands, cleanupCommands...)

	// 8. iptables 정리 명령어
	iptablesCommands := []string{
		fmt.Sprintf("echo '%s' | sudo -S iptables -F", password),
		fmt.Sprintf("echo '%s' | sudo -S iptables -t nat -F", password),
		fmt.Sprintf("echo '%s' | sudo -S iptables -t mangle -F", password),
	}
	allCommands = append(allCommands, iptablesCommands...)

	// 9. 서비스 비활성화 명령어
	disableCommands := []string{
		fmt.Sprintf("echo '%s' | sudo -S systemctl disable kubelet || true", password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl disable etcd || true", password),
	}
	allCommands = append(allCommands, disableCommands...)

	// 10. 패키지 제거 명령어
	removeCommands := []string{
		fmt.Sprintf("echo '%s' | sudo -S DEBIAN_FRONTEND=noninteractive apt-get remove --allow-change-held-packages -y kubeadm kubectl kubelet kubernetes-cni || true", password),
		fmt.Sprintf("echo '%s' | sudo -S DEBIAN_FRONTEND=noninteractive apt-get purge -y kubeadm kubectl kubelet kubernetes-cni || true", password),
		fmt.Sprintf("echo '%s' | sudo -S DEBIAN_FRONTEND=noninteractive apt-get autoremove -y || true", password),
	}
	allCommands = append(allCommands, removeCommands...)

	// 11. 로그 파일 완료 메시지 추가
	logFinishCommands := []string{
		fmt.Sprintf("echo '===== 마스터 노드 %s 삭제 작업 완료 =====' >> /tmp/master_delete.log", serverName),
		fmt.Sprintf("echo '완료 시간: %s' >> /tmp/master_delete.log", time.Now().Format(time.RFC3339)),
	}
	allCommands = append(allCommands, logFinishCommands...)

	return allCommands, nil
}

// 네임스페이스 및 파드 상태 확인 명령어 준비 함수
func prepareGetNamespaceAndPodStatusCommands(params map[string]interface{}) ([]string, error) {
	namespace := getStringParameter(params["namespace"])
	password := getStringParameter(params["password"]) // sudo를 위한 비밀번호

	if namespace == "" {
		return nil, fmt.Errorf("namespace 파라미터가 필요합니다")
	}

	var commands []string
	sudoPrefix := ""
	if password != "" {
		sudoPrefix = fmt.Sprintf("echo '%s' | sudo -S ", password)
	}

	// 1. 네임스페이스 존재 확인 명령어
	//    -o name 옵션으로 존재하면 "namespace/[name]" 출력, 없으면 에러.
	//    에러 시 stderr를 무시하고 'NAMESPACE_NOT_FOUND' 출력
	nsCheckCmd := fmt.Sprintf("%skubectl get namespace %s -o name 2>/dev/null || echo 'not found'", sudoPrefix, namespace)
	commands = append(commands, nsCheckCmd)

	// 2. 파드 상태 확인 명령어 (핸들러에서 네임스페이스 존재 여부 확인 후 조건부 실행 또는 결과 파싱 시 사용)
	//    --no-headers 옵션으로 헤더 제거
	podStatusCmd := fmt.Sprintf("%skubectl get pods -n %s -o custom-columns=NAME:.metadata.name,STATUS:.status.phase,RESTARTS:.status.containerStatuses[0].restartCount 2>/dev/null", sudoPrefix, namespace)
	commands = append(commands, podStatusCmd)

	return commands, nil
}
