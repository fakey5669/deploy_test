package command

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/k8scontrol/backend/internal/utils"
	"github.com/k8scontrol/backend/pkg/ssh"
)

// CommandTemplate은 액션별 명령어 템플릿을 정의합니다
type CommandTemplate struct {
	Commands     []string
	PrepareFunc  func(params map[string]interface{}) ([]string, error)
	ValidateFunc func(params map[string]interface{}) error
}

// CommandTarget은 명령어 실행 대상을 정의합니다
type CommandTarget struct {
	Hops []ssh.HopConfig
}

// GetDescription은 CommandTarget의 간략한 설명을 반환합니다
func (ct *CommandTarget) GetDescription() string {
	if ct == nil || len(ct.Hops) == 0 {
		return "대상 없음"
	}

	lastHop := ct.Hops[len(ct.Hops)-1]

	if len(ct.Hops) == 1 {
		return fmt.Sprintf("%s:%d", lastHop.Host, lastHop.Port)
	}

	return fmt.Sprintf("%s:%d (hop %d개)", lastHop.Host, lastHop.Port, len(ct.Hops))
}

// CommandManager는 쿠버네티스 관련 명령을 관리합니다
type CommandManager struct {
	commandMap     map[string]CommandTemplate
	sshUtils       utils.SSHUtils
	commandTimeout int // 명령어 실행 타임아웃 (밀리초)
}

// NewCommandManager는 새 CommandManager 인스턴스를 생성합니다
func NewCommandManager() *CommandManager {
	return &CommandManager{
		commandMap:     make(map[string]CommandTemplate),
		sshUtils:       *utils.NewSSHUtils(),
		commandTimeout: 30000, // 기본 타임아웃 30초
	}
}

// RegisterCommand는 새로운 명령어 템플릿을 등록합니다
func (cm *CommandManager) RegisterCommand(action string, template CommandTemplate) {
	cm.commandMap[action] = template
}

// HasCommandTemplate은 지정된 액션에 대한 명령어 템플릿이 존재하는지 확인합니다
func (cm *CommandManager) HasCommandTemplate(action string) bool {
	_, exists := cm.commandMap[action]
	return exists
}

// ExecuteAction은 지정된 액션을 실행하고 결과를 반환합니다
func (cm *CommandManager) ExecuteAction(action string, params map[string]interface{}, target *CommandTarget) ([]ssh.CommandResult, error) {
	// 액션 로깅
	log.Printf("[CommandManager] 액션 실행: %s, 파라미터: %+v", action, params)

	// 액션에 대한 명령어 템플릿 가져오기
	commands, err := cm.PrepareAction(action, params)
	if err != nil {
		log.Printf("[CommandManager] 액션 %s 명령어 준비 실패: %v", action, err)
		return nil, err
	}
	log.Printf("[CommandManager] 액션 %s의 명령어 %d개 준비 완료", action, len(commands))

	// 대상이 없으면 기본 대상 사용
	if target == nil {
		target = &CommandTarget{}
	}

	// 명령어 실행
	startTime := time.Now()
	results, err := cm.ExecuteCustomCommands(target, commands)
	executionTime := time.Since(startTime)

	if err != nil {
		log.Printf("[CommandManager] 액션 %s 실행 실패 (소요시간: %v): %v", action, executionTime, err)
		return nil, err
	}

	log.Printf("[CommandManager] 액션 %s 실행 성공 (소요시간: %v): %d개의 결과", action, executionTime, len(results))
	return results, nil
}

// ExecuteCustomCommands는 주어진 명령어를 대상 서버에서 실행합니다
func (cm *CommandManager) ExecuteCustomCommands(target *CommandTarget, commands []string) ([]ssh.CommandResult, error) {
	// 로그 기록
	if target == nil || len(target.Hops) == 0 {
		log.Printf("[CommandManager] 오류: 대상 서버 정보가 없습니다")
		return nil, fmt.Errorf("대상 서버 정보가 없습니다")
	}
	log.Printf("[CommandManager] 커스텀 명령어 실행: %d개의 명령어, 대상: %s", len(commands), target.GetDescription())

	// SSH 유틸리티 초기화
	sshUtils := utils.NewSSHUtils()

	// 명령어 실행
	startTime := time.Now()
	results, err := sshUtils.ExecuteCommands(target.Hops, commands, cm.commandTimeout)
	executionTime := time.Since(startTime)

	// 실행 결과 로깅
	if err != nil {
		// 오류 타입에 따라 더 상세한 로그 추가
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "timed out") {
			log.Printf("[CommandManager] 오류: 커스텀 명령어 실행 중 타임아웃 발생 (소요시간: %v): %v", executionTime, err)
		} else if strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "connect") {
			log.Printf("[CommandManager] 오류: 커스텀 명령어 실행 중 연결 실패 (소요시간: %v): %v", executionTime, err)
		} else if strings.Contains(err.Error(), "authentication") || strings.Contains(err.Error(), "auth") {
			log.Printf("[CommandManager] 오류: 커스텀 명령어 실행 중 인증 실패 (소요시간: %v): %v", executionTime, err)
		} else {
			log.Printf("[CommandManager] 오류: 커스텀 명령어 실행 중 에러 발생 (소요시간: %v): %v", executionTime, err)
		}
		return nil, err
	}

	log.Printf("[CommandManager] 명령어 실행 성공 (소요시간: %v): %d개의 결과", executionTime, len(results))
	return results, nil
}

// PrepareAction은 액션에 맞는 명령어를 준비합니다
func (cm *CommandManager) PrepareAction(action string, params map[string]interface{}) ([]string, error) {
	// 1. 액션에 맞는 명령어 템플릿 가져오기
	template, exists := cm.commandMap[action]
	if !exists {
		log.Printf("[CommandManager] 오류: 지원하지 않는 액션입니다: %s", action)
		return nil, fmt.Errorf("지원하지 않는 액션입니다: %s", action)
	}

	// 2. 파라미터 검증
	if template.ValidateFunc != nil {
		if err := template.ValidateFunc(params); err != nil {
			log.Printf("[CommandManager] 오류: 파라미터 검증 실패: %v", err)
			return nil, fmt.Errorf("파라미터 검증 실패: %w", err)
		}
	}

	// 3. 명령어 준비
	commands := template.Commands
	if template.PrepareFunc != nil {
		var err error
		commands, err = template.PrepareFunc(params)
		if err != nil {
			log.Printf("[CommandManager] 오류: 명령어 준비 실패: %v", err)
			return nil, fmt.Errorf("명령어 준비 실패: %w", err)
		}
	}

	return commands, nil
}

// SetCommandTimeout은 명령어 실행 타임아웃을 설정합니다
func (cm *CommandManager) SetCommandTimeout(timeout int) {
	cm.commandTimeout = timeout
}

// GetCommandTimeout은 현재 설정된 명령어 실행 타임아웃을 반환합니다
func (cm *CommandManager) GetCommandTimeout() int {
	return cm.commandTimeout
}

// GetRegisteredActions는 등록된 모든 액션 목록을 반환합니다
func (cm *CommandManager) GetRegisteredActions() []string {
	actions := make([]string, 0, len(cm.commandMap))
	for action := range cm.commandMap {
		actions = append(actions, action)
	}
	return actions
}
