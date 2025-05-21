package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/k8scontrol/backend/internal/db"
	"github.com/k8scontrol/backend/internal/utils"
	"github.com/k8scontrol/backend/pkg/ssh"
	gossh "golang.org/x/crypto/ssh"
)

// InfraKubernetesHandler 쿠버네티스 관련 API 핸들러
type InfraKubernetesHandler struct {
	DB *sql.DB
}

// NewInfraKubernetesHandler 새 InfraKubernetesHandler 생성
func NewInfraKubernetesHandler(db *sql.DB) *InfraKubernetesHandler {
	return &InfraKubernetesHandler{DB: db}
}

// DeployKubernetes 쿠버네티스 배포를 처리합니다.
func (h *InfraKubernetesHandler) DeployKubernetes(c *gin.Context) {
	var request struct {
		ID           int             `json:"id"`            // 서버 ID
		Hops         []ssh.HopConfig `json:"hops"`          // SSH 연결 정보
		RepoURL      string          `json:"repo_url"`      // Git 저장소 URL
		Branch       string          `json:"branch"`        // Git 브랜치 (기본값: main)
		UsernameRepo string          `json:"username_repo"` // 저장소 접근을 위한 사용자 이름 (선택)
		PasswordRepo string          `json:"password_repo"` // 저장소 접근을 위한 비밀번호 (선택)
		Namespace    string          `json:"namespace"`     // 쿠버네티스 네임스페이스
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

	if request.Namespace == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "네임스페이스는 필수 항목입니다."})
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
	repoURL := request.RepoURL
	repoURL = strings.TrimPrefix(repoURL, "https://")
	repoURL = strings.TrimPrefix(repoURL, "http://")
	repoURL = strings.TrimSuffix(repoURL, ".git")
	parts := strings.Split(repoURL, "/")
	repoName := repoURL
	if len(parts) > 0 {
		repoName = parts[len(parts)-1]
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

	// 명령 실행
	results, err := sshUtils.ExecuteCommands(hops, commands, 600000) // 10분 타임아웃
	if err != nil {
		var formattedResults []map[string]interface{}
		for _, result := range results {
			formattedResults = append(formattedResults, map[string]interface{}{
				"command":  result.Command,
				"output":   result.Output,
				"error":    result.Error,
				"exitCode": result.ExitCode,
			})
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "명령 실행 중 오류가 발생했습니다: " + err.Error(),
			"logs":    formattedResults,
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
		var formattedResults []map[string]interface{}
		for _, result := range results {
			formattedResults = append(formattedResults, map[string]interface{}{
				"command":  result.Command,
				"output":   result.Output,
				"error":    result.Error,
				"exitCode": result.ExitCode,
			})
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Git 저장소 클론에 실패했습니다. 저장소 URL과 인증 정보를 확인하세요.",
			"logs":    formattedResults,
		})
		return
	}

	// k8s 디렉토리 확인
	k8sDir := fmt.Sprintf("%s/k8s", workDir)
	checkK8sDirCmd := fmt.Sprintf("ls -la %s", k8sDir)
	k8sResults, err := sshUtils.ExecuteCommands(hops, []string{checkK8sDirCmd}, 60000)
	if err != nil {
		var formattedResults []map[string]interface{}
		for _, result := range results {
			formattedResults = append(formattedResults, map[string]interface{}{
				"command":  result.Command,
				"output":   result.Output,
				"error":    result.Error,
				"exitCode": result.ExitCode,
			})
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "k8s 디렉토리 확인 실패: " + err.Error(),
			"logs":    formattedResults,
		})
		return
	}

	// k8s 디렉토리 내용 로깅
	for _, result := range k8sResults {
		if strings.Contains(result.Command, "ls -la") {
			log.Printf("k8s 디렉토리 내용:\n%s", result.Output)
		}
	}

	// k8s 디렉토리의 YAML 파일들 확인 (여러 방법 시도)
	var yamlFiles []string

	// 방법 1: ls 명령어로 찾기
	for _, result := range k8sResults {
		if strings.Contains(result.Command, "ls -la") {
			// YAML 파일 찾기
			lines := strings.Split(result.Output, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasSuffix(line, ".yaml") || strings.HasSuffix(line, ".yml") {
					// 파일명만 추출
					parts := strings.Fields(line)
					if len(parts) > 0 {
						fileName := parts[len(parts)-1]
						yamlFiles = append(yamlFiles, fileName)
					}
				}
			}
		}
	}

	// 방법 2: ls *.yaml 명령어 시도
	if len(yamlFiles) == 0 {
		lsYamlCmd := fmt.Sprintf("cd %s && ls *.yaml *.yml 2>/dev/null || echo ''", k8sDir)
		lsResults, err := sshUtils.ExecuteCommands(hops, []string{lsYamlCmd}, 60000)
		if err == nil {
			for _, result := range lsResults {
				if result.Output != "" && !strings.Contains(result.Output, "No such file or directory") {
					files := strings.Split(result.Output, "\n")
					for _, file := range files {
						file = strings.TrimSpace(file)
						if file != "" && (strings.HasSuffix(file, ".yaml") || strings.HasSuffix(file, ".yml")) {
							yamlFiles = append(yamlFiles, file)
						}
					}
				}
			}
		}
	}

	// 방법 3: find 명령어로 찾기
	if len(yamlFiles) == 0 {
		findYamlCmd := fmt.Sprintf("find %s -maxdepth 1 -name '*.yaml' -o -name '*.yml'", k8sDir)
		findResults, err := sshUtils.ExecuteCommands(hops, []string{findYamlCmd}, 60000)
		if err == nil {
			for _, result := range findResults {
				if strings.Contains(result.Command, "find") && result.Output != "" {
					lines := strings.Split(result.Output, "\n")
					for _, line := range lines {
						line = strings.TrimSpace(line)
						if line != "" {
							// 파일 경로에서 파일명만 추출
							parts := strings.Split(line, "/")
							if len(parts) > 0 {
								fileName := parts[len(parts)-1]
								yamlFiles = append(yamlFiles, fileName)
							}
						}
					}
				}
			}
		}
	}

	// 디버깅 정보 추가
	log.Printf("발견된 YAML 파일 수: %d", len(yamlFiles))
	log.Printf("YAML 파일 목록: %v", yamlFiles)

	// 직접 디렉토리 내용 확인
	listDirCmd := fmt.Sprintf("cd %s && find . -type f | sort", k8sDir)
	listResults, _ := sshUtils.ExecuteCommands(hops, []string{listDirCmd}, 60000)
	for _, result := range listResults {
		if strings.Contains(result.Command, "find") {
			log.Printf("k8s 디렉토리 파일 목록 (find):\n%s", result.Output)
		}
	}

	if len(yamlFiles) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "k8s 디렉토리에 YAML 파일이 없습니다.",
			"k8s_dir": k8sDir,
		})
		return
	}

	// 네임스페이스 확인 및 생성
	namespaceCommands := []string{
		fmt.Sprintf("echo '%s' | sudo -S kubectl get namespace %s || echo '%s' | sudo -S kubectl create namespace %s",
			password, request.Namespace, password, request.Namespace),
	}

	_, err = sshUtils.ExecuteCommands(hops, namespaceCommands, 60000)
	if err != nil {
		log.Printf("네임스페이스 확인/생성 실패: %v", err)
	}

	// YAML 파일 처리 - namespace 설정 제거
	modifiedYamlDir := fmt.Sprintf("%s/k8s_modified", workDir)
	// 수정된 YAML을 저장할 디렉토리 생성
	createModifiedDirCmd := fmt.Sprintf("mkdir -p %s", modifiedYamlDir)
	_, err = sshUtils.ExecuteCommands(hops, []string{createModifiedDirCmd}, 60000)
	if err != nil {
		log.Printf("수정된 YAML 디렉토리 생성 실패: %v", err)
	}

	// 각 YAML 파일에서 namespace 제거
	for _, yamlFile := range yamlFiles {
		// YAML 파일 내용 읽기
		readYamlCmd := fmt.Sprintf("cat %s/%s", k8sDir, yamlFile)
		yamlResults, err := sshUtils.ExecuteCommands(hops, []string{readYamlCmd}, 60000)
		if err != nil {
			log.Printf("YAML 파일 %s 읽기 실패: %v", yamlFile, err)
			continue
		}

		// 이 부분은 로깅과 디버깅을 위한 용도로만 사용
		for _, result := range yamlResults {
			if strings.Contains(result.Command, "cat") {
				log.Printf("원본 YAML 파일 %s의 크기: %d bytes", yamlFile, len(result.Output))
				break
			}
		}

		// YAML 파일에서 namespace 제거
		modifiedYamlPath := fmt.Sprintf("%s/%s", modifiedYamlDir, yamlFile)

		// sed를 사용하여 "namespace: xxx" 행만 제거 (띄어쓰기 다양성 고려)
		removeNamespaceCmd := fmt.Sprintf("cat %s/%s | sed '/^[[:space:]]*namespace:/d' > %s",
			k8sDir, yamlFile, modifiedYamlPath)
		_, err = sshUtils.ExecuteCommands(hops, []string{removeNamespaceCmd}, 60000)
		if err != nil {
			log.Printf("YAML 파일 %s에서 namespace 제거 실패: %v", yamlFile, err)
			continue
		}

		// 수정된 파일 내용 확인 (디버깅용)
		checkModifiedCmd := fmt.Sprintf("cat %s", modifiedYamlPath)
		checkResults, err := sshUtils.ExecuteCommands(hops, []string{checkModifiedCmd}, 60000)
		if err != nil {
			log.Printf("수정된 YAML 파일 %s 확인 실패: %v", yamlFile, err)
		} else {
			var modifiedContent string
			for _, result := range checkResults {
				if strings.Contains(result.Command, "cat") {
					modifiedContent = result.Output
					break
				}
			}
			log.Printf("수정된 YAML 파일 %s 내용:\n%s", yamlFile, modifiedContent)
		}
	}

	// kubectl 명령 실행하여 YAML 파일 적용
	var applyCommands []string

	// kubectl 설치 확인
	applyCommands = append(applyCommands,
		fmt.Sprintf("which kubectl || (echo '%s' | sudo -S apt-get update && echo '%s' | sudo -S apt-get install -y kubectl)", password, password))

	// 파일 적용 순서 조정: 시크릿 파일을 먼저 적용
	var secretFiles []string
	var otherFiles []string

	for _, yamlFile := range yamlFiles {
		// 파일 내용을 읽어서 시크릿인지 확인
		checkFileCmd := fmt.Sprintf("cat %s/%s | grep -i 'kind:[[:space:]]*Secret'", modifiedYamlDir, yamlFile)
		checkResults, err := sshUtils.ExecuteCommands(hops, []string{checkFileCmd}, 60000)

		isSecret := false
		if err == nil {
			for _, result := range checkResults {
				if strings.Contains(result.Command, "grep") && result.Output != "" {
					isSecret = true
					break
				}
			}
		}

		if isSecret || strings.Contains(strings.ToLower(yamlFile), "secret") {
			secretFiles = append(secretFiles, yamlFile)
		} else {
			otherFiles = append(otherFiles, yamlFile)
		}
	}

	// 1단계: 시크릿 파일 먼저 적용
	log.Printf("시크릿 파일 적용 (%d개): %v", len(secretFiles), secretFiles)
	for _, yamlFile := range secretFiles {
		modifiedYamlPath := fmt.Sprintf("%s/%s", modifiedYamlDir, yamlFile)
		applyCmd := fmt.Sprintf("echo '%s' | sudo -S kubectl apply -f %s -n %s",
			password, modifiedYamlPath, request.Namespace)
		applyCommands = append(applyCommands, applyCmd)
	}

	// 시크릿 적용 후 약간의 대기 시간 추가 (시크릿 적용 완료 대기)
	if len(secretFiles) > 0 {
		applyCommands = append(applyCommands, "sleep 5")
	}

	// 2단계: 나머지 파일 적용
	log.Printf("일반 파일 적용 (%d개): %v", len(otherFiles), otherFiles)
	for _, yamlFile := range otherFiles {
		modifiedYamlPath := fmt.Sprintf("%s/%s", modifiedYamlDir, yamlFile)
		applyCmd := fmt.Sprintf("echo '%s' | sudo -S kubectl apply -f %s -n %s",
			password, modifiedYamlPath, request.Namespace)
		applyCommands = append(applyCommands, applyCmd)
	}

	// 명령 실행
	applyResults, err := sshUtils.ExecuteCommands(hops, applyCommands, 300000) // 5분 타임아웃

	// 실행 결과 수집
	var applyOutputs []map[string]interface{}
	for _, result := range applyResults {
		applyOutputs = append(applyOutputs, map[string]interface{}{
			"command":  result.Command,
			"output":   result.Output,
			"error":    result.Error,
			"exitCode": result.ExitCode,
		})
	}

	var formattedResults []map[string]interface{}
	for _, result := range results {
		formattedResults = append(formattedResults, map[string]interface{}{
			"command":  result.Command,
			"output":   result.Output,
			"error":    result.Error,
			"exitCode": result.ExitCode,
		})
	}

	// 작업 완료 후 클론한 폴더 제거
	cleanupCmd := fmt.Sprintf("echo '%s' | sudo -S rm -rf %s", password, workDir)
	_, cleanupErr := sshUtils.ExecuteCommands(hops, []string{cleanupCmd}, 60000)
	if cleanupErr != nil {
		log.Printf("작업 디렉토리 정리 실패: %v", cleanupErr)
	} else {
		log.Printf("작업 디렉토리 정리 완료: %s", workDir)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       fmt.Sprintf("k8s 디렉토리의 YAML 파일들을 네임스페이스 %s에 적용했습니다.", request.Namespace),
		"namespace":     request.Namespace,
		"yaml_files":    yamlFiles,
		"apply_results": applyOutputs,
		"logs":          formattedResults,
	})
}

// DeleteNamespace 쿠버네티스 네임스페이스 삭제를 처리합니다.
func (h *InfraKubernetesHandler) DeleteNamespace(c *gin.Context) {
	var request struct {
		Hops      []ssh.HopConfig `json:"hops"`      // SSH 연결 정보
		Namespace string          `json:"namespace"` // 삭제할 쿠버네티스 네임스페이스
	}

	// 요청 파싱
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Println("JSON 바인딩 오류:", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "잘못된 요청입니다."})
		return
	}

	// 필수 필드 검증
	if request.Namespace == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "네임스페이스는 필수 항목입니다."})
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

	// 네임스페이스 삭제 명령
	deleteCmd := fmt.Sprintf("echo '%s' | sudo -S kubectl delete namespace %s", password, request.Namespace)
	results, err := sshUtils.ExecuteCommands(request.Hops, []string{deleteCmd}, 300000) // 5분 타임아웃

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
			"error":   "네임스페이스 삭제 중 오류가 발생했습니다: " + err.Error(),
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
			message = "네임스페이스 삭제 실패: " + result.Error
			break
		}
	}

	if success {
		// 네임스페이스가 완전히 삭제될 때까지 대기
		log.Printf("네임스페이스 %s 삭제 명령 실행 완료, 완전한 삭제 대기 중...", request.Namespace)

		// 최대 5분(300초) 동안 폴링하며 대기
		maxWaitTime := 300
		interval := 5 // 5초마다 확인

		for i := 0; i < maxWaitTime; i += interval {
			// 네임스페이스 존재 여부 확인
			checkCmd := fmt.Sprintf("echo '%s' | sudo -S kubectl get namespace %s -o name 2>/dev/null || echo 'not found'",
				password, request.Namespace)
			checkResults, err := sshUtils.ExecuteCommands(request.Hops, []string{checkCmd}, 30000)

			namespaceExists := true
			if err == nil {
				for _, result := range checkResults {
					if strings.Contains(result.Output, "not found") || result.Output == "" {
						namespaceExists = false
						break
					}
				}
			}

			if !namespaceExists {
				log.Printf("네임스페이스 %s가 완전히 삭제되었습니다. (소요 시간: %d초)", request.Namespace, i)
				break
			}

			// 5초 대기 후 다시 확인
			if i+interval < maxWaitTime {
				sleepCmd := fmt.Sprintf("sleep %d", interval)
				sshUtils.ExecuteCommands(request.Hops, []string{sleepCmd}, 30000)
			}
		}

		// 최종 확인
		finalCheckCmd := fmt.Sprintf("echo '%s' | sudo -S kubectl get namespace %s -o name 2>/dev/null || echo 'not found'",
			password, request.Namespace)
		finalResults, _ := sshUtils.ExecuteCommands(request.Hops, []string{finalCheckCmd}, 30000)

		namespaceStillExists := false
		for _, result := range finalResults {
			if !strings.Contains(result.Output, "not found") && result.Output != "" {
				namespaceStillExists = true
				break
			}
		}

		if namespaceStillExists {
			message = fmt.Sprintf("네임스페이스 %s 삭제가 시작되었지만, 완전히 삭제되지 않았습니다. 리소스 정리가 계속 진행 중입니다.", request.Namespace)
		} else {
			message = fmt.Sprintf("네임스페이스 %s가 성공적으로 삭제되었습니다.", request.Namespace)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   success,
		"message":   message,
		"logs":      formattedResults,
		"namespace": request.Namespace,
	})
}

// GetNamespaceAndPodStatus 네임스페이스와 파드 상태를 확인합니다.
func (h *InfraKubernetesHandler) GetNamespaceAndPodStatus(c *gin.Context) {
	var request struct {
		Hops      []ssh.HopConfig `json:"hops"`      // SSH 연결 정보
		Namespace string          `json:"namespace"` // 확인할 쿠버네티스 네임스페이스
	}

	// 요청 파싱
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Println("JSON 바인딩 오류:", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "잘못된 요청입니다."})
		return
	}

	// 필수 필드 검증
	if request.Namespace == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "네임스페이스는 필수 항목입니다."})
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

	// 네임스페이스 존재 여부 확인
	namespaceCmd := fmt.Sprintf("echo '%s' | sudo -S kubectl get namespace %s -o name 2>/dev/null || echo 'not found'",
		password, request.Namespace)
	namespaceResults, err := sshUtils.ExecuteCommands(request.Hops, []string{namespaceCmd}, 30000)

	namespaceExists := false
	if err == nil {
		for _, result := range namespaceResults {
			if !strings.Contains(result.Output, "not found") && result.Output != "" {
				namespaceExists = true
				break
			}
		}
	}

	// 파드 상태 확인
	var pods []map[string]interface{}
	if namespaceExists {
		// 파드 정보 가져오기 (이름, 상태, 재시작 횟수)
		podCmd := fmt.Sprintf("echo '%s' | sudo -S kubectl get pods -n %s -o custom-columns=NAME:.metadata.name,STATUS:.status.phase,RESTARTS:.status.containerStatuses[0].restartCount 2>/dev/null",
			password, request.Namespace)
		podResults, err := sshUtils.ExecuteCommands(request.Hops, []string{podCmd}, 30000)

		if err == nil {
			for _, result := range podResults {
				if result.Output != "" {
					// 헤더 제거
					lines := strings.Split(result.Output, "\n")
					if len(lines) > 1 {
						// 헤더 이후의 각 라인 처리
						for i := 1; i < len(lines); i++ {
							line := strings.TrimSpace(lines[i])
							if line == "" {
								continue
							}

							// 공백으로 분리
							fields := strings.Fields(line)
							if len(fields) >= 3 {
								pod := map[string]interface{}{
									"name":     fields[0],
									"status":   fields[1],
									"restarts": fields[2],
								}
								pods = append(pods, pod)
							}
						}
					}
				}
			}
		}
	}

	// 결과 반환
	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"namespace":        request.Namespace,
		"namespace_exists": namespaceExists,
		"pods":             pods,
	})
}

// GetPodLogs 특정 파드의 로그를 가져옵니다.
func (h *InfraKubernetesHandler) GetPodLogs(c *gin.Context) {
	var request struct {
		Hops      []ssh.HopConfig `json:"hops"`      // SSH 연결 정보
		Namespace string          `json:"namespace"` // 쿠버네티스 네임스페이스
		PodName   string          `json:"pod_name"`  // 파드 이름
		Lines     int             `json:"lines"`     // 가져올 로그 라인 수 (기본값: 100)
	}

	// 요청 파싱
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Println("JSON 바인딩 오류:", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "잘못된 요청입니다."})
		return
	}

	// 필수 필드 검증
	if request.Namespace == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "네임스페이스는 필수 항목입니다."})
		return
	}

	if request.PodName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "파드 이름은 필수 항목입니다."})
		return
	}

	if len(request.Hops) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "SSH 연결 정보(hops)가 필요합니다."})
		return
	}

	// 기본값 설정
	if request.Lines <= 0 {
		request.Lines = 100 // 기본값: 100줄
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

	// 1. 파드 존재 여부 확인
	podCmd := fmt.Sprintf("echo '%s' | sudo -S kubectl get pod %s -n %s -o name || echo 'not found'",
		password, request.PodName, request.Namespace)

	session, err := client.NewSession()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("SSH 세션 생성 실패: %v", err),
		})
		return
	}

	var podOutput bytes.Buffer
	session.Stdout = &podOutput

	if err := session.Run(podCmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("파드 확인 명령 실행 실패: %v", err),
		})
		return
	}
	session.Close()

	podExists := !strings.Contains(podOutput.String(), "not found") && strings.TrimSpace(podOutput.String()) != ""

	if !podExists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   fmt.Sprintf("파드 '%s'가 네임스페이스 '%s'에 존재하지 않습니다.", request.PodName, request.Namespace),
		})
		return
	}

	// 2. 로그 추출 - 완전한 로그를 가져오기 위해 임시 파일 사용
	// 임시 파일명 생성
	tmpFileName := fmt.Sprintf("/tmp/pod_logs_%s_%s_%d.txt", request.Namespace, request.PodName, time.Now().Unix())

	// 로그를 임시 파일로 저장 - 오류 출력 포함하여 전체 정보 보존
	logCmd := fmt.Sprintf("echo '%s' | sudo -S kubectl logs --tail=%d %s -n %s > %s",
		password, request.Lines, request.PodName, request.Namespace, tmpFileName)

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
		"success":    true,
		"namespace":  request.Namespace,
		"pod_name":   request.PodName,
		"lines":      request.Lines,
		"logs":       cleanupKubernetesLogs(logsOutput.String()),
		"pod_exists": podExists,
	})
}

// cleanupKubernetesLogs는 로그 출력에서 sudo 패스워드 프롬프트를 제거합니다
func cleanupKubernetesLogs(logs string) string {
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
func (h *InfraKubernetesHandler) createSSHClient(hops []ssh.HopConfig) (*gossh.Client, error) {
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

// 특정 파드를 재시작합니다.
func (h *InfraKubernetesHandler) RestartPod(c *gin.Context) {
	var request struct {
		Hops      []ssh.HopConfig `json:"hops"`      // SSH 연결 정보
		Namespace string          `json:"namespace"` // 쿠버네티스 네임스페이스
		PodName   string          `json:"pod_name"`  // 삭제할 파드 이름
	}

	// 요청 파싱
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Println("JSON 바인딩 오류:", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "잘못된 요청입니다."})
		return
	}

	// 필수 필드 검증
	if request.Namespace == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "네임스페이스는 필수 항목입니다."})
		return
	}

	if request.PodName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "파드 이름은 필수 항목입니다."})
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

	// 파드 삭제 명령 실행
	deleteCmd := fmt.Sprintf("echo '%s' | sudo -S kubectl delete pod %s -n %s",
		password, request.PodName, request.Namespace)

	results, err := sshUtils.ExecuteCommands(request.Hops, []string{deleteCmd}, 300000) // 5분 타임아웃

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
			"error":   "파드 재시작 중 오류가 발생했습니다: " + err.Error(),
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
			message = "파드 재시작 실패: " + result.Error
			break
		}
	}

	if success {
		message = fmt.Sprintf("파드 %s가 성공적으로 재시작되었습니다.", request.PodName)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   success,
		"message":   message,
		"logs":      formattedResults,
		"namespace": request.Namespace,
		"pod_name":  request.PodName,
	})
}
