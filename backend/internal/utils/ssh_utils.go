package utils

import (
	"time"

	"github.com/k8scontrol/backend/pkg/ssh"
)

// SSHUtils는 SSH 기능을 제공하는 유틸리티입니다.
type SSHUtils struct {
	ssh *ssh.SSHService
}

// NewSSHUtils는 새로운 SSHUtils 인스턴스를 생성합니다.
func NewSSHUtils() *SSHUtils {
	return &SSHUtils{
		ssh: ssh.NewSSHService(),
	}
}

// ExecuteCommands는 SSH를 통해 명령어를 실행합니다.
func (u *SSHUtils) ExecuteCommands(hops []ssh.HopConfig, finalCommands []string, timeoutMs int) ([]ssh.CommandResult, error) {
	// 타임아웃 설정 (밀리초 -> 시간 단위로 변환)
	timeout := time.Duration(timeoutMs) * time.Millisecond
	if timeoutMs == 0 {
		timeout = 120 * time.Second // 기본값 120초
	}

	// SSH 명령어 실행
	return u.ssh.ExecuteCommands(hops, finalCommands, timeout)
}

// ExecuteCommandsOnServer는 단일 서버에 SSH 명령어를 실행하는 간편 메서드입니다.
func (u *SSHUtils) ExecuteCommandsOnServer(host string, port int, username, password string, commands []string, timeoutMs int) ([]ssh.CommandResult, error) {
	hop := ssh.HopConfig{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
	}

	return u.ExecuteCommands([]ssh.HopConfig{hop}, commands, timeoutMs)
}

// IsSSHError는 에러가 SSH 에러인지 확인합니다.
func (u *SSHUtils) IsSSHError(err error) bool {
	_, ok := err.(ssh.SSHError)
	return ok
}

// GetSSHErrorType은 SSH 에러의 타입을 반환합니다.
func (u *SSHUtils) GetSSHErrorType(err error) string {
	if sshErr, ok := err.(ssh.SSHError); ok {
		return string(sshErr.Type)
	}
	return ""
}
