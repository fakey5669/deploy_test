package api

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/k8scontrol/backend/internal/db"
)

// 서비스 관련 액션 상수
const (
	// 기본 CRUD 액션 (쿠버네티스 핸들러와 동일한 액션명 사용)
	// ActionGetServices     = "getServices"
	// ActionGetServiceById  = "getServiceById"
	// ActionCreateService   = "createService"
	// ActionUpdateService   = "updateService"
	// ActionDeleteService   = "deleteService"

	// 서비스 관리 액션
	ActionGetServiceStatus = "getServiceStatus"
	ActionDeployService    = "deployService"
	ActionRestartService   = "restartService"
	ActionStopService      = "stopService"
	ActionRemoveService    = "removeService"

	// 도커 관련 액션
	ActionGetDockerFiles    = "getDockerFiles"
	ActionGetDockerfile     = "getDockerfile"
	ActionGetDockerCompose  = "getDockerCompose"
	ActionSaveDockerfile    = "saveDockerfile"
	ActionSaveDockerCompose = "saveDockerCompose"
)

// ServiceHandler 서비스 관련 API 핸들러
type ServiceHandler struct {
	DB *sql.DB
}

// ActionRequest는 액션 기반 API 요청을 표현합니다
type ServiceActionRequest struct {
	Action     string                 `json:"action"`
	Parameters map[string]interface{} `json:"parameters"`
}

// NewServiceHandler 새 ServiceHandler 생성
func NewServiceHandler(db *sql.DB) *ServiceHandler {
	return &ServiceHandler{DB: db}
}

// HandleRequest는 모든 서비스 관련 요청을 처리하는 통합 핸들러입니다
func (h *ServiceHandler) HandleRequest(c *gin.Context) {
	var request ServiceActionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request format: " + err.Error(),
		})
		return
	}

	// 액션 요청 로그 기록
	log.Printf("[Service API 요청] 액션: %s, 파라미터: %+v", request.Action, request.Parameters)

	// 액션에 따라 적절한 핸들러 호출
	switch request.Action {
	// 기본 CRUD 액션 (문자열로 직접 비교)
	case "getServices":
		h.handleGetServices(c, request.Parameters)
	case "getServiceById":
		h.handleGetServiceById(c, request.Parameters)
	case "createService":
		h.handleCreateService(c, request.Parameters)
	case "updateService":
		h.handleUpdateService(c, request.Parameters)
	case "deleteService":
		h.handleDeleteService(c, request.Parameters)

	// 서비스 관리 액션
	case ActionGetServiceStatus:
		h.handleGetServiceStatus(c, request.Parameters)
	case ActionDeployService:
		h.handleDeployService(c, request.Parameters)
	case ActionRestartService:
		h.handleRestartService(c, request.Parameters)
	case ActionStopService:
		h.handleStopService(c, request.Parameters)
	case ActionRemoveService:
		h.handleRemoveService(c, request.Parameters)

	// 도커 관련 액션
	case ActionGetDockerFiles:
		h.handleGetDockerFiles(c, request.Parameters)
	// case ActionGetDockerfile:
	// 	h.handleGetDockerfile(c, request.Parameters)
	// case ActionGetDockerCompose:
	// 	h.handleGetDockerCompose(c, request.Parameters)
	case ActionSaveDockerfile:
		h.handleSaveDockerfile(c, request.Parameters)
	case ActionSaveDockerCompose:
		h.handleSaveDockerCompose(c, request.Parameters)

	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Unknown action: " + request.Action,
		})
	}
}

// 모든 서비스 조회 핸들러
func (h *ServiceHandler) handleGetServices(c *gin.Context, params map[string]interface{}) {
	log.Printf("[서비스 조회] 모든 서비스 조회 요청")
	services, err := db.GetAllServices(h.DB)
	if err != nil {
		log.Printf("[서비스 조회 오류] %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	log.Printf("[서비스 조회 성공] %d개의 서비스 조회됨", len(services))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    services,
	})
}

// ID로 서비스 조회 핸들러
func (h *ServiceHandler) handleGetServiceById(c *gin.Context, params map[string]interface{}) {
	idParam, ok := params["id"]
	if !ok {
		log.Printf("[서비스 조회 오류] id 파라미터 누락")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing required parameter: id",
		})
		return
	}

	var id int
	switch v := idParam.(type) {
	case float64:
		id = int(v)
	case int:
		id = v
	case string:
		var err error
		id, err = strconv.Atoi(v)
		if err != nil {
			log.Printf("[서비스 조회 오류] 유효하지 않은 ID 형식: %s", v)
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid service ID format",
			})
			return
		}
	default:
		log.Printf("[서비스 조회 오류] 지원되지 않는 ID 타입: %T", idParam)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid service ID type",
		})
		return
	}

	log.Printf("[서비스 조회] ID %d로 서비스 조회 중", id)
	service, err := db.GetServiceByID(h.DB, id)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[서비스 조회 오류] ID %d에 해당하는 서비스 없음", id)
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Service not found",
			})
			return
		}
		log.Printf("[서비스 조회 오류] %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	log.Printf("[서비스 조회 성공] ID: %d, 이름: %s", service.ID, service.Name)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    service,
	})
}

// 새 서비스 생성 핸들러
func (h *ServiceHandler) handleCreateService(c *gin.Context, params map[string]interface{}) {
	log.Printf("[서비스 생성] 새 서비스 생성 요청")

	// 파라미터를 db.Service 구조체로 변환
	service := db.Service{}

	if name, ok := params["name"].(string); ok {
		service.Name = name
	} else {
		log.Printf("[서비스 생성 오류] 이름 파라미터 누락 또는 유효하지 않음")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing or invalid required parameter: name",
		})
		return
	}

	if domain, ok := params["domain"].(string); ok {
		service.Domain = sql.NullString{String: domain, Valid: true}
	} else {
		service.Domain = sql.NullString{Valid: false}
	}

	if namespace, ok := params["namespace"].(string); ok && namespace != "" {
		// 네임스페이스 값이 있는 경우 사용
		service.Namespace = sql.NullString{String: namespace, Valid: true}
	} else {
		log.Printf("[서비스 생성 오류] 네임스페이스 파라미터 누락 또는 유효하지 않음")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing or invalid required parameter: namespace",
		})
		return
	}

	if gitlabURL, ok := params["gitlab_url"].(string); ok {
		service.GitlabURL = sql.NullString{String: gitlabURL, Valid: true}
	} else {
		service.GitlabURL = sql.NullString{Valid: false}
	}

	if gitlabID, ok := params["gitlab_id"].(string); ok {
		service.GitlabID = sql.NullString{String: gitlabID, Valid: true}
	} else {
		service.GitlabID = sql.NullString{Valid: false}
	}

	if gitlabPassword, ok := params["gitlab_password"].(string); ok {
		service.GitlabPassword = sql.NullString{String: gitlabPassword, Valid: true}
	} else {
		service.GitlabPassword = sql.NullString{Valid: false}
	}

	if gitlabToken, ok := params["gitlab_token"].(string); ok {
		service.GitlabToken = sql.NullString{String: gitlabToken, Valid: true}
	} else {
		service.GitlabToken = sql.NullString{Valid: false}
	}

	if gitlabBranch, ok := params["gitlab_branch"].(string); ok {
		service.GitlabBranch = sql.NullString{String: gitlabBranch, Valid: true}
	} else {
		service.GitlabBranch = sql.NullString{String: "main", Valid: true} // 기본값으로 main 설정
	}

	if userIDRaw, ok := params["user_id"]; ok {
		switch v := userIDRaw.(type) {
		case float64:
			service.UserID = int64(v)
		case int:
			service.UserID = int64(v)
		case string:
			if id, err := strconv.Atoi(v); err == nil {
				service.UserID = int64(id)
			} else {
				service.UserID = 0
			}
		default:
			service.UserID = 0
		}
	} else {
		service.UserID = 0
	}

	// InfraID 처리
	if infraIDRaw, ok := params["infra_id"]; ok && infraIDRaw != nil {
		switch v := infraIDRaw.(type) {
		case float64:
			service.InfraID = sql.NullInt64{Int64: int64(v), Valid: true}
		case int:
			service.InfraID = sql.NullInt64{Int64: int64(v), Valid: true}
		case string:
			if id, err := strconv.Atoi(v); err == nil {
				service.InfraID = sql.NullInt64{Int64: int64(id), Valid: true}
			} else {
				service.InfraID = sql.NullInt64{Valid: false}
			}
		default:
			service.InfraID = sql.NullInt64{Valid: false}
		}
	} else {
		service.InfraID = sql.NullInt64{Valid: false}
	}

	log.Printf("[서비스 생성] 서비스 데이터: 이름=%s, 도메인=%s, 네임스페이스=%s, GitLab=%s, 인프라ID=%d",
		service.Name, service.Domain.String, service.Namespace.String, service.GitlabURL.String, service.InfraID.Int64)

	// 서비스 생성
	id, err := db.CreateService(h.DB, service)
	if err != nil {
		log.Printf("[서비스 생성 오류] %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 생성된 서비스 조회
	newService, err := db.GetServiceByID(h.DB, id)
	if err != nil {
		log.Printf("[서비스 생성 후 조회 오류] %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	log.Printf("[서비스 생성 성공] ID: %d, 이름: %s", newService.ID, newService.Name)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    newService,
	})
}

// 서비스 업데이트 핸들러
func (h *ServiceHandler) handleUpdateService(c *gin.Context, params map[string]interface{}) {
	// ID 파라미터 검증
	idParam, ok := params["id"]
	if !ok {
		log.Printf("[서비스 업데이트 오류] id 파라미터 누락")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing required parameter: id",
		})
		return
	}

	var id int
	switch v := idParam.(type) {
	case float64:
		id = int(v)
	case int:
		id = v
	case string:
		var err error
		id, err = strconv.Atoi(v)
		if err != nil {
			log.Printf("[서비스 업데이트 오류] 유효하지 않은 ID 형식: %s", v)
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid service ID format",
			})
			return
		}
	default:
		log.Printf("[서비스 업데이트 오류] 지원되지 않는 ID 타입: %T", idParam)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid service ID type",
		})
		return
	}

	// 기존 서비스 조회
	service, err := db.GetServiceByID(h.DB, id)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[서비스 업데이트 오류] ID %d에 해당하는 서비스 없음", id)
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Service not found",
			})
			return
		}
		log.Printf("[서비스 업데이트 오류] 서비스 조회 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 파라미터가 제공된 경우에만 필드 업데이트
	if name, ok := params["name"].(string); ok && name != "" {
		service.Name = name
	}

	if domain, ok := params["domain"].(string); ok {
		service.Domain = sql.NullString{String: domain, Valid: true}
	}

	if namespace, ok := params["namespace"].(string); ok {
		if namespace == "" {
			log.Printf("[서비스 업데이트 오류] 네임스페이스 값이 비어있음")
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Namespace cannot be empty",
			})
			return
		}
		service.Namespace = sql.NullString{String: namespace, Valid: true}
	}

	if gitlabURL, ok := params["gitlab_url"].(string); ok {
		service.GitlabURL = sql.NullString{String: gitlabURL, Valid: true}
	}

	if gitlabID, ok := params["gitlab_id"].(string); ok {
		service.GitlabID = sql.NullString{String: gitlabID, Valid: true}
	}

	if gitlabPassword, ok := params["gitlab_password"].(string); ok {
		service.GitlabPassword = sql.NullString{String: gitlabPassword, Valid: true}
	}

	if gitlabToken, ok := params["gitlab_token"].(string); ok {
		service.GitlabToken = sql.NullString{String: gitlabToken, Valid: true}
	}

	if gitlabBranch, ok := params["gitlab_branch"].(string); ok {
		service.GitlabBranch = sql.NullString{String: gitlabBranch, Valid: true}
	}

	if userIDRaw, ok := params["user_id"]; ok {
		switch v := userIDRaw.(type) {
		case float64:
			service.UserID = int64(v)
		case int:
			service.UserID = int64(v)
		case string:
			if id, err := strconv.Atoi(v); err == nil {
				service.UserID = int64(id)
			}
		}
	}

	if infraIDRaw, ok := params["infra_id"]; ok && infraIDRaw != nil {
		switch v := infraIDRaw.(type) {
		case float64:
			service.InfraID = sql.NullInt64{Int64: int64(v), Valid: true}
		case int:
			service.InfraID = sql.NullInt64{Int64: int64(v), Valid: true}
		case string:
			if id, err := strconv.Atoi(v); err == nil {
				service.InfraID = sql.NullInt64{Int64: int64(id), Valid: true}
			}
		}
	}

	// 서비스 업데이트
	if err := db.UpdateService(h.DB, service); err != nil {
		log.Printf("[서비스 업데이트 오류] 업데이트 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 업데이트된 서비스 조회
	updatedService, err := db.GetServiceByID(h.DB, id)
	if err != nil {
		log.Printf("[서비스 업데이트 후 조회 오류] %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	log.Printf("[서비스 업데이트 성공] ID: %d, 이름: %s", updatedService.ID, updatedService.Name)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    updatedService,
	})
}

// 서비스 삭제 핸들러
func (h *ServiceHandler) handleDeleteService(c *gin.Context, params map[string]interface{}) {
	// ID 파라미터 검증
	idParam, ok := params["id"]
	if !ok {
		log.Printf("[서비스 삭제 오류] id 파라미터 누락")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing required parameter: id",
		})
		return
	}

	var id int
	switch v := idParam.(type) {
	case float64:
		id = int(v)
	case int:
		id = v
	case string:
		var err error
		id, err = strconv.Atoi(v)
		if err != nil {
			log.Printf("[서비스 삭제 오류] 유효하지 않은 ID 형식: %s", v)
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid service ID format",
			})
			return
		}
	default:
		log.Printf("[서비스 삭제 오류] 지원되지 않는 ID 타입: %T", idParam)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid service ID type",
		})
		return
	}

	// 서비스 삭제
	log.Printf("[서비스 삭제] ID %d 서비스 삭제 중", id)
	if err := db.DeleteService(h.DB, id); err != nil {
		log.Printf("[서비스 삭제 오류] %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	log.Printf("[서비스 삭제 성공] ID: %d", id)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":      id,
			"message": "Service deleted successfully",
		},
	})
}

// 서비스 상태 조회 핸들러
func (h *ServiceHandler) handleGetServiceStatus(c *gin.Context, params map[string]interface{}) {
	// ID 파라미터 검증
	idParam, ok := params["id"]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing required parameter: id",
		})
		return
	}

	var id int
	switch v := idParam.(type) {
	case float64:
		id = int(v)
	case int:
		id = v
	case string:
		var err error
		id, err = strconv.Atoi(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid service ID format",
			})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid service ID type",
		})
		return
	}

	// 서비스 조회
	service, err := db.GetServiceByID(h.DB, id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Service not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// TODO: 실제 쿠버네티스 상태 조회 로직 구현
	// 현재는 더미 데이터로 응답
	statusData := map[string]interface{}{
		"id":         service.ID,
		"name":       service.Name,
		"gitlab_url": service.GitlabURL.String,
		"gitlab_id":  service.GitlabID.String,
		"infra_id":   service.InfraID.Int64,
		"namespace":  service.Namespace.String,
		"kubernetesStatus": map[string]interface{}{
			"namespace": map[string]string{
				"name":   service.Namespace.String,
				"status": "Active",
			},
			"pods": []map[string]interface{}{
				{
					"name":     service.Namespace.String + "-pod-1",
					"status":   "Running",
					"ready":    true,
					"restarts": 0,
				},
				{
					"name":     service.Namespace.String + "-pod-2",
					"status":   "Running",
					"ready":    true,
					"restarts": 1,
				},
			},
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    statusData,
	})
}

// 서비스 배포 핸들러
func (h *ServiceHandler) handleDeployService(c *gin.Context, params map[string]interface{}) {
	// ID 파라미터 검증
	idParam, ok := params["id"]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing required parameter: id",
		})
		return
	}

	var id int
	switch v := idParam.(type) {
	case float64:
		id = int(v)
	case int:
		id = v
	case string:
		var err error
		id, err = strconv.Atoi(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid service ID format",
			})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid service ID type",
		})
		return
	}

	// 서비스 조회 (존재 여부 확인)
	_, err := db.GetServiceByID(h.DB, id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Service not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// TODO: 실제 서비스 배포 로직 구현

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":     id,
			"status": "deploying",
		},
	})
}

// 서비스 재시작 핸들러
func (h *ServiceHandler) handleRestartService(c *gin.Context, params map[string]interface{}) {
	// ID 파라미터 검증
	idParam, ok := params["id"]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing required parameter: id",
		})
		return
	}

	var id int
	switch v := idParam.(type) {
	case float64:
		id = int(v)
	case int:
		id = v
	case string:
		var err error
		id, err = strconv.Atoi(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid service ID format",
			})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid service ID type",
		})
		return
	}

	// 서비스 조회 (존재 여부 확인)
	_, err := db.GetServiceByID(h.DB, id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Service not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// TODO: 실제 서비스 재시작 로직 구현

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":     id,
			"status": "restarting",
		},
	})
}

// 서비스 중지 핸들러
func (h *ServiceHandler) handleStopService(c *gin.Context, params map[string]interface{}) {
	// ID 파라미터 검증
	idParam, ok := params["id"]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing required parameter: id",
		})
		return
	}

	var id int
	switch v := idParam.(type) {
	case float64:
		id = int(v)
	case int:
		id = v
	case string:
		var err error
		id, err = strconv.Atoi(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid service ID format",
			})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid service ID type",
		})
		return
	}

	// 서비스 조회 (존재 여부 확인)
	_, err := db.GetServiceByID(h.DB, id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Service not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// TODO: 실제 서비스 중지 로직 구현

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":     id,
			"status": "stopping",
		},
	})
}

// 서비스 제거 핸들러
func (h *ServiceHandler) handleRemoveService(c *gin.Context, params map[string]interface{}) {
	// ID 파라미터 검증
	idParam, ok := params["id"]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing required parameter: id",
		})
		return
	}

	var id int
	switch v := idParam.(type) {
	case float64:
		id = int(v)
	case int:
		id = v
	case string:
		var err error
		id, err = strconv.Atoi(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid service ID format",
			})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid service ID type",
		})
		return
	}

	// 서비스 조회 (존재 여부 확인)
	_, err := db.GetServiceByID(h.DB, id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Service not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// TODO: 실제 쿠버네티스에서 서비스 제거 로직 구현
	// 그 다음에 DB에서 제거

	if err := db.DeleteService(h.DB, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":     id,
			"status": "removed",
		},
	})
}

// 서비스의 도커 파일 정보 조회 핸들러
func (h *ServiceHandler) handleGetDockerFiles(c *gin.Context, params map[string]interface{}) {
	// ID 파라미터 검증
	idParam, ok := params["id"]
	if !ok {
		log.Printf("[도커 파일 조회 오류] id 파라미터 누락")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing required parameter: id",
		})
		return
	}

	var id int
	switch v := idParam.(type) {
	case float64:
		id = int(v)
	case int:
		id = v
	case string:
		var err error
		id, err = strconv.Atoi(v)
		if err != nil {
			log.Printf("[도커 파일 조회 오류] 유효하지 않은 ID 형식: %s", v)
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid service ID format",
			})
			return
		}
	default:
		log.Printf("[도커 파일 조회 오류] 지원되지 않는 ID 타입: %T", idParam)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid service ID type",
		})
		return
	}

	// 서비스 조회 (존재 여부 확인)
	service, err := db.GetServiceByID(h.DB, id)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[도커 파일 조회 오류] ID %d에 해당하는 서비스 없음", id)
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Service not found",
			})
			return
		}
		log.Printf("[도커 파일 조회 오류] 서비스 조회 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// GitLab 연결 확인
	if !service.GitlabURL.Valid || service.GitlabURL.String == "" {
		log.Printf("[도커 파일 조회 오류] 서비스 ID %d에 GitLab URL 정보가 없음", id)
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "GitLab repository not connected",
		})
		return
	}

	// GitLab 인증 정보 확인 (ID와 (Password 또는 Token) 중 하나 이상 필요)
	if !service.GitlabID.Valid || service.GitlabID.String == "" {
		log.Printf("[도커 파일 조회 오류] 서비스 ID %d에 GitLab ID 정보가 없음", id)
		c.JSON(http.StatusOK, gin.H{
			"success":          true,
			"hasDockerfile":    false,
			"hasDockerCompose": false,
			"message":          "GitLab ID is missing",
		})
		return
	}

	// Password나 Token 중 하나 이상 필요
	hasPassword := service.GitlabPassword.Valid && service.GitlabPassword.String != ""
	hasToken := service.GitlabToken.Valid && service.GitlabToken.String != ""

	if !hasPassword && !hasToken {
		log.Printf("[도커 파일 조회 오류] 서비스 ID %d에 GitLab 인증 정보(Password or Token) 누락", id)
		c.JSON(http.StatusOK, gin.H{
			"success":          true,
			"hasDockerfile":    false,
			"hasDockerCompose": false,
			"message":          "GitLab authentication information (Password or Token) is missing",
		})
		return
	}

	// GitLab API를 사용하여 파일 존재 여부 및 내용 확인
	hasDockerfile := false
	hasDockerCompose := false
	var dockerfileContent string
	var dockerComposeContent string

	// 브랜치 확인 (기본값: main)
	gitlabBranch := "main"
	if service.GitlabBranch.Valid && service.GitlabBranch.String != "" {
		gitlabBranch = service.GitlabBranch.String
	}

	// GitLab URL에서 프로젝트 ID 또는 경로 추출
	projectPath, err := extractProjectPathFromURL(service.GitlabURL.String)
	if err != nil {
		log.Printf("[도커 파일 조회 오류] GitLab URL 파싱 실패 - URL: %s, 오류: %v", service.GitlabURL.String, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid GitLab URL format: " + err.Error(),
		})
		return
	}

	// GitLab API URL 구성
	baseApiURL := getGitLabAPIBaseURL(service.GitlabURL.String)

	// URL 인코딩된 프로젝트 경로 생성 - GitLab API는 URL 인코딩된 프로젝트 경로를 요구함
	encodedProjectPath := url.PathEscape(projectPath)

	// 디버그 로깅 추가
	log.Printf("[GitLab API 디버그] 서비스 ID %d, API URL: %s, 프로젝트: %s, 인코딩 경로: %s, 브랜치: %s, 사용자: %s",
		id, baseApiURL, projectPath, encodedProjectPath, gitlabBranch, service.GitlabID.String)

	// 가능한 경로들 - 루트 디렉토리 및 일반적인 위치들
	possibleDockerfilePaths := []string{
		"Dockerfile",
		"docker/Dockerfile",
		"Docker/Dockerfile",
		"dockerfiles/Dockerfile",
		"Dockerfiles/Dockerfile",
		"deploy/Dockerfile",
		"docs/Dockerfile",
		"src/Dockerfile",
		"app/Dockerfile",
		"backend/Dockerfile",
		"frontend/Dockerfile",
		"build/Dockerfile",
		"config/Dockerfile",
		"infra/Dockerfile",
		"k8s/Dockerfile",
		"kubernetes/Dockerfile",
		"container/Dockerfile",
		"containers/Dockerfile",
		"devops/Dockerfile",
		"ci/Dockerfile",
		".docker/Dockerfile",
		".devcontainer/Dockerfile",
	}

	foundDockerfile := false
	for _, path := range possibleDockerfilePaths {
		dockerfileURL := fmt.Sprintf("%s/projects/%s/repository/files/%s/raw?ref=%s",
			baseApiURL, encodedProjectPath, url.PathEscape(path), gitlabBranch)

		log.Printf("[GitLab API 디버그] Dockerfile 요청 URL 시도: %s", dockerfileURL)

		dockerfileResp, err := makeGitLabAPIRequest(dockerfileURL, service.GitlabID.String, service.GitlabPassword.String, service.GitlabToken.String)
		if err != nil {
			log.Printf("[GitLab API 오류] Dockerfile 요청 실패: %v", err)
			continue
		}

		if dockerfileResp.StatusCode == http.StatusOK {
			// Dockerfile 존재
			hasDockerfile = true
			foundDockerfile = true

			// 내용 읽기
			bodyBytes, err := io.ReadAll(dockerfileResp.Body)
			if err != nil {
				log.Printf("[도커 파일 조회 오류] Dockerfile 내용 읽기 실패: %v", err)
			} else {
				dockerfileContent = string(bodyBytes)
				log.Printf("[GitLab API 디버그] Dockerfile 찾음: %s", path)
			}
			dockerfileResp.Body.Close()
			break // 파일을 찾았으므로 반복 중단
		} else {
			log.Printf("[GitLab API 디버그] Dockerfile 요청 실패 - 상태 코드: %d, 경로: %s", dockerfileResp.StatusCode, path)
			if dockerfileResp != nil {
				dockerfileResp.Body.Close()
			}
		}
	}

	if !foundDockerfile {
		log.Printf("[GitLab API 디버그] Dockerfile을 모든 가능한 경로에서 찾지 못했습니다")
	}

	// 2. docker-compose 파일 조회 (.yml 및 .yaml 두 가지 확장자 모두 시도)
	// 가능한 경로들
	possibleComposeFileBases := []string{
		"docker-compose",
		"docker/docker-compose",
		"Docker/docker-compose",
		"compose/docker-compose",
		"deploy/docker-compose",
		"docs/docker-compose",
	}

	possibleExtensions := []string{".yml", ".yaml"}

	foundCompose := false

	for _, basePath := range possibleComposeFileBases {
		for _, ext := range possibleExtensions {
			composePath := basePath + ext
			composeURL := fmt.Sprintf("%s/projects/%s/repository/files/%s/raw?ref=%s",
				baseApiURL, encodedProjectPath, url.PathEscape(composePath), gitlabBranch)

			log.Printf("[GitLab API 디버그] Compose 파일 요청 URL 시도: %s", composeURL)

			composeResp, err := makeGitLabAPIRequest(composeURL, service.GitlabID.String, service.GitlabPassword.String, service.GitlabToken.String)
			if err == nil && composeResp.StatusCode == http.StatusOK {
				// docker-compose 파일 존재
				hasDockerCompose = true
				foundCompose = true

				// 내용 읽기
				bodyBytes, err := io.ReadAll(composeResp.Body)
				if err != nil {
					log.Printf("[도커 파일 조회 오류] docker-compose 파일 내용 읽기 실패: %v", err)
				} else {
					dockerComposeContent = string(bodyBytes)

					// 서비스 이름 변수 치환
					dockerComposeContent = strings.ReplaceAll(dockerComposeContent, "${service.Name}", service.Name)
					log.Printf("[GitLab API 디버그] Compose 파일 찾음: %s", composePath)
				}
				composeResp.Body.Close()
				break // 파일을 찾았으므로 반복 중단
			} else if composeResp != nil {
				composeResp.Body.Close()
			}
		}

		if foundCompose {
			break // 파일을 찾았으므로 반복 중단
		}
	}

	if !foundCompose {
		log.Printf("[GitLab API 디버그] docker-compose 파일을 모든 가능한 경로에서 찾지 못했습니다")
	}

	// 3. 결과 반환
	log.Printf("[도커 파일 조회 성공] 서비스 ID %d, 도커파일: %t, 도커컴포즈: %t",
		id, hasDockerfile, hasDockerCompose)

	c.JSON(http.StatusOK, gin.H{
		"success":              true,
		"hasDockerfile":        hasDockerfile,
		"hasDockerCompose":     hasDockerCompose,
		"dockerfileContent":    dockerfileContent,
		"dockerComposeContent": dockerComposeContent,
	})
}

// GitLab URL에서 프로젝트 경로 추출
func extractProjectPathFromURL(gitlabURL string) (string, error) {
	// http(s)://[host]/[namespace]/[project].git 형식의 URL 파싱
	parsedURL, err := url.Parse(gitlabURL)
	if err != nil {
		return "", err
	}

	// 경로에서 .git 접미사 제거
	path := parsedURL.Path
	path = strings.TrimSuffix(path, ".git")
	path = strings.TrimPrefix(path, "/")

	if path == "" {
		return "", fmt.Errorf("유효한 프로젝트 경로를 추출할 수 없습니다")
	}

	// GitLab API는 URL 인코딩된 형식을 사용함 (일부 인스턴스는 프로젝트 ID를 사용할 수도 있음)
	return path, nil
}

// GitLab 인스턴스의 기본 API URL 생성
func getGitLabAPIBaseURL(gitlabURL string) string {
	parsedURL, err := url.Parse(gitlabURL)
	if err != nil {
		// 기본값으로 GitLab.com API 사용
		return "https://gitlab.com/api/v4"
	}

	// 호스트 추출 (예: gitlab.com, git.example.com 등)
	host := parsedURL.Host
	scheme := parsedURL.Scheme
	if scheme == "" {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s/api/v4", scheme, host)
}

// GitLab API 요청 수행
func makeGitLabAPIRequest(apiURL, username, password string, token string) (*http.Response, error) {
	// HTTP 요청 생성
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		log.Printf("[GitLab API 오류] 요청 생성 실패 - URL: %s, 오류: %v", apiURL, err)
		return nil, err
	}

	// 토큰이 제공된 경우 우선 사용
	if token != "" {
		req.Header.Add("PRIVATE-TOKEN", token)
		log.Printf("[GitLab API 디버그] Private Token으로 인증 - 토큰 길이: %d", len(token))
	} else if strings.HasPrefix(username, "glpat-") || strings.HasPrefix(password, "glpat-") {
		// 개인 액세스 토큰(PAT) 사용 - Private-Token 헤더로 설정
		if strings.HasPrefix(username, "glpat-") {
			req.Header.Add("PRIVATE-TOKEN", username)
			log.Printf("[GitLab API 디버그] 개인 액세스 토큰으로 인증 - 토큰 길이: %d", len(username))
		} else {
			req.Header.Add("PRIVATE-TOKEN", password)
			log.Printf("[GitLab API 디버그] 개인 액세스 토큰으로 인증 - 토큰 길이: %d", len(password))
		}
	} else {
		// 기본 사용자명/비밀번호 인증
		auth := username + ":" + password
		basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
		req.Header.Add("Authorization", basicAuth)
		log.Printf("[GitLab API 디버그] 기본 인증(사용자명/비밀번호)으로 인증 - 사용자: %s", username)
	}

	// 콘텐츠 타입 명시
	req.Header.Add("Accept", "application/json")

	// 요청 핸들러 - 타임아웃 10초 설정
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)

	// 디버깅을 위한 응답 헤더 로깅
	if err != nil {
		log.Printf("[GitLab API 오류] 요청 실패 - URL: %s, 오류: %v", apiURL, err)
		return nil, err
	}

	log.Printf("[GitLab API 디버그] 응답 상태 코드: %d, 헤더: %v, URL: %s",
		resp.StatusCode, resp.Header, apiURL)

	// 응답 본문 로깅 (에러 발생 시)
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("[GitLab API 오류] 응답 본문: %s", string(bodyBytes))
		resp.Body.Close()
		return nil, fmt.Errorf("GitLab API 요청 실패 - 상태 코드: %d", resp.StatusCode)
	}

	return resp, nil
}

// GitLab API POST/PUT 요청 수행 (파일 생성/수정)
func makeGitLabAPIPostRequest(apiURL, username, password, token, method string, body io.Reader) (*http.Response, error) {
	// HTTP 요청 생성
	req, err := http.NewRequest(method, apiURL, body)
	if err != nil {
		log.Printf("[GitLab API 오류] 요청 생성 실패 - URL: %s, 오류: %v", apiURL, err)
		return nil, err
	}

	// 컨텐츠 타입 설정
	req.Header.Add("Content-Type", "application/json")

	// 토큰이 제공된 경우 우선 사용
	if token != "" {
		req.Header.Add("PRIVATE-TOKEN", token)
		log.Printf("[GitLab API 디버그] Private Token으로 인증 - 토큰 길이: %d", len(token))
	} else if strings.HasPrefix(username, "glpat-") || strings.HasPrefix(password, "glpat-") {
		// 개인 액세스 토큰(PAT) 사용 - Private-Token 헤더로 설정
		if strings.HasPrefix(username, "glpat-") {
			req.Header.Add("PRIVATE-TOKEN", username)
			log.Printf("[GitLab API 디버그] 개인 액세스 토큰으로 인증 - 토큰 길이: %d", len(username))
		} else {
			req.Header.Add("PRIVATE-TOKEN", password)
			log.Printf("[GitLab API 디버그] 개인 액세스 토큰으로 인증 - 토큰 길이: %d", len(password))
		}
	} else {
		// 기본 사용자명/비밀번호 인증
		auth := username + ":" + password
		basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
		req.Header.Add("Authorization", basicAuth)
		log.Printf("[GitLab API 디버그] 기본 인증(사용자명/비밀번호)으로 인증 - 사용자: %s", username)
	}

	// 요청 핸들러 - 타임아웃 20초 설정
	client := &http.Client{
		Timeout: 20 * time.Second,
	}
	resp, err := client.Do(req)

	if err != nil {
		log.Printf("[GitLab API 오류] 요청 실패 - URL: %s, 오류: %v", apiURL, err)
		return nil, err
	}

	log.Printf("[GitLab API 디버그] 응답 상태 코드: %d, 헤더: %v, URL: %s",
		resp.StatusCode, resp.Header, apiURL)

	// 에러 응답 로깅
	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("[GitLab API 오류] 응답 본문: %s", string(bodyBytes))
		resp.Body.Close()
		return nil, fmt.Errorf("GitLab API 요청 실패 - 상태 코드: %d", resp.StatusCode)
	}

	return resp, nil
}

// 서비스의 Dockerfile 저장 핸들러
func (h *ServiceHandler) handleSaveDockerfile(c *gin.Context, params map[string]interface{}) {
	// ID 파라미터 검증
	idParam, ok := params["id"]
	if !ok {
		log.Printf("[Dockerfile 저장 오류] id 파라미터 누락")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing required parameter: id",
		})
		return
	}

	var id int
	switch v := idParam.(type) {
	case float64:
		id = int(v)
	case int:
		id = v
	case string:
		var err error
		id, err = strconv.Atoi(v)
		if err != nil {
			log.Printf("[Dockerfile 저장 오류] 유효하지 않은 ID 형식: %s", v)
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid service ID format",
			})
			return
		}
	default:
		log.Printf("[Dockerfile 저장 오류] 지원되지 않는 ID 타입: %T", idParam)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid service ID type",
		})
		return
	}

	// 콘텐츠 파라미터 검증
	content, ok := params["content"].(string)
	if !ok {
		log.Printf("[Dockerfile 저장 오류] content 파라미터 누락 또는 유효하지 않음")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing or invalid required parameter: content",
		})
		return
	}

	// 커밋 메시지 파라미터 (선택 사항)
	commitMessage, _ := params["commitMessage"].(string)
	if commitMessage == "" {
		commitMessage = "Update Dockerfile"
	}

	// 서비스 조회 (존재 여부 확인)
	service, err := db.GetServiceByID(h.DB, id)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[Dockerfile 저장 오류] ID %d에 해당하는 서비스 없음", id)
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Service not found",
			})
			return
		}
		log.Printf("[Dockerfile 저장 오류] 서비스 조회 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// GitLab 연결 확인
	if !service.GitlabURL.Valid || service.GitlabURL.String == "" {
		log.Printf("[Dockerfile 저장 오류] 서비스 ID %d에 GitLab 정보가 없음", id)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "GitLab repository not connected",
		})
		return
	}

	// GitLab 인증 정보 확인
	if !service.GitlabID.Valid || service.GitlabID.String == "" {
		log.Printf("[Dockerfile 저장 오류] 서비스 ID %d에 GitLab ID 정보가 없음", id)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "GitLab credentials are missing",
		})
		return
	}

	// 비밀번호나 토큰 중 하나 이상 필요
	hasPassword := service.GitlabPassword.Valid && service.GitlabPassword.String != ""
	hasToken := service.GitlabToken.Valid && service.GitlabToken.String != ""

	if !hasPassword && !hasToken {
		log.Printf("[Dockerfile 저장 오류] 서비스 ID %d에 GitLab 인증 정보(Password 또는 Token) 누락", id)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "GitLab authentication information (Password or Token) is missing",
		})
		return
	}

	// GitLab URL에서 프로젝트 ID 또는 경로 추출
	projectPath, err := extractProjectPathFromURL(service.GitlabURL.String)
	if err != nil {
		log.Printf("[Dockerfile 저장 오류] GitLab URL 파싱 실패 - URL: %s, 오류: %v", service.GitlabURL.String, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid GitLab URL format: " + err.Error(),
		})
		return
	}

	// GitLab API URL 구성
	baseApiURL := getGitLabAPIBaseURL(service.GitlabURL.String)

	// URL 인코딩된 프로젝트 경로 생성
	encodedProjectPath := url.PathEscape(projectPath)

	// 브랜치 확인 (기본값: main)
	gitlabBranch := "main"
	if service.GitlabBranch.Valid && service.GitlabBranch.String != "" {
		gitlabBranch = service.GitlabBranch.String
	}

	// 파일 경로 (Dockerfile)
	filePath := "Dockerfile"
	encodedFilePath := url.PathEscape(filePath)

	// 파일 존재 여부 확인을 위한 URL
	fileCheckURL := fmt.Sprintf("%s/projects/%s/repository/files/%s?ref=%s",
		baseApiURL, encodedProjectPath, encodedFilePath, gitlabBranch)

	// GitLab 연결 인증 정보 설정
	username := ""
	password := ""
	token := ""

	if service.GitlabID.Valid {
		username = service.GitlabID.String
	}

	if service.GitlabPassword.Valid {
		password = service.GitlabPassword.String
	}

	if service.GitlabToken.Valid {
		token = service.GitlabToken.String
	}

	// 파일 존재 여부 확인
	fileCheckResp, err := makeGitLabAPIRequest(fileCheckURL, username, password, token)

	// HTTP 메서드 결정 (파일이 존재하면 PUT, 없으면 POST)
	httpMethod := "POST" // 기본값은 POST (파일 생성)
	if err == nil && fileCheckResp.StatusCode == http.StatusOK {
		// 파일이 존재하면 PUT (파일 업데이트)
		httpMethod = "PUT"
		fileCheckResp.Body.Close()
	} else if fileCheckResp != nil {
		fileCheckResp.Body.Close()
	}

	// 파일 저장/업데이트 API URL
	fileUpdateURL := fmt.Sprintf("%s/projects/%s/repository/files/%s",
		baseApiURL, encodedProjectPath, encodedFilePath)

	// API 요청 본문 구성
	requestBody := map[string]interface{}{
		"branch":         gitlabBranch,
		"content":        content,
		"commit_message": commitMessage,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		log.Printf("[Dockerfile 저장 오류] JSON 마샬링 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Error preparing request: " + err.Error(),
		})
		return
	}

	// GitLab API 호출하여 파일 저장/업데이트
	resp, err := makeGitLabAPIPostRequest(
		fileUpdateURL,
		username,
		password,
		token,
		httpMethod,
		bytes.NewBuffer(jsonBody),
	)

	if err != nil {
		log.Printf("[Dockerfile 저장 오류] GitLab API 호출 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Error saving file to GitLab: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	// 응답 확인
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("[Dockerfile 저장 성공] 서비스 ID %d, 커밋 메시지: %s", id, commitMessage)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Dockerfile saved successfully",
		})
	} else {
		// 응답 본문 읽기
		bodyBytes, _ := io.ReadAll(resp.Body)
		errorMsg := string(bodyBytes)
		log.Printf("[Dockerfile 저장 오류] GitLab API 응답 오류: %s", errorMsg)

		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "GitLab API error: " + errorMsg,
		})
	}
}

// 서비스의 Docker Compose 파일 저장 핸들러
func (h *ServiceHandler) handleSaveDockerCompose(c *gin.Context, params map[string]interface{}) {
	// ID 파라미터 검증
	idParam, ok := params["id"]
	if !ok {
		log.Printf("[Docker Compose 저장 오류] id 파라미터 누락")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing required parameter: id",
		})
		return
	}

	var id int
	switch v := idParam.(type) {
	case float64:
		id = int(v)
	case int:
		id = v
	case string:
		var err error
		id, err = strconv.Atoi(v)
		if err != nil {
			log.Printf("[Docker Compose 저장 오류] 유효하지 않은 ID 형식: %s", v)
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid service ID format",
			})
			return
		}
	default:
		log.Printf("[Docker Compose 저장 오류] 지원되지 않는 ID 타입: %T", idParam)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid service ID type",
		})
		return
	}

	// 콘텐츠 파라미터 검증
	content, ok := params["content"].(string)
	if !ok {
		log.Printf("[Docker Compose 저장 오류] content 파라미터 누락 또는 유효하지 않음")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Missing or invalid required parameter: content",
		})
		return
	}

	// 커밋 메시지 파라미터 (선택 사항)
	commitMessage, _ := params["commitMessage"].(string)
	if commitMessage == "" {
		commitMessage = "Update docker-compose.yml"
	}

	// 서비스 조회 (존재 여부 확인)
	service, err := db.GetServiceByID(h.DB, id)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[Docker Compose 저장 오류] ID %d에 해당하는 서비스 없음", id)
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Service not found",
			})
			return
		}
		log.Printf("[Docker Compose 저장 오류] 서비스 조회 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// GitLab 연결 확인
	if !service.GitlabURL.Valid || service.GitlabURL.String == "" {
		log.Printf("[Docker Compose 저장 오류] 서비스 ID %d에 GitLab 정보가 없음", id)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "GitLab repository not connected",
		})
		return
	}

	// GitLab 인증 정보 확인
	if !service.GitlabID.Valid || service.GitlabID.String == "" {
		log.Printf("[Docker Compose 저장 오류] 서비스 ID %d에 GitLab ID 정보가 없음", id)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "GitLab credentials are missing",
		})
		return
	}

	// 비밀번호나 토큰 중 하나 이상 필요
	hasPassword := service.GitlabPassword.Valid && service.GitlabPassword.String != ""
	hasToken := service.GitlabToken.Valid && service.GitlabToken.String != ""

	if !hasPassword && !hasToken {
		log.Printf("[Docker Compose 저장 오류] 서비스 ID %d에 GitLab 인증 정보(Password 또는 Token) 누락", id)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "GitLab authentication information (Password or Token) is missing",
		})
		return
	}

	// GitLab URL에서 프로젝트 ID 또는 경로 추출
	projectPath, err := extractProjectPathFromURL(service.GitlabURL.String)
	if err != nil {
		log.Printf("[Docker Compose 저장 오류] GitLab URL 파싱 실패 - URL: %s, 오류: %v", service.GitlabURL.String, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid GitLab URL format: " + err.Error(),
		})
		return
	}

	// GitLab API URL 구성
	baseApiURL := getGitLabAPIBaseURL(service.GitlabURL.String)

	// URL 인코딩된 프로젝트 경로 생성
	encodedProjectPath := url.PathEscape(projectPath)

	// 브랜치 확인 (기본값: main)
	gitlabBranch := "main"
	if service.GitlabBranch.Valid && service.GitlabBranch.String != "" {
		gitlabBranch = service.GitlabBranch.String
	}

	// 파일 경로 (docker-compose.yml)
	filePath := "docker-compose.yml"
	encodedFilePath := url.PathEscape(filePath)

	// 파일 존재 여부 확인을 위한 URL
	fileCheckURL := fmt.Sprintf("%s/projects/%s/repository/files/%s?ref=%s",
		baseApiURL, encodedProjectPath, encodedFilePath, gitlabBranch)

	// GitLab 연결 인증 정보 설정
	username := ""
	password := ""
	token := ""

	if service.GitlabID.Valid {
		username = service.GitlabID.String
	}

	if service.GitlabPassword.Valid {
		password = service.GitlabPassword.String
	}

	if service.GitlabToken.Valid {
		token = service.GitlabToken.String
	}

	// 파일 존재 여부 확인
	fileCheckResp, err := makeGitLabAPIRequest(fileCheckURL, username, password, token)

	// HTTP 메서드 결정 (파일이 존재하면 PUT, 없으면 POST)
	httpMethod := "POST" // 기본값은 POST (파일 생성)
	if err == nil && fileCheckResp.StatusCode == http.StatusOK {
		// 파일이 존재하면 PUT (파일 업데이트)
		httpMethod = "PUT"
		fileCheckResp.Body.Close()
	} else if fileCheckResp != nil {
		fileCheckResp.Body.Close()
	}

	// 파일 저장/업데이트 API URL
	fileUpdateURL := fmt.Sprintf("%s/projects/%s/repository/files/%s",
		baseApiURL, encodedProjectPath, encodedFilePath)

	// API 요청 본문 구성
	requestBody := map[string]interface{}{
		"branch":         gitlabBranch,
		"content":        content,
		"commit_message": commitMessage,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		log.Printf("[Docker Compose 저장 오류] JSON 마샬링 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Error preparing request: " + err.Error(),
		})
		return
	}

	// GitLab API 호출하여 파일 저장/업데이트
	resp, err := makeGitLabAPIPostRequest(
		fileUpdateURL,
		username,
		password,
		token,
		httpMethod,
		bytes.NewBuffer(jsonBody),
	)

	if err != nil {
		log.Printf("[Docker Compose 저장 오류] GitLab API 호출 실패: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Error saving file to GitLab: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	// 응답 확인
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("[Docker Compose 저장 성공] 서비스 ID %d, 커밋 메시지: %s", id, commitMessage)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Docker Compose file saved successfully",
		})
	} else {
		// 응답 본문 읽기
		bodyBytes, _ := io.ReadAll(resp.Body)
		errorMsg := string(bodyBytes)
		log.Printf("[Docker Compose 저장 오류] GitLab API 응답 오류: %s", errorMsg)

		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "GitLab API error: " + errorMsg,
		})
	}
}
