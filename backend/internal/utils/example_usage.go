package utils

import (
	"fmt"
	"log"

	"github.com/k8scontrol/backend/pkg/ssh"
)

// 이 파일은 SSH 유틸리티 사용 예제를 제공합니다.
// 실제 코드에서는 필요한 곳에서 SSHUtils를 가져다 사용하면 됩니다.

// ExampleSSHUsage는 SSH 유틸리티 사용 예제를 보여줍니다.
func ExampleSSHUsage() {
	// SSHUtils 인스턴스 생성
	sshUtils := NewSSHUtils()

	// 단일 서버에 명령어 실행 (간편 메서드 사용)
	host := "example.com"
	port := 22
	username := "user"
	password := "password"
	commands := []string{"ls -la", "whoami", "uptime"}
	timeoutMs := 30000 // 30초

	results, err := sshUtils.ExecuteCommandsOnServer(host, port, username, password, commands, timeoutMs)
	if err != nil {
		if sshUtils.IsSSHError(err) {
			errorType := sshUtils.GetSSHErrorType(err)
			log.Printf("SSH Error: %s - %s", errorType, err.Error())
		} else {
			log.Printf("Error: %s", err.Error())
		}
		return
	}

	// 결과 출력
	for _, result := range results {
		fmt.Printf("Command: %s\n", result.Command)
		fmt.Printf("Exit Code: %d\n", result.ExitCode)
		fmt.Printf("Output: %s\n", result.Output)
		if result.Error != "" {
			fmt.Printf("Error: %s\n", result.Error)
		}
		fmt.Println("-------------------")
	}

	// 다중 서버 SSH 연결 (홉 사용)
	bastionHop := ssh.HopConfig{
		Host:     "bastion.example.com",
		Port:     22,
		Username: "bastion-user",
		Password: "bastion-password",
	}

	targetHop := ssh.HopConfig{
		Host:     "private-server.internal",
		Port:     22,
		Username: "target-user",
		Password: "target-password",
	}

	multiHopResults, err := sshUtils.ExecuteCommands(
		[]ssh.HopConfig{bastionHop, targetHop},
		[]string{"hostname", "df -h"},
		60000, // 60초
	)

	if err != nil {
		log.Printf("Multi-hop SSH Error: %s", err.Error())
		return
	}

	fmt.Println("Multi-hop SSH Results:")
	for _, result := range multiHopResults {
		fmt.Printf("Command: %s\n", result.Command)
		fmt.Printf("Output: %s\n", result.Output)
		fmt.Println("-------------------")
	}
}
