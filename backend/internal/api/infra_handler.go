package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/k8scontrol/backend/internal/db"
	"github.com/k8scontrol/backend/internal/utils"
	"github.com/k8scontrol/backend/pkg/ssh"
)

// InfraHandler 인프라 관련 API 핸들러
type InfraHandler struct {
	DB *sql.DB
}
type Hop struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
	Port     int    `json:"port"`
}

// NewInfraHandler 새 InfraHandler 생성
func NewInfraHandler(db *sql.DB) *InfraHandler {
	return &InfraHandler{DB: db}
}

// 모든 명령어가 성공했는지 확인하는 함수
func allCommandsSuccessful(results []ssh.CommandResult) bool {
	for _, result := range results {
		if result.ExitCode != 0 {
			return false
		}
	}
	return true
}

// InstallLoadBalancer 로드 밸런서 설치
func (h *InfraHandler) InstallLoadBalancer(c *gin.Context) {
	var requestBody struct {
		ID   int             `json:"id"`
		Hops []ssh.HopConfig `json:"hops"`
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		log.Println("JSON 바인딩 오류:", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "잘못된 요청입니다."})
		return
	}

	// 서버 정보 가져오기
	serverInfo, err := db.GetServerInfo(h.DB, requestBody.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 요청 본문에서 hops가 제공되었는지 확인하고, 그렇지 않으면 DB에서 가져옴
	var hops []ssh.HopConfig
	if len(requestBody.Hops) > 0 {
		hops = requestBody.Hops
	} else {
		if err := json.Unmarshal([]byte(serverInfo.Hops), &hops); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "hops 파싱 중 오류가 발생했습니다."})
			return
		}
	}

	ha := serverInfo.HA

	if len(hops) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "SSH 연결 정보(hops)가 필요합니다."})
		return
	}

	// 마지막 hop의 패스워드 사용
	password := ""
	if len(hops) > 0 {
		password = hops[len(hops)-1].Password
	}

	if ha == "Y" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "이 서버에는 이미 HAProxy가 설치되어 있습니다."})
		return
	}

	// HAProxy 설정 및 설치 스크립트 생성
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

	sshUtils := utils.NewSSHUtils()
	finalCommands := []string{
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

	results, err := sshUtils.ExecuteCommands(hops, finalCommands, 60000)
	fmt.Print(results)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "SSH 명령어 실행 중 오류가 발생했습니다."})
		return
	}

	// 로그 파일 확인 명령어 - 파일 권한 문제 해결을 위해 sudo 사용
	checkLogCommands := []string{
		// 로그 파일이 존재하는지 확인 (sudo 사용)
		fmt.Sprintf("echo '%s' | sudo -S ls -la /tmp/haproxy_install.log 2>/dev/null && echo 'LOG_EXISTS=true' || echo 'LOG_EXISTS=false'", password),
		// 로그 파일에 '로드 밸런서 설치 완료' 문자열이 있는지 확인 (sudo 사용)
		fmt.Sprintf("echo '%s' | sudo -S grep -q '로드 밸런서 설치 완료' /tmp/haproxy_install.log 2>/dev/null && echo 'INSTALL_COMPLETE=true' || echo 'INSTALL_COMPLETE=false'", password),
		// 로그 파일에 오류 메시지가 있는지 확인 (sudo 사용)
		fmt.Sprintf("echo '%s' | sudo -S grep -i 'error\\|failed\\|실패' /tmp/haproxy_install.log 2>/dev/null && echo 'HAS_ERRORS=true' || echo 'HAS_ERRORS=false'", password),
		// 로그 파일 내용 가져오기 (sudo 사용)
		fmt.Sprintf("echo '%s' | sudo -S cat /tmp/haproxy_install.log 2>/dev/null || echo '로그 파일을 읽을 수 없습니다.'", password),
	}

	logResults, logErr := sshUtils.ExecuteCommands(hops, checkLogCommands, 30000)

	// 로그 확인 결과 초기화
	logExists := false
	installComplete := false
	hasErrors := false
	logContent := "로그 내용을 가져올 수 없습니다."

	if logErr == nil && len(logResults) >= 4 {
		logExists = strings.Contains(logResults[0].Output, "LOG_EXISTS=true")
		installComplete = strings.Contains(logResults[1].Output, "INSTALL_COMPLETE=true")
		hasErrors = strings.Contains(logResults[2].Output, "HAS_ERRORS=true")
		logContent = logResults[3].Output
	}

	// 로그 내용에 '로드 밸런서 설치 완료'가 있으면 설치 완료로 간주
	if strings.Contains(logContent, "로드 밸런서 설치 완료") {
		installComplete = true
		logExists = true
	}

	// 설치 성공 여부 판단
	installSuccess := (logExists && installComplete && !hasErrors) || strings.Contains(logContent, "로드 밸런서 설치 완료")

	if installSuccess {
		// HA 상태를 Y로 업데이트
		err := db.UpdateServerHAStatus(h.DB, requestBody.ID)
		if err != nil {
			log.Printf("HA 상태 업데이트 중 오류 발생: %v", err)
			c.JSON(http.StatusOK, gin.H{
				"success":    true,
				"message":    "HAProxy가 성공적으로 설치되었지만 DB 상태 업데이트에 실패했습니다.",
				"warning":    err.Error(),
				"logContent": logContent,
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"message":   "HAProxy가 성공적으로 설치되었습니다.",
			"ha_status": "Y",
		})
	} else {
		// 설치 실패 또는 불완전한 경우
		errorMsg := "HAProxy 설치가 완료되지 않았거나 오류가 발생했습니다."
		if !logExists {
			errorMsg = "설치 로그 파일을 찾을 수 없습니다."
		} else if hasErrors {
			errorMsg = "설치 로그에 오류가 발생했습니다."
		} else if !installComplete {
			errorMsg = "설치가 완료되지 않았습니다."
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   errorMsg,
			"details": gin.H{
				"logExists":       logExists,
				"installComplete": installComplete,
				"hasErrors":       hasErrors,
			},
			"logContent": logContent,
		})
	}
}

// InstallMaster 쿠버네티스 마스터 노드 설치
func (h *InfraHandler) InstallFirstMaster(c *gin.Context) {
	var requestBody struct {
		Password   string          `json:"password"`
		ID         int             `json:"id"`
		Hops       []ssh.HopConfig `json:"hops"`        // 마스터 노드 SSH 연결 정보
		LBHops     []ssh.HopConfig `json:"lb_hops"`     // 로드 밸런서 SSH 연결 정보
		LBPassword string          `json:"lb_password"` // 로드 밸런서 서버 패스워드
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		log.Println("JSON 바인딩 오류:", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "잘못된 요청입니다."})
		return
	}

	// 쿠버네티스 마스터 노드 서버 정보 가져오기
	serverInfo, err := db.GetServerInfo(h.DB, requestBody.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 서버 이름 변수 설정 (DB에서 가져온 값 사용)
	serverName := serverInfo.ServerName
	log.Printf("DB에서 가져온 서버 이름: %s를 사용합니다.", serverName)

	// 마스터 노드 IP 주소 가져오기
	sshUtils := utils.NewSSHUtils()
	ipCmd := []string{"ip -4 addr show | awk '/inet / && $2 ~ /^192/ {print $2}' | cut -d/ -f1 | head -n 1"}
	ipResults, err := sshUtils.ExecuteCommands(requestBody.Hops, ipCmd, 30000)
	if err != nil || len(ipResults) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "마스터 노드 IP 주소를 가져오는 중 오류가 발생했습니다."})
		return
	}
	masterIP := strings.TrimSpace(ipResults[0].Output)
	log.Printf("마스터 노드 IP 주소: %s", masterIP)

	// 항상 기본 포트 6443 사용
	port := "6443"
	var lbIP string = masterIP // 기본값으로 마스터 IP 사용

	// 로드 밸런서 패스워드가 제공되었는지 확인
	if requestBody.LBPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "로드 밸런서 패스워드가 제공되지 않았습니다."})
		return
	}

	// 로드 밸런서 IP 주소 가져오기
	lbIpCmd := []string{"ip -4 addr show | awk '/inet / && $2 ~ /^192/ {print $2}' | cut -d/ -f1 | head -n 1"}
	lbIpResults, err := sshUtils.ExecuteCommands(requestBody.LBHops, lbIpCmd, 30000)
	if err != nil || len(lbIpResults) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "로드 밸런서 IP 주소를 가져오는 중 오류가 발생했습니다."})
		return
	}
	lbIP = strings.TrimSpace(lbIpResults[0].Output)
	log.Printf("로드 밸런서 IP 주소: %s", lbIP)

	// HAProxy 설정 업데이트 - 스크립트 파일 사용
	haproxyUpdateCmd := []string{
		// 1. 스크립트 파일 생성
		fmt.Sprintf(`cat > /tmp/update_haproxy.sh << 'EOF'
#!/bin/bash
set -e

# 백업 생성
cp /etc/haproxy/haproxy.cfg /etc/haproxy/haproxy.cfg.bak.$(date +%%Y%%m%%d%%H%%M%%S)

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

echo "HAProxy 설정이 성공적으로 업데이트되었습니다."
EOF`, serverName, masterIP, port, serverName, masterIP, port),

		// 2. 스크립트 실행 권한 부여
		"chmod +x /tmp/update_haproxy.sh",

		// 3. sudo로 스크립트 실행
		fmt.Sprintf("echo '%s' | sudo -S bash /tmp/update_haproxy.sh", requestBody.LBPassword),

		// 4. 임시 스크립트 파일 삭제
		"rm -f /tmp/update_haproxy.sh",
	}

	lbResults, err := sshUtils.ExecuteCommands(requestBody.LBHops, haproxyUpdateCmd, 60000) // 타임아웃 60초로 증가
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "로드 밸런서 HAProxy 설정 업데이트 중 오류가 발생했습니다.", "errorDetails": err.Error()})
		return
	}

	// 로드 밸런서 설정 결과 확인
	if !allCommandsSuccessful(lbResults) {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "로드 밸런서 HAProxy 설정 업데이트 중 오류가 발생했습니다.", "errorDetails": lbResults[len(lbResults)-2].Error}) // 스크립트 실행 결과 오류 반환
		return
	}
	log.Println("로드 밸런서 HAProxy 설정이 성공적으로 업데이트되었습니다.")

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

	// 3. 마스터 노드에 설치 스크립트 실행
	finalCommands := []string{
		// 1. 스크립트를 파일로 저장
		fmt.Sprintf("cat > /tmp/install_k8s.sh << 'EOL'\n%s\nEOL", installScript),
		// 2. 실행 권한 부여
		"chmod +x /tmp/install_k8s.sh",
		// 3. 로그 파일에 출력 저장하면서 스크립트 실행 (sudo 권한으로)
		fmt.Sprintf("echo '%s' | sudo -S bash /tmp/install_k8s.sh > /tmp/k8s_install.log 2>&1 & echo $! > /tmp/k8s_install.pid", requestBody.Password),
		// 4. 설치 시작 확인
		"echo '쿠버네티스 설치가 백그라운드에서 시작되었습니다. 로그 파일: /tmp/k8s_install.log, PID: '$(cat /tmp/k8s_install.pid)",
		// 5. 백그라운드에서 로그 모니터링 및 join 명령어 추출 스크립트 실행
		fmt.Sprintf(`nohup bash -c "
			# 설치가 완료될 때까지 대기 (최대 30분)
			max_wait=1800
			elapsed=0
			while [ \$elapsed -lt \$max_wait ]; do
				if grep -q \"설치 완료\" /tmp/k8s_install.log 2>/dev/null; then
					echo \"쿠버네티스 설치가 완료되었습니다. join 명령어를 추출합니다.\"
					
					# 로그 파일에서 주요 부분 추출
					echo \"로그 파일에서 worker 노드 join 지침 찾기...\"
					
					# 여러 가지 방법으로 worker 노드용 join 명령어 추출 시도
					# 방법 1: \"Then you can join any number of worker nodes\" 이후 부분에서 multi-line 텍스트 추출
					worker_join_cmd=\$(grep -A 15 \"Then you can join any number of worker nodes\" /tmp/k8s_install.log | grep -A 3 \"kubeadm join\" | grep -v \"You can now join\" | sed -e '/^$/,\$d' | tr '\n' ' ' | sed -e 's/\\\\//g' | sed -e 's/^[[:space:]]*//' | sed -e 's/[[:space:]]*\$//')
					
					# 방법 2: 'kubeadm join'으로 시작하는 두 줄 추출해서 공백 제거 후 합치기
					if [ -z \"\$worker_join_cmd\" ]; then
						echo \"방법 1 실패, 대체 방법 시도 중...\"
						worker_join_start=\$(grep -n \"kubeadm join\" /tmp/k8s_install.log | grep -v \"control-plane\" | tail -1 | cut -d: -f1)
						if [ ! -z \"\$worker_join_start\" ]; then
							worker_join_cmd=\$(sed -n \"\${worker_join_start},\$((\$worker_join_start+1))p\" /tmp/k8s_install.log | tr -d '\\\\' | tr '\n' ' ' | sed -e 's/^[[:space:]]*//' | sed -e 's/[[:space:]]*\$//')
							echo \"방법 2로 추출: \$worker_join_cmd\"
						fi
					fi
					
					# 방법 3: 전체 로그에서 kubeadm join 명령어 추출 (멀티라인 고려)
					if [ -z \"\$worker_join_cmd\" ] || ! echo \"\$worker_join_cmd\" | grep -q \"discovery-token-ca-cert-hash\"; then
						echo \"이전 방법 실패 또는 불완전한 명령어, 최종 방법 시도 중...\"
						# 로그에서 kubeadm join이 있는 행과 그 다음 행을 함께 추출
						worker_join_cmd=\$(grep -n \"kubeadm join\" /tmp/k8s_install.log | grep -v \"control-plane\" | tail -1)
						line_num=\$(echo \"\$worker_join_cmd\" | cut -d':' -f1)
						if [ ! -z \"\$line_num\" ]; then
							worker_join_cmd=\$(sed -n \"\${line_num},\$((\$line_num+1))p\" /tmp/k8s_install.log | tr -d '\\\\' | tr '\n' ' ' | sed -e 's/^[[:space:]]*//' | sed -e 's/[[:space:]]*\$//')
							echo \"방법 3으로 추출: \$worker_join_cmd\"
						fi
					fi
					
					# 결과 정리 - discovery-token-ca-cert-hash가 포함되어 있는지 확인
					if [ ! -z \"\$worker_join_cmd\" ] && ! echo \"\$worker_join_cmd\" | grep -q \"discovery-token-ca-cert-hash\"; then
						echo \"불완전한 명령어 (discovery-token-ca-cert-hash 없음), 추가 추출 시도...\"
						# 10줄 더 체크하여 discovery-token-ca-cert-hash 부분 찾기
						token_part=\$(echo \"\$worker_join_cmd\" | grep -o \"kubeadm join [^ ]* --token [^ ]*\" || echo \"\")
						hash_part=\$(grep -A 10 \"\$token_part\" /tmp/k8s_install.log | grep -o \"discovery-token-ca-cert-hash sha256:[a-f0-9]*\" | head -1 || echo \"\")
						
						if [ ! -z \"\$token_part\" ] && [ ! -z \"\$hash_part\" ]; then
							worker_join_cmd=\"\$token_part --\$hash_part\"
							echo \"재구성된 명령어: \$worker_join_cmd\"
						fi
					fi
					
					# 불필요한 명령어(mkdir, cp 등) 제거
					if [ ! -z \"\$worker_join_cmd\" ]; then
						# 'kubeadm join'으로 시작하고 'discovery-token-ca-cert-hash'로 끝나는 부분만 추출
						clean_join_cmd=\$(echo \"\$worker_join_cmd\" | grep -o \"kubeadm join.*discovery-token-ca-cert-hash sha256:[a-f0-9]*\" || echo \"\")
						if [ ! -z \"\$clean_join_cmd\" ]; then
							worker_join_cmd=\"\$clean_join_cmd\"
							echo \"정리된 명령어: \$worker_join_cmd\"
						else
							# 대체 방법: + 또는 mkdir 등이 있으면 그 전까지만 사용
							clean_join_cmd=\$(echo \"\$worker_join_cmd\" | sed 's/\\s\\+\\(mkdir\\|cp\\|sudo\\|[+&|]\\).*//' || echo \"\")
							if [ ! -z \"\$clean_join_cmd\" ] && [ \"\$clean_join_cmd\" != \"\$worker_join_cmd\" ]; then
								worker_join_cmd=\"\$clean_join_cmd\"
								echo \"정리된 명령어 (대체 방법): \$worker_join_cmd\"
							fi
						fi
					fi
					
					# control-plane 인증서 키 추출
					control_plane_cert=\$(grep -A 10 \"You can now join any number of the control-plane node\" /tmp/k8s_install.log | grep -o \"\\--control-plane --certificate-key [a-zA-Z0-9]\\+\" | head -1)
					
					# 인증서 키를 찾지 못한 경우 대체 방법
					if [ -z \"\$control_plane_cert\" ]; then
						echo \"기본 방법으로 인증서 키를 찾지 못했습니다. 대체 방법 시도 중...\"
						control_plane_cert=\$(grep -o \"\\--control-plane --certificate-key [a-zA-Z0-9]\\+\" /tmp/k8s_install.log | head -1)
					fi
					
					# 결과 저장
					if [ ! -z \"\$worker_join_cmd\" ]; then
						echo \"\$worker_join_cmd\" > /tmp/k8s_join_command.txt
						echo \"worker 노드용 join 명령어가 성공적으로 추출되었습니다.\"
						echo \"추출된 join 명령어: \$worker_join_cmd\"
					else
						# 강제로 kubeadm join 명령어 찾기 시도
						echo \"모든 방법이 실패했습니다. 로그 파일에서 kubeadm join이 포함된 모든 줄을 확인합니다.\"
						grep \"kubeadm join\" /tmp/k8s_install.log > /tmp/k8s_all_join_commands.txt
						echo \"join 명령어 추출 실패, 모든 join 명령어를 /tmp/k8s_all_join_commands.txt에 저장했습니다.\"
					fi
					
					if [ ! -z \"\$control_plane_cert\" ]; then
						echo \"\$control_plane_cert\" > /tmp/k8s_certificate_key.txt
						echo \"인증서 키가 성공적으로 추출되었습니다.\"
						echo \"추출된 인증서 키: \$control_plane_cert\"
					else
						echo \"인증서 키 추출 실패\" > /tmp/k8s_certificate_key_error.txt
						echo \"로그 파일에서 인증서 키를 찾을 수 없습니다.\"
					fi
					
					# 디버깅을 위해 로그 파일의 관련 부분 저장
					grep -A 20 -B 5 \"You can now join any number\" /tmp/k8s_install.log > /tmp/k8s_join_section.log
					grep -A 20 -B 5 \"worker nodes by running\" /tmp/k8s_install.log >> /tmp/k8s_join_section.log
					
					# 임시 파일 정리 (k8s_install.log는 유지)
					rm -f /tmp/extract_join_cmd.log
					
					# 한 번만 실행하고 종료
					exit 0
				fi
				sleep 10
				elapsed=\$((\$elapsed + 10))
			done
			
			echo \"최대 대기 시간에 도달했습니다. 설치 완료 메시지를 찾지 못했습니다.\"
			# 실패 시에도 임시 파일 정리
			rm -f /tmp/k8s_certificate_key.txt /tmp/k8s_join_error.txt /tmp/extract_join_cmd.log
			exit 1
		" > /tmp/extract_join_cmd.log 2>&1 &`),
	}

	results, err := sshUtils.ExecuteCommands(requestBody.Hops, finalCommands, 30000) // 30초 (스크립트 시작만 확인)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "SSH 명령어 실행 중 오류가 발생했습니다.", "errorDetails": err.Error()})
		return
	}

	// 결과 처리
	if allCommandsSuccessful(results) {
		output := results[len(results)-1].Output // 마지막 명령어의 출력 (설치 시작 확인)

		// 설치 완료 후 join 명령어 확인을 위한 API 엔드포인트 정보 추가
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "쿠버네티스 마스터 노드 설치가 백그라운드에서 시작되었습니다.",
			"details": output,
			"logFile": "/tmp/k8s_install.log",
			"note":    "설치 진행 상황을 확인하려면 로그 파일을 확인하세요.",
		})

		// 백그라운드에서 join 명령어 확인 및 DB 업데이트
		go func() {
			// 설치 완료 메시지를 확인하는 명령어
			checkInstallCompleteCmd := []string{
				"while ! grep -q '설치 완료' /tmp/k8s_install.log 2>/dev/null; do sleep 10; done; echo '설치가 완료되었습니다.'",
			}

			// 설치 완료될 때까지 대기
			_, err := sshUtils.ExecuteCommands(requestBody.Hops, checkInstallCompleteCmd, 1800000) // 30분 타임아웃
			if err != nil {
				log.Printf("설치 완료 확인 중 오류 발생: %v", err)
				return
			}

			log.Printf("설치 완료 메시지를 확인했습니다. join 명령어를 추출합니다.")

			// join 명령어 확인 - 여러 종류의 명령어 추출 시도
			checkCommands := []string{
				// 멀티라인 join 명령어 처리 - 2줄 추출 후 공백 제거하여 연결
				`grep -A 15 "Then you can join any number of worker nodes" /tmp/k8s_install.log | grep -A 3 "kubeadm join" | grep -v "You can now join" | sed -e '/^$/,$d' | tr '\n' ' ' | sed -e 's/\\\\//g' | sed -e 's/^[[:space:]]*//' | sed -e 's/[[:space:]]*\$//'`,
				// control-plane 인증서 키 추출 (control-plane 노드 섹션 이후의 명령어)
				`grep -A 10 "You can now join any number of the control-plane node" /tmp/k8s_install.log | grep -o "\\--control-plane --certificate-key [a-zA-Z0-9]\\+" | head -1`,
				// 파일에서 직접 읽기
				`cat /tmp/k8s_join_command.txt 2>/dev/null || echo ""`,
				// kubeadm join이 있는 행 찾고, 그 다음 행까지 포함해서 추출
				`join_line=$(grep -n "kubeadm join" /tmp/k8s_install.log | grep -v "control-plane" | tail -1 | cut -d: -f1) && [ ! -z "$join_line" ] && sed -n "${join_line},$(($join_line+1))p" /tmp/k8s_install.log | tr -d '\\\\' | tr '\n' ' ' | sed -e 's/^[[:space:]]*//' | sed -e 's/[[:space:]]*\$//'`,
			}

			checkResults, err := sshUtils.ExecuteCommands(requestBody.Hops, checkCommands, 30000)
			if err != nil || len(checkResults) < 1 {
				log.Printf("join 명령어 확인 중 오류 발생: %v", err)
				return
			}

			// worker 노드용 join 명령어
			joinCommand := strings.TrimSpace(checkResults[0].Output)
			log.Printf("첫번째 방법으로 추출한 join 명령어: '%s'", joinCommand)

			// join 명령어 정리: 불필요한 부분(mkdir, cp 등) 제거
			if strings.Contains(joinCommand, "kubeadm join") {
				// discovery-token-ca-cert-hash 이후에 불필요한 내용이 있는 경우 제거
				if strings.Contains(joinCommand, "discovery-token-ca-cert-hash sha256:") {
					pattern := `(kubeadm join [^[:space:]]*:[0-9]* --token [^ ]* --discovery-token-ca-cert-hash sha256:[a-f0-9]*)`
					re := regexp.MustCompile(pattern)
					matches := re.FindStringSubmatch(joinCommand)
					if len(matches) > 0 {
						joinCommand = matches[0]
						log.Printf("정규식으로 정리한 join 명령어: '%s'", joinCommand)
					} else {
						// 대체 방법: +, mkdir 등이 있으면 그 전까지만 사용
						parts := strings.Split(joinCommand, " + ")
						if len(parts) > 1 {
							joinCommand = strings.TrimSpace(parts[0])
							log.Printf("+ 기호로 정리한 join 명령어: '%s'", joinCommand)
						} else if strings.Contains(joinCommand, "mkdir") {
							parts = strings.Split(joinCommand, "mkdir")
							if len(parts) > 1 {
								joinCommand = strings.TrimSpace(parts[0])
								log.Printf("mkdir로 정리한 join 명령어: '%s'", joinCommand)
							}
						}
					}
				}
			}

			// control-plane 용 인증서 키 추출
			certificateKey := ""
			if len(checkResults) > 1 && checkResults[1].Output != "" {
				certificateKey = strings.TrimSpace(checkResults[1].Output)
				log.Printf("추출된 인증서 키 부분: %s", certificateKey)
			}

			// 디버깅 로그 출력
			if len(checkResults) > 2 {
				log.Printf("로그 파일 앞부분: %s", checkResults[2].Output)
			}
			if len(checkResults) > 3 {
				log.Printf("로그 파일 뒷부분: %s", checkResults[3].Output)
			}

			// 유효한 join 명령어가 있는지 확인
			if joinCommand != "" && strings.Contains(joinCommand, "kubeadm join") {
				log.Printf("유효한 join 명령어를 찾았습니다. DB 업데이트를 시도합니다.")

				// DB에 join 명령어와 인증서 키 저장
				log.Printf("저장할 join_command: %s, certificate_key: %s", joinCommand, certificateKey)
				err := db.UpdateServerJoinCommand(h.DB, requestBody.ID, joinCommand, certificateKey)
				if err != nil {
					log.Printf("join 명령어 DB 업데이트 중 오류 발생: %v", err)
					return
				}

				log.Printf("서버 ID %d의 join 명령어가 성공적으로 업데이트되었습니다.", requestBody.ID)
			} else {
				log.Printf("유효한 join 명령어를 찾지 못했습니다. 로그 파일 확인 필요")
				// 마지막 시도 - 전체 로그에서 join 명령어가 있는지 확인
				lastAttemptCmd := []string{
					`grep -n "kubeadm join" /tmp/k8s_install.log`,
				}
				lastResults, _ := sshUtils.ExecuteCommands(requestBody.Hops, lastAttemptCmd, 30000)
				if len(lastResults) > 0 {
					log.Printf("로그 파일의 kubeadm join 라인들: %s", lastResults[0].Output)
				}
			}
		}()
	} else {
		// 실패한 명령어와 그 결과를 반환
		failedCommands := []gin.H{}
		for _, result := range results {
			if result.ExitCode != 0 {
				failedCommands = append(failedCommands, gin.H{
					"command":  result.Command,
					"output":   result.Output,
					"error":    result.Error,
					"exitCode": result.ExitCode,
				})
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":        false,
			"error":          "쿠버네티스 마스터 노드 설치 시작 중 오류가 발생했습니다.",
			"failedCommands": failedCommands,
		})
	}
}

func (h *InfraHandler) JoinMaster(c *gin.Context) {
	var requestBody struct {
		Password   string          `json:"password"`
		ID         int             `json:"id"`          // 현재 마스터 노드 ID
		MainID     int             `json:"main_id"`     // 메인 마스터 노드 ID
		Hops       []ssh.HopConfig `json:"hops"`        // 마스터 노드 SSH 연결 정보
		LBHops     []ssh.HopConfig `json:"lb_hops"`     // 로드 밸런서 SSH 연결 정보
		LBPassword string          `json:"lb_password"` // 로드 밸런서 서버 패스워드
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		log.Println("JSON 바인딩 오류:", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "잘못된 요청입니다."})
		return
	}

	// 쿠버네티스 마스터 노드 서버 정보 가져오기
	serverInfo, err := db.GetServerInfo(h.DB, requestBody.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 서버 이름 변수 설정 (DB에서 가져온 값 사용)
	serverName := serverInfo.ServerName
	log.Printf("DB에서 가져온 서버 이름: %s를 사용합니다.", serverName)

	// 마스터 노드 IP 주소 가져오기
	sshUtils := utils.NewSSHUtils()
	ipCmd := []string{"ip -4 addr show | awk '/inet / && $2 ~ /^192/ {print $2}' | cut -d/ -f1 | head -n 1"}
	ipResults, err := sshUtils.ExecuteCommands(requestBody.Hops, ipCmd, 30000)
	if err != nil || len(ipResults) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "마스터 노드 IP 주소를 가져오는 중 오류가 발생했습니다."})
		return
	}
	masterIP := strings.TrimSpace(ipResults[0].Output)
	log.Printf("마스터 노드 IP 주소: %s", masterIP)

	// 항상 기본 포트 6443 사용
	port := "6443"
	var lbIP string = masterIP // 기본값으로 마스터 IP 사용

	// 로드 밸런서 패스워드가 제공되었는지 확인
	if requestBody.LBPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "로드 밸런서 패스워드가 제공되지 않았습니다."})
		return
	}

	// 로드 밸런서 IP 주소 가져오기
	lbIpCmd := []string{"ip -4 addr show | awk '/inet / && $2 ~ /^192/ {print $2}' | cut -d/ -f1 | head -n 1"}
	lbIpResults, err := sshUtils.ExecuteCommands(requestBody.LBHops, lbIpCmd, 30000)
	if err != nil || len(lbIpResults) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "로드 밸런서 IP 주소를 가져오는 중 오류가 발생했습니다."})
		return
	}
	lbIP = strings.TrimSpace(lbIpResults[0].Output)
	log.Printf("로드 밸런서 IP 주소: %s", lbIP)

	// HAProxy 설정 업데이트 - 스크립트 파일 사용
	haproxyUpdateCmd := []string{
		// 1. 스크립트 파일 생성
		fmt.Sprintf(`cat > /tmp/update_haproxy.sh << 'EOF'
#!/bin/bash
set -e

# 백업 생성
cp /etc/haproxy/haproxy.cfg /etc/haproxy/haproxy.cfg.bak.$(date +%%Y%%m%%d%%H%%M%%S)

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

echo "HAProxy 설정이 성공적으로 업데이트되었습니다."
EOF`, serverName, masterIP, port, serverName, masterIP, port),

		// 2. 스크립트 실행 권한 부여
		"chmod +x /tmp/update_haproxy.sh",

		// 3. sudo로 스크립트 실행
		fmt.Sprintf("echo '%s' | sudo -S bash /tmp/update_haproxy.sh", requestBody.LBPassword),

		// 4. 임시 스크립트 파일 삭제
		"rm -f /tmp/update_haproxy.sh",
	}

	lbResults, err := sshUtils.ExecuteCommands(requestBody.LBHops, haproxyUpdateCmd, 60000) // 타임아웃 60초로 증가
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "로드 밸런서 HAProxy 설정 업데이트 중 오류가 발생했습니다.", "errorDetails": err.Error()})
		return
	}

	// 로드 밸런서 설정 결과 확인
	if !allCommandsSuccessful(lbResults) {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "로드 밸런서 HAProxy 설정 업데이트 중 오류가 발생했습니다.", "errorDetails": lbResults[len(lbResults)-2].Error}) // 스크립트 실행 결과 오류 반환
		return
	}
	log.Println("로드 밸런서 HAProxy 설정이 성공적으로 업데이트되었습니다.")

	// 메인 마스터 노드의 join_command와 certificate_key 가져오기
	mainMasterInfo, err := db.GetServerInfo(h.DB, requestBody.MainID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "메인 마스터 노드 정보를 가져오는 중 오류가 발생했습니다: " + err.Error()})
		return
	}

	joinCommand := mainMasterInfo.JoinCommand
	certificateKey := mainMasterInfo.CertificateKey

	if joinCommand == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "메인 마스터 노드의 join 명령어가 없습니다. 먼저 마스터 노드를 설치해야 합니다."})
		return
	}

	log.Printf("가져온 join 명령어: %s", joinCommand)
	log.Printf("가져온 인증서 키: %s", certificateKey)

	// 2. 쿠버네티스 마스터 노드 join 스크립트
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
`, port, lbIP, serverName, joinCommand, certificateKey)

	// 3. 마스터 노드에 설치 스크립트 실행
	finalCommands := []string{
		// 1. 스크립트를 파일로 저장
		fmt.Sprintf("cat > /tmp/join_k8s.sh << 'EOL'\n%s\nEOL", installScript),
		// 2. 실행 권한 부여
		"chmod +x /tmp/join_k8s.sh",
		// 3. 로그 파일에 출력 저장하면서 스크립트 실행 (sudo 권한으로)
		fmt.Sprintf("echo '%s' | sudo -S bash /tmp/join_k8s.sh > /tmp/k8s_join.log 2>&1 & echo $! > /tmp/k8s_join.pid", requestBody.Password),
		// 4. 설치 시작 확인
		"echo '쿠버네티스 마스터 노드 조인이 백그라운드에서 시작되었습니다. 로그 파일: /tmp/k8s_join.log, PID: '$(cat /tmp/k8s_join.pid)",
	}

	results, err := sshUtils.ExecuteCommands(requestBody.Hops, finalCommands, 30000) // 30초 (스크립트 시작만 확인)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "SSH 명령어 실행 중 오류가 발생했습니다.", "errorDetails": err.Error()})
		return
	}

	// 결과 처리
	if allCommandsSuccessful(results) {
		output := results[len(results)-1].Output // 마지막 명령어의 출력 (설치 시작 확인)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "쿠버네티스 마스터 노드 조인이 백그라운드에서 시작되었습니다.",
			"details": output,
			"logFile": "/tmp/k8s_join.log",
			"note":    "조인 진행 상황을 확인하려면 로그 파일을 확인하세요.",
		})

		// 백그라운드에서 조인 완료 확인
		go func() {
			// 조인 완료 메시지를 확인하는 명령어
			checkJoinCompleteCmd := []string{
				"while ! grep -q '마스터 노드 조인 완료' /tmp/k8s_join.log 2>/dev/null; do sleep 10; done; echo '조인이 완료되었습니다.'",
			}

			// 조인 완료될 때까지 대기
			_, err := sshUtils.ExecuteCommands(requestBody.Hops, checkJoinCompleteCmd, 900000) // 15분 타임아웃
			if err != nil {
				log.Printf("조인 완료 확인 중 오류 발생: %v", err)
				return
			}

			log.Printf("마스터 노드 조인이 완료되었습니다.")
		}()
	} else {
		// 실패한 명령어와 그 결과를 반환
		failedCommands := []gin.H{}
		for _, result := range results {
			if result.ExitCode != 0 {
				failedCommands = append(failedCommands, gin.H{
					"command":  result.Command,
					"output":   result.Output,
					"error":    result.Error,
					"exitCode": result.ExitCode,
				})
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":        false,
			"error":          "쿠버네티스 마스터 노드 조인 시작 중 오류가 발생했습니다.",
			"failedCommands": failedCommands,
		})
	}
}

func (h *InfraHandler) JoinWorker(c *gin.Context) {
	var requestBody struct {
		Password string          `json:"password"`
		ID       int             `json:"id"`      // 현재 워커 노드 ID
		MainID   int             `json:"main_id"` // 메인 마스터 노드 ID
		Hops     []ssh.HopConfig `json:"hops"`    // 워커 노드 SSH 연결 정보
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		log.Println("JSON 바인딩 오류:", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "잘못된 요청입니다."})
		return
	}

	// 쿠버네티스 워커 노드 서버 정보 가져오기
	serverInfo, err := db.GetServerInfo(h.DB, requestBody.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 서버 이름 변수 설정 (DB에서 가져온 값 사용)
	serverName := serverInfo.ServerName
	log.Printf("DB에서 가져온 서버 이름: %s를 사용합니다.", serverName)

	// 메인 마스터 노드의 join_command 가져오기
	mainMasterInfo, err := db.GetServerInfo(h.DB, requestBody.MainID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "메인 마스터 노드 정보를 가져오는 중 오류가 발생했습니다: " + err.Error()})
		return
	}

	joinCommand := mainMasterInfo.JoinCommand

	if joinCommand == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "메인 마스터 노드의 join 명령어가 없습니다. 먼저 마스터 노드를 설치해야 합니다."})
		return
	}

	log.Printf("가져온 join 명령어: %s", joinCommand)

	// 2. 쿠버네티스 워커 노드 join 스크립트
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

echo "워커 노드 조인 완료"
`, serverName, joinCommand)

	// 3. 워커 노드에 설치 스크립트 실행
	finalCommands := []string{
		// 1. 스크립트를 파일로 저장
		fmt.Sprintf("cat > /tmp/join_k8s.sh << 'EOL'\n%s\nEOL", installScript),
		// 2. 실행 권한 부여
		"chmod +x /tmp/join_k8s.sh",
		// 3. 로그 파일에 출력 저장하면서 스크립트 실행 (sudo 권한으로)
		fmt.Sprintf("echo '%s' | sudo -S bash /tmp/join_k8s.sh > /tmp/k8s_join.log 2>&1 & echo $! > /tmp/k8s_join.pid", requestBody.Password),
		// 4. 설치 시작 확인
		"echo '쿠버네티스 워커 노드 조인이 백그라운드에서 시작되었습니다. 로그 파일: /tmp/k8s_join.log, PID: '$(cat /tmp/k8s_join.pid)",
	}

	sshUtils := utils.NewSSHUtils()
	results, err := sshUtils.ExecuteCommands(requestBody.Hops, finalCommands, 30000) // 30초 (스크립트 시작만 확인)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "SSH 명령어 실행 중 오류가 발생했습니다.", "errorDetails": err.Error()})
		return
	}

	// 결과 처리
	if allCommandsSuccessful(results) {
		output := results[len(results)-1].Output // 마지막 명령어의 출력 (설치 시작 확인)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "쿠버네티스 워커 노드 조인이 백그라운드에서 시작되었습니다.",
			"details": output,
			"logFile": "/tmp/k8s_join.log",
			"note":    "조인 진행 상황을 확인하려면 로그 파일을 확인하세요.",
		})

		// 백그라운드에서 조인 완료 확인
		go func() {
			// 조인 완료 메시지를 확인하는 명령어
			checkJoinCompleteCmd := []string{
				"while ! grep -q '워커 노드 조인 완료' /tmp/k8s_join.log 2>/dev/null; do sleep 10; done; echo '조인이 완료되었습니다.'",
			}

			// 조인 완료될 때까지 대기
			_, err := sshUtils.ExecuteCommands(requestBody.Hops, checkJoinCompleteCmd, 900000) // 15분 타임아웃
			if err != nil {
				log.Printf("조인 완료 확인 중 오류 발생: %v", err)
				return
			}

			log.Printf("워커 노드 조인이 완료되었습니다.")
		}()
	} else {
		// 실패한 명령어와 그 결과를 반환
		failedCommands := []gin.H{}
		for _, result := range results {
			if result.ExitCode != 0 {
				failedCommands = append(failedCommands, gin.H{
					"command":  result.Command,
					"output":   result.Output,
					"error":    result.Error,
					"exitCode": result.ExitCode,
				})
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":        false,
			"error":          "쿠버네티스 워커 노드 조인 시작 중 오류가 발생했습니다.",
			"failedCommands": failedCommands,
		})
	}
}
func (h *InfraHandler) DeleteWorker(c *gin.Context) {
	var requestBody struct {
		Password     string          `json:"password"`
		MainPassword string          `json:"main_password"`
		ID           int             `json:"id"`        // 현재 워커 노드 ID
		MainID       int             `json:"main_id"`   // 메인 마스터 노드 ID
		Hops         []ssh.HopConfig `json:"hops"`      // 워커 노드 SSH 연결 정보
		MainHops     []ssh.HopConfig `json:"main_hops"` // 메인 마스터 노드 SSH 연결 정보
	}

	// 이 부분이 누락되었습니다! JSON 요청 본문을 구조체에 바인딩하는 코드
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		log.Println("JSON 바인딩 오류:", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "잘못된 요청입니다."})
		return
	}

	// 디버깅을 위한 로그 추가
	log.Printf("요청 받은 워커 노드 ID: %d, 메인 노드 ID: %d", requestBody.ID, requestBody.MainID)

	// 워커 노드 서버 정보 가져오기
	serverInfo, err := db.GetServerInfo(h.DB, requestBody.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 서버 이름 변수 설정 (DB에서 가져온 값 사용)
	serverName := serverInfo.ServerName
	log.Printf("DB에서 가져온 서버 이름: %s를 삭제합니다.", serverName)

	// 메인 마스터 노드의 SSH 연결 정보 가져오기
	var masterHops []ssh.HopConfig
	masterHops = requestBody.MainHops

	// 마스터 노드에서 실행할 명령어 (cordon, drain, delete)
	masterCommands := []string{
		// 1. 노드 cordon (새로운 파드 스케줄링 방지)
		fmt.Sprintf("echo '%s' | sudo -S kubectl cordon %s", requestBody.MainPassword, serverName),

		// 2. 노드 drain (기존 파드 안전하게 제거)
		fmt.Sprintf("echo '%s' | sudo -S kubectl drain %s --ignore-daemonsets --delete-emptydir-data --force", requestBody.MainPassword, serverName),

		// 3. 노드 삭제
		fmt.Sprintf("echo '%s' | sudo -S kubectl delete node %s", requestBody.MainPassword, serverName),
	}

	// 워커 노드에서 실행할 명령어 (쿠버네티스 관련 패키지 제거)
	workerCommands := []string{
		// 1. kubeadm reset 실행
		fmt.Sprintf("echo '%s' | sudo -S kubeadm reset -f", requestBody.Password),

		// 2. CNI 설정 파일 제거
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /etc/cni/net.d/*", requestBody.Password),

		// 3. iptables 규칙 정리
		fmt.Sprintf("echo '%s' | sudo -S iptables -F", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S iptables -t nat -F", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S iptables -t mangle -F", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S iptables -X", requestBody.Password),

		// 4. IPVS 테이블 정리 (클러스터가 IPVS를 사용한 경우)
		fmt.Sprintf("echo '%s' | sudo -S ipvsadm --clear 2>/dev/null || true", requestBody.Password),

		// 5. kubeconfig 파일 정리
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /root/.kube", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /etc/kubernetes/admin.conf", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /etc/kubernetes/kubelet.conf", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf ~/.kube", requestBody.Password),

		// 6. 서비스 중지
		fmt.Sprintf("echo '%s' | sudo -S systemctl stop kubelet", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl stop containerd", requestBody.Password),

		// 7. 프로세스 강제 종료
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 kube-apiserver 2>/dev/null || true", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 kube-scheduler 2>/dev/null || true", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 kube-controller-manager 2>/dev/null || true", requestBody.Password),

		// 8. 마운트 해제 및 파드 디렉토리 정리
		fmt.Sprintf("echo '%s' | sudo -S umount -l /var/lib/kubelet/pods/* 2>/dev/null || true", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /var/lib/kubelet/pods/*", requestBody.Password),

		// 9. 쿠버네티스 관련 디렉토리 강제 정리
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /var/lib/kubelet", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /var/lib/etcd", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /etc/kubernetes", requestBody.Password),

		// 10. systemd 서비스 비활성화
		fmt.Sprintf("echo '%s' | sudo -S systemctl disable kubelet", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl disable containerd", requestBody.Password),

		// 11. 패키지 제거 (대화형 프롬프트 비활성화)
		fmt.Sprintf("echo '%s' | sudo -S DEBIAN_FRONTEND=noninteractive apt-get remove --allow-change-held-packages -y kubeadm kubectl kubelet kubernetes-cni || true", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S DEBIAN_FRONTEND=noninteractive apt-get purge -y kubeadm kubectl kubelet kubernetes-cni || true", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S DEBIAN_FRONTEND=noninteractive apt-get clean || true", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S DEBIAN_FRONTEND=noninteractive apt-get autoremove -y || true", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl disable containerd || true", requestBody.Password),
	}

	// SSH 유틸리티 초기화
	sshUtils := utils.NewSSHUtils()

	// 1. 마스터 노드에서 cordon, drain, delete 실행
	log.Printf("마스터 노드에서 노드 %s의 cordon, drain, delete 작업 실행 중...", serverName)
	masterResults, err := sshUtils.ExecuteCommands(masterHops, masterCommands, 300000) // 5분 타임아웃
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "마스터 노드에서 명령어 실행 중 오류가 발생했습니다.", "errorDetails": err.Error()})
		return
	}

	// 2. 워커 노드에서 쿠버네티스 관련 패키지 제거
	log.Printf("워커 노드 %s에서 쿠버네티스 관련 패키지 제거 중...", serverName)
	workerResults, err := sshUtils.ExecuteCommands(requestBody.Hops, workerCommands, 300000) // 5분 타임아웃

	// 워커 노드 접속 실패는 무시 (이미 종료되었을 수 있음)
	if err != nil {
		log.Printf("워커 노드 접속 실패 (이미 종료되었을 수 있음): %v", err)
	}

	// 3. DB에서 워커 노드 삭제
	err = db.DeleteWorker(h.DB, requestBody.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "DB에서 워커 노드 삭제 중 오류가 발생했습니다.", "errorDetails": err.Error()})
		return
	}

	// 결과 처리
	masterOutput := ""
	for _, result := range masterResults {
		masterOutput += result.Output + "\n"
	}

	workerOutput := ""
	if workerResults != nil {
		for _, result := range workerResults {
			workerOutput += result.Output + "\n"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("워커 노드 %s가 성공적으로 삭제되었습니다.", serverName),
		"details": gin.H{
			"masterNodeOperations": masterOutput,
			"workerNodeCleanup":    workerOutput,
		},
	})
}
func (h *InfraHandler) DeleteMaster(c *gin.Context) {
	var requestBody struct {
		Password           string          `json:"password"`      // 삭제할 마스터 노드 패스워드
		ID                 int             `json:"id"`            // 삭제할 마스터 노드 ID
		Hops               []ssh.HopConfig `json:"hops"`          // 삭제할 마스터 노드 SSH 연결 정보
		MainMasterID       int             `json:"main_id"`       // 메인 마스터 노드 ID
		MainMasterHops     []ssh.HopConfig `json:"main_hops"`     // 메인 마스터 노드 SSH 연결 정보
		MainMasterPassword string          `json:"main_password"` // 메인 마스터 노드 패스워드
		LBHops             []ssh.HopConfig `json:"lb_hops"`       // 로드 밸런서 SSH 연결 정보
		LBPassword         string          `json:"lb_password"`   // 로드 밸런서 패스워드
	}

	// JSON 요청 본문을 구조체에 바인딩
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		log.Println("JSON 바인딩 오류:", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "잘못된 요청입니다."})
		return
	}

	// 디버깅을 위한 로그 추가
	log.Printf("요청 받은 마스터 노드 ID: %d", requestBody.ID)
	log.Printf("SSH 연결 정보: %+v", requestBody.Hops)

	// 삭제할 마스터 노드 서버 정보 가져오기
	serverInfo, err := db.GetServerInfo(h.DB, requestBody.ID)
	if err != nil {
		log.Printf("서버 정보 가져오기 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 서버 이름 변수 설정 (DB에서 가져온 값 사용)
	serverName := serverInfo.ServerName
	log.Printf("DB에서 가져온 서버 이름: %s를 삭제합니다.", serverName)
	log.Printf("서버 정보: %+v", serverInfo)

	// 메인 마스터 노드 여부 확인
	isMainMaster := serverInfo.JoinCommand != "" && serverInfo.CertificateKey != ""
	log.Printf("마스터 노드 %s는 메인 마스터 노드입니까? %v", serverName, isMainMaster)

	// 서버의 infra_id 가져오기
	var infraID int
	err = h.DB.QueryRow("SELECT infra_id FROM servers WHERE id = ?", requestBody.ID).Scan(&infraID)
	if err != nil {
		log.Printf("infra_id 가져오기 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "서버의 infra_id를 가져오는 중 오류가 발생했습니다.", "errorDetails": err.Error()})
		return
	}
	log.Printf("서버의 infra_id: %d", infraID)

	// 같은 infra_id에 다른 마스터 노드가 있는지 확인
	var otherMasterCount int
	query := `
		SELECT COUNT(*) 
		FROM servers 
		WHERE infra_id = ? 
		AND id != ? 
		AND type LIKE '%master%'
	`
	err = h.DB.QueryRow(query, infraID, requestBody.ID).Scan(&otherMasterCount)
	if err != nil {
		log.Printf("다른 마스터 노드 확인 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "다른 마스터 노드 확인 중 오류가 발생했습니다.", "errorDetails": err.Error()})
		return
	}
	log.Printf("같은 infra_id에 다른 마스터 노드 수: %d", otherMasterCount)

	// 메인 마스터 노드이고 다른 마스터 노드가 있는 경우 삭제 거부
	if isMainMaster && otherMasterCount > 0 {
		log.Printf("경고: 다른 마스터 노드가 있는 상태에서 메인 마스터 노드를 삭제하려고 합니다.")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "다른 마스터 노드가 있는 상태에서 메인 마스터 노드는 삭제할 수 없습니다. 먼저 다른 마스터 노드를 모두 삭제하세요.",
		})
		return
	}

	// SSH 유틸리티 초기화
	sshUtils := utils.NewSSHUtils()

	// 로드 밸런서에서 해당 마스터 노드 제거
	if len(requestBody.LBHops) > 0 && requestBody.LBPassword != "" {
		log.Printf("로드 밸런서에서 마스터 노드 %s 제거 시작", serverName)

		// HAProxy 설정 업데이트 스크립트
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
			fmt.Sprintf("echo '%s' | sudo -S bash /tmp/remove_server.sh", requestBody.LBPassword),

			// 임시 스크립트 파일 삭제
			"rm -f /tmp/remove_server.sh",
		}

		_, err = sshUtils.ExecuteCommands(requestBody.LBHops, haproxyUpdateCmd, 60000)
		if err != nil {
			log.Printf("로드 밸런서 HAProxy 설정 업데이트 실패: %v", err)
			// 치명적이지 않으므로 계속 진행
		} else {
			log.Printf("로드 밸런서 HAProxy 설정에서 마스터 노드 %s 제거 완료", serverName)
		}
	}

	// 로그 파일 설정 명령
	logSetupCommands := []string{
		fmt.Sprintf("echo '===== 마스터 노드 %s 삭제 작업 시작 =====' > /tmp/master_delete.log", serverName),
		fmt.Sprintf("echo '시작 시간: %s' >> /tmp/master_delete.log", time.Now().Format(time.RFC3339)),
	}

	// 로그 파일 설정 실행
	_, err = sshUtils.ExecuteCommands(requestBody.Hops, logSetupCommands, 10000)
	if err != nil {
		log.Printf("로그 파일 설정 실패: %v", err)
		// 치명적이지 않으므로 계속 진행
	}

	var mainMasterCleanupOutput string
	var autoCleanupPerformed bool
	var masterCleanupOutput string

	// 메인 마스터가 아니면서 다른 마스터 노드가 있는 경우
	if !isMainMaster && otherMasterCount > 0 {
		log.Printf("일반 마스터 노드 삭제 - 메인 마스터에서 노드 정리 작업 수행")

		// 1단계: 메인 마스터에서 노드 제거
		mainNodeCommands := []string{
			// 1.1. 노드 드레인 (파드 제거)
			fmt.Sprintf("echo '%s' | sudo -S kubectl drain %s --delete-emptydir-data --force --ignore-daemonsets", requestBody.MainMasterPassword, serverName),

			// 1.2. 노드 삭제
			fmt.Sprintf("echo '%s' | sudo -S kubectl delete node %s", requestBody.MainMasterPassword, serverName),
		}

		mainNodeResults, err := sshUtils.ExecuteCommands(requestBody.MainMasterHops, mainNodeCommands, 60000)
		if err != nil {
			log.Printf("메인 마스터에서 노드 제거 실패: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "메인 마스터에서 노드 제거 실패", "errorDetails": err.Error()})
			return
		}

		for _, result := range mainNodeResults {
			mainMasterCleanupOutput += result.Output + "\n"
		}

		// 2단계: etcd 멤버 리스트 확인 및 제거
		etcdListCmd := []string{
			fmt.Sprintf("echo '%s' | sudo -S ETCDCTL_API=3 etcdctl --endpoints=localhost:2379 --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/server.crt --key=/etc/kubernetes/pki/etcd/server.key member list", requestBody.MainMasterPassword),
		}

		etcdListResult, err := sshUtils.ExecuteCommands(requestBody.MainMasterHops, etcdListCmd, 30000)
		if err == nil {
			mainMasterCleanupOutput += etcdListResult[0].Output + "\n"

			// etcd 멤버 ID 추출
			etcdFindCmd := []string{
				fmt.Sprintf("echo '%s' | sudo -S bash -c \"ETCDCTL_API=3 etcdctl --endpoints=localhost:2379 --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/server.crt --key=/etc/kubernetes/pki/etcd/server.key member list | grep %s | cut -d',' -f1\"", requestBody.MainMasterPassword, serverName),
			}

			etcdFindResult, err := sshUtils.ExecuteCommands(requestBody.MainMasterHops, etcdFindCmd, 30000)
			if err == nil {
				etcdMemberID := strings.TrimSpace(etcdFindResult[0].Output)
				if etcdMemberID != "" {
					// etcd 멤버 제거
					etcdRemoveCmd := []string{
						fmt.Sprintf("echo '%s' | sudo -S ETCDCTL_API=3 etcdctl --endpoints=localhost:2379 --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/server.crt --key=/etc/kubernetes/pki/etcd/server.key member remove %s", requestBody.MainMasterPassword, etcdMemberID),
					}

					etcdRemoveResult, err := sshUtils.ExecuteCommands(requestBody.MainMasterHops, etcdRemoveCmd, 30000)
					if err == nil {
						mainMasterCleanupOutput += etcdRemoveResult[0].Output + "\n"
					}
				}
			}
		}

		// etcd 최종 확인
		time.Sleep(5 * time.Second)
		nodeListCmd := []string{
			fmt.Sprintf("echo '%s' | sudo -S kubectl get nodes", requestBody.MainMasterPassword),
		}

		nodeListResult, err := sshUtils.ExecuteCommands(requestBody.MainMasterHops, nodeListCmd, 30000)
		if err == nil {
			mainMasterCleanupOutput += nodeListResult[0].Output + "\n"
		}

		autoCleanupPerformed = true
		time.Sleep(10 * time.Second)
	} else if isMainMaster && otherMasterCount == 0 {
		// 메인 마스터 노드이면서 다른 마스터 노드가 없는 경우 (마지막 마스터)
		log.Printf("메인 마스터 노드 삭제 - 클러스터 리셋 수행")
	}

	// 대상 노드 서비스 중지
	stopServicesCmd := []string{
		fmt.Sprintf("echo '%s' | sudo -S systemctl stop kubelet || true", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl stop etcd || true", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 etcd || true", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 kube-apiserver || true", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 kube-scheduler || true", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 kube-controller-manager || true", requestBody.Password),
	}

	_, _ = sshUtils.ExecuteCommands(requestBody.Hops, stopServicesCmd, 60000)

	// kubeadm reset 및 정리
	cleanupCommands := []string{
		fmt.Sprintf("echo '%s' | sudo -S kubeadm reset -f", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /etc/cni/net.d/*", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S iptables -F", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S iptables -t nat -F", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S iptables -t mangle -F", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S iptables -X", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S ipvsadm --clear", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /root/.kube", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /etc/kubernetes/admin.conf", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /etc/kubernetes/kubelet.conf", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl stop kubelet", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl stop containerd", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 kube-apiserver", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 kube-scheduler", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 kube-controller-manager", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S umount -l /var/lib/kubelet/pods/* || true", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /var/lib/kubelet/pods/*", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /var/lib/kubelet", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /var/lib/etcd", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /etc/kubernetes", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl disable kubelet", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /opt/cni", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /usr/bin/kubectl", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /usr/bin/kubeadm", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /usr/bin/kubelet", requestBody.Password),
	}

	cleanupResults, err := sshUtils.ExecuteCommands(requestBody.Hops, cleanupCommands, 120000)
	if err == nil {
		for _, result := range cleanupResults {
			masterCleanupOutput += result.Output + "\n"
		}
	}

	// 패키지 제거
	removeCommands := []string{
		fmt.Sprintf("echo '%s' | sudo -S DEBIAN_FRONTEND=noninteractive apt-get remove --allow-change-held-packages -y kubeadm kubectl kubelet kubernetes-cni || true", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S DEBIAN_FRONTEND=noninteractive apt-get purge -y kubeadm kubectl kubelet kubernetes-cni || true", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S DEBIAN_FRONTEND=noninteractive apt-get clean || true", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S DEBIAN_FRONTEND=noninteractive apt-get autoremove -y || true", requestBody.Password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl disable containerd || true", requestBody.Password),
	}

	_, _ = sshUtils.ExecuteCommands(requestBody.Hops, removeCommands, 180000)

	// DB에서 마스터 노드 삭제
	err = db.DeleteMaster(h.DB, requestBody.ID)
	if err != nil {
		log.Printf("DB에서 마스터 노드 삭제 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "DB에서 마스터 노드 삭제 실패", "errorDetails": err.Error()})
		return
	}

	// 마스터 노드 삭제 성공 후 로드 밸런서에서 해당 마스터 노드 제거
	if len(requestBody.LBHops) > 0 && requestBody.LBPassword != "" {
		log.Printf("로드 밸런서에서 마스터 노드 %s 제거 시작", serverName)

		// HAProxy 설정 업데이트 스크립트
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
			fmt.Sprintf("echo '%s' | sudo -S bash /tmp/remove_server.sh", requestBody.LBPassword),

			// 임시 스크립트 파일 삭제
			"rm -f /tmp/remove_server.sh",
		}

		_, err = sshUtils.ExecuteCommands(requestBody.LBHops, haproxyUpdateCmd, 60000)
		if err != nil {
			log.Printf("로드 밸런서 HAProxy 설정 업데이트 실패: %v", err)
			// 마스터 노드 삭제는 이미 성공했으므로 경고 로그만 남기고 계속 진행
		} else {
			log.Printf("로드 밸런서 HAProxy 설정에서 마스터 노드 %s 제거 완료", serverName)
		}
	}

	// 로그 파일에 완료 메시지 추가
	logFinishCommands := []string{
		fmt.Sprintf("echo '===== 마스터 노드 %s 삭제 작업 완료 =====' >> /tmp/master_delete.log", serverName),
		fmt.Sprintf("echo '완료 시간: %s' >> /tmp/master_delete.log", time.Now().Format(time.RFC3339)),
	}

	_, _ = sshUtils.ExecuteCommands(requestBody.Hops, logFinishCommands, 10000)

	// 결과 반환
	var warningMessage string
	var manualSteps string

	if isMainMaster {
		warningMessage = fmt.Sprintf("메인 마스터 노드 %s가 삭제되었습니다. 클러스터가 완전히 제거되었습니다.", serverName)
		manualSteps = "클러스터를 다시 사용하려면 새로운 클러스터를 구성해야 합니다."
	} else if autoCleanupPerformed {
		warningMessage = fmt.Sprintf("마스터 노드 %s가 성공적으로 삭제되었습니다. 메인 마스터 노드에서 추가 정리 작업이 수행되었습니다.", serverName)
		manualSteps = "추가 정리 작업이 자동으로 수행되었습니다. 더 이상의 수동 작업이 필요하지 않습니다."
	} else {
		warningMessage = fmt.Sprintf("마스터 노드 %s가 성공적으로 삭제되었습니다.", serverName)
		manualSteps = "클러스터에서 노드가 제거되었습니다."
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": warningMessage,
		"details": gin.H{
			"mainMasterCleanup":    mainMasterCleanupOutput,
			"masterNodeCleanup":    masterCleanupOutput,
			"manualSteps":          manualSteps,
			"isMainMaster":         isMainMaster,
			"otherMasterCount":     otherMasterCount,
			"autoCleanupPerformed": autoCleanupPerformed,
			"logFileLocation":      "/tmp/master_delete.log",
		},
	})
}
func (h *InfraHandler) GetInfras(c *gin.Context) {
	infras, err := db.GetAllInfras(h.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, infras)
}

func (h *InfraHandler) GetInfraById(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service ID"})
		return
	}

	infra, err := db.GetInfraById(h.DB, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, infra)
}

func (h *InfraHandler) CreateInfra(c *gin.Context) {
	var infra db.Infra
	if err := c.ShouldBindJSON(&infra); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	id, err := db.CreateInfra(h.DB, infra)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	infra.Id = id
	c.JSON(http.StatusCreated, infra)
}

func (h *InfraHandler) UpdateInfra(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service Id"})
		return
	}

	var infra db.Infra
	if err := c.ShouldBindJSON(&infra); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	infra.Id = id
	err = db.UpdateInfra(h.DB, infra)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, infra)
}

func (h *InfraHandler) DeleteInfra(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service Id"})
		return
	}

	err = db.DeleteInfra(h.DB, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Infra deleted successfully"})
}

// ImportKubernetesInfra 기존 쿠버네티스 클러스터에서 정보를 가져와 external 타입의 인프라로 등록합니다.
func (h *InfraHandler) ImportKubernetesInfra(c *gin.Context) {
	// JSON 요청 파싱
	var request struct {
		Name string          `json:"name"` // 인프라 이름
		Hops []ssh.HopConfig `json:"hops"` // SSH 연결 정보
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "잘못된 요청입니다: " + err.Error()})
		return
	}

	// 필수 필드 확인
	if request.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "인프라 이름은 필수 항목입니다."})
		return
	}

	if len(request.Hops) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "SSH 연결 정보는 필수 항목입니다."})
		return
	}

	// SSH 유틸리티 생성
	sshUtils := utils.NewSSHUtils()

	// 마지막 hop의 패스워드 가져오기
	lastHopPassword := ""
	if len(request.Hops) > 0 {
		lastHopPassword = request.Hops[len(request.Hops)-1].Password
	}

	// 단계 1: kubectl 명령어 가능한지 확인 (sudo -S 사용)
	kubectlCheckCmd := fmt.Sprintf("echo '%s' | sudo -S which kubectl || echo 'KUBECTL_NOT_FOUND'", lastHopPassword)
	kubectlResults, err := sshUtils.ExecuteCommands(request.Hops, []string{kubectlCheckCmd}, 30000)

	if err != nil || len(kubectlResults) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "SSH 연결 중 오류가 발생했습니다: " + err.Error(),
		})
		return
	}

	if strings.Contains(kubectlResults[0].Output, "KUBECTL_NOT_FOUND") {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "kubectl 명령어를 찾을 수 없습니다. 쿠버네티스가 설치되어 있는지 확인하세요.",
		})
		return
	}

	// 단계 2: 클러스터 정보 수집
	commands := []string{
		// 클러스터 정보
		fmt.Sprintf("echo '%s' | sudo -S kubectl cluster-info", lastHopPassword),
		// 노드 정보
		fmt.Sprintf("echo '%s' | sudo -S kubectl get nodes -o wide", lastHopPassword),
		// 네임스페이스 목록
		fmt.Sprintf("echo '%s' | sudo -S kubectl get namespaces", lastHopPassword),
	}

	results, err := sshUtils.ExecuteCommands(request.Hops, commands, 60000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "쿠버네티스 정보 수집 중 오류가 발생했습니다: " + err.Error(),
		})
		return
	}

	// 클러스터 정보 파싱
	namespaceInfo := ""

	for i, result := range results {
		if result.ExitCode != 0 {
			log.Printf("명령 실행 실패: %s, 오류: %s", result.Command, result.Error)
			continue
		}

		switch i {
		case 2:
			namespaceInfo = result.Output
		}
	}

	// JSON 문자열로 변환
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "클러스터 정보 처리 중 오류가 발생했습니다: " + err.Error(),
		})
		return
	}

	// 데이터베이스에 인프라 등록
	tx, err := h.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "데이터베이스 트랜잭션 시작 중 오류가 발생했습니다: " + err.Error(),
		})
		return
	}
	defer tx.Rollback()

	// 1. 인프라 등록 (타입은 external)
	var infraID int
	err = tx.QueryRow(
		"INSERT INTO infras (name, type, info) VALUES (?, ?, ?) RETURNING id",
		request.Name, "external_kubernetes", request.Name,
	).Scan(&infraID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "인프라 등록 중 오류가 발생했습니다: " + err.Error(),
		})
		return
	}

	// 2. 서버 등록
	// hops 정보에서 민감한 정보(username, password) 제거
	hopsWithoutCredentials := make([]map[string]interface{}, 0, len(request.Hops))
	for _, hop := range request.Hops {
		// host와 port만 포함
		hopInfo := map[string]interface{}{
			"host": hop.Host,
			"port": hop.Port,
		}
		hopsWithoutCredentials = append(hopsWithoutCredentials, hopInfo)
	}

	// 민감 정보가 제거된 hops 정보만 JSON으로 변환
	hopsJSON, _ := json.Marshal(hopsWithoutCredentials)

	_, err = tx.Exec(
		"INSERT INTO servers (infra_id, server_name, hops, type, last_checked) VALUES (?, ?, ?, ?, ?)",
		infraID, request.Name, string(hopsJSON), "external_kubernetes", time.Now().Format("2006-01-02 15:04:05"),
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "서버 등록 중 오류가 발생했습니다: " + err.Error(),
		})
		return
	}

	// 트랜잭션 커밋
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "데이터베이스 커밋 중 오류가 발생했습니다: " + err.Error(),
		})
		return
	}

	// 파싱된 쿠버네티스 리소스 정보
	var namespaces []string
	var registeredNamespaces []string

	// 네임스페이스 목록 파싱
	namespaceLines := strings.Split(namespaceInfo, "\n")
	for i, line := range namespaceLines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue // 헤더나 빈 줄 건너뛰기
		}
		fields := strings.Fields(line)
		if len(fields) >= 1 {
			namespace := fields[0]
			namespaces = append(namespaces, namespace)

			// 기본 네임스페이스는 services 테이블에 등록하지 않음 (kube-system, default 등)
			if namespace == "kube-system" || namespace == "default" ||
				namespace == "kube-public" || namespace == "kube-node-lease" {
				continue
			}

			// services 테이블에 네임스페이스 등록
			_, err := h.DB.Exec(
				"INSERT INTO services (name, namespace, infra_id, user_id) VALUES (?, ?, ?, ?)",
				namespace, namespace, infraID, 1,
			)

			if err != nil {
				log.Printf("네임스페이스 %s 등록 중 오류: %s", namespace, err.Error())
				continue
			}

			registeredNamespaces = append(registeredNamespaces, namespace)
		}
	}

	// 응답 구성
	c.JSON(http.StatusOK, gin.H{
		"success":             true,
		"message":             "외부 쿠버네티스 클러스터를 성공적으로 가져왔습니다.",
		"infra_id":            infraID,
		"namespaces":          namespaces,
		"registered_services": registeredNamespaces,
		"server_name":         request.Name,
	})
}

// Docker Compose 프로젝트 이름 추출 함수
func extractProjectName(containerName string) string {
	// 컨테이너 이름에서 프로젝트 이름 추출
	// 예: smartbiz_smart-biz_1 -> smartbiz
	parts := strings.Split(containerName, "_")
	if len(parts) > 0 {
		return parts[0]
	}
	return "unknown"
}

// ImportDockerInfra 수정
func (h *InfraHandler) ImportDockerInfra(c *gin.Context) {
	// JSON 요청 파싱
	var request struct {
		Name string          `json:"name"` // 인프라 이름
		Hops []ssh.HopConfig `json:"hops"` // SSH 연결 정보
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "잘못된 요청입니다: " + err.Error()})
		return
	}

	// 필수 필드 확인
	if request.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "인프라 이름은 필수 항목입니다."})
		return
	}

	if len(request.Hops) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "SSH 연결 정보는 필수 항목입니다."})
		return
	}

	// 마지막 hop의 패스워드 가져오기
	lastHopPassword := ""
	if len(request.Hops) > 0 {
		lastHopPassword = request.Hops[len(request.Hops)-1].Password
	}

	// SSH 유틸리티 생성
	sshUtils := utils.NewSSHUtils()

	// 단계 1: docker 명령어 가능한지 확인 (sudo -S 사용)
	dockerCheckCmd := fmt.Sprintf("echo '%s' | sudo -S docker version || echo 'DOCKER_NOT_FOUND'", lastHopPassword)
	dockerResults, err := sshUtils.ExecuteCommands(request.Hops, []string{dockerCheckCmd}, 30000)

	if err != nil || len(dockerResults) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "SSH 연결 중 오류가 발생했습니다: " + err.Error(),
		})
		return
	}

	if strings.Contains(dockerResults[0].Output, "DOCKER_NOT_FOUND") {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "docker 명령어를 찾을 수 없습니다. 도커가 설치되어 있는지 확인하세요.",
		})
		return
	}

	// 단계 2: 컨테이너 정보 수집
	containerCmd := fmt.Sprintf("echo '%s' | sudo -S docker ps -a --format '{{.ID}}|{{.Names}}|{{.Image}}|{{.Networks}}'", lastHopPassword)
	containerResults, err := sshUtils.ExecuteCommands(request.Hops, []string{containerCmd}, 30000)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "도커 컨테이너 정보 수집 중 오류가 발생했습니다: " + err.Error(),
		})
		return
	}

	// 디버깅 정보 출력
	log.Printf("컨테이너 명령어 실행 결과:\n%s", containerResults[0].Output)

	// 데이터베이스에 인프라 등록
	tx, err := h.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "데이터베이스 트랜잭션 시작 중 오류가 발생했습니다: " + err.Error(),
		})
		return
	}
	defer tx.Rollback()

	// 1. 인프라 등록 (타입은 docker)
	var infraID int
	err = tx.QueryRow(
		"INSERT INTO infras (name, type, info) VALUES (?, ?, ?) RETURNING id",
		request.Name, "external_docker", request.Name,
	).Scan(&infraID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "인프라 등록 중 오류가 발생했습니다: " + err.Error(),
		})
		return
	}

	// 2. 서버 등록
	// hops 정보에서 민감한 정보(username, password) 제거
	hopsWithoutCredentials := make([]map[string]interface{}, 0, len(request.Hops))
	for _, hop := range request.Hops {
		// host와 port만 포함
		hopInfo := map[string]interface{}{
			"host": hop.Host,
			"port": hop.Port,
		}
		hopsWithoutCredentials = append(hopsWithoutCredentials, hopInfo)
	}

	// 민감 정보가 제거된 hops 정보만 JSON으로 변환
	hopsJSON, _ := json.Marshal(hopsWithoutCredentials)

	_, err = tx.Exec(
		"INSERT INTO servers (infra_id, server_name, hops, type, last_checked) VALUES (?, ?, ?, ?, ?)",
		infraID, request.Name, string(hopsJSON), "external_docker", time.Now().Format("2006-01-02 15:04:05"),
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "서버 등록 중 오류가 발생했습니다: " + err.Error(),
		})
		return
	}

	// 트랜잭션 커밋
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "데이터베이스 커밋 중 오류가 발생했습니다: " + err.Error(),
		})
		return
	}

	// 그룹별 컨테이너 정보 파싱
	var serviceGroups = make(map[string][]map[string]string)
	var registeredServices []string

	// 컨테이너 정보 파싱 및 그룹핑
	lines := strings.Split(containerResults[0].Output, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) >= 4 {
			containerName := parts[1]
			projectName := extractProjectName(containerName)

			// k8scontrol 관련 서비스는 하나의 그룹으로 묶기
			if strings.HasPrefix(containerName, "k8scontrol-") {
				projectName = "k8scontrol"
			}

			containerInfo := map[string]string{
				"name":    containerName,
				"image":   parts[2],
				"network": parts[3],
			}

			if _, exists := serviceGroups[projectName]; !exists {
				serviceGroups[projectName] = make([]map[string]string, 0)
				registeredServices = append(registeredServices, projectName)

				// services 테이블에 서비스 그룹 등록
				_, err := h.DB.Exec(
					"INSERT INTO services (name, infra_id, user_id) VALUES (?, ?, ?)",
					projectName, infraID, 1,
				)
				if err != nil {
					log.Printf("서비스 %s 등록 중 오류: %s", projectName, err.Error())
					continue
				}
			}
			serviceGroups[projectName] = append(serviceGroups[projectName], containerInfo)
		}
	}

	// 응답 구성
	c.JSON(http.StatusOK, gin.H{
		"success":             true,
		"message":             "도커 환경을 성공적으로 가져왔습니다.",
		"infra_id":            infraID,
		"server_name":         request.Name,
		"registered_services": registeredServices,
		"service_groups":      serviceGroups,
	})
}

// contains 문자열 배열에 특정 문자열이 포함되어 있는지 확인
func contains(arr []string, str string) bool {
	for _, a := range arr {
		if a == str {
			return true
		}
	}
	return false
}

// collectResources 시스템 리소스 정보를 수집합니다
func collectResources(requestBody struct {
	Hops []ssh.HopConfig `json:"hops"`
}) (map[string]string, error) {
	// 마지막 hop의 비밀번호 추출
	lastHopPassword := ""
	if len(requestBody.Hops) > 0 {
		lastHopPassword = requestBody.Hops[len(requestBody.Hops)-1].Password
	}

	// 리소스 정보 수집 명령어
	commands := []string{
		// CPU 정보
		"cat /proc/cpuinfo | grep 'model name' | head -1 | cut -d ':' -f 2 | sed 's/^[ \t]*//'",
		"nproc --all",
		"top -bn1 | grep 'Cpu(s)' | awk '{print 100 - $1}'",

		// 메모리 정보
		"free -m | grep Mem | awk '{print $2}'",
		"free -m | grep Mem | awk '{print $3}'",
		"free -m | grep Mem | awk '{print $4}'",
		"free | grep Mem | awk '{printf \"%.2f\", $3/$2 * 100}'",

		// 디스크 정보 (sudo 사용)
		fmt.Sprintf("echo '%s' | sudo -S df -h / | tail -1 | awk '{print $2}'", lastHopPassword),
		fmt.Sprintf("echo '%s' | sudo -S df -h / | tail -1 | awk '{print $3}'", lastHopPassword),
		fmt.Sprintf("echo '%s' | sudo -S df -h / | tail -1 | awk '{print $4}'", lastHopPassword),
		fmt.Sprintf("echo '%s' | sudo -S df -h / | tail -1 | awk '{print $5}'", lastHopPassword),

		// 네트워크 정보 (sudo 사용)
		fmt.Sprintf("echo '%s' | sudo -S ip -4 addr show | grep inet | awk '{print $NF, $2}' | grep -v '127.0.0.1'", lastHopPassword),

		// OS 정보
		"hostname",
		"cat /etc/os-release | grep PRETTY_NAME | cut -d '\"' -f 2",
		"uname -r",
	}

	// SSH 유틸리티 인스턴스 생성
	sshUtils := utils.NewSSHUtils()

	// 명령어 실행 (60초 타임아웃)
	results, err := sshUtils.ExecuteCommands(requestBody.Hops, commands, 60000)
	if err != nil {
		return nil, err
	}

	// 결과를 맵으로 변환
	resourceMap := make(map[string]string)

	// 결과가 충분한지 확인
	if len(results) < len(commands) {
		return nil, fmt.Errorf("일부 리소스 정보를 가져오지 못했습니다")
	}

	// 결과 매핑
	resourceMap["cpu_model"] = strings.TrimSpace(results[0].Output)
	resourceMap["cpu_cores"] = strings.TrimSpace(results[1].Output)
	resourceMap["cpu_usage"] = strings.TrimSpace(results[2].Output)

	resourceMap["mem_total"] = strings.TrimSpace(results[3].Output)
	resourceMap["mem_used"] = strings.TrimSpace(results[4].Output)
	resourceMap["mem_free"] = strings.TrimSpace(results[5].Output)
	resourceMap["mem_usage"] = strings.TrimSpace(results[6].Output)

	resourceMap["disk_total"] = strings.TrimSpace(results[7].Output)
	resourceMap["disk_used"] = strings.TrimSpace(results[8].Output)
	resourceMap["disk_free"] = strings.TrimSpace(results[9].Output)
	resourceMap["disk_usage"] = strings.TrimSpace(results[10].Output)

	resourceMap["network_info"] = strings.TrimSpace(results[11].Output)

	resourceMap["hostname"] = strings.TrimSpace(results[12].Output)
	resourceMap["os_name"] = strings.TrimSpace(results[13].Output)
	resourceMap["kernel"] = strings.TrimSpace(results[14].Output)

	return resourceMap, nil
}

// CalculateResources SSH 연결을 통해 서버의 리소스 정보를 가져옵니다.
func (h *InfraHandler) CalculateResources(c *gin.Context) {
	var requestBody struct {
		Hops []ssh.HopConfig `json:"hops"`
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "잘못된 요청입니다.", "detail": err.Error()})
		return
	}

	if len(requestBody.Hops) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "SSH 연결 정보(hops)가 필요합니다."})
		return
	}

	// collectResources 함수를 호출하여 리소스 정보 수집
	resourceMap, err := collectResources(requestBody)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "서버 리소스 정보 수집 중 오류가 발생했습니다.",
			"detail":  err.Error(),
		})
		return
	}

	// 네트워크 정보 파싱
	networkInfos := []gin.H{}
	for _, line := range strings.Split(resourceMap["network_info"], "\n") {
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 2 {
			network := gin.H{
				"interface": fields[0],
				"ip":        fields[1],
			}
			networkInfos = append(networkInfos, network)
		}
	}

	// 결과 반환
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "서버 리소스 정보를 성공적으로 가져왔습니다.",
		"host_info": gin.H{
			"hostname": resourceMap["hostname"],
			"os":       resourceMap["os_name"],
			"kernel":   resourceMap["kernel"],
		},
		"cpu": gin.H{
			"model":         resourceMap["cpu_model"],
			"cores":         resourceMap["cpu_cores"],
			"usage_percent": resourceMap["cpu_usage"],
		},
		"memory": gin.H{
			"total_mb":      resourceMap["mem_total"],
			"used_mb":       resourceMap["mem_used"],
			"free_mb":       resourceMap["mem_free"],
			"usage_percent": resourceMap["mem_usage"],
		},
		"disk": gin.H{
			"root_total":         resourceMap["disk_total"],
			"root_used":          resourceMap["disk_used"],
			"root_free":          resourceMap["disk_free"],
			"root_usage_percent": resourceMap["disk_usage"],
		},
	})
}

// CalculateNodes SSH 연결을 통해 쿠버네티스 클러스터의 노드 정보를 가져옵니다.
func (h *InfraHandler) CalculateNodes(c *gin.Context) {
	var requestBody struct {
		Hops []ssh.HopConfig `json:"hops"`
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "잘못된 요청입니다.", "detail": err.Error()})
		return
	}

	if len(requestBody.Hops) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "SSH 연결 정보(hops)가 필요합니다."})
		return
	}

	// 마지막 hop의 비밀번호 추출
	lastHopPassword := ""
	if len(requestBody.Hops) > 0 {
		lastHopPassword = requestBody.Hops[len(requestBody.Hops)-1].Password
	}

	// SSH 유틸리티 인스턴스 생성
	sshUtils := utils.NewSSHUtils()

	// 쿠버네티스 노드 정보 수집 명령어 (sudo 권한 필요)
	commands := []string{
		fmt.Sprintf("echo '%s' | sudo -S kubectl get nodes -o wide", lastHopPassword),
	}

	// 명령어 실행 (60초 타임아웃)
	results, err := sshUtils.ExecuteCommands(requestBody.Hops, commands, 60000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "쿠버네티스 노드 정보 수집 중 오류가 발생했습니다.",
			"detail":  err.Error(),
		})
		return
	}

	// 결과가 없으면 오류 반환
	if len(results) == 0 || results[0].ExitCode != 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "쿠버네티스 노드 정보를 가져오지 못했습니다.",
			"detail":  "명령어 실행 결과가 없거나 오류가 발생했습니다.",
		})
		return
	}

	// 출력 결과 파싱
	output := results[0].Output
	lines := strings.Split(output, "\n")

	// 노드 정보 저장 변수
	var nodes []map[string]string
	var masterCount, workerCount int

	// 헤더 라인을 제외하고 각 라인 처리
	for i, line := range lines {
		// 첫 번째 라인은 헤더이므로 건너뜀
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}

		// 공백으로 필드 분리
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		nodeName := fields[0]
		nodeInfo := map[string]string{
			"name":   nodeName,
			"status": fields[1],
		}

		// 역할 확인 (master 또는 worker)
		// 노드 라벨을 확인하는 추가 명령 실행
		roleCmd := fmt.Sprintf("echo '%s' | sudo -S kubectl get node %s --show-labels", lastHopPassword, nodeName)
		roleResults, err := sshUtils.ExecuteCommands(requestBody.Hops, []string{roleCmd}, 30000)

		if err == nil && len(roleResults) > 0 && roleResults[0].ExitCode == 0 {
			roleOutput := roleResults[0].Output
			// 마스터 노드는 node-role.kubernetes.io/master 또는 node-role.kubernetes.io/control-plane 라벨을 가짐
			if strings.Contains(roleOutput, "node-role.kubernetes.io/master") ||
				strings.Contains(roleOutput, "node-role.kubernetes.io/control-plane") {
				nodeInfo["role"] = "master"
				masterCount++
			} else {
				nodeInfo["role"] = "worker"
				workerCount++
			}
		} else {
			// 라벨 확인에 실패한 경우 이름으로 추측 (덜 정확함)
			if strings.Contains(strings.ToLower(nodeName), "master") ||
				strings.Contains(strings.ToLower(nodeName), "control") {
				nodeInfo["role"] = "master"
				masterCount++
			} else {
				nodeInfo["role"] = "worker"
				workerCount++
			}
		}

		// 노드 정보 추가
		nodes = append(nodes, nodeInfo)
	}

	// 총 노드 수 계산
	totalNodes := masterCount + workerCount

	// 결과 반환
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "쿠버네티스 노드 정보를 성공적으로 가져왔습니다.",
		"nodes": gin.H{
			"total":  totalNodes,
			"master": masterCount,
			"worker": workerCount,
			"list":   nodes,
		},
	})
}
