package db

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

// 환경 확인 함수
func isProduction() bool {
	env := strings.ToLower(os.Getenv("APP_ENV"))
	return env == "production" || env == "prod"
}

// Connect establishes a connection to the MariaDB database
func Connect() (*sql.DB, error) {
	var dbUser, dbPassword, dbHost, dbPort, dbName string

	// 프로덕션 환경인지 확인
	if isProduction() {
		// 프로덕션 환경에서의 하드코딩된 값
		dbUser = "lw"
		dbPassword = "line9876"
		dbHost = "k8scontrol-db"
		dbPort = "3306"
		dbName = "k8scontrol"
	} else {
		// 개발 환경에서의 하드코딩된 값
		dbUser = "lw"
		dbPassword = "line9876"
		dbHost = "lineworldap.iptime.org"
		dbPort = "4445"
		dbName = "k8scontrol"
	}

	// 데이터베이스 연결 문자열 생성
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", dbUser, dbPassword, dbHost, dbPort, dbName)

	// 데이터베이스 연결
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	// 연결 테스트
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

// 환경 변수 가져오기 (기본값 지원)
func getEnvWithDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
