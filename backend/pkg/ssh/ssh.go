package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// Target은 SSH 대상 서버와 관련 설정을 나타냅니다
type Target struct {
	Hops []HopConfig
}

// HopConfig는 SSH 연결을 위한 설정을 정의합니다
type HopConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// CommandResult는 명령어 실행 결과를 저장합니다
type CommandResult struct {
	Command  string `json:"command"`
	Output   string `json:"output"`
	Error    string `json:"error"`
	ExitCode int    `json:"exitCode"`
}

// ErrorType은 SSH 연결 또는 명령 실행 중 발생할 수 있는 오류 유형을 정의합니다
type ErrorType string

const (
	AuthenticationFailed   ErrorType = "AUTHENTICATION_FAILED"
	ConnectionRefused      ErrorType = "CONNECTION_REFUSED"
	ConnectionTimeout      ErrorType = "CONNECTION_TIMEOUT"
	HostNotFound           ErrorType = "HOST_NOT_FOUND"
	TunnelingFailed        ErrorType = "TUNNELING_FAILED"
	CommandExecutionFailed ErrorType = "COMMAND_EXECUTION_FAILED"
	ValidationError        ErrorType = "VALIDATION_ERROR"
	UnknownError           ErrorType = "UNKNOWN_ERROR"
)

// SSHError는 SSH 연결 또는 명령 실행 중 발생하는 오류를 나타냅니다
type SSHError struct {
	Type    ErrorType `json:"type"`
	Message string    `json:"message"`
	Host    string    `json:"host,omitempty"`
	Command string    `json:"command,omitempty"`
}

// Error는 error 인터페이스를 구현합니다
func (e SSHError) Error() string {
	return e.Message
}

// SSHService는 SSH 연결 및 명령어 실행을 담당하는 서비스입니다
type SSHService struct{}

// NewSSHService는 새로운 SSHService 인스턴스를 생성합니다
func NewSSHService() *SSHService {
	return &SSHService{}
}

// getSSHClientConfig는 SSH 클라이언트 설정을 생성합니다
func getSSHClientConfig(hop HopConfig, timeout time.Duration) *ssh.ClientConfig {
	return &ssh.ClientConfig{
		User: hop.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(hop.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}
}

// ExecuteCommands는 여러 SSH 호스트를 통해 연결하고 최종 호스트에서 명령어를 실행합니다
func (s *SSHService) ExecuteCommands(hops []HopConfig, finalCommands []string, timeout time.Duration) ([]CommandResult, error) {
	if timeout == 0 {
		timeout = 120 * time.Second // 기본 타임아웃 120초
	}

	if len(hops) == 0 {
		return nil, SSHError{
			Type:    ValidationError,
			Message: "At least one hop configuration is required",
		}
	}

	if len(finalCommands) == 0 {
		return nil, SSHError{
			Type:    ValidationError,
			Message: "At least one command is required",
		}
	}

	var currentClient *ssh.Client
	var clients []*ssh.Client
	var results []CommandResult

	// 함수 종료 시 모든 클라이언트 연결 종료
	defer func() {
		for i := len(clients) - 1; i >= 0; i-- {
			clients[i].Close()
		}
	}()

	// 첫 번째 호스트에 직접 연결
	firstHop := hops[0]
	if firstHop.Port == 0 {
		firstHop.Port = 22 // 기본 SSH 포트
	}

	config := getSSHClientConfig(firstHop, timeout)
	var err error
	currentClient, err = ssh.Dial("tcp", fmt.Sprintf("%s:%d", firstHop.Host, firstHop.Port), config)
	if err != nil {
		return nil, mapSSHError(err, firstHop.Host)
	}
	clients = append(clients, currentClient)

	// 추가 호스트가 있으면 터널링을 통해 연결
	for i := 1; i < len(hops); i++ {
		hop := hops[i]
		if hop.Port == 0 {
			hop.Port = 22
		}

		// 이전 호스트를 통해 터널 설정
		conn, err := currentClient.Dial("tcp", fmt.Sprintf("%s:%d", hop.Host, hop.Port))
		if err != nil {
			return nil, SSHError{
				Type:    TunnelingFailed,
				Message: fmt.Sprintf("Tunneling failed to %s: %s", hop.Host, err.Error()),
				Host:    hop.Host,
			}
		}

		// 터널을 통해 SSH 연결 설정
		ncc, chans, reqs, err := ssh.NewClientConn(conn, fmt.Sprintf("%s:%d", hop.Host, hop.Port), getSSHClientConfig(hop, timeout))
		if err != nil {
			conn.Close()
			return nil, mapSSHError(err, hop.Host)
		}

		client := ssh.NewClient(ncc, chans, reqs)
		currentClient = client
		clients = append(clients, client)
	}

	// 최종 호스트에서 명령어 실행
	for _, cmd := range finalCommands {
		session, err := currentClient.NewSession()
		if err != nil {
			return results, SSHError{
				Type:    CommandExecutionFailed,
				Message: fmt.Sprintf("Failed to create session: %s", err.Error()),
				Host:    hops[len(hops)-1].Host,
			}
		}

		// 결과 및 오류 스트림 설정
		var stdoutBuf, stderrBuf io.Reader
		stdoutPipe, err := session.StdoutPipe()
		if err != nil {
			session.Close()
			return results, SSHError{
				Type:    CommandExecutionFailed,
				Message: fmt.Sprintf("Failed to setup stdout pipe: %s", err.Error()),
				Command: cmd,
			}
		}
		stdoutBuf = stdoutPipe

		stderrPipe, err := session.StderrPipe()
		if err != nil {
			session.Close()
			return results, SSHError{
				Type:    CommandExecutionFailed,
				Message: fmt.Sprintf("Failed to setup stderr pipe: %s", err.Error()),
				Command: cmd,
			}
		}
		stderrBuf = stderrPipe

		// 명령어 실행 및 결과 수집
		result := CommandResult{
			Command: cmd,
		}

		// 컨텍스트를 사용하여 타임아웃 설정
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		errCh := make(chan error, 1)
		go func() {
			errCh <- session.Run(cmd)
		}()

		// 실행 결과 수집
		stdoutData := make([]byte, 1024)
		stderrData := make([]byte, 1024)

		var output, errOutput string

		// 비동기적으로 stdout 읽기
		stdoutCh := make(chan []byte, 1)
		go func() {
			for {
				n, err := stdoutBuf.Read(stdoutData)
				if n > 0 {
					stdoutCh <- stdoutData[:n]
				}
				if err == io.EOF || err != nil {
					close(stdoutCh)
					return
				}
			}
		}()

		// 비동기적으로 stderr 읽기
		stderrCh := make(chan []byte, 1)
		go func() {
			for {
				n, err := stderrBuf.Read(stderrData)
				if n > 0 {
					stderrCh <- stderrData[:n]
				}
				if err == io.EOF || err != nil {
					close(stderrCh)
					return
				}
			}
		}()

		// 모든 출력 채널과 명령어 완료를 기다림
		for {
			select {
			case <-ctx.Done():
				session.Close()
				return results, SSHError{
					Type:    ConnectionTimeout,
					Message: fmt.Sprintf("Command execution timed out after %v", timeout),
					Command: cmd,
				}
			case data, ok := <-stdoutCh:
				if ok {
					output += string(data)
				}
			case data, ok := <-stderrCh:
				if ok {
					errOutput += string(data)
				}
			case err := <-errCh:
				result.Output = output
				result.Error = errOutput

				// 종료 코드 확인
				exitCode := 0
				if err != nil {
					if exitErr, ok := err.(*ssh.ExitError); ok {
						exitCode = exitErr.ExitStatus()
					} else {
						session.Close()
						return results, SSHError{
							Type:    CommandExecutionFailed,
							Message: fmt.Sprintf("Command execution error: %s", err.Error()),
							Command: cmd,
						}
					}
				}
				result.ExitCode = exitCode

				results = append(results, result)
				session.Close()
				goto NextCommand
			}
		}

	NextCommand:
	}

	return results, nil
}

// mapSSHError는 SSH 오류를 SSHError 타입으로 변환합니다
func mapSSHError(err error, host string) SSHError {
	// 네트워크 오류 확인
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return SSHError{
				Type:    ConnectionTimeout,
				Message: fmt.Sprintf("Connection timeout while connecting to %s", host),
				Host:    host,
			}
		}
	}

	// 다양한 SSH 오류 유형에 따라 적절한 오류 반환
	if errors.Is(err, &net.OpError{}) || errors.Is(err, &net.AddrError{}) {
		// 연결 거부 또는 호스트 찾기 오류
		if msg := err.Error(); strings.Contains(msg, "connection refused") {
			return SSHError{
				Type:    ConnectionRefused,
				Message: fmt.Sprintf("Connection refused to %s", host),
				Host:    host,
			}
		} else if strings.Contains(msg, "no such host") || strings.Contains(msg, "lookup") {
			return SSHError{
				Type:    HostNotFound,
				Message: fmt.Sprintf("Host not found: %s", host),
				Host:    host,
			}
		}
	}

	// 인증 오류
	if msg := err.Error(); strings.Contains(msg, "auth") || strings.Contains(msg, "authentication") {
		return SSHError{
			Type:    AuthenticationFailed,
			Message: fmt.Sprintf("Authentication failed for %s", host),
			Host:    host,
		}
	}

	// 기타 오류
	return SSHError{
		Type:    UnknownError,
		Message: fmt.Sprintf("Unknown error while connecting to %s: %s", host, err.Error()),
		Host:    host,
	}
}
