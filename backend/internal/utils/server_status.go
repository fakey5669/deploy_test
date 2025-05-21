package utils

import (
	"fmt"
	"strings"
	"time"

	"github.com/k8scontrol/backend/pkg/ssh"
)

// ServerStatus는 서버의 상태 정보를 나타냅니다.
type ServerStatus struct {
	Installed   bool   // 필요한 소프트웨어가 설치되었는지 여부
	Running     bool   // 서비스가 실행 중인지 여부
	IsMaster    bool   // 마스터 노드인지 여부 (마스터/워커 노드에만 적용)
	IsWorker    bool   // 워커 노드인지 여부 (마스터/워커 노드에만 적용)
	LastChecked string // 마지막 확인 시간
	ServerType  string // 서버 타입 (ha, master, worker)
}

// ServerStatusUtils는 서버 상태 확인을 위한 유틸리티입니다.
type ServerStatusUtils struct {
	sshUtils *SSHUtils
}

// NewServerStatusUtils는 새 ServerStatusUtils 인스턴스를 생성합니다.
func NewServerStatusUtils() *ServerStatusUtils {
	return &ServerStatusUtils{
		sshUtils: NewSSHUtils(),
	}
}

// GetServerStatus는 SSH를 통해 서버의 상태를 확인합니다.
func (u *ServerStatusUtils) GetServerStatus(
	hops []ssh.HopConfig,
	serverType string,
) (*ServerStatus, error) {
	// 지원하는 서버 타입 확인
	serverType = strings.ToLower(serverType)
	if !isValidServerType(serverType) {
		return nil, fmt.Errorf("지원하지 않는 서버 타입입니다: %s", serverType)
	}

	// 상태 확인 명령어 생성
	cmd := getStatusCommand(serverType)

	// 상태 확인 실행
	status, err := u.executeStatusCheck(hops, cmd, serverType)
	if err != nil {
		return nil, err
	}

	return status, nil
}

// isValidServerType은 서버 타입이 유효한지 확인합니다.
func isValidServerType(serverType string) bool {
	validTypes := []string{"ha", "master", "worker"}
	for _, t := range validTypes {
		if serverType == t {
			return true
		}
	}
	return false
}

// getStatusCommand는 서버 타입에 따른 상태 확인 명령어를 반환합니다.
func getStatusCommand(serverType string) string {
	var cmd string

	switch serverType {
	case "ha":
		cmd = "echo '===START==='; " +
			"if dpkg -l | grep -q haproxy; then echo 'INSTALLED=true'; else echo 'INSTALLED=false'; fi; " +
			"if systemctl status haproxy | grep -q 'Active: active (running)'; then echo 'RUNNING=true'; else echo 'RUNNING=false'; fi; " +
			"echo '===END==='"
	case "master":
		cmd = "echo '===START==='; " +
			"if command -v kubectl >/dev/null 2>&1 && command -v kubelet >/dev/null 2>&1; then echo 'INSTALLED=true'; else echo 'INSTALLED=false'; fi; " +
			"if systemctl status kubelet | grep -q 'Active: active (running)'; then echo 'KUBELET_RUNNING=true'; else echo 'KUBELET_RUNNING=false'; fi; " +
			"if kubectl get nodes 2>/dev/null | grep -q $(hostname) | grep -q master; then echo 'IS_MASTER=true'; else echo 'IS_MASTER=false'; fi; " +
			"if kubectl get nodes 2>/dev/null | grep -q $(hostname) | grep -v master; then echo 'IS_WORKER=true'; else echo 'IS_WORKER=false'; fi; " +
			"if kubectl get nodes 2>/dev/null | grep -q $(hostname); then echo 'NODE_REGISTERED=true'; else echo 'NODE_REGISTERED=false'; fi; " +
			"echo '===END==='"
	case "worker":
		cmd = "echo '===START==='; " +
			"if command -v kubectl >/dev/null 2>&1 && command -v kubelet >/dev/null 2>&1; then echo 'INSTALLED=true'; else echo 'INSTALLED=false'; fi; " +
			"if systemctl status kubelet | grep -q 'Active: active (running)'; then echo 'KUBELET_RUNNING=true'; else echo 'KUBELET_RUNNING=false'; fi; " +
			"if kubectl get nodes 2>/dev/null | grep -q $(hostname) | grep -v master; then echo 'IS_WORKER=true'; else echo 'IS_WORKER=false'; fi; " +
			"if kubectl get nodes 2>/dev/null | grep -q $(hostname); then echo 'NODE_REGISTERED=true'; else echo 'NODE_REGISTERED=false'; fi; " +
			"echo '===END==='"
	}

	return cmd
}

// executeStatusCheck는 SSH를 통해 상태 확인 명령어를 실행하고 결과를 파싱합니다.
func (u *ServerStatusUtils) executeStatusCheck(hops []ssh.HopConfig, cmd string, serverType string) (*ServerStatus, error) {
	// 기본값 설정
	status := &ServerStatus{
		Installed:   false,
		Running:     false,
		IsMaster:    false,
		IsWorker:    false,
		LastChecked: time.Now().Format("2006-01-02 15:04:05"),
		ServerType:  serverType,
	}

	// 최대 10번 재시도
	var output string
	success := false

	for attempt := 1; attempt <= 10; attempt++ {
		fmt.Printf("명령어 실행 시도 %d/10...\n", attempt)

		results, err := u.sshUtils.ExecuteCommands(hops, []string{cmd}, 20000)
		if err != nil {
			fmt.Printf("시도 %d 실패: %v\n", attempt, err)
			if attempt < 3 {
				time.Sleep(1 * time.Second)
				continue
			}
			break
		}

		if len(results) > 0 {
			output = results[0].Output
			if strings.Contains(output, "===START===") && strings.Contains(output, "===END===") {
				success = true
				break
			}
		}

		if attempt < 3 {
			time.Sleep(1 * time.Second)
		}
	}

	// 명령어 실행 성공 시 결과 파싱
	if success {
		fmt.Printf("명령어 실행 결과: %s\n", output)

		if strings.Contains(output, "INSTALLED=true") {
			status.Installed = true
		}

		// 서버 타입에 따라 다른 방식으로 running 상태 판단
		if serverType == "ha" {
			if strings.Contains(output, "RUNNING=true") {
				status.Running = true
			}
		} else if serverType == "master" || serverType == "worker" {
			kubeletRunning := strings.Contains(output, "KUBELET_RUNNING=true")
			nodeRegistered := strings.Contains(output, "NODE_REGISTERED=true")

			if strings.Contains(output, "IS_MASTER=true") {
				status.IsMaster = true
			}

			if strings.Contains(output, "IS_WORKER=true") {
				status.IsWorker = true
			}

			// 마스터 노드는 kubelet이 실행 중이고 마스터로 등록되어 있어야 함
			if serverType == "master" {
				status.Running = kubeletRunning && status.IsMaster && nodeRegistered
			}

			// 워커 노드는 kubelet이 실행 중이고 워커로 등록되어 있어야 함
			if serverType == "worker" {
				status.Running = kubeletRunning && status.IsWorker && nodeRegistered
			}
		}
	} else {
		fmt.Println("모든 시도 실패 - 기본값 사용")
	}

	fmt.Printf("최종 상태 - installed: %v, running: %v, isMaster: %v, isWorker: %v\n",
		status.Installed, status.Running, status.IsMaster, status.IsWorker)
	return status, nil
}

// GetServerStatusWithSingleHop은 단일 서버에 대한 상태 확인을 위한 편의 함수입니다.
func (u *ServerStatusUtils) GetServerStatusWithSingleHop(
	host string,
	port int,
	username string,
	password string,
	serverType string,
) (*ServerStatus, error) {
	hop := ssh.HopConfig{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
	}

	return u.GetServerStatus([]ssh.HopConfig{hop}, serverType)
}
