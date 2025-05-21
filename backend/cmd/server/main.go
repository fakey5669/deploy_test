package main

import (
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/k8scontrol/backend/internal/api"
	"github.com/k8scontrol/backend/internal/db"
)

// @title K8S Control API
// @version 1.0
// @description API Server for K8S Control Application
// @host localhost:8080
// @BasePath /api/v1
func main() {
	// 환경 변수 로드
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// 서버 포트 설정
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// 데이터베이스 연결
	dbConn, err := db.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbConn.Close()

	// Gin 라우터 설정
	router := gin.Default()

	// CORS 설정
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "https://kc.mipllab.com", "http://kc.mipllab.com"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// API 라우트 설정
	api.SetupRoutes(router, dbConn)
	api.InfraRoutes(router, dbConn)
	api.InfraDockerRoutes(router, dbConn)
	api.InfraKubernetesRoutes(router, dbConn)
	api.ServerRoutes(router, dbConn)

	// 서버 시작
	log.Printf("Server running on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
