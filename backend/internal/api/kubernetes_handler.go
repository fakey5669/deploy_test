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
	"github.com/k8scontrol/backend/internal/command"
	"github.com/k8scontrol/backend/internal/db"
	"github.com/k8scontrol/backend/internal/utils"
	"github.com/k8scontrol/backend/pkg/ssh"
	gossh "golang.org/x/crypto/ssh"
)

// KubernetesHandler는 쿠버네티스 관련 액션을 처리합니다
type KubernetesHandler struct {
	cmdManager *command.CommandManager
	db         *sql.DB
}

// CommandRequest는 명령어 API 요청 구조를 정의합니다
type CommandRequest struct {
	Action     string                 `json:"action"`
	Parameters map[string]interface{} `json:"parameters"`
}

// 액션 타입 상수
const (
	// 인프라 관련 액션
	ActionGetInfras             = "getInfras"
	ActionGetInfraById          = "getInfraById"
	ActionCreateInfra           = "createInfra"
	ActionUpdateInfra           = "updateInfra"
	ActionDeleteInfra           = "deleteInfra"
	ActionInstallLoadBalancer   = "installLoadBalancer"
	ActionInstallFirstMaster    = "installFirstMaster"
	ActionJoinMaster            = "joinMaster"
	ActionJoinWorker            = "joinWorker"
	ActionImportKubernetesInfra = "importKubernetesInfra"

	// 서버 관련 액션
	ActionGetServers    = "getServers"
	ActionGetServerById = "getServerById"
	ActionCreateServer  = "createServer"
	ActionUpdateServer  = "updateServer"
	ActionDeleteServer  = "deleteServer"
	ActionRestartServer = "restartServer"
	ActionStartServer   = "startServer"
	ActionStopServer    = "stopServer"

	// 노드 관련 액션
	ActionGetNodeStatus            = "getNodeStatus"
	ActionRemoveNode               = "removeNode"
	ActionDeleteWorker             = "deleteWorker"
	ActionDeleteMaster             = "deleteMaster"
	ActionGetNamespaceAndPodStatus = "getNamespaceAndPodStatus"
	ActionCalculateResources       = "calculateResources"
	ActionCalculateNodes           = "calculateNodes"
	ActionDeployKubernetes         = "deployKubernetes"
	ActionDeleteNamespace          = "deleteNamespace"
	ActionGetPodLogs               = "getPodLogs"
	ActionRestartPod               = "restartPod"
)

// NewKubernetesHandler는 새로운 KubernetesHandler 인스턴스를 생성합니다
func NewKubernetesHandler(db *sql.DB) *KubernetesHandler {
	manager := command.NewCommandManager()
	command.RegisterKubernetesCommands(manager)

	return &KubernetesHandler{
		cmdManager: manager,
		db:         db,
	}
}

// HandleRequest는 쿠버네티스 관련 모든 요청을 처리합니다
func (h *KubernetesHandler) HandleRequest(c *gin.Context) {
	var request CommandRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "잘못된 요청 형식: " + err.Error(),
		})
		return
	}

	// 액션 요청 로그 기록
	log.Printf("[API 요청] 액션: %s, 파라미터: %+v", request.Action, request.Parameters)

	// 필수 파라미터 검증
	if request.Action == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "action 필드가 필요합니다",
		})
		return
	}

	// 액션 처리
	switch request.Action {
	// 인프라 관련 액션
	case ActionGetInfras:
		h.handleGetInfras(c)
	case ActionGetInfraById:
		h.handleGetInfraById(c, request)
	case ActionCreateInfra:
		h.handleCreateInfra(c, request)
	case ActionUpdateInfra:
		h.handleUpdateInfra(c, request)
	case ActionDeleteInfra:
		h.handleDeleteInfra(c, request)
	case ActionImportKubernetesInfra:
		h.handleImportKubernetesInfra(c, request)

	// 서버 관련 액션
	case ActionGetServers:
		h.handleGetServers(c, request)
	case ActionGetServerById:
		h.handleGetServerById(c, request)
	case ActionCreateServer:
		h.handleCreateServer(c, request)
	case ActionUpdateServer:
		h.handleUpdateServer(c, request)
	case ActionDeleteServer:
		h.handleDeleteServer(c, request)

	// 쿠버네티스 관련 액션
	case ActionInstallLoadBalancer:
		h.handleInstallLoadBalancer(c, request)
	case ActionInstallFirstMaster:
		h.handleInstallFirstMaster(c, request)
	case ActionJoinMaster:
		h.handleJoinMaster(c, request)
	case ActionJoinWorker:
		h.handleJoinWorker(c, request)
	case ActionDeleteMaster:
		h.handleDeleteMaster(c, request)
	case ActionDeleteWorker:
		h.handleDeleteWorker(c, request)
	case ActionGetNodeStatus:
		h.handleGetNodeStatus(c, request)
	case ActionGetNamespaceAndPodStatus:
		h.handleGetNamespaceAndPodStatus(c, request)

	case ActionCalculateResources:
		h.handleCalculateResources(c, request)
	case ActionCalculateNodes:
		h.handleCalculateNodes(c, request)

	case ActionDeployKubernetes:
		h.handleDeployKubernetes(c, request)
	case ActionDeleteNamespace:
		h.handleDeleteNamespace(c, request)
	case ActionGetPodLogs:
		h.handleGetPodLogs(c, request)
	case ActionRestartPod:
		h.handleRestartPod(c, request)
	default:
		h.handleOtherAction(c, request)
	}
}

// handleInstallLoadBalancer 함수는 로드밸런서 설치를 처리합니다
func (h *KubernetesHandler) handleInstallLoadBalancer(c *gin.Context, request CommandRequest) {
	// 1. 서버 ID 유효성 검사 및 서버 정보 가져오기
	serverID, err := getIntParameter(request.Parameters["server_id"])
	if err != nil {
		log.Printf("[로드밸런서 설치 오류] 유효하지 않은 server_id: %v", request.Parameters["server_id"])
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "유효한 server_id가 필요합니다"})
		return
	}

	// 서버 정보 가져오기
	serverInfo, err := db.GetServerInfo(h.db, serverID)
	if err != nil {
		log.Printf("[로드밸런서 설치 오류] 서버 정보 조회 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "서버 정보를 가져올 수 없습니다"})
		return
	}

	// SSH 연결 정보(hops) 가져오기 - 요청 파라미터의 hops 또는 DB에서 가져오기
	var hops []ssh.HopConfig
	if hopsData, ok := request.Parameters["hops"].([]interface{}); ok && len(hopsData) > 0 {
		// hopsData를 []ssh.HopConfig로 변환
		hops = make([]ssh.HopConfig, len(hopsData))
		for i, hop := range hopsData {
			if hopMap, ok := hop.(map[string]interface{}); ok {
				host, _ := hopMap["host"].(string)
				username, _ := hopMap["username"].(string)
				password, _ := hopMap["password"].(string)
				port := 22 // 기본값
				if portVal, ok := hopMap["port"].(float64); ok {
					port = int(portVal)
				} else if portStr, ok := hopMap["port"].(string); ok {
					if portInt, err := strconv.Atoi(portStr); err == nil {
						port = portInt
					}
				}
				hops[i] = ssh.HopConfig{
					Host:     host,
					Port:     port,
					Username: username,
					Password: password,
				}
			}
		}
	} else {
		// 요청에 hops가 없는 경우 DB에서 가져오기
		if err := json.Unmarshal([]byte(serverInfo.Hops), &hops); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "hops 파싱 중 오류가 발생했습니다: " + err.Error(),
			})
			return
		}
	}

	if len(hops) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "SSH 연결 정보(hops)가 필요합니다."})
		return
	}

	ha := serverInfo.HA
	if ha == "Y" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "이 서버에는 이미 HAProxy가 설치되어 있습니다."})
		return
	}

	// 마지막 hop의 패스워드 사용
	password := ""
	if len(hops) > 0 {
		password = hops[len(hops)-1].Password
		log.Printf("[로드밸런서 설치] 마지막 hop의 패스워드를 사용합니다")
	}

	// 1. kubernetes_commands.go에서 정의된 명령어 준비
	commandParams := map[string]interface{}{
		"server_id": serverID,
		"password":  password,
	}

	// PrepareAction을 사용하여 명령어 배열 가져오기
	commands, err := h.cmdManager.PrepareAction(command.ActionInstallLoadBalancer, commandParams)
	if err != nil {
		log.Printf("[로드밸런서 설치 오류] 명령어 준비 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "명령어 준비 중 오류가 발생했습니다: " + err.Error(),
		})
		return
	}

	// 2. 명령어 실행을 위한 대상 설정

	// 3. 설치 명령어 실행 (처음 11개 명령어는 설치 명령어)
	// 설치 시간이 길 수 있으므로 더 긴 타임아웃 사용 (5분)
	sshUtils := utils.NewSSHUtils()
	installResults, err := sshUtils.ExecuteCommands(hops, commands[:11], 300000) // 5분 타임아웃

	if err != nil {
		log.Printf("[로드밸런서 설치 오류] 설치 명령어 실행 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "HAProxy 설치 중 오류가 발생했습니다: " + err.Error(),
		})
		return
	}

	log.Printf("[로드밸런서 설치] 설치 명령어 실행 완료: %s", utils.TruncateString(installResults[len(installResults)-1].Output, 100))

	// 4. 로그 확인 명령어 실행 (마지막 4개 명령어는 로그 확인 명령어)
	logResults, err := sshUtils.ExecuteCommands(hops, commands[11:], 30000)
	if err != nil {
		log.Printf("[로드밸런서 설치 오류] 로그 확인 명령어 실행 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "설치 로그 확인 중 오류가 발생했습니다: " + err.Error(),
		})
		return
	}

	log.Printf("==========: %s, %s", logResults, err)

	// 로그 확인 결과 초기화
	logExists := false
	installComplete := false
	hasErrors := false
	logContent := "로그 내용을 가져올 수 없습니다."

	if err == nil && len(logResults) >= 4 {
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
		err := db.UpdateServerHAStatus(h.db, serverID)
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

func (h *KubernetesHandler) handleGetNodeStatus(c *gin.Context, request CommandRequest) {
	// 1. 필요한 파라미터 확인
	serverID, err := getIntParameter(request.Parameters["server_id"])
	if err != nil {
		log.Printf("[노드 상태 확인 오류] 유효하지 않은 server_id: %v", request.Parameters["server_id"])
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "server_id 파라미터가 필요합니다",
		})
		return
	}

	// 2. 서버 정보 가져오기
	serverInfo, err := db.GetServerByID(h.db, serverID)
	if err != nil {
		log.Printf("[노드 상태 확인 오류] 서버 정보 조회 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "서버 정보를 가져오는데 실패했습니다: " + err.Error(),
		})
		return
	}

	// 3. hops 정보 파싱
	var hops []ssh.HopConfig
	if hopsData, ok := request.Parameters["hops"].([]interface{}); ok && len(hopsData) > 0 {
		// hopsData를 []ssh.HopConfig로 변환
		hops = make([]ssh.HopConfig, len(hopsData))
		for i, hop := range hopsData {
			if hopMap, ok := hop.(map[string]interface{}); ok {
				host, _ := hopMap["host"].(string)
				portStr, _ := hopMap["port"].(string)
				port := 22 // 기본값
				if portVal, err := strconv.Atoi(portStr); err == nil {
					port = portVal
				} else if portFloat, ok := hopMap["port"].(float64); ok {
					port = int(portFloat)
				}
				username, _ := hopMap["username"].(string)
				password, _ := hopMap["password"].(string)
				hops[i] = ssh.HopConfig{
					Host:     host,
					Port:     port,
					Username: username,
					Password: password,
				}
			}
		}
	} else {
		// 요청에 hops가 없는 경우 DB에서 가져오기
		if err := json.Unmarshal([]byte(serverInfo.Hops), &hops); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "hops 파싱 중 오류가 발생했습니다: " + err.Error(),
			})
			return
		}
		log.Printf("[노드 상태 확인] DB에서 %d개의 hop 정보 파싱 완료", len(hops))
	}

	// 4. 노드 타입 가져오기
	nodeType, ok := request.Parameters["type"].(string)
	if !ok {
		// 파라미터에 타입이 없으면 서버 정보에서 가져옴
		nodeType = serverInfo.Type
	}

	// 5. 명령어 실행 대상 설정
	sshUtils := utils.NewSSHUtils()

	// 6. 명령어 준비 및 실행 - 파라미터 맵 설정
	// 마지막 hop에서 비밀번호 추출 (sudo 명령어 실행 시 필요)
	password := ""
	if len(hops) > 0 {
		password = hops[len(hops)-1].Password
	}

	cmdParams := map[string]interface{}{
		"type":        nodeType,
		"server_name": serverInfo.ServerName,
		"password":    password,
	}

	// 명령어 준비
	commands, err := h.cmdManager.PrepareAction(command.ActionGetNodeStatus, cmdParams)
	if err != nil {
		log.Printf("[노드 상태 확인 오류] 명령어 준비 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "명령어 준비 중 오류가 발생했습니다: " + err.Error(),
		})
		return
	}

	// 최대 10번 재시도
	var output string
	success := false

	log.Printf("[노드 상태 확인] 명령어 실행 시작 (최대 10회 시도)")
	for attempt := 1; attempt <= 10; attempt++ {
		log.Printf("[노드 상태 확인] 시도 %d/10", attempt)
		startTime := time.Now()

		results, err := sshUtils.ExecuteCommands(hops, commands, 20000)
		executionTime := time.Since(startTime)

		if err != nil {
			// 오류 타입에 따라 더 상세한 로그 추가
			if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "timed out") {
				log.Printf("[노드 상태 확인 오류] 시도 %d 타임아웃 발생 (소요시간: %v): %v",
					attempt, executionTime, err)
			} else if strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "connect") {
				log.Printf("[노드 상태 확인 오류] 시도 %d 연결 실패 (소요시간: %v): %v",
					attempt, executionTime, err)
			} else if strings.Contains(err.Error(), "authentication") || strings.Contains(err.Error(), "auth") {
				log.Printf("[노드 상태 확인 오류] 시도 %d 인증 실패 (소요시간: %v): %v",
					attempt, executionTime, err)
			} else {
				log.Printf("[노드 상태 확인 오류] 시도 %d 실패 (소요시간: %v): %v",
					attempt, executionTime, err)
			}

			if attempt < 3 {
				log.Printf("[노드 상태 확인] 1초 후 재시도...")
				time.Sleep(1 * time.Second)
				continue
			}
			break
		}

		if len(results) > 0 {
			output = results[0].Output
			if strings.Contains(output, "===START===") && strings.Contains(output, "===END===") {
				success = true
				log.Printf("[노드 상태 확인] 시도 %d 성공 (소요시간: %v)", attempt, executionTime)
				break
			} else {
				log.Printf("[노드 상태 확인 오류] 시도 %d - 명령 실행됨 but 올바른 출력 없음 (소요시간: %v), 출력: %s",
					attempt, executionTime, utils.TruncateString(output, 100))
			}
		} else {
			log.Printf("[노드 상태 확인 오류] 시도 %d - 명령 실행됨 but 결과 없음 (소요시간: %v)",
				attempt, executionTime)
		}

		if attempt < 3 {
			log.Printf("[노드 상태 확인] 1초 후 재시도...")
			time.Sleep(1 * time.Second)
		}
	}

	// 7. 기본값 설정 - 연결 문제 시 기본적으로 false 반환
	installed := false
	running := false
	isMaster := false
	isWorker := false

	// 현재 시간
	currentTime := time.Now()
	lastCheckedStr := currentTime.Format("2006-01-02 15:04:05")

	// 명령어 실행 성공 시 결과 파싱
	if success {
		log.Printf("[노드 상태 확인] 결과 분석 중")
		log.Printf("[노드 상태 확인] 실행 결과 상세: %s", utils.TruncateString(output, 300))

		if strings.Contains(output, "INSTALLED=true") {
			installed = true
		}

		// 서버 타입에 따라 다른 방식으로 running 상태 판단
		nodeType = strings.ToLower(nodeType)
		if nodeType == "ha" {
			if strings.Contains(output, "RUNNING=true") {
				running = true
			}
			log.Printf("[노드 상태 확인] HA 노드 상태: installed=%v, running=%v", installed, running)
		} else if nodeType == "master" || nodeType == "worker" {
			kubeletRunning := strings.Contains(output, "KUBELET_RUNNING=true")
			nodeRegistered := strings.Contains(output, "NODE_REGISTERED=true")

			if strings.Contains(output, "IS_MASTER=true") {
				isMaster = true
			}

			if strings.Contains(output, "IS_WORKER=true") {
				isWorker = true
			}

			// 마스터 노드는 kubelet이 실행 중이고 마스터로 등록되어 있어야 함
			if nodeType == "master" {
				running = kubeletRunning && isMaster && nodeRegistered
				log.Printf("[노드 상태 확인] 마스터 노드 상태: installed=%v, running=%v, kubelet=%v, isMaster=%v, registered=%v",
					installed, running, kubeletRunning, isMaster, nodeRegistered)
			}

			// 워커 노드는 kubelet이 실행 중이고 워커로 등록되어 있어야 함
			if nodeType == "worker" {
				running = kubeletRunning
				log.Printf("[노드 상태 확인] 워커 노드 상태: installed=%v, running=%v, kubelet=%v",
					installed, running, kubeletRunning)
			}
		}
	} else {
		log.Printf("[노드 상태 확인 경고] 모든 시도 실패 - 기본값 사용")
	}

	// 8. 마지막 확인 시간 업데이트
	err = db.UpdateServerLastChecked(h.db, serverID, currentTime)
	if err != nil {
		// 로그만 기록하고 계속 진행
		log.Printf("[노드 상태 확인 경고] 마지막 확인 시간 업데이트 실패: %v", err)
	} else {
		log.Printf("[노드 상태 확인] 마지막 확인 시간 업데이트 성공: %s", currentTime.Format(time.RFC3339))
	}

	// HA 노드가 설치되고 실행 중이면 Ha 컬럼을 'Y'로 업데이트
	if nodeType == "ha" && installed && running {
		// Ha 필드 업데이트 함수 호출
		if err := db.UpdateServerHaStatus(h.db, serverID, "Y"); err != nil {
			log.Printf("[노드 상태 확인 경고] HA 상태 업데이트 실패: %v", err)
		} else {
			log.Printf("[노드 상태 확인] HA 노드 ID: %d의 Ha 필드가 'Y'로 업데이트되었습니다.", serverID)
		}
	}

	// 9. 응답 생성
	response := gin.H{
		"success": true,
		"status": gin.H{
			"installed": installed,
			"running":   running,
		},
		"lastChecked": lastCheckedStr,
	}

	// 마스터/워커 노드인 경우 추가 정보 제공
	if nodeType == "master" || nodeType == "worker" {
		response["status"].(gin.H)["isMaster"] = isMaster
		response["status"].(gin.H)["isWorker"] = isWorker
	}

	log.Printf("[노드 상태 확인 완료] 서버 ID: %d, 타입: %s, 설치됨: %v, 실행 중: %v",
		serverID, nodeType, installed, running)
	c.JSON(http.StatusOK, response)
}

// handleInstallFirstMaster는 쿠버네티스 첫 번째 마스터 노드 설치를 처리합니다
func (h *KubernetesHandler) handleInstallFirstMaster(c *gin.Context, request CommandRequest) {
	// 1. 필요한 파라미터 확인
	serverID, err := getIntParameter(request.Parameters["server_id"])
	if err != nil {
		log.Printf("[마스터 노드 설치 오류] 유효하지 않은 server_id: %v", request.Parameters["server_id"])
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "server_id 파라미터가 필요합니다",
		})
		return
	}

	// 2. 서버 정보 가져오기
	serverInfo, err := db.GetServerByID(h.db, serverID)
	if err != nil {
		log.Printf("[마스터 노드 설치 오류] 서버 정보 조회 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "서버 정보를 가져오는데 실패했습니다: " + err.Error(),
		})
		return
	}

	// 서버 이름 변수 설정 (DB에서 가져온 값 사용)
	serverName := serverInfo.ServerName
	log.Printf("DB에서 가져온 서버 이름: %s를 사용합니다.", serverName)

	// SSH 연결 정보(hops) 가져오기 - 요청 파라미터의 hops 또는 DB에서 가져오기
	var hops []ssh.HopConfig
	if hopsData, ok := request.Parameters["hops"].([]interface{}); ok && len(hopsData) > 0 {
		// hopsData를 []ssh.HopConfig로 변환
		hops = make([]ssh.HopConfig, len(hopsData))
		for i, hop := range hopsData {
			if hopMap, ok := hop.(map[string]interface{}); ok {
				host, _ := hopMap["host"].(string)
				username, _ := hopMap["username"].(string)
				password, _ := hopMap["password"].(string)
				port := 22 // 기본값
				if portVal, ok := hopMap["port"].(float64); ok {
					port = int(portVal)
				} else if portStr, ok := hopMap["port"].(string); ok {
					if portInt, err := strconv.Atoi(portStr); err == nil {
						port = portInt
					}
				}
				hops[i] = ssh.HopConfig{
					Host:     host,
					Port:     port,
					Username: username,
					Password: password,
				}
			}
		}
	} else {
		// 요청에 hops가 없는 경우 DB에서 가져오기
		if err := json.Unmarshal([]byte(serverInfo.Hops), &hops); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "hops 파싱 중 오류가 발생했습니다: " + err.Error(),
			})
			return
		}
	}

	// // 4. 명령어 실행 대상 설정
	// target := &command.CommandTarget{
	// 	Hops: hops,
	// }

	// SSH 연결 정보(hops) 가져오기 - 요청 파라미터의 hops 또는 DB에서 가져오기
	var lb_hops []ssh.HopConfig
	if lbHopsData, ok := request.Parameters["lb_hops"].([]interface{}); ok && len(lbHopsData) > 0 {
		// hopsData를 []ssh.HopConfig로 변환
		lb_hops = make([]ssh.HopConfig, len(lbHopsData))
		for i, hop := range lbHopsData {
			if hopMap, ok := hop.(map[string]interface{}); ok {
				host, _ := hopMap["host"].(string)
				username, _ := hopMap["username"].(string)
				password, _ := hopMap["password"].(string)
				port := 22 // 기본값
				if portVal, ok := hopMap["port"].(float64); ok {
					port = int(portVal)
				} else if portStr, ok := hopMap["port"].(string); ok {
					if portInt, err := strconv.Atoi(portStr); err == nil {
						port = portInt
					}
				}
				lb_hops[i] = ssh.HopConfig{
					Host:     host,
					Port:     port,
					Username: username,
					Password: password,
				}
			}
		}
	}

	// 5. 필요한 매개변수 설정
	password := ""
	if pass, ok := request.Parameters["password"].(string); ok {
		password = pass
	}

	// 마스터 노드 IP 주소 가져오기
	sshUtils := utils.NewSSHUtils()
	ipCmd := []string{"ip -4 addr show | awk '/inet / && $2 ~ /^192/ {print $2}' | cut -d/ -f1 | head -n 1"}
	ipResults, err := sshUtils.ExecuteCommands(hops, ipCmd, 30000)
	if err != nil || len(ipResults) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "마스터 노드 IP 주소를 가져오는 중 오류가 발생했습니다."})
		return
	}

	masterIP := strings.TrimSpace(ipResults[0].Output)
	log.Printf("마스터 노드 IP 주소: %s", masterIP)

	lbIP := ""
	if lb, ok := request.Parameters["lb_ip"].(string); ok {
		lbIP = lb
	}

	// 로드 밸런서 IP 주소 가져오기
	lbIpCmd := []string{"ip -4 addr show | awk '/inet / && $2 ~ /^192/ {print $2}' | cut -d/ -f1 | head -n 1"}
	lbIpResults, err := sshUtils.ExecuteCommands(lb_hops, lbIpCmd, 30000)
	if err != nil || len(lbIpResults) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "로드 밸런서 IP 주소를 가져오는 중 오류가 발생했습니다."})
		return
	}

	// HAProxy 업데이트 실행
	if len(lb_hops) > 0 {
		lbTarget := &command.CommandTarget{
			Hops: lb_hops,
		}

		// HAProxy 업데이트 명령어 준비
		haproxyUpdateParams := map[string]interface{}{
			"server_name": serverName,
			"master_ip":   masterIP,
			"port":        "6443",
			"lb_password": request.Parameters["lb_password"],
		}

		// HAProxy 업데이트 명령어 가져오기
		commandSets := make(map[string][]string)
		haproxyUpdateCmd, err := h.cmdManager.PrepareAction("updateHAProxy", haproxyUpdateParams)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "HAProxy 업데이트 명령어 준비 중 오류가 발생했습니다."})
			return
		}
		commandSets["haproxyUpdate"] = haproxyUpdateCmd

		// HAProxy 설정 업데이트 실행
		_, err = h.cmdManager.ExecuteCustomCommands(lbTarget, commandSets["haproxyUpdate"])
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "로드 밸런서 HAProxy 설정 업데이트 중 오류가 발생했습니다."})
			return
		}

		log.Println("로드 밸런서 HAProxy 설정이 성공적으로 업데이트되었습니다.")
	}

	// 6. 명령어 준비
	commands, err := h.cmdManager.PrepareAction(command.ActionInstallFirstMaster, map[string]interface{}{
		"password":    password,
		"lb_ip":       lbIP,
		"server_name": serverName,
	})
	if err != nil {
		log.Printf("[마스터 노드 설치 오류] 명령어 준비 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "명령어 준비 중 오류가 발생했습니다: " + err.Error(),
		})
		return
	}

	// 7. 설치 명령어 실행 (첫 5개 명령어)
	// 설치는 백그라운드에서 실행되므로 빠르게 완료됨
	results, err := sshUtils.ExecuteCommands(hops, commands[:5], 30000)
	if err != nil {
		log.Printf("[마스터 노드 설치 오류] 설치 명령어 실행 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "쿠버네티스 마스터 노드 설치 중 오류가 발생했습니다: " + err.Error(),
		})
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
			_, err := sshUtils.ExecuteCommands(hops, checkInstallCompleteCmd, 1800000) // 30분 타임아웃
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

			checkResults, err := sshUtils.ExecuteCommands(hops, checkCommands, 30000)
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
				err := db.UpdateServerJoinCommand(h.db, serverID, joinCommand, certificateKey)
				if err != nil {
					log.Printf("join 명령어 DB 업데이트 중 오류 발생: %v", err)
					return
				}

				log.Printf("서버 ID %d의 join 명령어가 성공적으로 업데이트되었습니다.", serverID)
			} else {
				log.Printf("유효한 join 명령어를 찾지 못했습니다. 로그 파일 확인 필요")
				// 마지막 시도 - 전체 로그에서 join 명령어가 있는지 확인
				lastAttemptCmd := []string{
					`grep -n "kubeadm join" /tmp/k8s_install.log`,
				}
				lastResults, _ := sshUtils.ExecuteCommands(hops, lastAttemptCmd, 30000)
				if len(lastResults) > 0 {
					log.Printf("로그 파일의 kubeadm join 라인들: %s", lastResults[0].Output)
				}
			}
		}()
	} else {
		// 실패한 명령어가 있는지 확인하지만 bash 구문 오류는 무시
		hasRealFailed := false
		for _, result := range results {
			// 구문 오류와 같은 일부 오류는 실제로 설치에 영향을 미치지 않을 수 있음
			if result.ExitCode != 0 &&
				!strings.Contains(result.Error, "syntax error near unexpected token") &&
				!strings.Contains(result.Error, "line") {
				hasRealFailed = true
				break
			}
		}

		// 구문 오류만 있는 경우에는 설치가 성공적으로 시작된 것으로 간주
		if !hasRealFailed {
			output := "설치 스크립트가 실행되었으나 일부 비중요 경고가 있었습니다."
			if len(results) > 0 && results[len(results)-1].Output != "" {
				output = results[len(results)-1].Output
			}

			// 성공으로 응답
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "쿠버네티스 마스터 노드 설치가 백그라운드에서 시작되었습니다.",
				"details": output,
				"logFile": "/tmp/k8s_install.log",
				"note":    "설치 진행 상황을 확인하려면 로그 파일을 확인하세요.",
			})

			// 백그라운드에서 join 명령어를 확인하는 코드 실행
			go func() {
				// 설치 완료 메시지를 확인하는 명령어
				checkInstallCompleteCmd := []string{
					"while ! grep -q '설치 완료' /tmp/k8s_install.log 2>/dev/null; do sleep 10; done; echo '설치가 완료되었습니다.'",
				}

				// 설치 완료될 때까지 대기
				_, err := sshUtils.ExecuteCommands(hops, checkInstallCompleteCmd, 1800000) // 30분 타임아웃
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

				checkResults, err := sshUtils.ExecuteCommands(hops, checkCommands, 30000)
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

				// 유효한 join 명령어가 있는지 확인
				if joinCommand != "" && strings.Contains(joinCommand, "kubeadm join") {
					log.Printf("유효한 join 명령어를 찾았습니다. DB 업데이트를 시도합니다.")

					// DB에 join 명령어와 인증서 키 저장
					log.Printf("저장할 join_command: %s, certificate_key: %s", joinCommand, certificateKey)
					err := db.UpdateServerJoinCommand(h.db, serverID, joinCommand, certificateKey)
					if err != nil {
						log.Printf("join 명령어 DB 업데이트 중 오류 발생: %v", err)
						return
					}

					log.Printf("서버 ID %d의 join 명령어가 성공적으로 업데이트되었습니다.", serverID)
				} else {
					log.Printf("유효한 join 명령어를 찾지 못했습니다. 로그 파일 확인 필요")
					// 마지막 시도 - 전체 로그에서 join 명령어가 있는지 확인
					lastAttemptCmd := []string{
						`grep -n "kubeadm join" /tmp/k8s_install.log`,
					}
					lastResults, _ := sshUtils.ExecuteCommands(hops, lastAttemptCmd, 30000)
					if len(lastResults) > 0 {
						log.Printf("로그 파일의 kubeadm join 라인들: %s", lastResults[0].Output)
					}
				}
			}()

			return
		}

		// 실제 설치에 영향을 미치는 오류인 경우 원래 오류 메시지 표시
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

// handleJoinMaster 함수는 마스터 노드 조인을 처리합니다
func (h *KubernetesHandler) handleJoinMaster(c *gin.Context, request CommandRequest) {

	serverID, err := getIntParameter(request.Parameters["server_id"])
	if err != nil {
		log.Printf("[마스터 노드 설치 오류] 유효하지 않은 server_id: %v", request.Parameters["server_id"])
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "server_id 파라미터가 필요합니다",
		})
		return
	}

	// 쿠버네티스 마스터 노드 서버 정보 가져오기
	serverInfo, err := db.GetServerInfo(h.db, serverID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 서버 이름 변수 설정 (DB에서 가져온 값 사용)
	serverName := serverInfo.ServerName
	log.Printf("DB에서 가져온 서버 이름: %s를 사용합니다.", serverName)

	var hops []ssh.HopConfig
	if hopsData, ok := request.Parameters["hops"].([]interface{}); ok && len(hopsData) > 0 {
		// hopsData를 []ssh.HopConfig로 변환
		hops = make([]ssh.HopConfig, len(hopsData))
		for i, hop := range hopsData {
			if hopMap, ok := hop.(map[string]interface{}); ok {
				host, _ := hopMap["host"].(string)
				username, _ := hopMap["username"].(string)
				password, _ := hopMap["password"].(string)
				port := 22 // 기본값
				if portVal, ok := hopMap["port"].(float64); ok {
					port = int(portVal)
				} else if portStr, ok := hopMap["port"].(string); ok {
					if portInt, err := strconv.Atoi(portStr); err == nil {
						port = portInt
					}
				}
				hops[i] = ssh.HopConfig{
					Host:     host,
					Port:     port,
					Username: username,
					Password: password,
				}
			}
		}
	} else {
		// 요청에 hops가 없는 경우 DB에서 가져오기
		if err := json.Unmarshal([]byte(serverInfo.Hops), &hops); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "hops 파싱 중 오류가 발생했습니다: " + err.Error(),
			})
			return
		}
	}

	var lb_hops []ssh.HopConfig
	if lbHopsData, ok := request.Parameters["lb_hops"].([]interface{}); ok && len(lbHopsData) > 0 {
		// hopsData를 []ssh.HopConfig로 변환
		lb_hops = make([]ssh.HopConfig, len(lbHopsData))
		for i, hop := range lbHopsData {
			if hopMap, ok := hop.(map[string]interface{}); ok {
				host, _ := hopMap["host"].(string)
				username, _ := hopMap["username"].(string)
				password, _ := hopMap["password"].(string)
				port := 22 // 기본값
				if portVal, ok := hopMap["port"].(float64); ok {
					port = int(portVal)
				} else if portStr, ok := hopMap["port"].(string); ok {
					if portInt, err := strconv.Atoi(portStr); err == nil {
						port = portInt
					}
				}
				lb_hops[i] = ssh.HopConfig{
					Host:     host,
					Port:     port,
					Username: username,
					Password: password,
				}
			}
		}
	}

	// 마스터 노드 IP 주소 가져오기
	target := &command.CommandTarget{
		Hops: hops,
	}

	ipCmd := []string{"ip -4 addr show | awk '/inet / && $2 ~ /^192/ {print $2}' | cut -d/ -f1 | head -n 1"}
	ipResults, err := h.cmdManager.ExecuteCustomCommands(target, ipCmd)
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
	if request.Parameters["lb_password"] == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "로드 밸런서 패스워드가 제공되지 않았습니다."})
		return
	}

	// 로드 밸런서 IP 주소 가져오기
	lbTarget := &command.CommandTarget{
		Hops: lb_hops,
	}

	lbIpCmd := []string{"ip -4 addr show | awk '/inet / && $2 ~ /^192/ {print $2}' | cut -d/ -f1 | head -n 1"}
	lbIpResults, err := h.cmdManager.ExecuteCustomCommands(lbTarget, lbIpCmd)
	if err != nil || len(lbIpResults) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "로드 밸런서 IP 주소를 가져오는 중 오류가 발생했습니다."})
		return
	}
	lbIP = strings.TrimSpace(lbIpResults[0].Output)
	log.Printf("로드 밸런서 IP 주소: %s", lbIP)

	// 메인 마스터 노드의 join_command와 certificate_key 가져오기
	mainID, err := getIntParameter(request.Parameters["main_id"])
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "유효하지 않은 main_id 파라미터입니다."})
		return
	}

	mainMasterInfo, err := db.GetServerInfo(h.db, mainID)
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

	// 명령어 패키지에서 명령어 준비
	commandParams := map[string]interface{}{
		"server_name":     serverName,
		"master_ip":       masterIP,
		"join_command":    joinCommand,
		"certificate_key": certificateKey,
		"password":        request.Parameters["password"],
		"lb_password":     request.Parameters["lb_password"],
		"port":            port,
		"lb_ip":           lbIP,
	}

	commandSets, err := command.PrepareJoinMasterCommands(commandParams)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "명령어 준비 중 오류가 발생했습니다: " + err.Error()})
		return
	}

	// 1. HAProxy 설정 업데이트
	lbResults, err := h.cmdManager.ExecuteCustomCommands(lbTarget, commandSets["haproxyUpdate"])
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

	// 2. 마스터 노드 조인 스크립트 실행
	results, err := h.cmdManager.ExecuteCustomCommands(target, commandSets["joinScript"])
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
			checkJoinCompleteCmd := commandSets["checkJoin"]

			// 조인 완료될 때까지 대기
			_, err := h.cmdManager.ExecuteCustomCommands(target, checkJoinCompleteCmd)
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

// handleJoinWorker 함수는 워커 노드 조인을 처리합니다
func (h *KubernetesHandler) handleJoinWorker(c *gin.Context, request CommandRequest) {

	serverID, err := getIntParameter(request.Parameters["server_id"])
	if err != nil {
		log.Printf("[워커 노드 설치 오류] 유효하지 않은 server_id: %v", request.Parameters["server_id"])
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "server_id 파라미터가 필요합니다",
		})
		return
	}

	// 쿠버네티스 워커 노드 서버 정보 가져오기
	serverInfo, err := db.GetServerInfo(h.db, serverID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 서버 이름 변수 설정 (DB에서 가져온 값 사용)
	serverName := serverInfo.ServerName
	log.Printf("DB에서 가져온 서버 이름: %s를 사용합니다.", serverName)

	// 메인 마스터 노드의 join_command와 certificate_key 가져오기
	mainID, err := getIntParameter(request.Parameters["main_id"])
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "유효하지 않은 main_id 파라미터입니다."})
		return
	}

	mainMasterInfo, err := db.GetServerInfo(h.db, mainID)
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

	var hops []ssh.HopConfig
	if hopsData, ok := request.Parameters["hops"].([]interface{}); ok && len(hopsData) > 0 {
		// hopsData를 []ssh.HopConfig로 변환
		hops = make([]ssh.HopConfig, len(hopsData))
		for i, hop := range hopsData {
			if hopMap, ok := hop.(map[string]interface{}); ok {
				host, _ := hopMap["host"].(string)
				username, _ := hopMap["username"].(string)
				password, _ := hopMap["password"].(string)
				port := 22 // 기본값
				if portVal, ok := hopMap["port"].(float64); ok {
					port = int(portVal)
				} else if portStr, ok := hopMap["port"].(string); ok {
					if portInt, err := strconv.Atoi(portStr); err == nil {
						port = portInt
					}
				}
				hops[i] = ssh.HopConfig{
					Host:     host,
					Port:     port,
					Username: username,
					Password: password,
				}
			}
		}
	} else {
		// 요청에 hops가 없는 경우 DB에서 가져오기
		if err := json.Unmarshal([]byte(serverInfo.Hops), &hops); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "hops 파싱 중 오류가 발생했습니다: " + err.Error(),
			})
			return
		}
	}

	// 명령 실행을 위한 target 생성
	target := command.CommandTarget{
		Hops: hops,
	}

	// CommandManager를 통해 명령어 준비
	joinWorkerParams := map[string]interface{}{
		"server_name":  serverName,
		"join_command": joinCommand,
		"password":     request.Parameters["password"],
	}

	// 명령어 준비
	finalCommands, err := h.cmdManager.PrepareAction("joinWorker", joinWorkerParams)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "워커 노드 조인 명령어 준비 중 오류가 발생했습니다.", "errorDetails": err.Error()})
		return
	}

	// CommandManager를 통한 명령 실행
	results, err := h.cmdManager.ExecuteCustomCommands(&target, finalCommands)
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

			// 백그라운드 명령 실행을 위한 새로운 target 생성
			bgTarget := command.CommandTarget{
				Hops: hops,
			}

			// 조인 완료될 때까지 대기
			_, err := h.cmdManager.ExecuteCustomCommands(&bgTarget, checkJoinCompleteCmd)
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

// handleDeleteWorker 함수는 워커 노드 삭제
func (h *KubernetesHandler) handleDeleteWorker(c *gin.Context, request CommandRequest) {
	serverID, err := getIntParameter(request.Parameters["server_id"])
	if err != nil {
		log.Printf("[워커 노드 삭제 오류] 유효하지 않은 server_id: %v", request.Parameters["server_id"])
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "server_id 파라미터가 필요합니다",
		})
		return
	}

	// 쿠버네티스 워커 노드 서버 정보 가져오기
	serverInfo, err := db.GetServerInfo(h.db, serverID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 서버 이름 변수 설정 (DB에서 가져온 값 사용)
	serverName := serverInfo.ServerName
	log.Printf("DB에서 가져온 서버 이름: %s를 사용합니다.", serverName)

	var hops []ssh.HopConfig
	if hopsData, ok := request.Parameters["hops"].([]interface{}); ok && len(hopsData) > 0 {
		// hopsData를 []ssh.HopConfig로 변환
		hops = make([]ssh.HopConfig, len(hopsData))
		for i, hop := range hopsData {
			if hopMap, ok := hop.(map[string]interface{}); ok {
				host, _ := hopMap["host"].(string)
				username, _ := hopMap["username"].(string)
				password, _ := hopMap["password"].(string)
				port := 22 // 기본값
				if portVal, ok := hopMap["port"].(float64); ok {
					port = int(portVal)
				} else if portStr, ok := hopMap["port"].(string); ok {
					if portInt, err := strconv.Atoi(portStr); err == nil {
						port = portInt
					}
				}
				hops[i] = ssh.HopConfig{
					Host:     host,
					Port:     port,
					Username: username,
					Password: password,
				}
			}
		}
	} else {
		// 요청에 hops가 없는 경우 DB에서 가져오기
		if err := json.Unmarshal([]byte(serverInfo.Hops), &hops); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "hops 파싱 중 오류가 발생했습니다: " + err.Error(),
			})
			return
		}
	}

	var masterHops []ssh.HopConfig
	if masterHopsData, ok := request.Parameters["main_hops"].([]interface{}); ok && len(masterHopsData) > 0 {
		// hopsData를 []ssh.HopConfig로 변환
		masterHops = make([]ssh.HopConfig, len(masterHopsData))
		for i, hop := range masterHopsData {
			if hopMap, ok := hop.(map[string]interface{}); ok {
				host, _ := hopMap["host"].(string)
				username, _ := hopMap["username"].(string)
				password, _ := hopMap["password"].(string)
				port := 22 // 기본값
				if portVal, ok := hopMap["port"].(float64); ok {
					port = int(portVal)
				} else if portStr, ok := hopMap["port"].(string); ok {
					if portInt, err := strconv.Atoi(portStr); err == nil {
						port = portInt
					}
				}
				masterHops[i] = ssh.HopConfig{
					Host:     host,
					Port:     port,
					Username: username,
					Password: password,
				}
			}
		}
	}

	// 명령어 실행을 위한 파라미터 준비
	params := map[string]interface{}{
		"server_name":   serverName,
		"main_password": request.Parameters["main_password"],
		"password":      request.Parameters["password"],
	}

	// CommandManager를 통해 명령어 준비
	commands, err := h.cmdManager.PrepareAction(ActionDeleteWorker, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "명령어 준비 중 오류가 발생했습니다: " + err.Error(),
		})
		return
	}

	// 마스터 노드에서 실행할 명령어 (처음 3개)
	masterCommands := commands[:3]
	// 워커 노드에서 실행할 명령어 (나머지)
	workerCommands := commands[3:]

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
	workerResults, err := sshUtils.ExecuteCommands(hops, workerCommands, 300000) // 5분 타임아웃

	// 워커 노드 접속 실패는 무시 (이미 종료되었을 수 있음)
	if err != nil {
		log.Printf("워커 노드 접속 실패 (이미 종료되었을 수 있음): %v", err)
	}

	// 3. DB에서 워커 노드 삭제
	err = db.DeleteWorker(h.db, serverID)
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

// handleDeleteMaster 함수는 마스터 노드 삭제
func (h *KubernetesHandler) handleDeleteMaster(c *gin.Context, request CommandRequest) {
	serverID, err := getIntParameter(request.Parameters["server_id"])
	if err != nil {
		log.Printf("[마스터 노드 삭제 오류] 유효하지 않은 server_id: %v", request.Parameters["server_id"])
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "server_id 파라미터가 필요합니다",
		})
		return
	}

	// 쿠버네티스 마스터 노드 서버 정보 가져오기
	serverInfo, err := db.GetServerInfo(h.db, serverID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 서버 이름 변수 설정 (DB에서 가져온 값 사용)
	serverName := serverInfo.ServerName
	log.Printf("DB에서 가져온 서버 이름: %s를 사용합니다.", serverName)

	var hops []ssh.HopConfig
	if hopsData, ok := request.Parameters["hops"].([]interface{}); ok && len(hopsData) > 0 {
		// hopsData를 []ssh.HopConfig로 변환
		hops = make([]ssh.HopConfig, len(hopsData))
		for i, hop := range hopsData {
			if hopMap, ok := hop.(map[string]interface{}); ok {
				host, _ := hopMap["host"].(string)
				username, _ := hopMap["username"].(string)
				password, _ := hopMap["password"].(string)
				port := 22 // 기본값
				if portVal, ok := hopMap["port"].(float64); ok {
					port = int(portVal)
				} else if portStr, ok := hopMap["port"].(string); ok {
					if portInt, err := strconv.Atoi(portStr); err == nil {
						port = portInt
					}
				}
				hops[i] = ssh.HopConfig{
					Host:     host,
					Port:     port,
					Username: username,
					Password: password,
				}
			}
		}
	} else {
		// 요청에 hops가 없는 경우 DB에서 가져오기
		if err := json.Unmarshal([]byte(serverInfo.Hops), &hops); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "hops 파싱 중 오류가 발생했습니다: " + err.Error(),
			})
			return
		}
	}

	var lbHops []ssh.HopConfig
	if lbHopsData, ok := request.Parameters["lb_hops"].([]interface{}); ok && len(lbHopsData) > 0 {
		// hopsData를 []ssh.HopConfig로 변환
		lbHops = make([]ssh.HopConfig, len(lbHopsData))
		for i, hop := range lbHopsData {
			if hopMap, ok := hop.(map[string]interface{}); ok {
				host, _ := hopMap["host"].(string)
				username, _ := hopMap["username"].(string)
				password, _ := hopMap["password"].(string)
				port := 22 // 기본값
				if portVal, ok := hopMap["port"].(float64); ok {
					port = int(portVal)
				} else if portStr, ok := hopMap["port"].(string); ok {
					if portInt, err := strconv.Atoi(portStr); err == nil {
						port = portInt
					}
				}
				lbHops[i] = ssh.HopConfig{
					Host:     host,
					Port:     port,
					Username: username,
					Password: password,
				}
			}
		}
	}

	var masterHops []ssh.HopConfig
	if masterHopsData, ok := request.Parameters["main_hops"].([]interface{}); ok && len(masterHopsData) > 0 {
		// hopsData를 []ssh.HopConfig로 변환
		masterHops = make([]ssh.HopConfig, len(masterHopsData))
		for i, hop := range masterHopsData {
			if hopMap, ok := hop.(map[string]interface{}); ok {
				host, _ := hopMap["host"].(string)
				username, _ := hopMap["username"].(string)
				password, _ := hopMap["password"].(string)
				port := 22 // 기본값
				if portVal, ok := hopMap["port"].(float64); ok {
					port = int(portVal)
				} else if portStr, ok := hopMap["port"].(string); ok {
					if portInt, err := strconv.Atoi(portStr); err == nil {
						port = portInt
					}
				}
				masterHops[i] = ssh.HopConfig{
					Host:     host,
					Port:     port,
					Username: username,
					Password: password,
				}
			}
		}
	}

	password := request.Parameters["password"]
	mainPassword := request.Parameters["main_password"]
	lbPassword := request.Parameters["lb_password"]

	// 메인 마스터 노드 여부 확인
	isMainMaster := serverInfo.JoinCommand != "" && serverInfo.CertificateKey != ""
	log.Printf("마스터 노드 %s는 메인 마스터 노드입니까? %v", serverName, isMainMaster)

	// 서버의 infra_id 가져오기
	var infraID int
	err = h.db.QueryRow("SELECT infra_id FROM servers WHERE id = ?", serverID).Scan(&infraID)
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
	err = h.db.QueryRow(query, infraID, serverID).Scan(&otherMasterCount)
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
	if len(lbHops) > 0 && lbPassword != "" {
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
			fmt.Sprintf("echo '%s' | sudo -S bash /tmp/remove_server.sh", lbPassword),

			// 임시 스크립트 파일 삭제
			"rm -f /tmp/remove_server.sh",
		}

		_, err = sshUtils.ExecuteCommands(lbHops, haproxyUpdateCmd, 60000)
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
	_, err = sshUtils.ExecuteCommands(hops, logSetupCommands, 10000)
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
			fmt.Sprintf("echo '%s' | sudo -S kubectl drain %s --delete-emptydir-data --force --ignore-daemonsets", mainPassword, serverName),

			// 1.2. 노드 삭제
			fmt.Sprintf("echo '%s' | sudo -S kubectl delete node %s", mainPassword, serverName),
		}

		mainNodeResults, err := sshUtils.ExecuteCommands(masterHops, mainNodeCommands, 60000)
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
			fmt.Sprintf("echo '%s' | sudo -S ETCDCTL_API=3 etcdctl --endpoints=localhost:2379 --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/server.crt --key=/etc/kubernetes/pki/etcd/server.key member list", mainPassword),
		}

		etcdListResult, err := sshUtils.ExecuteCommands(masterHops, etcdListCmd, 30000)
		if err == nil {
			mainMasterCleanupOutput += etcdListResult[0].Output + "\n"

			// etcd 멤버 ID 추출
			etcdFindCmd := []string{
				fmt.Sprintf("echo '%s' | sudo -S bash -c \"ETCDCTL_API=3 etcdctl --endpoints=localhost:2379 --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/server.crt --key=/etc/kubernetes/pki/etcd/server.key member list | grep %s | cut -d',' -f1\"", mainPassword, serverName),
			}

			etcdFindResult, err := sshUtils.ExecuteCommands(masterHops, etcdFindCmd, 30000)
			if err == nil {
				etcdMemberID := strings.TrimSpace(etcdFindResult[0].Output)
				if etcdMemberID != "" {
					// etcd 멤버 제거
					etcdRemoveCmd := []string{
						fmt.Sprintf("echo '%s' | sudo -S ETCDCTL_API=3 etcdctl --endpoints=localhost:2379 --cacert=/etc/kubernetes/pki/etcd/ca.crt --cert=/etc/kubernetes/pki/etcd/server.crt --key=/etc/kubernetes/pki/etcd/server.key member remove %s", mainPassword, etcdMemberID),
					}

					etcdRemoveResult, err := sshUtils.ExecuteCommands(masterHops, etcdRemoveCmd, 30000)
					if err == nil {
						mainMasterCleanupOutput += etcdRemoveResult[0].Output + "\n"
					}
				}
			}
		}

		// etcd 최종 확인
		time.Sleep(5 * time.Second)
		nodeListCmd := []string{
			fmt.Sprintf("echo '%s' | sudo -S kubectl get nodes", mainPassword),
		}

		nodeListResult, err := sshUtils.ExecuteCommands(masterHops, nodeListCmd, 30000)
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
		fmt.Sprintf("echo '%s' | sudo -S systemctl stop kubelet || true", password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl stop etcd || true", password),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 etcd || true", password),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 kube-apiserver || true", password),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 kube-scheduler || true", password),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 kube-controller-manager || true", password),
	}

	_, _ = sshUtils.ExecuteCommands(hops, stopServicesCmd, 60000)

	// kubeadm reset 및 정리
	cleanupCommands := []string{
		fmt.Sprintf("echo '%s' | sudo -S kubeadm reset -f", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /etc/cni/net.d/*", password),
		fmt.Sprintf("echo '%s' | sudo -S iptables -F", password),
		fmt.Sprintf("echo '%s' | sudo -S iptables -t nat -F", password),
		fmt.Sprintf("echo '%s' | sudo -S iptables -t mangle -F", password),
		fmt.Sprintf("echo '%s' | sudo -S iptables -X", password),
		fmt.Sprintf("echo '%s' | sudo -S ipvsadm --clear", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /root/.kube", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /etc/kubernetes/admin.conf", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /etc/kubernetes/kubelet.conf", password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl stop kubelet", password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl stop containerd", password),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 kube-apiserver", password),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 kube-scheduler", password),
		fmt.Sprintf("echo '%s' | sudo -S pkill -9 kube-controller-manager", password),
		fmt.Sprintf("echo '%s' | sudo -S umount -l /var/lib/kubelet/pods/* || true", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /var/lib/kubelet/pods/*", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /var/lib/kubelet", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /var/lib/etcd", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /etc/kubernetes", password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl disable kubelet", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /opt/cni", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /usr/bin/kubectl", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /usr/bin/kubeadm", password),
		fmt.Sprintf("echo '%s' | sudo -S rm -rf /usr/bin/kubelet", password),
	}

	cleanupResults, err := sshUtils.ExecuteCommands(hops, cleanupCommands, 120000)
	if err == nil {
		for _, result := range cleanupResults {
			masterCleanupOutput += result.Output + "\n"
		}
	}

	// 패키지 제거
	removeCommands := []string{
		fmt.Sprintf("echo '%s' | sudo -S DEBIAN_FRONTEND=noninteractive apt-get remove --allow-change-held-packages -y kubeadm kubectl kubelet kubernetes-cni || true", password),
		fmt.Sprintf("echo '%s' | sudo -S DEBIAN_FRONTEND=noninteractive apt-get purge -y kubeadm kubectl kubelet kubernetes-cni || true", password),
		fmt.Sprintf("echo '%s' | sudo -S DEBIAN_FRONTEND=noninteractive apt-get clean || true", password),
		fmt.Sprintf("echo '%s' | sudo -S DEBIAN_FRONTEND=noninteractive apt-get autoremove -y || true", password),
		fmt.Sprintf("echo '%s' | sudo -S systemctl disable containerd || true", password),
	}

	_, _ = sshUtils.ExecuteCommands(hops, removeCommands, 180000)

	// DB에서 마스터 노드 삭제
	err = db.DeleteMaster(h.db, serverID)
	if err != nil {
		log.Printf("DB에서 마스터 노드 삭제 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "DB에서 마스터 노드 삭제 실패", "errorDetails": err.Error()})
		return
	}

	// 마스터 노드 삭제 성공 후 로드 밸런서에서 해당 마스터 노드 제거
	if len(lbHops) > 0 && lbPassword != "" {
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
			fmt.Sprintf("echo '%s' | sudo -S bash /tmp/remove_server.sh", lbPassword),

			// 임시 스크립트 파일 삭제
			"rm -f /tmp/remove_server.sh",
		}

		_, err = sshUtils.ExecuteCommands(lbHops, haproxyUpdateCmd, 60000)
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

	_, _ = sshUtils.ExecuteCommands(hops, logFinishCommands, 10000)

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

// installOutput은 설치 명령어 실행 결과를 포맷팅하여 반환합니다
func installOutput(results []ssh.CommandResult) []map[string]string {
	output := make([]map[string]string, len(results))
	for i, result := range results {
		output[i] = map[string]string{
			"command": result.Command,
			"output":  result.Output,
			"error":   result.Error,
		}
	}
	return output
}

// 유틸리티 함수
func GetFloat64(val interface{}) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0
}

// 기타 액션 처리 (기본 명령 실행)
func (h *KubernetesHandler) handleOtherAction(c *gin.Context, request CommandRequest) {
	// 지원하지 않는 액션이면 에러 반환
	if !h.cmdManager.HasCommandTemplate(request.Action) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "지원하지 않는 액션입니다: " + request.Action,
		})
		return
	}

	// 명령 실행
	results, err := h.cmdManager.ExecuteAction(request.Action, request.Parameters, nil)
	if err != nil {
		log.Printf("[API 오류] 액션: %s, 오류: %v", request.Action, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "명령 실행 중 오류가 발생했습니다: " + err.Error(),
		})
		return
	}

	// 성공 로그 기록
	log.Printf("[API 성공] 액션: %s, 결과 개수: %d", request.Action, len(results))

	// 성공 응답 (기본)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"result": gin.H{
			"commandResults": results,
		},
	})
}

// 유틸리티 함수들
func getIntParameter(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	case json.Number:
		i, err := v.Int64()
		return int(i), err
	default:
		return 0, fmt.Errorf("유효하지 않은 숫자 형식: %v", value)
	}
}

// 인프라 CRUD 핸들러
func (h *KubernetesHandler) handleGetInfras(c *gin.Context) {
	infras, err := db.GetAllInfras(h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"infras":  infras,
	})
}

func (h *KubernetesHandler) handleGetInfraById(c *gin.Context, request CommandRequest) {
	idVal, exists := request.Parameters["id"]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "id 파라미터가 필요합니다",
		})
		return
	}

	id, err := getIntParameter(idVal)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "유효하지 않은 ID 형식입니다: " + err.Error(),
		})
		return
	}

	infra, err := db.GetInfraById(h.db, id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "인프라를 찾을 수 없습니다",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"infra":   infra,
	})
}

func (h *KubernetesHandler) handleCreateInfra(c *gin.Context, request CommandRequest) {
	var infra db.Infra

	// parameters에서 필요한 정보 추출
	if name, ok := request.Parameters["name"].(string); ok {
		infra.Name = name
	}
	if info, ok := request.Parameters["info"].(string); ok {
		infra.Info = info
	}
	// type 필드 추출 및 저장
	if typeVal, ok := request.Parameters["type"].(string); ok {
		infra.Type = typeVal
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "인프라 타입(type)은 필수 필드입니다.",
		})
		return
	}

	id, err := db.CreateInfra(h.db, infra)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 새로 생성된 인프라 조회
	infra, err = db.GetInfraById(h.db, id)
	if err != nil {
		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"message": "인프라가 생성되었지만 상세 정보를 가져오는데 실패했습니다",
			"id":      id,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"infra":   infra,
	})
}

func (h *KubernetesHandler) handleUpdateInfra(c *gin.Context, request CommandRequest) {
	idVal, exists := request.Parameters["id"]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "id 파라미터가 필요합니다",
		})
		return
	}

	id, err := getIntParameter(idVal)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "유효하지 않은 ID 형식입니다: " + err.Error(),
		})
		return
	}

	// 존재하는지 확인
	existingInfra, err := db.GetInfraById(h.db, id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "인프라를 찾을 수 없습니다",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// parameters에서 필요한 정보 추출
	if name, ok := request.Parameters["name"].(string); ok {
		existingInfra.Name = name
	}
	if info, ok := request.Parameters["info"].(string); ok {
		existingInfra.Info = info
	}
	// type 필드 추출 및 저장
	if typeVal, ok := request.Parameters["type"].(string); ok {
		existingInfra.Type = typeVal
	}

	if err := db.UpdateInfra(h.db, existingInfra); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 업데이트된 인프라 조회
	updatedInfra, err := db.GetInfraById(h.db, id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "인프라가 업데이트되었지만 상세 정보를 가져오는데 실패했습니다",
			"id":      id,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"infra":   updatedInfra,
	})
}

func (h *KubernetesHandler) handleDeleteInfra(c *gin.Context, request CommandRequest) {
	idVal, exists := request.Parameters["id"]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "id 파라미터가 필요합니다",
		})
		return
	}

	id, err := getIntParameter(idVal)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "유효하지 않은 ID 형식입니다: " + err.Error(),
		})
		return
	}

	if err := db.DeleteInfra(h.db, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "인프라가 성공적으로 삭제되었습니다",
	})
}

func (h *KubernetesHandler) handleImportKubernetesInfra(c *gin.Context, request CommandRequest) {
	name, ok := request.Parameters["name"].(string)
	if !ok || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "name 파라미터가 필요합니다",
		})
		return
	}

	info, ok := request.Parameters["info"].(string)
	if !ok || info == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "info 파라미터가 필요합니다",
		})
		return
	}

	var hops []ssh.HopConfig
	if hopsData, ok := request.Parameters["hops"].([]interface{}); ok && len(hopsData) > 0 {
		hops = make([]ssh.HopConfig, len(hopsData))
		for i, hop := range hopsData {
			if hopMap, ok := hop.(map[string]interface{}); ok {
				host, _ := hopMap["host"].(string)
				portStr, _ := hopMap["port"].(string)
				port := 22 // 기본값
				if portVal, err := strconv.Atoi(portStr); err == nil {
					port = portVal
				} else if portFloat, ok := hopMap["port"].(float64); ok {
					port = int(portFloat)
				}
				username, _ := hopMap["username"].(string)
				password, _ := hopMap["password"].(string)
				hops[i] = ssh.HopConfig{
					Host:     host,
					Port:     port,
					Username: username,
					Password: password,
				}
			} else {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error":   "hops 정보 형식이 올바르지 않습니다",
				})
				return
			}
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "SSH 연결 정보(hops)가 필요합니다",
		})
		return
	}

	// SSH 유틸리티 생성
	sshUtils := utils.NewSSHUtils()

	// 마지막 hop의 패스워드 가져오기
	lastHopPassword := ""
	if len(hops) > 0 {
		lastHopPassword = hops[len(hops)-1].Password
	}

	// 단계 1: kubectl 명령어 가능한지 확인 (sudo -S 사용)
	kubectlCheckCmd := fmt.Sprintf("echo '%s' | sudo -S which kubectl || echo 'KUBECTL_NOT_FOUND'", lastHopPassword)
	kubectlResults, err := sshUtils.ExecuteCommands(hops, []string{kubectlCheckCmd}, 30000)

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

	results, err := sshUtils.ExecuteCommands(hops, commands, 60000)
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
	tx, err := h.db.Begin()
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
		name, "external_kubernetes", info,
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
	hopsWithoutCredentials := make([]map[string]interface{}, 0, len(hops))
	for _, hop := range hops {
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
		infraID, name, string(hopsJSON), "external_kubernetes", time.Now().Format("2006-01-02 15:04:05"),
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
			_, err := h.db.Exec(
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
		"server_name":         name,
	})
}

// 서버 CRUD 핸들러
func (h *KubernetesHandler) handleGetServers(c *gin.Context, request CommandRequest) {
	var servers []db.Server
	var err error

	// 파라미터에서 인프라 ID 값 가져오기
	infraIDValue, hasInfraID := request.Parameters["infra_id"]

	// 인프라 ID가 필요함을 명시적으로 확인
	if !hasInfraID || infraIDValue == nil {
		log.Printf("[서버 조회 오류] 인프라 ID 파라미터가 제공되지 않았습니다")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "서버 조회 시 infra_id 파라미터가 필요합니다",
		})
		return
	}

	// infraIDValue를 float64로 변환 (JSON에서 숫자는 기본적으로 float64로 파싱됨)
	var infraID int
	if floatID, ok := infraIDValue.(float64); ok {
		infraID = int(floatID)
	} else if strID, ok := infraIDValue.(string); ok {
		// 문자열로 전달된 경우 변환 시도
		var convErr error
		infraID, convErr = strconv.Atoi(strID)
		if convErr != nil {
			log.Printf("[서버 조회 오류] 유효하지 않은 인프라 ID: %v", infraIDValue)
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "유효하지 않은 인프라 ID입니다",
			})
			return
		}
	} else {
		log.Printf("[서버 조회 오류] 유효하지 않은 인프라 ID 타입: %T, 값: %v", infraIDValue, infraIDValue)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "유효하지 않은 인프라 ID 형식입니다",
		})
		return
	}

	// 인프라 ID로 서버 조회
	servers, err = db.GetServersByInfraID(h.db, infraID)
	if err != nil {
		log.Printf("[서버 조회 오류] %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"servers": servers,
	})
}

func (h *KubernetesHandler) handleGetServerById(c *gin.Context, request CommandRequest) {
	idVal, exists := request.Parameters["id"]
	if !exists {
		log.Printf("[서버 조회 오류] id 파라미터 누락")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "id 파라미터가 필요합니다",
		})
		return
	}

	id, err := getIntParameter(idVal)
	if err != nil {
		log.Printf("[서버 조회 오류] 유효하지 않은 ID 형식: %v", idVal)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "유효하지 않은 ID 형식입니다: " + err.Error(),
		})
		return
	}

	server, err := db.GetServerByID(h.db, id)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[서버 조회 오류] ID %d에 해당하는 서버 없음", id)
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "서버를 찾을 수 없습니다",
			})
			return
		}
		log.Printf("[서버 조회 오류] %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"server":  server,
	})
}

func (h *KubernetesHandler) handleCreateServer(c *gin.Context, request CommandRequest) {
	var serverInput db.ServerInput

	// 노드 타입 먼저 확인 (HA 노드 여부 체크를 위해)
	isHANode := false
	if serverType, ok := request.Parameters["type"].(string); ok {
		serverInput.Type = serverType
		// HA 노드인지 확인 (정확한 타입 체크)
		isHANode = serverType == "ha" || strings.Contains(serverType, "ha")
	}

	// parameters에서 필요한 정보 추출
	if name, ok := request.Parameters["name"].(string); ok && !isHANode {
		// HA 노드가 아닌 경우에만 서버 이름 설정
		serverInput.ServerName = name
	}
	if infraIDVal, ok := request.Parameters["infra_id"]; ok {
		if infraID, err := getIntParameter(infraIDVal); err == nil {
			serverInput.InfraID = infraID
		}
	}
	if hops, ok := request.Parameters["hops"]; ok {
		hopsBytes, _ := json.Marshal(hops)
		serverInput.Hops = string(hopsBytes)
	}
	if joinCommand, ok := request.Parameters["join_command"].(string); ok {
		serverInput.JoinCommand = joinCommand
	}
	if certKey, ok := request.Parameters["certificate_key"].(string); ok {
		serverInput.CertificateKey = certKey
	}

	id, err := db.CreateServer(h.db, serverInput)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 생성된 서버 정보 조회
	server, err := db.GetServerByID(h.db, id)
	if err != nil {
		// 서버는 생성되었지만 조회 실패
		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"message": "서버가 생성되었지만 상세 정보를 가져오는데 실패했습니다",
			"id":      id,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"server":  server,
	})
}

func (h *KubernetesHandler) handleUpdateServer(c *gin.Context, request CommandRequest) {
	idVal, exists := request.Parameters["id"]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "id 파라미터가 필요합니다",
		})
		return
	}

	id, err := getIntParameter(idVal)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "유효하지 않은 ID 형식입니다: " + err.Error(),
		})
		return
	}

	// 존재하는지 확인
	existingServer, err := db.GetServerByID(h.db, id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "서버를 찾을 수 없습니다",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 업데이트할 필드를 담을 ServerInput 구조체 초기화 (기존 값으로)
	serverInput := db.ServerInput{
		ServerName:     existingServer.ServerName,
		InfraID:        existingServer.InfraID,
		Type:           existingServer.Type,
		Hops:           existingServer.Hops,
		JoinCommand:    existingServer.JoinCommand,
		CertificateKey: existingServer.CertificateKey,
		// HA:              existingServer.HA, // 이 줄 제거
		// LastChecked는 여기서 업데이트하지 않음 (getNodeStatus에서 처리)
	}

	// parameters에서 필요한 정보 추출하여 serverInput 업데이트
	if serverNameVal, nameExists := request.Parameters["server_name"]; nameExists {
		// server_name이 요청에 있고 빈 문자열이 아니면 업데이트
		if serverNameStr, ok := serverNameVal.(string); ok && serverNameStr != "" {
			serverInput.ServerName = serverNameStr
		}
	}
	if infraIDVal, ok := request.Parameters["infra_id"]; ok {
		if infraID, err := getIntParameter(infraIDVal); err == nil {
			serverInput.InfraID = infraID
		}
	}
	if typeVal, ok := request.Parameters["type"].(string); ok {
		serverInput.Type = typeVal // 타입 업데이트
	}
	if hops, ok := request.Parameters["hops"]; ok {
		hopsBytes, _ := json.Marshal(hops)
		serverInput.Hops = string(hopsBytes)
	}
	if joinCommand, ok := request.Parameters["join_command"].(string); ok {
		serverInput.JoinCommand = joinCommand
	}
	if certKey, ok := request.Parameters["certificate_key"].(string); ok {
		serverInput.CertificateKey = certKey
	}

	if err := db.UpdateServer(h.db, id, serverInput); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 업데이트된 서버 정보 조회
	server, err := db.GetServerByID(h.db, id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "서버가 업데이트되었지만 상세 정보를 가져오는데 실패했습니다",
			"id":      id,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"server":  server,
	})
}

func (h *KubernetesHandler) handleDeleteServer(c *gin.Context, request CommandRequest) {
	idVal, exists := request.Parameters["id"]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "id 파라미터가 필요합니다",
		})
		return
	}

	id, err := getIntParameter(idVal)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "유효하지 않은 ID 형식입니다: " + err.Error(),
		})
		return
	}

	if err := db.DeleteServer(h.db, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "서버가 성공적으로 삭제되었습니다",
	})
}

// handleGetNamespaceAndPodStatus 함수는 네임스페이스와 파드 상태 확인 요청을 처리합니다
func (h *KubernetesHandler) handleGetNamespaceAndPodStatus(c *gin.Context, request CommandRequest) {
	// 1. 필수 파라미터 확인 (namespace)
	namespace, ok := request.Parameters["namespace"].(string)
	if !ok || namespace == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "namespace 파라미터가 필요합니다",
		})
		return
	}

	// 2. hops 정보 파싱
	var hops []ssh.HopConfig
	if hopsData, ok := request.Parameters["hops"].([]interface{}); ok && len(hopsData) > 0 {
		hops = make([]ssh.HopConfig, len(hopsData))
		for i, hop := range hopsData {
			if hopMap, ok := hop.(map[string]interface{}); ok {
				host, _ := hopMap["host"].(string)
				portStr, _ := hopMap["port"].(string)
				port := 22 // 기본값
				if portVal, err := strconv.Atoi(portStr); err == nil {
					port = portVal
				} else if portFloat, ok := hopMap["port"].(float64); ok {
					port = int(portFloat)
				}
				username, _ := hopMap["username"].(string)
				password, _ := hopMap["password"].(string)
				hops[i] = ssh.HopConfig{
					Host:     host,
					Port:     port,
					Username: username,
					Password: password,
				}
			} else {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error":   "hops 정보 형식이 올바르지 않습니다",
				})
				return
			}
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "SSH 연결 정보(hops)가 필요합니다",
		})
		return
	}

	// 3. 마지막 hop의 비밀번호 추출 (sudo 실행용)
	password := ""
	if len(hops) > 0 {
		password = hops[len(hops)-1].Password
	}

	// 4. 명령어 준비 (CommandManager 사용)
	params := map[string]interface{}{
		"namespace": namespace,
		"password":  password,
	}

	preparedCommands, err := h.cmdManager.PrepareAction(ActionGetNamespaceAndPodStatus, params)
	if err != nil {
		log.Printf("[상태 확인 오류] 명령어 준비 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "명령어 준비 중 오류 발생: " + err.Error(),
		})
		return
	}

	if len(preparedCommands) < 2 {
		log.Printf("[상태 확인 오류] 명령어 준비 실패: 필요한 명령어 개수(2)보다 적음 (%d개)", len(preparedCommands))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "명령어 준비 중 내부 오류 발생",
		})
		return
	}

	nsCheckCmd := []string{preparedCommands[0]}   // 네임스페이스 확인 명령어
	podStatusCmd := []string{preparedCommands[1]} // 파드 상태 확인 명령어

	// 5. 네임스페이스 확인 명령어 실행
	target := &command.CommandTarget{Hops: hops}
	namespaceResults, err := h.cmdManager.ExecuteCustomCommands(target, nsCheckCmd)

	namespaceExists := false
	if err == nil {
		for _, result := range namespaceResults {
			if !strings.Contains(result.Output, "not found") && result.Output != "" {
				namespaceExists = true
				break
			}
		}
	}

	// 6. 파드 상태 확인 (네임스페이스가 존재하는 경우)
	var pods []map[string]interface{}
	if namespaceExists {
		log.Printf("[상태 확인 정보] 네임스페이스 '%s' 존재 확인, 파드 상태 확인 시작", namespace)
		podResults, err := h.cmdManager.ExecuteCustomCommands(target, podStatusCmd)

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

	// 7. 응답 반환
	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"namespace":        namespace,
		"namespace_exists": namespaceExists,
		"pods":             pods,
	})
}

// DeployKubernetes 쿠버네티스 배포를 처리합니다.
func (h *KubernetesHandler) handleDeployKubernetes(c *gin.Context, request CommandRequest) {
	// 필수 파라미터 추출
	repoURL, ok := request.Parameters["repo_url"].(string)
	if !ok || repoURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "저장소 URL은 필수 항목입니다."})
		return
	}

	namespace, ok := request.Parameters["namespace"].(string)
	if !ok || namespace == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "네임스페이스는 필수 항목입니다."})
		return
	}

	// ID 파라미터 추출
	serverIDParam, ok := request.Parameters["id"]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "서버 ID는 필수 항목입니다."})
		return
	}

	// ID를 정수로 변환
	serverID, err := getIntParameter(serverIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "서버 ID 형식이 올바르지 않습니다."})
		return
	}

	// 서버 정보 가져오기
	serverInfo, err := db.GetServerByID(h.db, serverID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 요청 본문에서 hops가 제공되었는지 확인하고, 그렇지 않으면 DB에서 가져옴
	var hops []ssh.HopConfig
	if hopsData, ok := request.Parameters["hops"].([]interface{}); ok && len(hopsData) > 0 {
		// hopsData를 []ssh.HopConfig로 변환
		hops = make([]ssh.HopConfig, len(hopsData))
		for i, hop := range hopsData {
			if hopMap, ok := hop.(map[string]interface{}); ok {
				host, _ := hopMap["host"].(string)
				username, _ := hopMap["username"].(string)
				password, _ := hopMap["password"].(string)
				port := 22 // 기본값
				if portVal, ok := hopMap["port"].(float64); ok {
					port = int(portVal)
				} else if portStr, ok := hopMap["port"].(string); ok {
					if portInt, err := strconv.Atoi(portStr); err == nil {
						port = portInt
					}
				}
				hops[i] = ssh.HopConfig{
					Host:     host,
					Port:     port,
					Username: username,
					Password: password,
				}
			}
		}
	} else {
		// 요청에 hops가 없는 경우 DB에서 가져오기
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

	// 선택적 파라미터 추출
	branch, _ := request.Parameters["branch"].(string)
	if branch == "" {
		branch = "main"
	}

	usernameRepo, _ := request.Parameters["username_repo"].(string)
	passwordRepo, _ := request.Parameters["password_repo"].(string)

	// 저장소 이름 추출 (URL의 마지막 부분)
	repoURLFormatted := repoURL
	repoURLFormatted = strings.TrimPrefix(repoURL, "https://")
	repoURLFormatted = strings.TrimPrefix(repoURL, "http://")
	repoURLFormatted = strings.TrimSuffix(repoURL, ".git")
	parts := strings.Split(repoURL, "/")
	repoName := repoURLFormatted
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
	if usernameRepo != "" && passwordRepo != "" {
		// URL에서 프로토콜 부분 제거
		repoNoProtocol := strings.TrimPrefix(repoURL, "https://")
		repoNoProtocol = strings.TrimPrefix(repoNoProtocol, "http://")

		// @ 문자가 포함된 사용자명 처리 (이메일 주소 등)
		encodedUsername := usernameRepo
		if strings.Contains(encodedUsername, "@") {
			// 사용자명이 이메일인 경우 URL 인코딩 적용
			encodedUsername = strings.ReplaceAll(encodedUsername, "@", "%40")
		}

		gitCmd = fmt.Sprintf("cd %s && git clone -b %s https://%s:%s@%s . 2>&1 || echo 'Git 클론 실패'",
			workDir,
			branch,
			encodedUsername,
			passwordRepo,
			repoNoProtocol)
	} else {
		gitCmd = fmt.Sprintf("cd %s && git clone -b %s %s . 2>&1 || echo 'Git 클론 실패'",
			workDir, branch, repoURL)
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
			password, namespace, password, namespace),
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
			password, modifiedYamlPath, namespace)
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
			password, modifiedYamlPath, namespace)
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
		"message":       fmt.Sprintf("k8s 디렉토리의 YAML 파일들을 네임스페이스 %s에 적용했습니다.", namespace),
		"namespace":     namespace,
		"yaml_files":    yamlFiles,
		"apply_results": applyOutputs,
		"logs":          formattedResults,
	})
}

// DeleteNamespace 쿠버네티스 네임스페이스 삭제를 처리합니다.
func (h *KubernetesHandler) handleDeleteNamespace(c *gin.Context, request CommandRequest) {

	// 1. 필수 파라미터 확인 (namespace)
	namespace, ok := request.Parameters["namespace"].(string)
	if !ok || namespace == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "namespace 파라미터가 필요합니다",
		})
		return
	}
	// 2. hops 정보 파싱
	var hops []ssh.HopConfig
	if hopsData, ok := request.Parameters["hops"].([]interface{}); ok && len(hopsData) > 0 {
		hops = make([]ssh.HopConfig, len(hopsData))
		for i, hop := range hopsData {
			if hopMap, ok := hop.(map[string]interface{}); ok {
				host, _ := hopMap["host"].(string)
				portStr, _ := hopMap["port"].(string)
				port := 22 // 기본값
				if portVal, err := strconv.Atoi(portStr); err == nil {
					port = portVal
				} else if portFloat, ok := hopMap["port"].(float64); ok {
					port = int(portFloat)
				}
				username, _ := hopMap["username"].(string)
				password, _ := hopMap["password"].(string)
				hops[i] = ssh.HopConfig{
					Host:     host,
					Port:     port,
					Username: username,
					Password: password,
				}
			} else {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error":   "hops 정보 형식이 올바르지 않습니다",
				})
				return
			}
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "SSH 연결 정보(hops)가 필요합니다",
		})
		return
	}

	// 3. 마지막 hop의 비밀번호 추출 (sudo 실행용)
	password := ""
	if len(hops) > 0 {
		password = hops[len(hops)-1].Password
	}
	// SSH 유틸리티 생성
	sshUtils := utils.NewSSHUtils()

	// 네임스페이스 삭제 명령
	deleteCmd := fmt.Sprintf("echo '%s' | sudo -S kubectl delete namespace %s", password, namespace)
	results, err := sshUtils.ExecuteCommands(hops, []string{deleteCmd}, 300000) // 5분 타임아웃

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
		log.Printf("네임스페이스 %s 삭제 명령 실행 완료, 완전한 삭제 대기 중...", namespace)

		// 최대 5분(300초) 동안 폴링하며 대기
		maxWaitTime := 300
		interval := 5 // 5초마다 확인

		for i := 0; i < maxWaitTime; i += interval {
			// 네임스페이스 존재 여부 확인
			checkCmd := fmt.Sprintf("echo '%s' | sudo -S kubectl get namespace %s -o name 2>/dev/null || echo 'not found'",
				password, namespace)
			checkResults, err := sshUtils.ExecuteCommands(hops, []string{checkCmd}, 30000)

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
				log.Printf("네임스페이스 %s가 완전히 삭제되었습니다. (소요 시간: %d초)", namespace, i)
				break
			}

			// 5초 대기 후 다시 확인
			if i+interval < maxWaitTime {
				sleepCmd := fmt.Sprintf("sleep %d", interval)
				sshUtils.ExecuteCommands(hops, []string{sleepCmd}, 30000)
			}
		}

		// 최종 확인
		finalCheckCmd := fmt.Sprintf("echo '%s' | sudo -S kubectl get namespace %s -o name 2>/dev/null || echo 'not found'",
			password, namespace)
		finalResults, _ := sshUtils.ExecuteCommands(hops, []string{finalCheckCmd}, 30000)

		namespaceStillExists := false
		for _, result := range finalResults {
			if !strings.Contains(result.Output, "not found") && result.Output != "" {
				namespaceStillExists = true
				break
			}
		}

		if namespaceStillExists {
			message = fmt.Sprintf("네임스페이스 %s 삭제가 시작되었지만, 완전히 삭제되지 않았습니다. 리소스 정리가 계속 진행 중입니다.", namespace)
		} else {
			message = fmt.Sprintf("네임스페이스 %s가 성공적으로 삭제되었습니다.", namespace)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   success,
		"message":   message,
		"logs":      formattedResults,
		"namespace": namespace,
	})
}

func (h *KubernetesHandler) handleRestartPod(c *gin.Context, request CommandRequest) {
	// 필수 필드 검증
	if request.Parameters["namespace"] == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "네임스페이스는 필수 항목입니다."})
		return
	}

	if request.Parameters["namespace"] == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "파드 이름은 필수 항목입니다."})
		return
	}

	var hops []ssh.HopConfig
	if hopsData, ok := request.Parameters["hops"].([]interface{}); ok && len(hopsData) > 0 {
		hops = make([]ssh.HopConfig, len(hopsData))
		for i, hop := range hopsData {
			if hopMap, ok := hop.(map[string]interface{}); ok {
				host, _ := hopMap["host"].(string)
				portStr, _ := hopMap["port"].(string)
				port := 22 // 기본값
				if portVal, err := strconv.Atoi(portStr); err == nil {
					port = portVal
				} else if portFloat, ok := hopMap["port"].(float64); ok {
					port = int(portFloat)
				}
				username, _ := hopMap["username"].(string)
				password, _ := hopMap["password"].(string)
				hops[i] = ssh.HopConfig{
					Host:     host,
					Port:     port,
					Username: username,
					Password: password,
				}
			} else {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error":   "hops 정보 형식이 올바르지 않습니다",
				})
				return
			}
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "SSH 연결 정보(hops)가 필요합니다",
		})
		return
	}

	// 마지막 hop의 패스워드 사용
	password := ""
	if len(hops) > 0 {
		password = hops[len(hops)-1].Password
	}

	podName := request.Parameters["pod_name"]
	namespace := request.Parameters["namespace"]

	// SSH 유틸리티 생성
	sshUtils := utils.NewSSHUtils()

	// 파드 삭제 명령 실행
	deleteCmd := fmt.Sprintf("echo '%s' | sudo -S kubectl delete pod %s -n %s",
		password, podName, namespace)

	results, err := sshUtils.ExecuteCommands(hops, []string{deleteCmd}, 300000) // 5분 타임아웃

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
		message = fmt.Sprintf("파드 %s가 성공적으로 재시작되었습니다.", podName)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   success,
		"message":   message,
		"logs":      formattedResults,
		"namespace": namespace,
		"pod_name":  podName,
	})
}

// handleGetPodLogs 특정 파드의 로그를 가져옵니다.
func (h *KubernetesHandler) handleGetPodLogs(c *gin.Context, request CommandRequest) {
	namespace, ok := request.Parameters["namespace"].(string)
	if !ok || namespace == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "네임스페이스는 필수 항목입니다."})
		return
	}

	podName, ok := request.Parameters["pod_name"].(string)
	if !ok || podName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "podname는 필수 항목입니다."})
		return
	}

	// 요청 본문에서 hops가 제공되었는지 확인하고, 그렇지 않으면 DB에서 가져옴
	var hops []ssh.HopConfig
	if hopsData, ok := request.Parameters["hops"].([]interface{}); ok && len(hopsData) > 0 {
		// hopsData를 []ssh.HopConfig로 변환
		hops = make([]ssh.HopConfig, len(hopsData))
		for i, hop := range hopsData {
			if hopMap, ok := hop.(map[string]interface{}); ok {
				host, _ := hopMap["host"].(string)
				username, _ := hopMap["username"].(string)
				password, _ := hopMap["password"].(string)
				port := 22 // 기본값
				if portVal, ok := hopMap["port"].(float64); ok {
					port = int(portVal)
				} else if portStr, ok := hopMap["port"].(string); ok {
					if portInt, err := strconv.Atoi(portStr); err == nil {
						port = portInt
					}
				}
				hops[i] = ssh.HopConfig{
					Host:     host,
					Port:     port,
					Username: username,
					Password: password,
				}
			}
		}
	}

	// lines 파라미터 처리 - 기본값 100줄, 숫자형 처리
	linesInt := 100 // 기본값: 100줄
	if linesVal, ok := request.Parameters["lines"]; ok {
		switch v := linesVal.(type) {
		case float64:
			linesInt = int(v)
		case int:
			linesInt = v
		case string:
			if l, err := strconv.Atoi(v); err == nil {
				linesInt = l
			}
		}
	}

	// SSH 클라이언트 직접 생성하여 로그 추출
	client, err := h.createSSHClient(hops)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("SSH 연결 실패: %v", err),
		})
		return
	}
	defer client.Close()

	// 마지막 Hop의 패스워드 가져오기
	password := hops[0].Password

	// 1. 파드 존재 여부 확인
	podCmd := fmt.Sprintf("echo '%s' | sudo -S kubectl get pod %s -n %s -o name || echo 'not found'",
		password, podName, namespace)

	podSession, err := client.NewSession()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("SSH 세션 생성 실패: %v", err),
		})
		return
	}

	var podOutput bytes.Buffer
	podSession.Stdout = &podOutput

	if err := podSession.Run(podCmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("파드 확인 명령 실행 실패: %v", err),
		})
		return
	}
	podSession.Close()

	podExists := !strings.Contains(podOutput.String(), "not found") && strings.TrimSpace(podOutput.String()) != ""

	if !podExists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   fmt.Sprintf("파드 '%s'가 네임스페이스 '%s'에 존재하지 않습니다.", podName, namespace),
		})
		return
	}

	// 2. 로그 추출 - 완전한 로그를 가져오기 위해 임시 파일 사용
	// 임시 파일명 생성
	tmpFileName := fmt.Sprintf("/tmp/pod_logs_%s_%s_%d.txt", namespace, podName, time.Now().Unix())

	// 로그를 임시 파일로 저장 - 오류 출력 포함하여 전체 정보 보존
	logCmd := fmt.Sprintf("echo '%s' | sudo -S /usr/bin/kubectl logs --tail=%d %s -n %s > %s 2>/tmp/kubectl_err_%d.log",
		password, linesInt, podName, namespace, tmpFileName, time.Now().Unix())

	logSession, err := client.NewSession()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("로그 세션 생성 실패: %v", err),
		})
		return
	}

	var stderr bytes.Buffer
	logSession.Stderr = &stderr

	if err := logSession.Run(logCmd); err != nil {
		errMsg := stderr.String()

		// 에러 로그 파일 확인 시도
		errCheckSession, _ := client.NewSession()
		var errFileOutput bytes.Buffer
		errCheckSession.Stdout = &errFileOutput
		errCheckSession.Run("cat /tmp/kubectl_err_*.log 2>/dev/null || echo 'Error log not found'")
		errCheckSession.Close()

		logSession.Close()
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": fmt.Sprintf("로그 명령 실행 실패: %v, stderr: %s, errlog: %s",
				err, errMsg, errFileOutput.String()),
		})
		return
	}
	logSession.Close()

	// 3. 임시 파일에서 로그 내용 읽기
	catCmd := fmt.Sprintf("cat %s", tmpFileName)
	catSession, err := client.NewSession()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("로그 읽기 세션 생성 실패: %v", err),
		})
		return
	}

	var logsOutput bytes.Buffer
	catSession.Stdout = &logsOutput

	if err := catSession.Run(catCmd); err != nil {
		catSession.Close()
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("로그 파일 읽기 실패: %v", err),
		})
		return
	}
	catSession.Close()

	// 4. 임시 파일 삭제
	cleanupCmd := fmt.Sprintf("rm -f %s /tmp/kubectl_err_*.log", tmpFileName)
	cleanupSession, err := client.NewSession()
	if err != nil {
		// 파일 삭제 실패해도 계속 진행
		cleanupSession.Close()
	} else {
		cleanupSession.Run(cleanupCmd)
		cleanupSession.Close()
	}

	// 응답 반환
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"namespace":  namespace,
		"pod_name":   podName,
		"lines":      linesInt,
		"logs":       cleanupKubernetesLogs(logsOutput.String()),
		"pod_exists": podExists,
	})
}

// SSH 클라이언트 생성을 위한 헬퍼 함수
func (h *KubernetesHandler) createSSHClient(hops []ssh.HopConfig) (*gossh.Client, error) {
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

// collectResources 시스템 리소스 정보를 수집합니다
func collectResource(hops []ssh.HopConfig) (map[string]string, error) {
	// 마지막 hop의 비밀번호 추출
	lastHopPassword := ""
	if len(hops) > 0 {
		lastHopPassword = hops[len(hops)-1].Password
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
	results, err := sshUtils.ExecuteCommands(hops, commands, 60000)
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

func (h *KubernetesHandler) handleCalculateResources(c *gin.Context, request CommandRequest) {
	// SSH 연결 정보(hops) 가져오기 - 요청 파라미터의 hops 또는 DB에서 가져오기
	var hops []ssh.HopConfig
	if hopsData, ok := request.Parameters["hops"].([]interface{}); ok && len(hopsData) > 0 {
		// hopsData를 []ssh.HopConfig로 변환
		hops = make([]ssh.HopConfig, len(hopsData))
		for i, hop := range hopsData {
			if hopMap, ok := hop.(map[string]interface{}); ok {
				host, _ := hopMap["host"].(string)
				username, _ := hopMap["username"].(string)
				password, _ := hopMap["password"].(string)
				port := 22 // 기본값
				if portVal, ok := hopMap["port"].(float64); ok {
					port = int(portVal)
				} else if portStr, ok := hopMap["port"].(string); ok {
					if portInt, err := strconv.Atoi(portStr); err == nil {
						port = portInt
					}
				}
				hops[i] = ssh.HopConfig{
					Host:     host,
					Port:     port,
					Username: username,
					Password: password,
				}
			}
		}
	}

	// collectResources 함수를 호출하여 리소스 정보 수집
	resourceMap, err := collectResource(hops)
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

func (h *KubernetesHandler) handleCalculateNodes(c *gin.Context, request CommandRequest) {
	var hops []ssh.HopConfig
	if hopsData, ok := request.Parameters["hops"].([]interface{}); ok && len(hopsData) > 0 {
		// hopsData를 []ssh.HopConfig로 변환
		hops = make([]ssh.HopConfig, len(hopsData))
		for i, hop := range hopsData {
			if hopMap, ok := hop.(map[string]interface{}); ok {
				host, _ := hopMap["host"].(string)
				username, _ := hopMap["username"].(string)
				password, _ := hopMap["password"].(string)
				port := 22 // 기본값
				if portVal, ok := hopMap["port"].(float64); ok {
					port = int(portVal)
				} else if portStr, ok := hopMap["port"].(string); ok {
					if portInt, err := strconv.Atoi(portStr); err == nil {
						port = portInt
					}
				}
				hops[i] = ssh.HopConfig{
					Host:     host,
					Port:     port,
					Username: username,
					Password: password,
				}
			}
		}
	}

	// 마지막 hop의 비밀번호 추출
	lastHopPassword := ""
	if len(hops) > 0 {
		lastHopPassword = hops[len(hops)-1].Password
	}

	// SSH 유틸리티 인스턴스 생성
	sshUtils := utils.NewSSHUtils()

	// 쿠버네티스 노드 정보 수집 명령어 (sudo 권한 필요)
	commands := []string{
		fmt.Sprintf("echo '%s' | sudo -S kubectl get nodes -o wide", lastHopPassword),
	}

	// 명령어 실행 (60초 타임아웃)
	results, err := sshUtils.ExecuteCommands(hops, commands, 60000)
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
		roleResults, err := sshUtils.ExecuteCommands(hops, []string{roleCmd}, 30000)

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
