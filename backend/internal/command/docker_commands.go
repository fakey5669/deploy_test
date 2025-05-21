package command

import (
	"fmt"
)

// 도커 관련 액션 상수 정의
const (
	// 도커 설치 및 관리
	ActionInstallDocker        = "installDocker"        // 도커 설치
	ActionCheckDockerStatus    = "checkDockerStatus"    // 도커 상태 확인
	ActionGetDockerVersion     = "getDockerVersion"     // 도커 버전 확인
	ActionRestartDockerService = "restartDockerService" // 도커 서비스 재시작
	ActionRemoveDocker         = "removeDocker"         // 도커 제거

	// 컨테이너 관리
	ActionListContainers   = "listContainers"   // 컨테이너 목록 조회
	ActionStartContainer   = "startContainer"   // 컨테이너 시작
	ActionStopContainer    = "stopContainer"    // 컨테이너 중지
	ActionRestartContainer = "restartContainer" // 컨테이너 재시작
	ActionRemoveContainer  = "removeContainer"  // 컨테이너 제거
	ActionCreateContainer  = "createContainer"  // 컨테이너 생성
	ActionGetContainerLogs = "getContainerLogs" // 컨테이너 로그 조회

	// 이미지 관리
	ActionListImages      = "listImages"      // 이미지 목록 조회
	ActionPullImage       = "pullImage"       // 이미지 다운로드
	ActionRemoveImage     = "removeImage"     // 이미지 제거
	ActionBuildImage      = "buildImage"      // 이미지 빌드
	ActionGetImageDetails = "getImageDetails" // 이미지 상세 정보

	// 볼륨 관리
	ActionListVolumes   = "listVolumes"   // 볼륨 목록 조회
	ActionCreateVolume  = "createVolume"  // 볼륨 생성
	ActionRemoveVolume  = "removeVolume"  // 볼륨 제거
	ActionInspectVolume = "inspectVolume" // 볼륨 검사
	ActionPruneVolumes  = "pruneVolumes"  // 미사용 볼륨 정리

	// 네트워크 관리
	ActionListNetworks   = "listNetworks"   // 네트워크 목록 조회
	ActionCreateNetwork  = "createNetwork"  // 네트워크 생성
	ActionRemoveNetwork  = "removeNetwork"  // 네트워크 제거
	ActionInspectNetwork = "inspectNetwork" // 네트워크 검사
	ActionPruneNetworks  = "pruneNetworks"  // 미사용 네트워크 정리

	// Docker Compose
	ActionComposeUp      = "composeUp"      // Docker Compose 구성 시작
	ActionComposeDown    = "composeDown"    // Docker Compose 구성 중지
	ActionComposeRestart = "composeRestart" // Docker Compose 구성 재시작
	ActionComposeLogs    = "composeLogs"    // Docker Compose 로그 조회
)

// RegisterDockerCommands는 도커 관련 명령어 템플릿을 등록합니다
func RegisterDockerCommands(manager *CommandManager) {
	// 도커 설치 명령어 등록
	manager.RegisterCommand(ActionInstallDocker, CommandTemplate{
		ValidateFunc: validateInstallDockerParams,
		PrepareFunc:  prepareInstallDockerCommands,
	})

	// 도커 상태 확인 명령어 등록
	manager.RegisterCommand(ActionCheckDockerStatus, CommandTemplate{
		ValidateFunc: validateDockerServerParams,
		PrepareFunc:  prepareCheckDockerStatusCommands,
	})

	// 도커 버전 확인 명령어 등록
	manager.RegisterCommand(ActionGetDockerVersion, CommandTemplate{
		ValidateFunc: validateDockerServerParams,
		PrepareFunc:  prepareGetDockerVersionCommands,
	})

	// 컨테이너 목록 조회 명령어 등록
	manager.RegisterCommand(ActionListContainers, CommandTemplate{
		ValidateFunc: validateDockerServerParams,
		PrepareFunc:  prepareListContainersCommands,
	})

	// 이미지 목록 조회 명령어 등록
	manager.RegisterCommand(ActionListImages, CommandTemplate{
		ValidateFunc: validateDockerServerParams,
		PrepareFunc:  prepareListImagesCommands,
	})

	// 도커 서비스 재시작 명령어 등록
	manager.RegisterCommand(ActionRestartDockerService, CommandTemplate{
		ValidateFunc: validateDockerServerParams,
		PrepareFunc:  prepareRestartDockerServiceCommands,
	})
}

// 공통 파라미터 검증 함수
func validateDockerServerParams(params map[string]interface{}) error {
	// 패스워드는 필수 파라미터
	if _, exists := params["password"]; !exists {
		return fmt.Errorf("password 파라미터가 필요합니다")
	}
	return nil
}

// 도커 설치 관련 함수
func validateInstallDockerParams(params map[string]interface{}) error {
	// 기본 파라미터 검증
	if err := validateDockerServerParams(params); err != nil {
		return err
	}
	return nil
}

// prepareInstallDockerCommands는 도커 설치 명령어를 준비합니다
func prepareInstallDockerCommands(params map[string]interface{}) ([]string, error) {
	password := getStringParameter(params["password"])

	// 공통 설치 준비 명령어
	prepCommands := []string{
		// 패키지 시스템 초기화 및 손상된 패키지 수정
		fmt.Sprintf("echo '%s' | sudo -S apt-get update > /tmp/docker_install.log 2>&1", password),
		fmt.Sprintf("echo '%s' | sudo -S apt-get install -y ca-certificates curl gnupg software-properties-common apt-transport-https >> /tmp/docker_install.log 2>&1", password),
		// APT 패키지 상태 복구 명령
		fmt.Sprintf("echo '%s' | sudo -S apt-get -f install >> /tmp/docker_install.log 2>&1 || true", password),
		fmt.Sprintf("echo '%s' | sudo -S dpkg --configure -a >> /tmp/docker_install.log 2>&1 || true", password),
	}

	// Docker 공식 설치 스크립트 사용 (가장 안정적인 방법)
	dockerScriptCommands := []string{
		// 기존 설치 파일 및 디렉토리 정리
		fmt.Sprintf("echo '%s' | sudo -S rm -f /etc/apt/sources.list.d/docker.list /etc/apt/keyrings/docker.gpg /etc/apt/keyrings/docker.asc /tmp/docker.gpg >> /tmp/docker_install.log 2>&1 || true", password),
		// 기존 도커 관련 패키지 제거
		fmt.Sprintf("echo '%s' | sudo -S apt-get remove -y docker docker-engine docker.io containerd runc >> /tmp/docker_install.log 2>&1 || true", password),
		fmt.Sprintf("echo '%s' | sudo -S apt-get autoremove -y >> /tmp/docker_install.log 2>&1 || true", password),
		// APT 업데이트
		fmt.Sprintf("echo '%s' | sudo -S apt-get update >> /tmp/docker_install.log 2>&1 || true", password),
		// Docker 공식 설치 스크립트 다운로드 및 실행
		fmt.Sprintf("echo '%s' | sudo -S curl -fsSL https://get.docker.com -o /tmp/get-docker.sh >> /tmp/docker_install.log 2>&1", password),
		fmt.Sprintf("echo '%s' | sudo -S sh /tmp/get-docker.sh >> /tmp/docker_install.log 2>&1", password),
		// Docker 서비스 시작 및 활성화
		fmt.Sprintf("echo '%s' | sudo -S systemctl start docker >> /tmp/docker_install.log 2>&1 || echo '%s' | sudo -S service docker start >> /tmp/docker_install.log 2>&1 || true", password, password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl enable docker >> /tmp/docker_install.log 2>&1 || echo '%s' | sudo -S service docker enable >> /tmp/docker_install.log 2>&1 || true", password, password),
		// 현재 사용자를 도커 그룹에 추가
		fmt.Sprintf("echo '%s' | sudo -S groupadd -f docker >> /tmp/docker_install.log 2>&1", password),
		fmt.Sprintf("echo '%s' | sudo -S usermod -aG docker $(whoami) >> /tmp/docker_install.log 2>&1", password),
		// 설치 확인
		"echo '도커 설치 시도 완료' >> /tmp/docker_install.log",
		fmt.Sprintf("echo '%s' | sudo -S docker --version >> /tmp/docker_install.log 2>&1 || echo '도커 명령어 실행 실패' >> /tmp/docker_install.log", password),
	}

	// 모든 패키지 업데이트 및 재시도
	retryDockerCommands := []string{
		// 패키지 업데이트 및 업그레이드
		fmt.Sprintf("echo '%s' | sudo -S apt-get update >> /tmp/docker_install_retry.log 2>&1 || true", password),
		fmt.Sprintf("echo '%s' | sudo -S apt-get upgrade -y >> /tmp/docker_install_retry.log 2>&1 || true", password),
		// 도커 패키지 직접 설치 (표준 방식)
		fmt.Sprintf("echo '%s' | sudo -S apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin >> /tmp/docker_install_retry.log 2>&1 || true", password),
		// 서비스 시작
		fmt.Sprintf("echo '%s' | sudo -S systemctl start docker >> /tmp/docker_install_retry.log 2>&1 || true", password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl enable docker >> /tmp/docker_install_retry.log 2>&1 || true", password),
	}

	// 스냅 패키지 관련 명령어는 handler에서 필요할 때 직접 실행하므로 여기서는 정의하지 않음

	// 설치 확인 명령어
	checkDockerCommands := []string{
		fmt.Sprintf("echo '%s' | sudo -S docker --version 2>/dev/null || echo 'DOCKER_NOT_FOUND'", password),
		fmt.Sprintf("echo '%s' | sudo -S cat /tmp/docker_install.log || echo '로그 파일을 읽을 수 없습니다.'", password),
		fmt.Sprintf("echo '%s' | sudo -S cat /tmp/docker_install_retry.log 2>/dev/null || echo '재시도 로그 없음'", password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl status docker 2>/dev/null || echo '%s' | sudo -S service docker status 2>/dev/null || echo 'docker 서비스 상태를 확인할 수 없습니다.'", password, password),
	}

	// 모든 명령어를 하나의 슬라이스로 통합
	allCommands := make([]string, 0)
	allCommands = append(allCommands, prepCommands...)
	allCommands = append(allCommands, dockerScriptCommands...)
	allCommands = append(allCommands, retryDockerCommands...)
	allCommands = append(allCommands, checkDockerCommands...)

	return allCommands, nil
}

// 도커 상태 확인 관련 함수
func prepareCheckDockerStatusCommands(params map[string]interface{}) ([]string, error) {
	password := getStringParameter(params["password"])

	// 도커 상태 확인 명령어
	statusCommands := []string{
		fmt.Sprintf("echo '%s' | sudo -S docker --version 2>/dev/null || echo 'DOCKER_NOT_FOUND'", password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl status docker 2>/dev/null || echo '%s' | sudo -S service docker status 2>/dev/null || echo 'docker 서비스 상태를 확인할 수 없습니다.'", password, password),
		fmt.Sprintf("echo '%s' | sudo -S docker info 2>/dev/null || echo 'DOCKER_INFO_FAILED'", password),
		fmt.Sprintf("echo '%s' | sudo -S docker ps -a 2>/dev/null | grep -v CONTAINER | wc -l || echo '0'", password),
	}

	return statusCommands, nil
}

// 도커 버전 확인 관련 함수
func prepareGetDockerVersionCommands(params map[string]interface{}) ([]string, error) {
	password := getStringParameter(params["password"])

	// 도커 버전 확인 명령어
	versionCommands := []string{
		fmt.Sprintf("echo '%s' | sudo -S docker --version 2>/dev/null || echo 'DOCKER_NOT_FOUND'", password),
		fmt.Sprintf("echo '%s' | sudo -S docker version 2>/dev/null || echo 'DOCKER_VERSION_FAILED'", password),
	}

	return versionCommands, nil
}

// 컨테이너 목록 조회 관련 함수
func prepareListContainersCommands(params map[string]interface{}) ([]string, error) {
	password := getStringParameter(params["password"])

	// 컨테이너 목록 조회 명령어 (JSON 형식)
	listContainersCommands := []string{
		fmt.Sprintf("echo '%s' | sudo -S docker ps -a --format '{{json .}}' 2>/dev/null || echo 'DOCKER_PS_FAILED'", password),
	}

	return listContainersCommands, nil
}

// 이미지 목록 조회 관련 함수
func prepareListImagesCommands(params map[string]interface{}) ([]string, error) {
	password := getStringParameter(params["password"])

	// 이미지 목록 조회 명령어 (JSON 형식)
	listImagesCommands := []string{
		fmt.Sprintf("echo '%s' | sudo -S docker images --format '{{json .}}' 2>/dev/null || echo 'DOCKER_IMAGES_FAILED'", password),
	}

	return listImagesCommands, nil
}

// 도커 서비스 재시작 관련 함수
func prepareRestartDockerServiceCommands(params map[string]interface{}) ([]string, error) {
	password := getStringParameter(params["password"])

	// 도커 서비스 재시작 명령어
	restartCommands := []string{
		fmt.Sprintf("echo '%s' | sudo -S systemctl restart docker >> /tmp/docker_restart.log 2>&1 || echo '%s' | sudo -S service docker restart >> /tmp/docker_restart.log 2>&1", password, password),
		fmt.Sprintf("echo '%s' | sudo -S docker info 2>/dev/null || echo 'DOCKER_INFO_FAILED'", password),
	}

	return restartCommands, nil
}
