package api

import (
	"database/sql"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func InfraRoutes(router *gin.Engine, db *sql.DB) {
	// API 버전 그룹
	v1 := router.Group("/api/v1")

	// 이제 인프라 핸들러는 사용하지 않음
	// infraHandler := NewInfraHandler(db)
	// v1.POST("/infra/deleteWorker", infraHandler.DeleteWorker)

	// 쿠버네티스 핸들러 초기화 (새로운 API 구조)
	infraHandler := NewInfraHandler(db)

	v1.POST("/infra/installLoadBalancer", infraHandler.InstallLoadBalancer)     // 0
	v1.POST("/infra/installFirstMaster", infraHandler.InstallFirstMaster)       // 0
	v1.POST("/infra/joinMaster", infraHandler.JoinMaster)                       // 0
	v1.POST("/infra/joinWorker", infraHandler.JoinWorker)                       // 0
	v1.POST("/infra/deleteWorker", infraHandler.DeleteWorker)                   // 0
	v1.POST("/infra/deleteMaster", infraHandler.DeleteMaster)                   // 0
	v1.POST("/infra/importKubernetesInfra", infraHandler.ImportKubernetesInfra) // 0
	v1.POST("/infra/importDockerInfra", infraHandler.ImportDockerInfra)         // 0
	v1.POST("/infra/calculateResources", infraHandler.CalculateResources)
	v1.POST("/infra/calculateNodes", infraHandler.CalculateNodes)
}
func InfraDockerRoutes(router *gin.Engine, db *sql.DB) {
	// API 버전 그룹
	v1 := router.Group("/api/v1")

	// 이제 인프라 핸들러는 사용하지 않음
	// infraHandler := NewInfraHandler(db)
	// v1.POST("/infra/deleteWorker", infraHandler.DeleteWorker)

	// 쿠버네티스 핸들러 초기화 (새로운 API 구조)
	infrDockerHandler := NewInfraDockerHandler(db)

	v1.POST("/docker/installDocker", infrDockerHandler.InstallDocker)                       // 0
	v1.POST("/docker/createDockerContainer", infrDockerHandler.CreateDockerContainer)       // 0
	v1.POST("/docker/deleteDockerContainer", infrDockerHandler.DeleteDockerContainer)       // 0
	v1.POST("/docker/getContainerStatus", infrDockerHandler.GetContainerStatus)             // 0
	v1.POST("/docker/getDockerLogs", infrDockerHandler.GetDockerLogs)                       //0
	v1.POST("/docker/uninstallDocker", infrDockerHandler.UninstallDocker)                   // 0
	v1.POST("/docker/deleteOneDockerContainer", infrDockerHandler.DeleteOneDockerContainer) // 0
	v1.POST("/docker/controlDockerContainer", infrDockerHandler.ControlDockerContainer)     // 0
}
func InfraKubernetesRoutes(router *gin.Engine, db *sql.DB) {
	// API 버전 그룹
	v1 := router.Group("/api/v1")

	// 이제 인프라 핸들러는 사용하지 않음
	// infraHandler := NewInfraHandler(db)
	// v1.POST("/infra/deleteWorker", infraHandler.DeleteWorker)

	// 쿠버네티스 핸들러 초기화 (새로운 API 구조)
	infrKubernetesHandler := NewInfraKubernetesHandler(db)

	v1.POST("/kubernetes/deployKubernetes", infrKubernetesHandler.DeployKubernetes)                 // 0
	v1.POST("/kubernetes/deleteNamespace", infrKubernetesHandler.DeleteNamespace)                   // 0
	v1.POST("/kubernetes/getNamespaceAndPodStatus", infrKubernetesHandler.GetNamespaceAndPodStatus) // 0
	v1.POST("/kubernetes/getPodLogs", infrKubernetesHandler.GetPodLogs)                             //0
	v1.POST("/kubernetes/restartPod", infrKubernetesHandler.RestartPod)
}
func ServerRoutes(router *gin.Engine, db *sql.DB) {
	// API 버전 그룹
	v1 := router.Group("/api/v1")

	// 이제 인프라 핸들러는 사용하지 않음
	// infraHandler := NewInfraHandler(db)
	// v1.POST("/infra/deleteWorker", infraHandler.DeleteWorker)

	// 쿠버네티스 핸들러 초기화 (새로운 API 구조)
	serverHandler := NewServerHandler(db)

	v1.POST("/server/status", serverHandler.GetServerStatus) // 0
}

func SetupRoutes(router *gin.Engine, db *sql.DB) {
	// API 버전 그룹
	v1 := router.Group("/api/v1")

	// 이제 인프라 핸들러는 사용하지 않음
	// infraHandler := NewInfraHandler(db)
	// v1.POST("/infra/deleteWorker", infraHandler.DeleteWorker)

	// 쿠버네티스 핸들러 초기화 (새로운 API 구조)
	kubernetesHandler := NewKubernetesHandler(db)

	// 쿠버네티스 엔드포인트 (단일 엔드포인트로 모든 액션 처리)
	v1.POST("/kubernetes", kubernetesHandler.HandleRequest)

	// 도커 핸들러 초기화 (새로운 API 구조)
	dockerHandler := NewDockerHandler(db)

	// 도커 엔드포인트 (단일 엔드포인트로 모든 액션 처리)
	v1.POST("/docker", dockerHandler.HandleRequest)

	// 서비스 핸들러 초기화 (새로운 API 구조)
	serviceHandler := NewServiceHandler(db)

	// 서비스 엔드포인트 (단일 엔드포인트로 모든 액션 처리)
	v1.POST("/service", serviceHandler.HandleRequest)

	// Swagger 문서 설정
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

// RegisterInfraDockerRoutes는 인프라 Docker 관련 경로를 등록합니다.
func RegisterInfraDockerRoutes(router *gin.RouterGroup, handler *InfraDockerHandler) {
	// ... existing code ...

	// 컨테이너 삭제 관련 경로 추가
	router.POST("/docker/deleteOneDockerContainer", handler.DeleteOneDockerContainer)

	// ... existing code ...
}
