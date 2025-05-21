package api

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/k8scontrol/backend/internal/db"
	"github.com/k8scontrol/backend/internal/utils"
	"github.com/k8scontrol/backend/pkg/ssh"
)

// ServerHandler 서버 관련 API 핸들러
type ServerHandler struct {
	DB *sql.DB
}

// NewServerHandler 새 ServerHandler 생성
func NewServerHandler(db *sql.DB) *ServerHandler {
	return &ServerHandler{DB: db}
}

// GetServers 모든 서버 조회
func (h *ServerHandler) GetServers(c *gin.Context) {
	// 인프라 ID로 필터링
	infraIDParam := c.Query("infra_id")
	typeParam := c.Query("type")

	// 인프라 ID 필터링이 있는 경우
	if infraIDParam != "" {
		infraID, err := strconv.Atoi(infraIDParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "유효하지 않은 인프라 ID입니다"})
			return
		}

		servers, err := db.GetServersByInfraID(h.DB, infraID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, servers)
		return
	}

	// 서버 타입 필터링이 있는 경우
	if typeParam != "" {
		servers, err := db.GetServersByType(h.DB, typeParam)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, servers)
		return
	}

	// 모든 서버 조회
	servers, err := db.GetAllServers(h.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, servers)
}

// GetServerById ID로 서버 조회
func (h *ServerHandler) GetServerById(c *gin.Context) {
	// 로그 기록
	log.Printf("[Server API 요청] GET /servers/%s", c.Param("id"))

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "유효하지 않은 서버 ID입니다"})
		return
	}

	server, err := db.GetServerByID(h.DB, id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "서버를 찾을 수 없습니다"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, server)
}

// CreateServer 새 서버 생성
func (h *ServerHandler) CreateServer(c *gin.Context) {
	var input db.ServerInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	id, err := db.CreateServer(h.DB, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 생성된 서버 정보 조회
	server, err := db.GetServerByID(h.DB, id)
	if err != nil {
		// 서버는 생성되었지만 조회 실패
		c.JSON(http.StatusCreated, gin.H{
			"id":      id,
			"message": "서버가 생성되었지만 상세 정보를 가져오는데 실패했습니다",
		})
		return
	}

	c.JSON(http.StatusCreated, server)
}

// UpdateServer 서버 업데이트
func (h *ServerHandler) UpdateServer(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Printf("[서버 업데이트 오류] 유효하지 않은 ID 형식: %s", c.Param("id"))
		c.JSON(http.StatusBadRequest, gin.H{"error": "유효하지 않은 서버 ID입니다"})
		return
	}

	// 해당 ID의 서버가 존재하는지 확인
	_, err = db.GetServerByID(h.DB, id)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[서버 업데이트 오류] ID %d에 해당하는 서버 없음", id)
			c.JSON(http.StatusNotFound, gin.H{"error": "서버를 찾을 수 없습니다"})
			return
		}
		log.Printf("[서버 업데이트 오류] %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var input db.ServerInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[서버 업데이트 오류] 요청 형식 오류: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := db.UpdateServer(h.DB, id, input); err != nil {
		log.Printf("[서버 업데이트 오류] %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 업데이트된 서버 정보 조회
	server, err := db.GetServerByID(h.DB, id)
	if err != nil {
		log.Printf("[서버 업데이트 경고] 서버 업데이트 성공했으나 조회 실패: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"id":      id,
			"message": "서버가 업데이트되었지만 상세 정보를 가져오는데 실패했습니다",
		})
		return
	}

	c.JSON(http.StatusOK, server)
}

// DeleteServer 서버 삭제
func (h *ServerHandler) DeleteServer(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Printf("[서버 삭제 오류] 유효하지 않은 ID 형식: %s", c.Param("id"))
		c.JSON(http.StatusBadRequest, gin.H{"error": "유효하지 않은 서버 ID입니다"})
		return
	}

	// 해당 ID의 서버가 존재하는지 확인
	server, err := db.GetServerByID(h.DB, id)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[서버 삭제 오류] ID %d에 해당하는 서버 없음", id)
			c.JSON(http.StatusNotFound, gin.H{"error": "서버를 찾을 수 없습니다"})
			return
		}
		log.Printf("[서버 삭제 오류] %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[서버 삭제] ID %d 서버 삭제 중 - 이름: %s, 타입: %s",
		id, server.ServerName, server.Type)

	if err := db.DeleteServer(h.DB, id); err != nil {
		log.Printf("[서버 삭제 오류] %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "서버가 성공적으로 삭제되었습니다"})
}

func (h *ServerHandler) GetServerStatus(c *gin.Context) {
	// 로그 기록
	log.Printf("[Server API 요청] POST /servers/status")

	// 요청 본문 파싱
	var requestBody struct {
		Hops []ssh.HopConfig `json:"hops"`
		Type string          `json:"type"` // 서버 타입 (ha, master, worker 등)
		ID   int             `json:"id"`   // 서버 ID
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		log.Printf("[서버 상태 확인 오류] JSON 바인딩 오류: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "잘못된 요청입니다."})
		return
	}

	if len(requestBody.Hops) == 0 {
		log.Printf("[서버 상태 확인 오류] SSH 연결 정보 누락")
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "SSH 연결 정보(hops)가 필요합니다."})
		return
	}

	log.Printf("[서버 상태 확인] 서버 ID: %d, 타입: %s, Hop 수: %d",
		requestBody.ID, requestBody.Type, len(requestBody.Hops))

	// Hop 정보 상세 로깅 (비밀번호 제외)
	for i, hop := range requestBody.Hops {
		log.Printf("[서버 상태 확인] Hop %d 정보: %s@%s:%d", i+1, hop.Username, hop.Host, hop.Port)
	}

	// 서버 ID가 제공된 경우 서버 정보 조회
	if requestBody.ID > 0 {
		server, err := db.GetServerByID(h.DB, requestBody.ID)
		if err != nil {
			log.Printf("[서버 상태 확인 경고] 서버 정보 조회 실패: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "서버 정보를 조회할 수 없습니다."})
			return
		}
		log.Printf("[서버 상태 확인] 서버 정보 조회 성공: 이름=%s, 타입=%s", server.ServerName, server.Type)
	}

	// 현재 시간 가져오기
	now := time.Now()
	lastCheckedStr := now.Format("2006-01-02 15:04:05")

	// 기본값 설정 - 연결 문제 시 기본적으로 false 반환
	installed := false
	running := false
	isMaster := false
	isWorker := false

	// SSH 유틸리티 생성
	sshUtils := utils.NewSSHUtils()

	// 단일 통합 명령어 생성 - 더 안정적인 방식
	var cmd string
	serverType := strings.ToLower(requestBody.Type)

	// 서버 정보 조회
	server, err := db.GetServerByID(h.DB, requestBody.ID)
	if err != nil {
		log.Printf("[서버 상태 확인 오류] 서버 정보 조회 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "서버 정보를 조회할 수 없습니다."})
		return
	}

	switch serverType {
	case "ha":
		cmd = "echo '===START==='; " +
			"if dpkg -l | grep -q haproxy; then echo 'INSTALLED=true'; else echo 'INSTALLED=false'; fi; " +
			"if systemctl status haproxy | grep -q 'Active: active (running)'; then echo 'RUNNING=true'; else echo 'RUNNING=false'; fi; " +
			"echo '===END==='"
		log.Printf("[서버 상태 확인] HA 노드 상태 확인 명령어 준비 완료")
	case "master":
		cmd = "echo '===START==='; " +
			"if command -v kubectl >/dev/null 2>&1 && command -v kubelet >/dev/null 2>&1; then echo 'INSTALLED=true'; else echo 'INSTALLED=false'; fi; " +
			"if systemctl status kubelet 2>/dev/null | grep -q 'Active: active (running)'; then echo 'KUBELET_RUNNING=true'; else echo 'KUBELET_RUNNING=false'; fi; " +
			"echo \"hostname=" + server.ServerName + "\"; " +
			"if echo '" + requestBody.Hops[len(requestBody.Hops)-1].Password + "' | sudo -S kubectl get nodes --no-headers 2>/dev/null | grep -E \"" + server.ServerName + ".*control-plane|" + server.ServerName + ".*master\"; then echo 'IS_MASTER=true'; else echo 'IS_MASTER=false'; fi; " +
			"if echo '" + requestBody.Hops[len(requestBody.Hops)-1].Password + "' | sudo -S kubectl get nodes --no-headers 2>/dev/null | grep \"" + server.ServerName + "\" | grep -vE \"control-plane|master\"; then echo 'IS_WORKER=true'; else echo 'IS_WORKER=false'; fi; " +
			"if echo '" + requestBody.Hops[len(requestBody.Hops)-1].Password + "' | sudo -S kubectl get nodes --no-headers 2>/dev/null | grep -q \"" + server.ServerName + "\"; then echo 'NODE_REGISTERED=true'; else echo 'NODE_REGISTERED=false'; fi; " +
			"echo '" + requestBody.Hops[len(requestBody.Hops)-1].Password + "' | sudo -S kubectl get nodes -o wide 2>/dev/null | grep \"" + server.ServerName + "\" || echo 'NODE_STATUS=NotFound'; " +
			"echo '===END==='"
		log.Printf("[서버 상태 확인] 마스터 노드 상태 확인 명령어 준비 완료")
	case "worker":
		cmd = "echo '===START==='; " +
			"if command -v kubectl >/dev/null 2>&1 && command -v kubelet >/dev/null 2>&1; then echo 'INSTALLED=true'; else echo 'INSTALLED=false'; fi; " +
			"if systemctl status kubelet 2>/dev/null | grep -q 'Active: active (running)'; then echo 'KUBELET_RUNNING=true'; else echo 'KUBELET_RUNNING=false'; fi; " +
			"echo \"hostname=" + server.ServerName + "\"; " +
			"echo '===END==='"
		log.Printf("[서버 상태 확인] 워커 노드 상태 확인 명령어 준비 완료")
	case "docker":
		cmd = "echo '===START==='; " +
			"if command -v docker >/dev/null; then echo 'DOCKER_INSTALLED=true'; else echo 'DOCKER_INSTALLED=false'; fi; " +
			"if systemctl status docker | grep -q 'Active: active (running)'; then echo 'DOCKER_RUNNING=true'; else echo 'DOCKER_RUNNING=false'; fi; " +
			"echo '" + requestBody.Hops[len(requestBody.Hops)-1].Password + "' | sudo -S docker info | grep 'Server Version' || echo 'DOCKER_VERSION=NotFound'; " +
			"echo '" + requestBody.Hops[len(requestBody.Hops)-1].Password + "' | sudo -S docker ps --format '{{.Names}}' || echo 'DOCKER_CONTAINERS=NotFound'; " +
			"echo '" + requestBody.Hops[len(requestBody.Hops)-1].Password + "' | sudo -S docker system df || echo 'DOCKER_DISK_USAGE=NotFound'; " +
			"echo '" + requestBody.Hops[len(requestBody.Hops)-1].Password + "' | sudo -S docker network ls --format '{{.Name}}' || echo 'DOCKER_NETWORKS=NotFound'; " +
			"echo '===END==='"
		log.Printf("[서버 상태 확인] 도커 노드 상태 확인 명령어 준비 완료")
	default:
		log.Printf("[서버 상태 확인 오류] 지원되지 않는 서버 타입: %s", serverType)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "지원하지 않는 서버 타입입니다."})
		return
	}

	// 최대 10번 재시도
	var output string
	success := false

	log.Printf("[서버 상태 확인] 명령어 실행 시작 (최대 10회 시도)")
	for attempt := 1; attempt <= 10; attempt++ {
		log.Printf("[서버 상태 확인] 시도 %d/10", attempt)
		startTime := time.Now()

		results, err := sshUtils.ExecuteCommands(requestBody.Hops, []string{cmd}, 20000)
		executionTime := time.Since(startTime)

		if err != nil {
			// 오류 타입에 따라 더 상세한 로그 추가
			if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "timed out") {
				log.Printf("[서버 상태 확인 오류] 시도 %d 타임아웃 발생 (소요시간: %v): %v",
					attempt, executionTime, err)
			} else if strings.Contains(err.Error(), "connection") || strings.Contains(err.Error(), "connect") {
				log.Printf("[서버 상태 확인 오류] 시도 %d 연결 실패 (소요시간: %v): %v",
					attempt, executionTime, err)
			} else if strings.Contains(err.Error(), "authentication") || strings.Contains(err.Error(), "auth") {
				log.Printf("[서버 상태 확인 오류] 시도 %d 인증 실패 (소요시간: %v): %v",
					attempt, executionTime, err)
			} else {
				log.Printf("[서버 상태 확인 오류] 시도 %d 실패 (소요시간: %v): %v",
					attempt, executionTime, err)
			}

			if attempt < 3 {
				log.Printf("[서버 상태 확인] 1초 후 재시도...")
				time.Sleep(1 * time.Second)
				continue
			}
			break
		}

		if len(results) > 0 {
			output = results[0].Output
			if strings.Contains(output, "===START===") && strings.Contains(output, "===END===") {
				success = true
				log.Printf("[서버 상태 확인] 시도 %d 성공 (소요시간: %v)", attempt, executionTime)
				break
			} else {
				log.Printf("[서버 상태 확인 오류] 시도 %d - 명령 실행됨 but 올바른 출력 없음 (소요시간: %v), 출력: %s",
					attempt, executionTime, utils.TruncateString(output, 100))
			}
		} else {
			log.Printf("[서버 상태 확인 오류] 시도 %d - 명령 실행됨 but 결과 없음 (소요시간: %v)",
				attempt, executionTime)
		}

		if attempt < 3 {
			log.Printf("[서버 상태 확인] 1초 후 재시도...")
			time.Sleep(1 * time.Second)
		}
	}

	// 명령어 실행 성공 시 결과 파싱
	if success {
		log.Printf("[서버 상태 확인] 결과 분석 중")
		log.Printf("[서버 상태 확인] 실행 결과 상세: %s", utils.TruncateString(output, 300))

		// 서버 타입에 따라 다른 방식으로 installed 상태 판단
		if serverType == "docker" {
			if strings.Contains(output, "DOCKER_INSTALLED=true") {
				installed = true
				log.Printf("[서버 상태 확인] 도커 설치됨: true")
			} else {
				log.Printf("[서버 상태 확인] 도커 설치됨: false")
			}
		} else {
			if strings.Contains(output, "INSTALLED=true") {
				installed = true
			}
		}

		// 서버 타입에 따라 다른 방식으로 running 상태 판단
		if serverType == "ha" {
			if strings.Contains(output, "RUNNING=true") {
				running = true
			}
			log.Printf("[서버 상태 확인] HA 노드 상태: installed=%v, running=%v", installed, running)
		} else if serverType == "master" || serverType == "worker" {
			kubeletRunning := strings.Contains(output, "KUBELET_RUNNING=true")
			nodeRegistered := strings.Contains(output, "NODE_REGISTERED=true")

			if strings.Contains(output, "IS_MASTER=true") {
				isMaster = true
			}

			if strings.Contains(output, "IS_WORKER=true") {
				isWorker = true
			}

			// 마스터 노드는 kubelet이 실행 중이고 마스터로 등록되어 있어야 함
			if serverType == "master" {
				running = kubeletRunning && isMaster && nodeRegistered
				log.Printf("[서버 상태 확인] 마스터 노드 상태: installed=%v, running=%v, kubelet=%v, isMaster=%v, registered=%v",
					installed, running, kubeletRunning, isMaster, nodeRegistered)
			}

			// 워커 노드는 kubelet이 실행 중이고 워커로 등록되어 있어야 함
			if serverType == "worker" {
				running = kubeletRunning
				log.Printf("[서버 상태 확인] 워커 노드 상태: installed=%v, running=%v, kubelet=%v",
					installed, running, kubeletRunning)
			}
		} else if serverType == "docker" {
			if strings.Contains(output, "DOCKER_RUNNING=true") {
				running = true
				log.Printf("[서버 상태 확인] 도커 실행 중: true")
			} else {
				log.Printf("[서버 상태 확인] 도커 실행 중: false")
			}
			log.Printf("[서버 상태 확인] 도커 노드 상태: installed=%v, running=%v", installed, running)
		}
	} else {
		log.Printf("[서버 상태 확인 경고] 모든 시도 실패 - 기본값 사용")

		// 도커 타입인 경우 결과에 DOCKER_INSTALLED=true와 DOCKER_RUNNING=true가 있으면 성공으로 간주
		if serverType == "docker" && strings.Contains(output, "DOCKER_INSTALLED=true") && strings.Contains(output, "DOCKER_RUNNING=true") {
			log.Printf("[서버 상태 확인] 도커 상태 확인 성공 (===END=== 없음)")
			installed = true
			running = true
			log.Printf("[서버 상태 확인] 도커 노드 상태: installed=%v, running=%v", installed, running)
		}
	}

	// 서버 ID가 제공된 경우 DB에 마지막 확인 시간 업데이트
	if requestBody.ID > 0 {
		err := db.UpdateServerLastChecked(h.DB, requestBody.ID, now)
		if err != nil {
			log.Printf("[서버 상태 확인 경고] 마지막 확인 시간 업데이트 실패: %v", err)
			// 업데이트 실패해도 계속 진행
		} else {
			log.Printf("[서버 상태 확인] 마지막 확인 시간 업데이트 성공: %s", now.Format(time.RFC3339))
		}
	}

	// 응답 반환 - 기본 응답
	response := gin.H{
		"success": true,
		"status": gin.H{
			"installed": installed,
			"running":   running,
		},
		"lastChecked": lastCheckedStr,
	}

	// 마스터/워커 노드인 경우 추가 정보 제공
	if serverType == "master" || serverType == "worker" {
		response["status"].(gin.H)["isMaster"] = isMaster
		response["status"].(gin.H)["isWorker"] = isWorker
	}

	log.Printf("[서버 상태 확인 완료] 서버 ID: %d, 타입: %s, 설치됨: %v, 실행 중: %v",
		requestBody.ID, serverType, installed, running)
	c.JSON(http.StatusOK, response)
}
