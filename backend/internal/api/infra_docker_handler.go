package api

import (
	"bytes"
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
	gossh "golang.org/x/crypto/ssh"
)

// InfraHandler 인프라 관련 API 핸들러
type InfraDockerHandler struct {
	DB *sql.DB
}

// NewInfraDockerHandler 새 InfraDockerHandler 생성
func NewInfraDockerHandler(db *sql.DB) *InfraDockerHandler {
	return &InfraDockerHandler{DB: db}
}

// InstallDocker는 원격 서버에 도커를 설치합니다.
func (h *InfraDockerHandler) InstallDocker(c *gin.Context) {
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

	if len(hops) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "SSH 연결 정보(hops)가 필요합니다."})
		return
	}

	// 마지막 hop의 패스워드 사용
	password := ""
	if len(hops) > 0 {
		password = hops[len(hops)-1].Password
	}

	// Ubuntu 버전 확인 명령어
	checkUbuntuVersionCmd := "lsb_release -rs || cat /etc/os-release | grep VERSION_ID | cut -d'\"' -f2"

	sshUtils := utils.NewSSHUtils()
	versionResults, err := sshUtils.ExecuteCommands(hops, []string{checkUbuntuVersionCmd}, 30000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Ubuntu 버전 확인 중 오류가 발생했습니다: " + err.Error()})
		return
	}

	if len(versionResults) == 0 || versionResults[0].ExitCode != 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Ubuntu 버전을 확인할 수 없습니다."})
		return
	}

	ubuntuVersion := strings.TrimSpace(versionResults[0].Output)
	log.Printf("Ubuntu 버전: %s", ubuntuVersion)

	// Docker가 이미 설치되어 있는지 확인
	checkDockerExistsCmd := []string{
		fmt.Sprintf("echo '%s' | sudo -S docker --version 2>/dev/null || echo 'DOCKER_NOT_FOUND'", password),
		fmt.Sprintf("echo '%s' | sudo -S docker ps -a 2>/dev/null | grep -v CONTAINER | wc -l || echo '0'", password), // 실행 중인 컨테이너 수 확인
	}

	existResults, _ := sshUtils.ExecuteCommands(hops, checkDockerExistsCmd, 30000)
	dockerExists := false
	containerCount := 0

	if len(existResults) >= 2 {
		// DOCKER_NOT_FOUND가 출력되지 않았고, Docker version 문자열이 포함되어 있으면 도커가 설치되어 있는 것
		dockerExists = !strings.Contains(existResults[0].Output, "DOCKER_NOT_FOUND") && strings.Contains(existResults[0].Output, "Docker version")
		containerCount, _ = strconv.Atoi(strings.TrimSpace(existResults[1].Output))
	}

	// 이미 도커가 설치되어 있고 컨테이너가 있는 경우 작업 중단
	if dockerExists {
		dockerVersionInfo := strings.TrimSpace(existResults[0].Output)

		// 컨테이너가 있으면 경고 메시지와 함께 현재 상태 반환
		if containerCount > 0 {
			c.JSON(http.StatusOK, gin.H{
				"success":           true,
				"message":           fmt.Sprintf("도커가 이미 설치되어 있으며 %d개의 컨테이너가 존재합니다. 재설치 없이 작업을 중단합니다.", containerCount),
				"docker_version":    dockerVersionInfo,
				"container_count":   containerCount,
				"already_installed": true,
			})
			return
		}

		// 컨테이너가 없으면 사용자에게 확인 요청 (이 API에서는 재설치 금지)
		c.JSON(http.StatusOK, gin.H{
			"success":           true,
			"message":           "도커가 이미 설치되어 있습니다. 재설치가 필요하면 기존 도커를 먼저 제거하세요.",
			"docker_version":    dockerVersionInfo,
			"container_count":   containerCount,
			"already_installed": true,
		})
		return
	}

	// 도커 설치 명령어 생성
	var installDockerCommands []string

	// 공통 설치 준비 명령어
	prepCommands := []string{
		// 패키지 시스템 초기화 및 손상된 패키지 수정
		fmt.Sprintf("echo '%s' | sudo -S apt-get update > /tmp/docker_install.log 2>&1", password),
		fmt.Sprintf("echo '%s' | sudo -S apt-get install -y ca-certificates curl gnupg software-properties-common apt-transport-https >> /tmp/docker_install.log 2>&1", password),
		// APT 패키지 상태 복구 명령
		fmt.Sprintf("echo '%s' | sudo -S apt-get -f install >> /tmp/docker_install.log 2>&1 || true", password),
		fmt.Sprintf("echo '%s' | sudo -S dpkg --configure -a >> /tmp/docker_install.log 2>&1 || true", password),
	}
	installDockerCommands = append(installDockerCommands, prepCommands...)

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

	installDockerCommands = append(installDockerCommands, dockerScriptCommands...)

	// 도커 설치 실행
	_, err = sshUtils.ExecuteCommands(hops, installDockerCommands, 300000) // 5분 타임아웃
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "도커 설치 중 오류가 발생했습니다: " + err.Error()})
		return
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

	_, retryErr := sshUtils.ExecuteCommands(hops, retryDockerCommands, 300000)
	if retryErr != nil {
		log.Println("도커 재설치 중 오류 발생:", retryErr)
	}

	// 설치 성공 여부 확인
	checkDockerCommands := []string{
		fmt.Sprintf("echo '%s' | sudo -S docker --version 2>/dev/null || echo 'DOCKER_NOT_FOUND'", password),
		fmt.Sprintf("echo '%s' | sudo -S cat /tmp/docker_install.log || echo '로그 파일을 읽을 수 없습니다.'", password),
		fmt.Sprintf("echo '%s' | sudo -S cat /tmp/docker_install_retry.log 2>/dev/null || echo '재시도 로그 없음'", password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl status docker 2>/dev/null || echo '%s' | sudo -S service docker status 2>/dev/null || echo 'docker 서비스 상태를 확인할 수 없습니다.'", password, password),
	}

	checkResults, checkErr := sshUtils.ExecuteCommands(hops, checkDockerCommands, 30000)

	dockerInstalled := false
	logContent := "도커 설치 로그를 가져올 수 없습니다."
	retryLogContent := ""
	serviceStatus := ""
	dockerVersion := ""

	if checkErr == nil && len(checkResults) >= 4 {
		// Docker version 문자열이 포함되어 있거나 DOCKER_INSTALLED=true가 포함되어 있으면 설치 성공
		dockerInstalled = strings.Contains(checkResults[0].Output, "Docker version") ||
			strings.Contains(checkResults[0].Output, "DOCKER_INSTALLED=true")

		// 도커 버전 정보 추출
		if strings.Contains(checkResults[0].Output, "Docker version") {
			dockerVersion = strings.TrimSpace(checkResults[0].Output)
		}

		logContent = checkResults[1].Output
		retryLogContent = checkResults[2].Output
		serviceStatus = checkResults[3].Output

		// 로그에서도 Docker version 문자열을 찾아봄 (설치는 성공했지만 확인 명령에서 놓칠 수 있음)
		if !dockerInstalled && (strings.Contains(logContent, "Docker version") || strings.Contains(retryLogContent, "Docker version")) {
			dockerInstalled = true
			log.Println("로그에서 Docker version 문자열을 찾았습니다. 설치 성공으로 판단합니다.")

			// 로그에서 도커 버전 정보 추출 시도
			if dockerVersion == "" {
				versionIndex := strings.Index(logContent, "Docker version")
				if versionIndex >= 0 {
					endOfLine := strings.Index(logContent[versionIndex:], "\n")
					if endOfLine > 0 {
						dockerVersion = strings.TrimSpace(logContent[versionIndex : versionIndex+endOfLine])
					} else {
						dockerVersion = "Docker version 감지됨 (세부 정보 없음)"
					}
				}
			}
		}

		// 서비스 상태에서도 active 상태인지 확인
		if !dockerInstalled && strings.Contains(serviceStatus, "Active: active") {
			dockerInstalled = true
			log.Println("Docker 서비스가 active 상태입니다. 설치 성공으로 판단합니다.")
		}
	}

	// 첫 번째 방법으로 실패한 경우 스냅으로 시도
	if !dockerInstalled {
		log.Println("Docker 설치 스크립트가 실패했습니다. 스냅 패키지로 시도합니다.")

		snapInstallCommands := []string{
			// 스냅으로 도커 설치 시도
			fmt.Sprintf("echo '%s' | sudo -S snap install docker >> /tmp/docker_install_snap.log 2>&1", password),
			// 버전 확인
			fmt.Sprintf("echo '%s' | sudo -S docker --version >> /tmp/docker_install_snap.log 2>&1 || echo '스냅 도커 명령어 실행 실패' >> /tmp/docker_install_snap.log", password),
		}

		_, snapErr := sshUtils.ExecuteCommands(hops, snapInstallCommands, 180000) // 3분 타임아웃

		// 스냅 설치 결과 확인
		if snapErr == nil {
			snapCheckCommands := []string{
				fmt.Sprintf("echo '%s' | sudo -S docker --version 2>/dev/null || echo 'DOCKER_NOT_FOUND'", password),
				fmt.Sprintf("echo '%s' | sudo -S cat /tmp/docker_install_snap.log || echo '스냅 로그 파일을 읽을 수 없습니다.'", password),
			}

			snapResults, _ := sshUtils.ExecuteCommands(hops, snapCheckCommands, 30000)

			if len(snapResults) >= 2 {
				dockerInstalled = strings.Contains(snapResults[0].Output, "Docker version") ||
					strings.Contains(snapResults[0].Output, "DOCKER_INSTALLED=true")

				// 스냅 로그 내용 가져오기
				snapLogContent := snapResults[1].Output

				// 로그에서도 Docker version 문자열을 찾아봄
				if !dockerInstalled && strings.Contains(snapLogContent, "Docker version") {
					dockerInstalled = true
					log.Println("스냅 로그에서 Docker version 문자열을 찾았습니다. 설치 성공으로 판단합니다.")
				}

				// 원래 로그와 스냅 로그 결합
				logContent += "\n\n=== 재시도 로그 ===\n" + retryLogContent + "\n\n=== 스냅 설치 시도 로그 ===\n" + snapLogContent
			}
		}
	} else {
		logContent += "\n\n=== 재시도 로그 ===\n" + retryLogContent
	}

	if dockerInstalled {
		c.JSON(http.StatusOK, gin.H{
			"success":        true,
			"message":        "도커가 성공적으로 설치되었습니다.",
			"docker_version": dockerVersion,
			"service_status": serviceStatus,
			"log":            logContent,
		})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":        false,
			"error":          "도커 설치가 실패했거나 확인할 수 없습니다.",
			"docker_version": dockerVersion,
			"service_status": serviceStatus,
			"log":            logContent,
		})
	}
}

// CreateDockerContainer는 Git 저장소에서 코드를 가져와 도커로 빌드하고 배포합니다.
func (h *InfraDockerHandler) CreateDockerContainer(c *gin.Context) {
	var request struct {
		ID             int             `json:"id"`              // 서버 ID
		Hops           []ssh.HopConfig `json:"hops"`            // SSH 연결 정보
		RepoURL        string          `json:"repo_url"`        // Git 저장소 URL
		Branch         string          `json:"branch"`          // Git 브랜치 (기본값: main)
		ComposePath    string          `json:"compose_path"`    // docker-compose.yml 경로 (기본값: ./docker-compose.yml)
		ComposeProject string          `json:"compose_project"` // 컴포즈 프로젝트 이름 (기본값: 저장소 이름)
		UsernameRepo   string          `json:"username_repo"`   // 저장소 접근을 위한 사용자 이름 (선택)
		PasswordRepo   string          `json:"password_repo"`   // 저장소 접근을 위한 비밀번호 (선택)
		ForceRecreate  bool            `json:"force_recreate"`  // 존재하는 컨테이너를 강제로 재생성할지 여부
		DockerRegistry string          `json:"docker_registry"` // Docker 레지스트리 URL (예: harbor.mipllab.com)
		DockerUsername string          `json:"docker_username"` // Docker 레지스트리 사용자 이름
		DockerPassword string          `json:"docker_password"` // Docker 레지스트리 비밀번호
	}

	// 요청 파싱
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Println("JSON 바인딩 오류:", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "잘못된 요청입니다."})
		return
	}

	// 필수 필드 검증
	if request.RepoURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "저장소 URL은 필수 항목입니다."})
		return
	}

	// 서버 정보 가져오기
	serverInfo, err := db.GetServerInfo(h.DB, request.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 요청 본문에서 hops가 제공되었는지 확인하고, 그렇지 않으면 DB에서 가져옴
	var hops []ssh.HopConfig
	if len(request.Hops) > 0 {
		hops = request.Hops
	} else {
		if err := json.Unmarshal([]byte(serverInfo.Hops), &hops); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "hops 파싱 중 오류가 발생했습니다."})
			return
		}
	}

	if len(hops) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "SSH 연결 정보(hops)가 필요합니다."})
		return
	}

	// 마지막 hop의 패스워드 사용
	password := ""
	if len(hops) > 0 {
		password = hops[len(hops)-1].Password
	}

	// 기본값 설정
	if request.Branch == "" {
		request.Branch = "main"
	}

	// docker-compose.yml 경로 설정 (기본: 저장소 루트의 docker-compose.yml)
	if request.ComposePath == "" {
		request.ComposePath = "docker-compose.yml" // ./ 제거
	} else if strings.HasPrefix(request.ComposePath, "./") {
		// 상대 경로에서 ./ 제거 (스냅 docker-compose와 호환성을 위해)
		request.ComposePath = strings.TrimPrefix(request.ComposePath, "./")
	}

	// 저장소 이름 추출 (URL의 마지막 부분)
	repoName := extractRepoName(request.RepoURL)

	// 컴포즈 프로젝트 이름 설정 (지정되지 않은 경우 저장소 이름 사용)
	if request.ComposeProject == "" {
		request.ComposeProject = repoName
	}

	// 작업 디렉토리 경로 설정
	workDir := fmt.Sprintf("/tmp/%s_build", repoName)

	// SSH 유틸리티 생성
	sshUtils := utils.NewSSHUtils()

	// 명령어 목록 생성
	var commands []string

	// 1. 작업 디렉토리 생성 및 이전 빌드 정리
	commands = append(commands,
		fmt.Sprintf("echo '%s' | sudo -S rm -rf %s", password, workDir),
		fmt.Sprintf("echo '%s' | sudo -S mkdir -p %s", password, workDir),
		fmt.Sprintf("echo '%s' | sudo -S chown $(whoami):$(whoami) %s", password, workDir)) // 디렉토리 권한 변경

	// 2. 필요한 도구 설치 확인 (Git)
	commands = append(commands,
		fmt.Sprintf("which git || (echo '%s' | sudo -S apt-get update && echo '%s' | sudo -S apt-get install -y git)", password, password))

	// 3. Docker Compose 경로 확인 및 설치
	commands = append(commands,
		"which docker-compose || echo 'DOCKER_COMPOSE_NOT_FOUND'",
		"which /snap/bin/docker-compose || echo 'SNAP_DOCKER_COMPOSE_NOT_FOUND'",
		fmt.Sprintf("which docker-compose || (echo '%s' | sudo -S apt-get update && echo '%s' | sudo -S apt-get install -y docker-compose)", password, password))

	// 4. 저장소 클론 (Git 인증 오류 로깅 개선)
	gitCmd := ""
	if request.UsernameRepo != "" && request.PasswordRepo != "" {
		// URL에서 프로토콜 부분 제거
		repoNoProtocol := strings.TrimPrefix(request.RepoURL, "https://")
		repoNoProtocol = strings.TrimPrefix(repoNoProtocol, "http://")

		// @ 문자가 포함된 사용자명 처리 (이메일 주소 등)
		encodedUsername := request.UsernameRepo
		if strings.Contains(encodedUsername, "@") {
			// 사용자명이 이메일인 경우 URL 인코딩 적용
			encodedUsername = strings.ReplaceAll(encodedUsername, "@", "%40")
		}

		gitCmd = fmt.Sprintf("cd %s && git clone -b %s https://%s:%s@%s . 2>&1 || echo 'Git 클론 실패'",
			workDir,
			request.Branch,
			encodedUsername,
			request.PasswordRepo,
			repoNoProtocol)
	} else {
		gitCmd = fmt.Sprintf("cd %s && git clone -b %s %s . 2>&1 || echo 'Git 클론 실패'",
			workDir, request.Branch, request.RepoURL)
	}
	commands = append(commands, gitCmd)

	// 5. 클론 성공 확인 및 디렉토리 내용 확인
	commands = append(commands, fmt.Sprintf("cd %s && ls -la", workDir))

	// 6. docker-compose.yml 파일 존재 확인 (절대 경로 사용)
	checkComposePath := request.ComposePath
	composeFullPath := fmt.Sprintf("%s/%s", workDir, checkComposePath)

	// yml과 yaml 확장자 모두 확인
	ymlPath := fmt.Sprintf("%s/docker-compose.yml", workDir)
	yamlPath := fmt.Sprintf("%s/docker-compose.yaml", workDir)

	// 둘 중 하나라도 있으면 그 파일 사용
	commands = append(commands, fmt.Sprintf("if [ -f %s ]; then echo 'YML_EXISTS'; elif [ -f %s ]; then echo 'YAML_EXISTS'; else echo 'ERROR: docker-compose.yml 또는 docker-compose.yaml 파일을 찾을 수 없습니다!'; fi", ymlPath, yamlPath))

	// 7. 기존 컨테이너 정리 (강제 재생성 옵션이 켜져있는 경우)
	if request.ForceRecreate {
		downCommand := fmt.Sprintf("cd %s && echo '%s' | sudo -S docker-compose -p %s down",
			workDir, password, request.ComposeProject)
		commands = append(commands, downCommand)
	}

	// 8.5 Docker 레지스트리 로그인 (필요한 경우)
	if request.DockerRegistry != "" && request.DockerUsername != "" && request.DockerPassword != "" {
		// Docker 로그인 명령 추가
		loginCmd := fmt.Sprintf("echo '%s' | docker login %s -u %s --password-stdin",
			request.DockerPassword, request.DockerRegistry, request.DockerUsername)
		commands = append(commands, loginCmd)
	}

	// 9. Docker Compose로 빌드 및 실행
	commands = append(commands, fmt.Sprintf("cd %s && echo '%s' | sudo -S docker-compose -p %s up -d --build",
		workDir, password, request.ComposeProject))

	// 10. 컨테이너 상태 확인
	commands = append(commands, fmt.Sprintf("cd %s && echo '%s' | sudo -S docker-compose -p %s ps",
		workDir, password, request.ComposeProject))

	// 11. 도커 이미지 및 컨테이너 목록 확인 (디버깅용)
	commands = append(commands,
		"docker images",
		"docker ps -a")

	// 12. 임시 빌드 폴더 정리
	commands = append(commands,
		fmt.Sprintf("echo '%s' | sudo -S rm -rf %s", password, workDir))

	// 명령 실행
	results, err := sshUtils.ExecuteCommands(hops, commands, 600000) // 10분 타임아웃
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "명령 실행 중 오류가 발생했습니다: " + err.Error(),
			"logs":    formatResults(results),
		})
		return
	}

	// Git 클론 성공 여부 확인
	gitSuccess := true
	for _, result := range results {
		if strings.Contains(result.Command, "git clone") && strings.Contains(result.Output, "실패") {
			gitSuccess = false
			break
		}
	}

	if !gitSuccess {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Git 저장소 클론에 실패했습니다. 저장소 URL과 인증 정보를 확인하세요.",
			"logs":    formatResults(results),
		})
		return
	}

	// 디렉토리 내용 확인
	var dirContent string
	for _, result := range results {
		if strings.Contains(result.Command, "ls -la") {
			dirContent = result.Output
			break
		}
	}

	// docker-compose.yml 파일 존재 여부 확인
	var yamlType string
	for _, result := range results {
		if strings.Contains(result.Command, "if [ -f") {
			if strings.Contains(result.Output, "YML_EXISTS") {
				yamlType = "yml"
			} else if strings.Contains(result.Output, "YAML_EXISTS") {
				yamlType = "yaml"
			} else if strings.Contains(result.Output, "ERROR") {
				// 두 파일 모두 찾을 수 없음
				c.JSON(http.StatusOK, gin.H{
					"success":     true,
					"message":     "루트 디렉토리에 docker-compose.yml 또는 docker-compose.yaml 파일이 없습니다.",
					"dir_content": dirContent,
					"logs":        formatResults(results),
				})
				return
			}
			break
		}
	}

	// 발견된 파일 기반으로 composeFullPath 업데이트
	if yamlType == "yml" {
		composeFullPath = ymlPath
	} else if yamlType == "yaml" {
		composeFullPath = yamlPath
	}

	// docker-compose up 명령 실행 결과 확인
	var dockerComposeUpSuccess bool
	var dockerComposePsOutput string
	var dockerComposeError string

	// 컨테이너와 이미지 정보 수집
	var dockerImages, dockerContainers string

	for _, result := range results {
		// docker-compose up 명령 결과 확인
		if strings.Contains(result.Command, "docker-compose") && strings.Contains(result.Command, "up -d") {
			if result.ExitCode == 0 {
				dockerComposeUpSuccess = true
			} else {
				dockerComposeUpSuccess = false
				dockerComposeError = result.Output
				if result.Error != "" {
					dockerComposeError += "\n" + result.Error
				}
			}
		}

		// docker-compose ps 명령 결과 확인
		if strings.Contains(result.Command, "docker-compose") && strings.Contains(result.Command, " ps") {
			dockerComposePsOutput = result.Output
		}

		// docker 이미지 및 컨테이너 정보 수집
		if result.Command == "docker images" {
			dockerImages = result.Output
		} else if result.Command == "docker ps -a" {
			dockerContainers = result.Output
		}
	}

	// 이미지 풀링 오류 확인
	var imagePullError bool
	var pullErrorMsg string

	if strings.Contains(dockerComposeError, "unauthorized") &&
		strings.Contains(dockerComposeError, "pull") {
		imagePullError = true
		pullErrorMsg = "Harbor 레지스트리에서 이미지를 가져오는 데 실패했습니다. 인증 정보나 이미지 이름을 확인하세요."
	}

	// 포트 충돌 오류 확인
	var portConflictError bool
	var portConflictMsg string

	if strings.Contains(dockerComposeError, "port is already allocated") {
		portConflictError = true

		// 어떤 포트가 충돌하는지 추출 시도
		re := regexp.MustCompile(`Bind for 0.0.0.0:(\d+) failed`)
		matches := re.FindStringSubmatch(dockerComposeError)

		if len(matches) > 1 {
			portConflictMsg = fmt.Sprintf("포트 %s가 이미 사용 중입니다. 다른 포트를 사용하거나 해당 포트를 사용하는 컨테이너를 중지하세요.", matches[1])
		} else {
			portConflictMsg = "포트 충돌이 발생했습니다. 이미 사용 중인 포트가 있습니다."
		}
	}

	// 배포 성공 여부 확인 - 다음 조건을 확인:
	// 1. docker-compose up 명령이 성공적으로 실행됨
	// 2. docker ps -a 또는 docker-compose ps 출력에 프로젝트 관련 컨테이너가 있음

	// 컨테이너 이름과 상태를 자세히 분석
	var projectContainersExist bool
	var runningContainers []string
	var exitedContainers []string
	var otherStateContainers []string

	// 1. docker ps -a 출력에서 프로젝트 관련 컨테이너 확인
	if dockerContainers != "" && len(dockerContainers) > 0 {
		// 컨테이너 ID, 이미지, 명령, 생성 시간, 상태, 포트, 이름 등이 포함된 라인별 분석
		containerLines := strings.Split(dockerContainers, "\n")

		for _, line := range containerLines {
			if strings.TrimSpace(line) == "" || strings.Contains(line, "CONTAINER ID") {
				continue // 빈 줄이나 헤더 스킵
			}

			// 컨테이너 라인 분석
			fields := strings.Fields(line)
			if len(fields) < 7 {
				continue // 최소 7개 필드가 있어야 함 (ID, 이미지, 명령, 생성, 상태, 포트, 이름)
			}

			// 마지막 필드가 컨테이너 이름
			containerName := fields[len(fields)-1]

			// 프로젝트 관련 컨테이너 이름 패턴 확인
			// 1. 프로젝트명과 정확히 일치하는 경우
			// 2. 프로젝트명_서비스명 패턴인 경우
			// 3. 서비스명이 프로젝트명과 동일한 경우
			isProjectContainer := containerName == request.ComposeProject ||
				strings.HasPrefix(containerName, request.ComposeProject+"_") ||
				strings.Contains(containerName, "_"+request.ComposeProject) ||
				containerName == "db" && strings.Contains(line, request.ComposeProject+"_db")

			if isProjectContainer {
				// 컨테이너 상태 확인 (5번째 필드가 상태 정보를 포함함)
				stateInfo := ""
				stateIdx := -1

				// 상태 필드 찾기 (Command 필드 뒤)
				for i, field := range fields {
					if strings.Contains(field, "\"") && i > 1 {
						stateIdx = i + 2 // Command 다음이 Created, 그 다음이 Status
						break
					}
				}

				if stateIdx >= 0 && stateIdx < len(fields) {
					stateInfo = fields[stateIdx]
				}

				if strings.Contains(line, " Up ") || strings.HasPrefix(stateInfo, "Up") {
					runningContainers = append(runningContainers, containerName)
				} else if strings.Contains(line, " Exited ") || strings.HasPrefix(stateInfo, "Exited") {
					exitedContainers = append(exitedContainers, containerName)
				} else if strings.Contains(line, " Created ") || strings.HasPrefix(stateInfo, "Created") {
					otherStateContainers = append(otherStateContainers, containerName)
				} else {
					otherStateContainers = append(otherStateContainers, containerName)
				}
			}
		}
	}

	// 2. docker-compose ps 출력에서도 컨테이너 확인
	if dockerComposePsOutput != "" && len(dockerComposePsOutput) > 0 {
		composeLines := strings.Split(dockerComposePsOutput, "\n")

		for _, line := range composeLines {
			if strings.TrimSpace(line) == "" || strings.Contains(line, "Name") || strings.Contains(line, "----") {
				continue // 빈 줄이나 헤더 스킵
			}

			// 컨테이너 이름이 첫 번째 열에 있음
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}

			containerName := fields[0]

			// 이미 찾은 컨테이너인지 확인
			found := false
			for _, name := range runningContainers {
				if name == containerName {
					found = true
					break
				}
			}

			if !found {
				for _, name := range exitedContainers {
					if name == containerName {
						found = true
						break
					}
				}
			}

			if !found {
				for _, name := range otherStateContainers {
					if name == containerName {
						found = true
						break
					}
				}
			}

			// 새로운 컨테이너 발견 시 상태 확인
			if !found {
				if strings.Contains(line, " Up ") {
					runningContainers = append(runningContainers, containerName)
				} else if strings.Contains(line, " Exit ") {
					exitedContainers = append(exitedContainers, containerName)
				} else {
					otherStateContainers = append(otherStateContainers, containerName)
				}
			}
		}
	}

	// 적어도 하나의 프로젝트 관련 컨테이너가 있으면 존재함으로 간주
	projectContainersExist = len(runningContainers) > 0 || len(exitedContainers) > 0 || len(otherStateContainers) > 0

	// 배포 성공 여부 결정 (up 명령이 성공했고 최소 하나의 컨테이너가 존재)
	deploySuccess := dockerComposeUpSuccess && projectContainersExist

	// 일부 성공 (컨테이너는 있지만 일부 문제 있음)
	partialSuccess := dockerComposeUpSuccess && projectContainersExist && (len(exitedContainers) > 0 || len(otherStateContainers) > 0)

	if deploySuccess {
		successMessage := fmt.Sprintf("'%s' 프로젝트가 Docker Compose로 성공적으로 배포되었습니다.", request.ComposeProject)
		if partialSuccess {
			successMessage = fmt.Sprintf("'%s' 프로젝트가 일부 성공적으로 배포되었습니다. 일부 컨테이너에 문제가 있을 수 있습니다.", request.ComposeProject)
		}

		c.JSON(http.StatusOK, gin.H{
			"success":                true,
			"message":                successMessage,
			"compose_project":        request.ComposeProject,
			"compose_path":           composeFullPath,
			"working_dir":            workDir,
			"docker_compose_bin":     composeFullPath,
			"container_status":       dockerContainers,
			"docker_images":          dockerImages,
			"docker_containers":      dockerContainers,
			"docker_compose_ps":      dockerComposePsOutput,
			"docker_compose_success": dockerComposeUpSuccess,
			"containers": gin.H{
				"running":     runningContainers,
				"exited":      exitedContainers,
				"other_state": otherStateContainers,
			},
			"logs": formatResults(results),
		})
	} else {
		// 오류 메시지 결정
		var errorMsg string

		if imagePullError {
			errorMsg = pullErrorMsg
		} else if portConflictError {
			errorMsg = portConflictMsg
		} else if !dockerComposeUpSuccess {
			errorMsg = "Docker Compose 실행 중 오류가 발생했습니다: " + dockerComposeError
		} else if !projectContainersExist {
			errorMsg = fmt.Sprintf("Docker Compose 명령은 성공했지만 '%s' 프로젝트의 컨테이너를 찾을 수 없습니다.", request.ComposeProject)
		} else {
			errorMsg = "Docker Compose 배포에 실패했습니다."
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"success":                false,
			"error":                  errorMsg,
			"compose_project":        request.ComposeProject,
			"compose_path":           composeFullPath,
			"working_dir":            workDir,
			"docker_compose_bin":     composeFullPath,
			"docker_images":          dockerImages,
			"docker_containers":      dockerContainers,
			"docker_compose_ps":      dockerComposePsOutput,
			"docker_compose_success": dockerComposeUpSuccess,
			"docker_compose_error":   dockerComposeError,
			"logs":                   formatResults(results),
		})
	}
}

// URL에서 저장소 이름 추출
func extractRepoName(repoURL string) string {
	// URL 끝의 .git 제거
	repoURL = strings.TrimSuffix(repoURL, ".git")

	// 마지막 / 이후의 부분을 저장소 이름으로 사용
	parts := strings.Split(repoURL, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	// 기본값
	return "app"
}

// 명령 실행 결과를 읽기 쉬운 형식으로 변환
func formatResults(results []ssh.CommandResult) string {
	var builder strings.Builder

	for _, result := range results {
		builder.WriteString(fmt.Sprintf("명령: %s\n", result.Command))
		if result.Output != "" {
			builder.WriteString(fmt.Sprintf("출력:\n%s\n", result.Output))
		}
		if result.Error != "" {
			builder.WriteString(fmt.Sprintf("오류:\n%s\n", result.Error))
		}
		builder.WriteString("------------------------------\n")
	}

	return builder.String()
}

// DeleteDockerContainer는 Git 저장소에서 코드를 가져와 Docker Compose down 명령을 실행합니다.
func (h *InfraDockerHandler) DeleteDockerContainer(c *gin.Context) {
	var request struct {
		ID             int             `json:"id"`              // 서버 ID
		Hops           []ssh.HopConfig `json:"hops"`            // SSH 연결 정보
		RepoURL        string          `json:"repo_url"`        // Git 저장소 URL
		Branch         string          `json:"branch"`          // Git 브랜치 (기본값: main)
		UsernameRepo   string          `json:"username_repo"`   // 저장소 접근을 위한 사용자 이름 (선택)
		PasswordRepo   string          `json:"password_repo"`   // 저장소 접근을 위한 비밀번호 (선택)
		ComposeProject string          `json:"compose_project"` // Docker Compose 프로젝트 이름
	}

	// 요청 파싱
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Println("JSON 바인딩 오류:", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "잘못된 요청입니다."})
		return
	}

	// 필수 필드 검증
	if request.RepoURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "저장소 URL은 필수 항목입니다."})
		return
	}

	// 서버 정보 가져오기
	serverInfo, err := db.GetServerInfo(h.DB, request.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 요청 본문에서 hops가 제공되었는지 확인하고, 그렇지 않으면 DB에서 가져옴
	var hops []ssh.HopConfig
	if len(request.Hops) > 0 {
		hops = request.Hops
	} else {
		if err := json.Unmarshal([]byte(serverInfo.Hops), &hops); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "hops 파싱 중 오류가 발생했습니다."})
			return
		}
	}

	if len(hops) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "SSH 연결 정보(hops)가 필요합니다."})
		return
	}

	// 마지막 hop의 패스워드 사용
	password := ""
	if len(hops) > 0 {
		password = hops[len(hops)-1].Password
	}

	// 기본값 설정
	if request.Branch == "" {
		request.Branch = "main"
	}

	// 저장소 이름 추출 (URL의 마지막 부분)
	repoName := extractRepoName(request.RepoURL)

	// 작업 디렉토리 경로 설정
	workDir := fmt.Sprintf("/tmp/%s_down", repoName)

	// SSH 유틸리티 생성
	sshUtils := utils.NewSSHUtils()

	// 현재 실행 중인 컨테이너 상태 확인 (작업 전)
	initialContainerCmd := fmt.Sprintf("echo '%s' | sudo -S docker ps -a", password)
	initialContainerResults, _ := sshUtils.ExecuteCommands(hops, []string{initialContainerCmd}, 30000)
	var initialContainers string
	if len(initialContainerResults) > 0 {
		initialContainers = initialContainerResults[0].Output
	}

	// 명령어 목록 생성
	var commands []string

	// 1. 작업 디렉토리 생성 및 이전 빌드 정리
	commands = append(commands,
		fmt.Sprintf("echo '%s' | sudo -S rm -rf %s", password, workDir),
		fmt.Sprintf("echo '%s' | sudo -S mkdir -p %s", password, workDir),
		fmt.Sprintf("echo '%s' | sudo -S chown $(whoami):$(whoami) %s", password, workDir))

	// 2. 필요한 도구 설치 확인 (Git)
	commands = append(commands,
		fmt.Sprintf("which git || (echo '%s' | sudo -S apt-get update && echo '%s' | sudo -S apt-get install -y git)", password, password))

	// 3. 저장소 클론 (Git 인증 오류 로깅 개선)
	gitCmd := ""
	if request.UsernameRepo != "" && request.PasswordRepo != "" {
		// URL에서 프로토콜 부분 제거
		repoNoProtocol := strings.TrimPrefix(request.RepoURL, "https://")
		repoNoProtocol = strings.TrimPrefix(repoNoProtocol, "http://")

		// @ 문자가 포함된 사용자명 처리 (이메일 주소 등)
		encodedUsername := request.UsernameRepo
		if strings.Contains(encodedUsername, "@") {
			// 사용자명이 이메일인 경우 URL 인코딩 적용
			encodedUsername = strings.ReplaceAll(encodedUsername, "@", "%40")
		}

		gitCmd = fmt.Sprintf("cd %s && git clone -b %s https://%s:%s@%s . 2>&1 || echo 'Git 클론 실패'",
			workDir,
			request.Branch,
			encodedUsername,
			request.PasswordRepo,
			repoNoProtocol)
	} else {
		gitCmd = fmt.Sprintf("cd %s && git clone -b %s %s . 2>&1 || echo 'Git 클론 실패'",
			workDir, request.Branch, request.RepoURL)
	}
	commands = append(commands, gitCmd)

	// 4. 클론 성공 확인 및 디렉토리 내용 확인
	commands = append(commands, fmt.Sprintf("cd %s && ls -la", workDir))

	// 5. docker-compose 파일 존재하는지 확인하고 내용 확인
	composeCheckCmd := fmt.Sprintf("cd %s && ([ -f docker-compose.yml ] && echo 'FOUND_YML' && cat docker-compose.yml) || ([ -f docker-compose.yaml ] && echo 'FOUND_YAML' && cat docker-compose.yaml) || echo 'ERROR: docker-compose.yml 또는 docker-compose.yaml 파일을 찾을 수 없습니다.'", workDir)
	commands = append(commands, composeCheckCmd)

	// 명령 실행
	results, err := sshUtils.ExecuteCommands(hops, commands, 300000) // 5분 타임아웃
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "명령 실행 중 오류가 발생했습니다: " + err.Error(),
			"logs":    formatResults(results),
		})
		return
	}

	// Git 클론 성공 여부 확인
	gitSuccess := true
	for _, result := range results {
		if strings.Contains(result.Command, "git clone") && strings.Contains(result.Output, "실패") {
			gitSuccess = false
			break
		}
	}

	if !gitSuccess {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Git 저장소 클론에 실패했습니다. 저장소 URL과 인증 정보를 확인하세요.",
			"logs":    formatResults(results),
		})
		return
	}

	// 디렉토리 내용 확인
	var dirContent string
	for _, result := range results {
		if strings.Contains(result.Command, "ls -la") {
			dirContent = result.Output
			break
		}
	}

	// docker-compose 파일 확인
	var composeContent string
	var composeFileFound bool

	for _, result := range results {
		if strings.Contains(result.Command, "[ -f docker-compose.yml ]") || strings.Contains(result.Command, "[ -f docker-compose.yaml ]") {
			if strings.Contains(result.Output, "FOUND_YML") {
				composeContent = result.Output
				composeFileFound = true
				break
			} else if strings.Contains(result.Output, "FOUND_YAML") {
				composeContent = result.Output
				composeFileFound = true
				break
			} else if strings.Contains(result.Output, "ERROR") {
				c.JSON(http.StatusOK, gin.H{
					"success":     true,
					"message":     "루트 디렉토리에 docker-compose.yml 또는 docker-compose.yaml 파일이 없습니다.",
					"dir_content": dirContent,
					"logs":        formatResults(results),
				})
				return
			}
		}
	}

	if !composeFileFound {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "docker-compose 파일을 찾을 수 없습니다.",
			"logs":    formatResults(results),
		})
		return
	}

	// 6. Docker Compose down 실행 - 여러 전략 시도
	var downerCommands []string
	var removeStrategy struct {
		ComposeDownSuccess   bool     `json:"compose_down_success"`
		ContainerNameSuccess bool     `json:"container_name_success"`
		PSMatchSuccess       bool     `json:"ps_match_success"`
		RemovedByStrategy    string   `json:"removed_by_strategy"`
		FailedCommands       []string `json:"failed_commands"`
		SuccessCommands      []string `json:"success_commands"`
	}

	// 6.1. 첫 번째 전략: 일반적인 docker-compose down 명령 (많은 경우 작동)
	// 두 파일 확장자 모두 시도
	// YML 파일로 시도
	composeDownCmdYml := fmt.Sprintf("cd %s && [ -f docker-compose.yml ] && echo '%s' | sudo -S docker-compose -p %s -f docker-compose.yml down -v --remove-orphans 2>&1 || echo 'YML_FILE_NOT_FOUND'",
		workDir, password, request.ComposeProject)
	downerCommands = append(downerCommands, composeDownCmdYml)

	// YAML 파일로 시도
	composeDownCmdYaml := fmt.Sprintf("cd %s && [ -f docker-compose.yaml ] && echo '%s' | sudo -S docker-compose -p %s -f docker-compose.yaml down -v --remove-orphans 2>&1 || echo 'YAML_FILE_NOT_FOUND'",
		workDir, password, request.ComposeProject)
	downerCommands = append(downerCommands, composeDownCmdYaml)

	// 6.2. 두 번째 전략: container-name으로 컨테이너 직접 제거
	// docker-compose 파일 내용에서 container_name을 추출
	containerNames := extractContainerNamesFromCompose(composeContent)
	if len(containerNames) > 0 {
		// 추출된 컨테이너 이름으로 직접 제거 명령 추가
		for _, containerName := range containerNames {
			stopCmd := fmt.Sprintf("echo '%s' | sudo -S docker stop %s 2>&1 || echo 'COMMAND_FAILED'",
				password, containerName)
			rmCmd := fmt.Sprintf("echo '%s' | sudo -S docker rm -f %s 2>&1 || echo 'COMMAND_FAILED'",
				password, containerName)
			downerCommands = append(downerCommands, stopCmd, rmCmd)
		}
	}

	// 6.3. 세 번째 전략: ps 명령으로 확인된 컨테이너 제거
	// 컨테이너 출력에서 관련 컨테이너 찾아서 제거
	containersInPS := extractRelatedContainersFromPS(initialContainers, repoName)
	if len(containersInPS) > 0 {
		for _, containerName := range containersInPS {
			// 앞의 명령에서 이미 제거되었을 수 있으므로 오류 무시
			stopCmd := fmt.Sprintf("echo '%s' | sudo -S docker stop %s 2>&1 || echo 'COMMAND_FAILED'",
				password, containerName)
			rmCmd := fmt.Sprintf("echo '%s' | sudo -S docker rm -f %s 2>&1 || echo 'COMMAND_FAILED'",
				password, containerName)
			downerCommands = append(downerCommands, stopCmd, rmCmd)
		}
	}

	// 6.4. 전략 간 중간 확인을 위한 컨테이너 상태 명령
	checkContainerCmd := fmt.Sprintf("echo '%s' | sudo -S docker ps -a", password)
	downerCommands = append(downerCommands, checkContainerCmd)

	// 7. 임시 폴더 정리
	downerCommands = append(downerCommands, fmt.Sprintf("echo '%s' | sudo -S rm -rf %s", password, workDir))

	// 명령 실행
	downResults, err := sshUtils.ExecuteCommands(hops, downerCommands, 180000) // 3분 타임아웃

	// 최종 컨테이너 상태와 각 전략의 결과 분석
	var finalContainerStatus string
	var downOutput string
	var containerStatusAfterStrategies string

	// 첫 번째 전략 (docker-compose down) 결과 분석
	for i, result := range downResults {
		// YML 파일 down 명령 결과 확인
		if i == 0 && strings.Contains(result.Command, "docker-compose -f docker-compose.yml down") {
			downOutput = result.Output
			// YML_FILE_NOT_FOUND가 출력되었는지 확인
			if strings.Contains(result.Output, "YML_FILE_NOT_FOUND") {
				// YML 파일이 없음 - 다음 명령 확인
				continue
			}

			// 성공적인 다운인지 확인
			if !strings.Contains(result.Output, "error") &&
				!strings.Contains(result.Output, "Error") &&
				!strings.Contains(result.Output, "ERROR") {
				removeStrategy.ComposeDownSuccess = true
				removeStrategy.RemovedByStrategy = "docker-compose down (yml)"
				removeStrategy.SuccessCommands = append(removeStrategy.SuccessCommands, "docker-compose down (yml)")
			} else {
				removeStrategy.FailedCommands = append(removeStrategy.FailedCommands, "docker-compose down (yml)")
			}
			continue
		}

		// YAML 파일 down 명령 결과 확인
		if i == 1 && strings.Contains(result.Command, "docker-compose -f docker-compose.yaml down") {
			// YML이 성공했으면 YAML 출력은 무시
			if removeStrategy.ComposeDownSuccess {
				continue
			}

			// YAML_FILE_NOT_FOUND가 출력되었는지 확인
			if strings.Contains(result.Output, "YAML_FILE_NOT_FOUND") {
				// YAML 파일이 없음
				continue
			}

			// downOutput이 비어 있으면 설정 (YML 명령이 실패했을 경우)
			if downOutput == "" {
				downOutput = result.Output
			}

			// 성공적인 다운인지 확인
			if !strings.Contains(result.Output, "error") &&
				!strings.Contains(result.Output, "Error") &&
				!strings.Contains(result.Output, "ERROR") {
				removeStrategy.ComposeDownSuccess = true
				removeStrategy.RemovedByStrategy = "docker-compose down (yaml)"
				removeStrategy.SuccessCommands = append(removeStrategy.SuccessCommands, "docker-compose down (yaml)")
			} else {
				removeStrategy.FailedCommands = append(removeStrategy.FailedCommands, "docker-compose down (yaml)")
			}
			continue
		}

		// 두 번째 전략 (container_name) 결과 분석
		if containerNames != nil && len(containerNames) > 0 {
			for _, name := range containerNames {
				if strings.Contains(result.Command, fmt.Sprintf("docker stop %s", name)) ||
					strings.Contains(result.Command, fmt.Sprintf("docker rm -f %s", name)) {
					if !strings.Contains(result.Output, "COMMAND_FAILED") {
						removeStrategy.ContainerNameSuccess = true
						if removeStrategy.RemovedByStrategy == "" {
							removeStrategy.RemovedByStrategy = "container_name direct removal"
						}
						cmd := ""
						if strings.Contains(result.Command, "docker stop") {
							cmd = fmt.Sprintf("docker stop %s", name)
						} else {
							cmd = fmt.Sprintf("docker rm -f %s", name)
						}
						removeStrategy.SuccessCommands = append(removeStrategy.SuccessCommands, cmd)
					} else {
						cmd := ""
						if strings.Contains(result.Command, "docker stop") {
							cmd = fmt.Sprintf("docker stop %s", name)
						} else {
							cmd = fmt.Sprintf("docker rm -f %s", name)
						}
						removeStrategy.FailedCommands = append(removeStrategy.FailedCommands, cmd)
					}
				}
			}
		}

		// 세 번째 전략 (PS 매치) 결과 분석
		if containersInPS != nil && len(containersInPS) > 0 {
			for _, name := range containersInPS {
				if strings.Contains(result.Command, fmt.Sprintf("docker stop %s", name)) ||
					strings.Contains(result.Command, fmt.Sprintf("docker rm -f %s", name)) {
					if !strings.Contains(result.Output, "COMMAND_FAILED") {
						removeStrategy.PSMatchSuccess = true
						if removeStrategy.RemovedByStrategy == "" {
							removeStrategy.RemovedByStrategy = "ps match container removal"
						}
						cmd := ""
						if strings.Contains(result.Command, "docker stop") {
							cmd = fmt.Sprintf("docker stop %s", name)
						} else {
							cmd = fmt.Sprintf("docker rm -f %s", name)
						}
						removeStrategy.SuccessCommands = append(removeStrategy.SuccessCommands, cmd)
					} else {
						cmd := ""
						if strings.Contains(result.Command, "docker stop") {
							cmd = fmt.Sprintf("docker stop %s", name)
						} else {
							cmd = fmt.Sprintf("docker rm -f %s", name)
						}
						removeStrategy.FailedCommands = append(removeStrategy.FailedCommands, cmd)
					}
				}
			}
		}

		// 중간 컨테이너 상태 확인
		if strings.Contains(result.Command, "docker ps -a") {
			containerStatusAfterStrategies = result.Output
			finalContainerStatus = result.Output // 마지막 상태 업데이트
		}
	}

	// 제거해야 할 모든 컨테이너 목록 (초기 컨테이너 + docker-compose 컨테이너)
	allTargetContainers := append(containersInPS, containerNames...)

	// 중복 제거
	targetContainers := []string{}
	seen := make(map[string]bool)
	for _, container := range allTargetContainers {
		if !seen[container] {
			seen[container] = true
			targetContainers = append(targetContainers, container)
		}
	}

	// 남아있는 컨테이너 확인
	containersRemoved := true
	var remainingContainers []string

	// 현재 실행 중인 컨테이너 이름 추출 (정확한 이름 매칭을 위해)
	currentContainerNames := []string{}
	if finalContainerStatus != "" {
		lines := strings.Split(finalContainerStatus, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "" || strings.Contains(line, "CONTAINER ID") {
				continue
			}

			fields := strings.Fields(line)
			if len(fields) > 0 {
				// 마지막 필드가 컨테이너 이름
				containerName := fields[len(fields)-1]
				currentContainerNames = append(currentContainerNames, containerName)
			}
		}
	}

	// 정확한 이름 매칭으로 남아있는 컨테이너 확인
	for _, targetContainer := range targetContainers {
		stillExists := false

		for _, currentContainer := range currentContainerNames {
			if targetContainer == currentContainer {
				stillExists = true
				remainingContainers = append(remainingContainers, targetContainer)
				break
			}
		}

		if stillExists {
			containersRemoved = false
		}
	}

	// 어떤 전략이 성공했는지 최종 결정
	if removeStrategy.RemovedByStrategy == "" {
		if containersRemoved {
			// 모든 컨테이너가 제거되었으나 특정 전략을 식별할 수 없음
			removeStrategy.RemovedByStrategy = "unknown (containers were removed)"
		} else {
			// 어떤 전략도 성공하지 못함
			removeStrategy.RemovedByStrategy = "none"
		}
	}

	if containersRemoved {
		c.JSON(http.StatusOK, gin.H{
			"success":            true,
			"message":            "컨테이너가 성공적으로 중지 및 제거되었습니다.",
			"container_names":    targetContainers,
			"container_status":   finalContainerStatus,
			"initial_containers": initialContainers,
			"down_output":        downOutput,
			"removal_strategy": gin.H{
				"compose_down_success":    removeStrategy.ComposeDownSuccess,
				"container_name_success":  removeStrategy.ContainerNameSuccess,
				"ps_match_success":        removeStrategy.PSMatchSuccess,
				"removed_by_strategy":     removeStrategy.RemovedByStrategy,
				"success_commands":        removeStrategy.SuccessCommands,
				"failed_commands":         removeStrategy.FailedCommands,
				"container_status_middle": containerStatusAfterStrategies,
			},
			"logs": formatResults(downResults),
		})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":              false,
			"message":              "일부 컨테이너를 제거하지 못했습니다.",
			"container_names":      targetContainers,
			"remaining_containers": remainingContainers,
			"container_status":     finalContainerStatus,
			"current_containers":   currentContainerNames,
			"initial_containers":   initialContainers,
			"down_output":          downOutput,
			"removal_strategy": gin.H{
				"compose_down_success":    removeStrategy.ComposeDownSuccess,
				"container_name_success":  removeStrategy.ContainerNameSuccess,
				"ps_match_success":        removeStrategy.PSMatchSuccess,
				"removed_by_strategy":     removeStrategy.RemovedByStrategy,
				"success_commands":        removeStrategy.SuccessCommands,
				"failed_commands":         removeStrategy.FailedCommands,
				"container_status_middle": containerStatusAfterStrategies,
			},
			"logs": formatResults(downResults),
		})
	}
}

// docker-compose 파일 내용에서 container_name 값 추출
func extractContainerNamesFromCompose(content string) []string {
	var containerNames []string
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		// container_name: 항목 찾기
		if strings.HasPrefix(trimmedLine, "container_name:") {
			parts := strings.SplitN(trimmedLine, ":", 2)
			if len(parts) == 2 {
				containerName := strings.TrimSpace(parts[1])
				// 따옴표 제거
				containerName = strings.Trim(containerName, "'\"")
				if containerName != "" {
					containerNames = append(containerNames, containerName)
				}
			}
		}
	}

	return containerNames
}

// docker ps 출력에서 관련 컨테이너 이름 추출
func extractRelatedContainersFromPS(psOutput string, repoName string) []string {
	var containerNames []string
	lines := strings.Split(psOutput, "\n")

	// 첫 번째 줄은 헤더이므로 건너뛰기
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// docker ps 출력의 마지막 열이 이름
		fields := strings.Fields(line)
		if len(fields) > 0 {
			name := fields[len(fields)-1]
			// 저장소 이름이 포함된 컨테이너만 선택
			if strings.Contains(name, repoName) {
				containerNames = append(containerNames, name)
			}
		}
	}

	return containerNames
}

// GetContainerStatus Docker 컨테이너의 상태 정보를 가져옵니다.
func (h *InfraDockerHandler) GetContainerStatus(c *gin.Context) {
	var request struct {
		Hops           []ssh.HopConfig `json:"hops"`            // SSH 연결 정보
		ComposeProject string          `json:"compose_project"` // 프로젝트 이름 (선택적)
	}

	// 요청 파싱
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Println("JSON 바인딩 오류:", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "잘못된 요청입니다."})
		return
	}

	// 필수 필드 검증
	if len(request.Hops) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "SSH 연결 정보(hops)가 필요합니다."})
		return
	}

	// 마지막 hop의 패스워드 사용
	password := ""
	if len(request.Hops) > 0 {
		password = request.Hops[len(request.Hops)-1].Password
	}

	// SSH 유틸리티 생성
	sshUtils := utils.NewSSHUtils()

	// 컨테이너 상태 정보 가져오기
	var containerCmd string
	if request.ComposeProject != "" {
		// 프로젝트 라벨을 사용하여 필터링
		containerCmd = fmt.Sprintf("echo '%s' | sudo -S docker ps -a --filter \"label=com.docker.compose.project=%s\" --format '{{.ID}}\t{{.Image}}\t{{.Status}}\t{{.Names}}\t{{.Ports}}\t{{.Size}}\t{{.CreatedAt}}'",
			password, request.ComposeProject)
	} else {
		// 모든 컨테이너 조회
		containerCmd = fmt.Sprintf("echo '%s' | sudo -S docker ps -a --format '{{.ID}}\t{{.Image}}\t{{.Status}}\t{{.Names}}\t{{.Ports}}\t{{.Size}}\t{{.CreatedAt}}'",
			password)
	}
	containerResults, err := sshUtils.ExecuteCommands(request.Hops, []string{containerCmd}, 30000)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "컨테이너 상태 정보를 가져오는 중 오류가 발생했습니다: " + err.Error(),
		})
		return
	}

	// 컨테이너 정보 파싱
	var containers []map[string]interface{}
	for _, result := range containerResults {
		if result.Output != "" {
			lines := strings.Split(result.Output, "\n")
			for _, line := range lines {
				fields := strings.Split(line, "\t")
				if len(fields) >= 7 {
					container := map[string]interface{}{
						"id":      fields[0],
						"image":   fields[1],
						"status":  fields[2],
						"name":    fields[3],
						"ports":   fields[4],
						"size":    fields[5],
						"created": fields[6],
					}
					containers = append(containers, container)
				}
			}
		}
	}

	// 이미지 정보 가져오기 (프로젝트 관련 컨테이너의 이미지만 필터링)
	imageCmd := fmt.Sprintf("echo '%s' | sudo -S docker images --format '{{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedSince}}'", password)
	imageResults, err := sshUtils.ExecuteCommands(request.Hops, []string{imageCmd}, 30000)

	var images []map[string]interface{}
	if err == nil {
		// 프로젝트 관련 컨테이너의 이미지 ID 목록 생성
		projectImageIDs := make(map[string]bool)
		for _, container := range containers {
			image := container["image"].(string)
			projectImageIDs[image] = true
		}

		for _, result := range imageResults {
			if result.Output != "" {
				lines := strings.Split(result.Output, "\n")
				for _, line := range lines {
					fields := strings.Split(line, "\t")
					if len(fields) >= 4 {
						imageName := fields[0] + ":" + fields[1]
						// 프로젝트 관련 컨테이너의 이미지만 포함
						if request.ComposeProject == "" || projectImageIDs[imageName] {
							image := map[string]interface{}{
								"repository": fields[0],
								"tag":        fields[1],
								"size":       fields[2],
								"created":    fields[3],
							}
							images = append(images, image)
						}
					}
				}
			}
		}
	}

	// 네트워크 정보 가져오기 (프로젝트 관련 네트워크만 필터링)
	var networkCmd string
	if request.ComposeProject != "" {
		// 프로젝트 이름으로 네트워크 필터링
		networkCmd = fmt.Sprintf("echo '%s' | sudo -S docker network ls --filter \"name=%s\" --format '{{.ID}}\t{{.Name}}\t{{.Driver}}\t{{.Scope}}'",
			password, request.ComposeProject)
	} else {
		// 모든 네트워크 조회
		networkCmd = fmt.Sprintf("echo '%s' | sudo -S docker network ls --format '{{.ID}}\t{{.Name}}\t{{.Driver}}\t{{.Scope}}'",
			password)
	}
	networkResults, err := sshUtils.ExecuteCommands(request.Hops, []string{networkCmd}, 30000)

	var networks []map[string]interface{}
	if err == nil {
		for _, result := range networkResults {
			if result.Output != "" {
				lines := strings.Split(result.Output, "\n")
				for _, line := range lines {
					fields := strings.Fields(line)
					if len(fields) >= 4 {
						network := map[string]interface{}{
							"id":     fields[0],
							"name":   fields[1],
							"driver": fields[2],
							"scope":  fields[3],
						}
						networks = append(networks, network)
					}
				}
			}
		}
	}

	// 볼륨 정보 가져오기 (프로젝트 관련 볼륨만 필터링)
	var volumeCmd string
	if request.ComposeProject != "" {
		// 프로젝트 이름으로 볼륨 필터링
		volumeCmd = fmt.Sprintf("echo '%s' | sudo -S docker volume ls --filter \"name=%s\" --format '{{.Name}}\t{{.Driver}}\t{{.Size}}'",
			password, request.ComposeProject)
	} else {
		// 모든 볼륨 조회
		volumeCmd = fmt.Sprintf("echo '%s' | sudo -S docker volume ls --format '{{.Name}}\t{{.Driver}}\t{{.Size}}'",
			password)
	}
	volumeResults, err := sshUtils.ExecuteCommands(request.Hops, []string{volumeCmd}, 30000)

	var volumes []map[string]interface{}
	if err == nil {
		for _, result := range volumeResults {
			if result.Output != "" {
				lines := strings.Split(result.Output, "\n")
				for _, line := range lines {
					fields := strings.Split(line, "\t")
					if len(fields) >= 3 {
						volume := map[string]interface{}{
							"name":   fields[0],
							"driver": fields[1],
							"size":   fields[2],
						}
						volumes = append(volumes, volume)
					}
				}
			}
		}
	}

	// 컨테이너 개수와 이미지 개수 계산
	containerCount := len(containers)
	imageCount := len(images)

	// 결과 반환
	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"compose_project": request.ComposeProject,
		"containers":      containers,
		"images":          images,
		"networks":        networks,
		"volumes":         volumes,
		"container_count": containerCount,
		"image_count":     imageCount,
	})
}

// GetDockerLogs 특정 도커 컨테이너의 로그를 가져옵니다.
func (h *InfraDockerHandler) GetDockerLogs(c *gin.Context) {
	var request struct {
		Hops        []ssh.HopConfig `json:"hops"`         // SSH 연결 정보
		ContainerID string          `json:"container_id"` // 컨테이너 ID 또는 이름
		Lines       int             `json:"lines"`        // 가져올 로그 라인 수 (기본값: 100)
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "잘못된 요청 형식입니다: " + err.Error(),
		})
		return
	}

	// 기본값 설정
	if request.Lines <= 0 {
		request.Lines = 100
	}

	// SSH 연결 정보 확인
	if len(request.Hops) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "SSH 연결 정보가 필요합니다",
		})
		return
	}

	// 컨테이너 ID 확인
	if request.ContainerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "컨테이너 ID 또는 이름이 필요합니다",
		})
		return
	}

	// SSH 클라이언트 직접 생성하여 로그 추출
	client, err := h.createSSHClient(request.Hops)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("SSH 연결 실패: %v", err),
		})
		return
	}
	defer client.Close()

	// 마지막 Hop의 패스워드 가져오기
	password := request.Hops[0].Password

	// 1. 컨테이너 상태 확인 - 파이프 처리를 위해 명령을 분리
	statusCmd := fmt.Sprintf("echo '%s' | sudo -S docker ps -a --filter \"id=%s\" --format \"{{.ID}}|{{.Names}}|{{.Status}}\"",
		password, request.ContainerID)

	session, err := client.NewSession()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("SSH 세션 생성 실패: %v", err),
		})
		return
	}

	var statusOutput bytes.Buffer
	session.Stdout = &statusOutput

	if err := session.Run(statusCmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("컨테이너 상태 확인 명령 실행 실패: %v", err),
		})
		return
	}
	session.Close()

	var containerExists bool
	var containerID, containerName, containerStatus string

	statusStr := strings.TrimSpace(statusOutput.String())
	if statusStr != "" {
		parts := strings.Split(statusStr, "|")
		if len(parts) >= 3 {
			containerExists = true
			containerID = parts[0]
			containerName = parts[1]
			containerStatus = parts[2]
		}
	}

	if !containerExists {
		c.JSON(http.StatusNotFound, gin.H{
			"success":          false,
			"container_exists": false,
			"error":            "컨테이너를 찾을 수 없습니다",
		})
		return
	}

	// 2. 로그 추출 - 완전한 로그를 가져오기 위해 임시 파일 사용
	// 임시 파일명 생성
	tmpFileName := fmt.Sprintf("/tmp/docker_logs_%s_%d.txt", containerID, time.Now().Unix())

	// 로그를 임시 파일로 저장 - 오류 출력 포함하여 전체 정보 보존
	logCmd := fmt.Sprintf("echo '%s' | sudo -S docker logs --tail %d %s > %s",
		password, request.Lines, containerID, tmpFileName)

	session, err = client.NewSession()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("SSH 세션 생성 실패: %v", err),
		})
		return
	}

	if err := session.Run(logCmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("로그 명령 실행 실패: %v", err),
		})
		return
	}
	session.Close()

	// 3. 임시 파일에서 로그 내용 읽기
	catCmd := fmt.Sprintf("cat %s", tmpFileName)
	session, err = client.NewSession()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("SSH 세션 생성 실패: %v", err),
		})
		return
	}

	var logsOutput bytes.Buffer
	session.Stdout = &logsOutput

	if err := session.Run(catCmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("로그 파일 읽기 실패: %v", err),
		})
		return
	}
	session.Close()

	// 4. 임시 파일 삭제
	cleanupCmd := fmt.Sprintf("rm -f %s", tmpFileName)
	session, err = client.NewSession()
	if err != nil {
		// 파일 삭제 실패해도 계속 진행
		session.Close()
	} else {
		session.Run(cleanupCmd)
		session.Close()
	}

	// 응답 반환
	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"container_exists": true,
		"container_id":     containerID,
		"container_name":   containerName,
		"container_status": containerStatus,
		"lines":            request.Lines,
		"logs":             cleanupLogs(logsOutput.String()),
	})
}

// cleanupLogs는 로그 출력에서 sudo 패스워드 프롬프트를 제거합니다
func cleanupLogs(logs string) string {
	// sudo 패스워드 프롬프트 텍스트만 제거 (줄은 유지)
	// "[sudo] password for" 패턴을 찾아 빈 문자열로 대체
	cleanedLogs := regexp.MustCompile(`\[sudo\] password for.*?:`).ReplaceAllString(logs, "")

	// 불필요한 빈 줄 제거
	// 연속된 빈 줄을 하나로 줄임
	emptyLines := regexp.MustCompile(`\n\s*\n`)
	cleanedLogs = emptyLines.ReplaceAllString(cleanedLogs, "\n")

	// 앞뒤 공백 제거
	return strings.TrimSpace(cleanedLogs)
}

// SSH 클라이언트 생성을 위한 헬퍼 함수
func (h *InfraDockerHandler) createSSHClient(hops []ssh.HopConfig) (*gossh.Client, error) {
	if len(hops) == 0 {
		return nil, fmt.Errorf("SSH 연결 정보가 없습니다")
	}

	hop := hops[0]

	sshConfig := &gossh.ClientConfig{
		User: hop.Username,
		Auth: []gossh.AuthMethod{
			gossh.Password(hop.Password),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         time.Duration(60) * time.Second,
	}

	hostPort := fmt.Sprintf("%s:%d", hop.Host, hop.Port)
	client, err := gossh.Dial("tcp", hostPort, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("SSH 연결 실패: %v", err)
	}

	return client, nil
}

// UninstallDocker는 도커를 완전히 제거합니다.
func (h *InfraDockerHandler) UninstallDocker(c *gin.Context) {
	var request struct {
		Hops []ssh.HopConfig `json:"hops"` // SSH 연결 정보
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		log.Println("JSON 바인딩 오류:", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "잘못된 요청입니다."})
		return
	}

	if len(request.Hops) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "SSH 연결 정보(hops)가 필요합니다."})
		return
	}

	// 마지막 hop의 패스워드 사용
	password := request.Hops[len(request.Hops)-1].Password

	// 도커 제거 명령어 목록
	commands := []string{
		// 환경 설정
		"export DEBIAN_FRONTEND=noninteractive",
		fmt.Sprintf("echo 'DOCKER_OPTS=\"--dns 8.8.8.8 --dns 8.8.4.4\"' | sudo -S DEBIAN_FRONTEND=noninteractive tee /etc/default/docker > /dev/null 2>&1"),

		// 모든 컨테이너 중지 및 삭제
		fmt.Sprintf("echo '%s' | sudo -S docker container stop $(echo '%s' | sudo -S docker container ls -aq) > /dev/null 2>&1 || true", password, password),
		fmt.Sprintf("echo '%s' | sudo -S docker container rm -f $(echo '%s' | sudo -S docker container ls -aq) > /dev/null 2>&1 || true", password, password),

		// 모든 이미지 삭제
		fmt.Sprintf("echo '%s' | sudo -S docker image rm -f $(echo '%s' | sudo -S docker image ls -aq) > /dev/null 2>&1 || true", password, password),

		// 모든 볼륨 삭제
		fmt.Sprintf("echo '%s' | sudo -S docker volume rm $(echo '%s' | sudo -S docker volume ls -q) > /dev/null 2>&1 || true", password, password),

		// 모든 네트워크 삭제 (bridge, host, none 제외)
		fmt.Sprintf("echo '%s' | sudo -S docker network rm $(echo '%s' | sudo -S docker network ls | awk '/bridge|host|none/ {next} {print $1}') > /dev/null 2>&1 || true", password, password),

		// 도커 서비스 중지 및 제거
		fmt.Sprintf("echo '%s' | sudo -S systemctl stop docker > /dev/null 2>&1 || true", password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl disable docker > /dev/null 2>&1 || true", password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl daemon-reload > /dev/null 2>&1 || true", password),

		// 도커 패키지 및 관련 파일 제거
		fmt.Sprintf("echo '%s' | sudo -S DEBIAN_FRONTEND=noninteractive apt-get remove --purge -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin > /dev/null 2>&1 || true", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /var/lib/docker /var/lib/containerd /etc/docker ~/.docker > /dev/null 2>&1 || true", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -f /etc/apt/sources.list.d/docker.list /etc/apt/keyrings/docker.asc > /dev/null 2>&1 || true", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /etc/systemd/system/docker.service /etc/systemd/system/docker.socket > /dev/null 2>&1 || true", password),
		fmt.Sprintf("echo '%s' | sudo -S groupdel docker > /dev/null 2>&1 || true", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -f $(which docker 2>/dev/null) /usr/local/bin/docker /usr/sbin/docker > /dev/null 2>&1 || true", password),

		// snap으로 설치된 도커 제거
		fmt.Sprintf("echo '%s' | sudo -S snap remove docker > /dev/null 2>&1 || true", password),

		// dpkg로 설치된 도커 관련 패키지 제거
		fmt.Sprintf("echo '%s' | sudo -S DEBIAN_FRONTEND=noninteractive dpkg --purge $(dpkg -l | awk '/docker/{print $2}') > /dev/null 2>&1 || true", password),

		// 시스템 정리
		fmt.Sprintf("echo '%s' | sudo -S DEBIAN_FRONTEND=noninteractive apt-get autoremove -y > /dev/null 2>&1 || true", password),
		fmt.Sprintf("echo '%s' | sudo -S DEBIAN_FRONTEND=noninteractive apt-get autoclean -y > /dev/null 2>&1 || true", password),

		// 도커 제거 확인
		fmt.Sprintf("echo '%s' | sudo -S docker --version 2>&1 || echo 'DOCKER_REMOVED'", password),
	}

	// SSH 유틸리티 생성
	sshUtils := utils.NewSSHUtils()

	// 명령 실행
	results, err := sshUtils.ExecuteCommands(request.Hops, commands, 300000) // 5분 타임아웃

	// 도커가 성공적으로 제거되었는지 확인
	dockerRemoved := false
	var finalCheck string

	if len(results) > 0 {
		finalCheck = results[len(results)-1].Output
		// 도커 명령을 찾을 수 없거나 DOCKER_REMOVED가 출력되면 제거 성공으로 판단
		dockerRemoved = strings.Contains(finalCheck, "DOCKER_REMOVED") ||
			strings.Contains(finalCheck, "command not found") ||
			strings.Contains(finalCheck, "not installed")
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "도커 제거 중 오류가 발생했습니다: " + err.Error(),
			"logs":    formatResults(results),
		})
		return
	}

	if dockerRemoved {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "도커가 성공적으로 제거되었습니다.",
			"logs":    formatResults(results),
		})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "도커 제거가 완료되었지만, 일부 구성 요소가 남아있을 수 있습니다.",
			"logs":    formatResults(results),
		})
	}
}

// DeleteOneDockerContainer는 특정 도커 컨테이너 하나만 삭제합니다.
func (h *InfraDockerHandler) DeleteOneDockerContainer(c *gin.Context) {
	var request struct {
		Hops        []ssh.HopConfig `json:"hops"`         // SSH 연결 정보
		ContainerID string          `json:"container_id"` // 삭제할 컨테이너 ID 또는 이름
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		log.Println("JSON 바인딩 오류:", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "잘못된 요청입니다."})
		return
	}

	// 필수 필드 검증
	if len(request.Hops) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "SSH 연결 정보(hops)가 필요합니다."})
		return
	}

	if request.ContainerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "컨테이너 ID 또는 이름이 필요합니다."})
		return
	}

	// 마지막 hop의 패스워드 사용
	password := request.Hops[len(request.Hops)-1].Password

	// SSH 유틸리티 생성
	sshUtils := utils.NewSSHUtils()

	// 먼저 모든 컨테이너 목록 확인 (디버깅 용도)
	listAllCmd := fmt.Sprintf("echo '%s' | sudo -S docker ps -a", password)
	listResults, _ := sshUtils.ExecuteCommands(request.Hops, []string{listAllCmd}, 30000)
	allContainers := ""
	if len(listResults) > 0 {
		allContainers = listResults[0].Output
	}

	// 1. 컨테이너 존재 여부 확인 (ID로 검색)
	checkContainerByIDCmd := fmt.Sprintf("echo '%s' | sudo -S docker ps -a --filter \"id=%s\" --format \"{{.ID}}|{{.Names}}|{{.Status}}\"",
		password, request.ContainerID)

	// 2. 컨테이너 존재 여부 확인 (이름으로 검색)
	checkContainerByNameCmd := fmt.Sprintf("echo '%s' | sudo -S docker ps -a --filter \"name=%s\" --format \"{{.ID}}|{{.Names}}|{{.Status}}\"",
		password, request.ContainerID)

	// 두 명령 실행
	checkCommands := []string{checkContainerByIDCmd, checkContainerByNameCmd}
	checkResults, err := sshUtils.ExecuteCommands(request.Hops, checkCommands, 30000)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":              false,
			"error":                "컨테이너 확인 중 오류가 발생했습니다: " + err.Error(),
			"debug_all_containers": allContainers,
		})
		return
	}

	var containerExists bool
	var containerID, containerName, containerStatus string

	// ID로 검색한 결과 확인
	if len(checkResults) > 0 && checkResults[0].Output != "" {
		statusStr := strings.TrimSpace(checkResults[0].Output)
		parts := strings.Split(statusStr, "|")
		if len(parts) >= 3 {
			containerExists = true
			containerID = parts[0]
			containerName = parts[1]
			containerStatus = parts[2]
		}
	}

	// 이름으로 검색한 결과 확인 (ID로 찾지 못한 경우)
	if !containerExists && len(checkResults) > 1 && checkResults[1].Output != "" {
		statusStr := strings.TrimSpace(checkResults[1].Output)
		lines := strings.Split(statusStr, "\n")
		// 결과가 여러 줄인 경우 첫 번째 줄만 사용
		if len(lines) > 0 {
			parts := strings.Split(lines[0], "|")
			if len(parts) >= 3 {
				containerExists = true
				containerID = parts[0]
				containerName = parts[1]
				containerStatus = parts[2]
			}
		}
	}

	if !containerExists {
		// ID 검색 결과와 이름 검색 결과 변수 선언
		idSearchResult := ""
		nameSearchResult := ""

		// 결과 추출
		if len(checkResults) > 0 {
			idSearchResult = checkResults[0].Output
		}
		if len(checkResults) > 1 {
			nameSearchResult = checkResults[1].Output
		}

		c.JSON(http.StatusNotFound, gin.H{
			"success":              false,
			"container_exists":     false,
			"error":                "컨테이너를 찾을 수 없습니다",
			"requested_id":         request.ContainerID,
			"debug_all_containers": allContainers,
			"id_search_result":     idSearchResult,
			"name_search_result":   nameSearchResult,
		})
		return
	}

	// 2. 컨테이너 중지 및 삭제 명령 실행
	var commands []string

	// 중지 명령 (이미 중지된 경우에도 오류 무시)
	stopCmd := fmt.Sprintf("echo '%s' | sudo -S docker stop %s 2>&1 || echo 'CONTAINER_ALREADY_STOPPED'",
		password, containerID)
	commands = append(commands, stopCmd)

	// 삭제 명령 (항상 강제 삭제)
	rmCmd := fmt.Sprintf("echo '%s' | sudo -S docker rm -f %s 2>&1",
		password, containerID)
	commands = append(commands, rmCmd)

	// 삭제 확인 명령 - 단순한 방식으로 변경: 해당 ID의 컨테이너가 있는지 직접 확인
	// -q 옵션은 컨테이너 ID만 반환, wc -l로 개수 카운트
	checkRemovedCmd := fmt.Sprintf("echo '%s' | sudo -S docker ps -a --filter \"id=%s\" -q | wc -l",
		password, containerID)
	commands = append(commands, checkRemovedCmd)

	// 삭제 후 컨테이너 목록 확인 (디버깅 용도)
	listAfterCmd := fmt.Sprintf("echo '%s' | sudo -S docker ps -a", password)
	commands = append(commands, listAfterCmd)

	// 명령 실행
	results, err := sshUtils.ExecuteCommands(request.Hops, commands, 60000) // 1분 타임아웃

	// 결과 분석
	var stopOutput, rmOutput, checkOutput, listAfterOutput string
	var containerRemoved bool

	if len(results) >= 4 {
		stopOutput = results[0].Output
		rmOutput = results[1].Output
		checkOutput = strings.TrimSpace(results[2].Output)
		listAfterOutput = results[3].Output

		// 컨테이너 개수가 0이면 성공적으로 삭제된 것
		containerRemoved = checkOutput == "0"

		// 추가 디버깅 정보를 로그에 기록
		log.Printf("컨테이너 삭제 확인: ID=%s, Count=%s, Removed=%v",
			containerID, checkOutput, containerRemoved)
	}

	if err != nil && !containerRemoved {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":          false,
			"container_exists": true,
			"container_id":     containerID,
			"container_name":   containerName,
			"container_status": containerStatus,
			"error":            "컨테이너 삭제 중 오류가 발생했습니다: " + err.Error(),
			"stop_output":      stopOutput,
			"rm_output":        rmOutput,
			"check_output":     checkOutput,
			"list_after":       listAfterOutput,
		})
		return
	}

	if containerRemoved {
		c.JSON(http.StatusOK, gin.H{
			"success":          true,
			"message":          "컨테이너가 성공적으로 삭제되었습니다.",
			"container_id":     containerID,
			"container_name":   containerName,
			"container_status": containerStatus,
			"stop_output":      stopOutput,
			"rm_output":        rmOutput,
			"check_output":     checkOutput,
			"list_after":       listAfterOutput,
		})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":          false,
			"container_exists": true,
			"error":            "컨테이너 삭제에 실패했습니다.",
			"container_id":     containerID,
			"container_name":   containerName,
			"container_status": containerStatus,
			"stop_output":      stopOutput,
			"rm_output":        rmOutput,
			"check_output":     checkOutput,
			"list_after":       listAfterOutput,
		})
	}
}

// ControlDockerContainer 컨테이너를 중지하거나 재시작합니다.
func (h *InfraDockerHandler) ControlDockerContainer(c *gin.Context) {
	var request struct {
		Hops        []ssh.HopConfig `json:"hops"`         // SSH 연결 정보
		ContainerID string          `json:"container_id"` // 컨테이너 ID
		ActionType  string          `json:"action_type"`  // 실행할 액션 (stop 또는 restart)
	}

	// 요청 파싱
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Println("JSON 바인딩 오류:", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "잘못된 요청입니다."})
		return
	}

	// 필수 필드 검증
	if request.ContainerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "컨테이너 ID는 필수 항목입니다."})
		return
	}

	if request.ActionType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "액션 타입은 필수 항목입니다."})
		return
	}

	if request.ActionType != "stop" && request.ActionType != "restart" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "액션 타입은 'stop' 또는 'restart'여야 합니다."})
		return
	}

	if len(request.Hops) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "SSH 연결 정보(hops)가 필요합니다."})
		return
	}

	// 마지막 hop의 패스워드 사용
	password := ""
	if len(request.Hops) > 0 {
		password = request.Hops[len(request.Hops)-1].Password
	}

	// SSH 유틸리티 생성
	sshUtils := utils.NewSSHUtils()

	// 컨테이너 존재 여부 확인
	checkCmd := fmt.Sprintf("echo '%s' | sudo -S docker ps -a --filter id=%s --format '{{.ID}}'", password, request.ContainerID)
	checkResults, err := sshUtils.ExecuteCommands(request.Hops, []string{checkCmd}, 30000)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "컨테이너 확인 중 오류가 발생했습니다: " + err.Error(),
		})
		return
	}

	containerExists := false
	for _, result := range checkResults {
		if result.Output != "" {
			containerExists = true
			break
		}
	}

	if !containerExists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   fmt.Sprintf("컨테이너 %s가 존재하지 않습니다.", request.ContainerID),
		})
		return
	}

	// 액션 실행
	var actionCmd string
	if request.ActionType == "stop" {
		actionCmd = fmt.Sprintf("echo '%s' | sudo -S docker stop %s", password, request.ContainerID)
	} else {
		actionCmd = fmt.Sprintf("echo '%s' | sudo -S docker restart %s", password, request.ContainerID)
	}

	results, err := sshUtils.ExecuteCommands(request.Hops, []string{actionCmd}, 300000) // 5분 타임아웃

	// 실행 결과 수집
	var formattedResults []map[string]interface{}
	for _, result := range results {
		formattedResults = append(formattedResults, map[string]interface{}{
			"command":  result.Command,
			"output":   result.Output,
			"error":    result.Error,
			"exitCode": result.ExitCode,
		})
	}

	// 에러 처리
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("컨테이너 %s 중 오류가 발생했습니다: %v", request.ActionType, err),
			"logs":    formattedResults,
		})
		return
	}

	// 결과 확인
	success := true
	var message string
	for _, result := range results {
		if result.ExitCode != 0 {
			success = false
			message = fmt.Sprintf("컨테이너 %s 실패: %s", request.ActionType, result.Error)
			break
		}
	}

	if success {
		message = fmt.Sprintf("컨테이너 %s가 성공적으로 %s되었습니다.", request.ContainerID, request.ActionType)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      success,
		"message":      message,
		"logs":         formattedResults,
		"container_id": request.ContainerID,
		"action_type":  request.ActionType,
	})
}
