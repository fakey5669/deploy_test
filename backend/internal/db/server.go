package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Server 모델 정의
type Server struct {
	ID             int        `json:"id"`
	ServerName     string     `json:"server_name"`
	Hops           string     `json:"hops"`
	JoinCommand    string     `json:"join_command"`
	CertificateKey string     `json:"certificate_key"`
	Type           string     `json:"type"` // 'master', 'worker', 'ha', 또는 복합값 (예: 'master,ha')
	InfraID        int        `json:"infra_id"`
	Ha             string     `json:"ha,omitempty"`           // 'Y' or 'N', 클라이언트 호환성 유지용
	LastChecked    *time.Time `json:"last_checked,omitempty"` // 마지막 상태 확인 시간, NULL 허용
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// ServerInput 서버 생성/수정 입력 모델
type ServerInput struct {
	ServerName     string `json:"server_name"`
	Hops           string `json:"hops"`
	JoinCommand    string `json:"join_command"`
	CertificateKey string `json:"certificate_key"`
	Type           string `json:"type" binding:"required"` // 'master', 'worker', 'ha' 또는 복합값
	InfraID        int    `json:"infra_id" binding:"required"`
}

// GetAllServers 모든 서버 조회
func GetAllServers(db *sql.DB) ([]Server, error) {
	query := `
		SELECT id, server_name, hops, join_command, certificate_key, type, infra_id, ha, last_checked, created_at, updated_at 
		FROM servers
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []Server
	for rows.Next() {
		var server Server
		var serverNameNull sql.NullString
		var joinCommandNull sql.NullString
		var certificateKeyNull sql.NullString
		var lastCheckedNull sql.NullTime

		err := rows.Scan(
			&server.ID,
			&serverNameNull,
			&server.Hops,
			&joinCommandNull,
			&certificateKeyNull,
			&server.Type,
			&server.InfraID,
			&server.Ha,
			&lastCheckedNull,
			&server.CreatedAt,
			&server.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// NULL 값 처리
		if serverNameNull.Valid {
			server.ServerName = serverNameNull.String
		} else {
			server.ServerName = ""
		}

		if joinCommandNull.Valid {
			server.JoinCommand = joinCommandNull.String
		} else {
			server.JoinCommand = ""
		}

		if certificateKeyNull.Valid {
			server.CertificateKey = certificateKeyNull.String
		} else {
			server.CertificateKey = ""
		}

		// LastChecked NULL 값 처리
		if lastCheckedNull.Valid {
			server.LastChecked = &lastCheckedNull.Time
		} else {
			server.LastChecked = nil // NULL 값이면 nil로 설정
		}

		servers = append(servers, server)
	}

	return servers, nil
}

// GetServersByInfraID 인프라 ID로 서버 목록 조회
func GetServersByInfraID(db *sql.DB, infraID int) ([]Server, error) {
	query := `
		SELECT id, server_name, hops, join_command, certificate_key, type, infra_id, ha, last_checked, created_at, updated_at 
		FROM servers
		WHERE infra_id = ?
	`

	rows, err := db.Query(query, infraID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []Server
	for rows.Next() {
		var server Server
		var serverNameNull sql.NullString
		var joinCommandNull sql.NullString
		var certificateKeyNull sql.NullString
		var lastCheckedNull sql.NullTime

		err := rows.Scan(
			&server.ID,
			&serverNameNull,
			&server.Hops,
			&joinCommandNull,
			&certificateKeyNull,
			&server.Type,
			&server.InfraID,
			&server.Ha,
			&lastCheckedNull,
			&server.CreatedAt,
			&server.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// NULL 값 처리
		if serverNameNull.Valid {
			server.ServerName = serverNameNull.String
		} else {
			server.ServerName = ""
		}

		if joinCommandNull.Valid {
			server.JoinCommand = joinCommandNull.String
		} else {
			server.JoinCommand = ""
		}

		if certificateKeyNull.Valid {
			server.CertificateKey = certificateKeyNull.String
		} else {
			server.CertificateKey = ""
		}

		// LastChecked NULL 값 처리
		if lastCheckedNull.Valid {
			server.LastChecked = &lastCheckedNull.Time
		} else {
			server.LastChecked = nil // NULL 값이면 nil로 설정
		}

		servers = append(servers, server)
	}

	return servers, nil
}

// GetServerByID ID로 서버 조회
func GetServerByID(db *sql.DB, id int) (Server, error) {
	query := `
		SELECT id, server_name, hops, join_command, certificate_key, type, infra_id, ha, last_checked, created_at, updated_at 
		FROM servers 
		WHERE id = ?
	`

	var server Server
	var serverNameNull sql.NullString
	var joinCommandNull sql.NullString
	var certificateKeyNull sql.NullString
	var lastCheckedNull sql.NullTime

	err := db.QueryRow(query, id).Scan(
		&server.ID,
		&serverNameNull,
		&server.Hops,
		&joinCommandNull,
		&certificateKeyNull,
		&server.Type,
		&server.InfraID,
		&server.Ha,
		&lastCheckedNull,
		&server.CreatedAt,
		&server.UpdatedAt,
	)

	if err == nil {
		// NULL 값 처리
		if serverNameNull.Valid {
			server.ServerName = serverNameNull.String
		} else {
			server.ServerName = ""
		}

		if joinCommandNull.Valid {
			server.JoinCommand = joinCommandNull.String
		} else {
			server.JoinCommand = ""
		}

		if certificateKeyNull.Valid {
			server.CertificateKey = certificateKeyNull.String
		} else {
			server.CertificateKey = ""
		}

		// LastChecked NULL 값 처리
		if lastCheckedNull.Valid {
			server.LastChecked = &lastCheckedNull.Time
		} else {
			server.LastChecked = nil // NULL 값이면 nil로 설정
		}
	}

	return server, err
}

// GetServersByType 타입으로 서버 목록 조회 (예: 'master', 'ha' 등)
func GetServersByType(db *sql.DB, serverType string) ([]Server, error) {
	// 타입 검색을 위한 LIKE 쿼리
	// 'master'로 검색하면 'master' 또는 'master,ha' 모두 매치되어야 함
	query := `
		SELECT id, server_name, hops, join_command, certificate_key, type, infra_id, ha, last_checked, created_at, updated_at 
		FROM servers
		WHERE type LIKE ? OR type LIKE ? OR type LIKE ?
	`

	rows, err := db.Query(query,
		serverType,
		serverType+",%",
		"%,"+serverType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []Server
	for rows.Next() {
		var server Server
		var serverNameNull sql.NullString
		var joinCommandNull sql.NullString
		var certificateKeyNull sql.NullString
		var lastCheckedNull sql.NullTime

		err := rows.Scan(
			&server.ID,
			&serverNameNull,
			&server.Hops,
			&joinCommandNull,
			&certificateKeyNull,
			&server.Type,
			&server.InfraID,
			&server.Ha,
			&lastCheckedNull,
			&server.CreatedAt,
			&server.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// NULL 값 처리
		if serverNameNull.Valid {
			server.ServerName = serverNameNull.String
		} else {
			server.ServerName = ""
		}

		if joinCommandNull.Valid {
			server.JoinCommand = joinCommandNull.String
		} else {
			server.JoinCommand = ""
		}

		if certificateKeyNull.Valid {
			server.CertificateKey = certificateKeyNull.String
		} else {
			server.CertificateKey = ""
		}

		// LastChecked NULL 값 처리
		if lastCheckedNull.Valid {
			server.LastChecked = &lastCheckedNull.Time
		} else {
			server.LastChecked = nil // NULL 값이면 nil로 설정
		}

		servers = append(servers, server)
	}

	return servers, nil
}

// GetServerByHopsAndInfraID Hops와 인프라 ID로 서버 조회
func GetServerByHopsAndInfraID(db *sql.DB, hops string, infraID int) (Server, error) {
	query := `
		SELECT id, server_name, hops, join_command, certificate_key, type, infra_id, ha, last_checked, created_at, updated_at 
		FROM servers 
		WHERE hops = ? AND infra_id = ?
	`

	var server Server
	var serverNameNull sql.NullString
	var joinCommandNull sql.NullString
	var certificateKeyNull sql.NullString
	var lastCheckedNull sql.NullTime

	err := db.QueryRow(query, hops, infraID).Scan(
		&server.ID,
		&serverNameNull,
		&server.Hops,
		&joinCommandNull,
		&certificateKeyNull,
		&server.Type,
		&server.InfraID,
		&server.Ha,
		&lastCheckedNull,
		&server.CreatedAt,
		&server.UpdatedAt,
	)

	if err == nil {
		// NULL 값 처리
		if serverNameNull.Valid {
			server.ServerName = serverNameNull.String
		} else {
			server.ServerName = ""
		}

		if joinCommandNull.Valid {
			server.JoinCommand = joinCommandNull.String
		} else {
			server.JoinCommand = ""
		}

		if certificateKeyNull.Valid {
			server.CertificateKey = certificateKeyNull.String
		} else {
			server.CertificateKey = ""
		}

		// LastChecked NULL 값 처리
		if lastCheckedNull.Valid {
			server.LastChecked = &lastCheckedNull.Time
		} else {
			server.LastChecked = nil // NULL 값이면 nil로 설정
		}
	}

	return server, err
}

// CreateServer 새 서버 생성
func CreateServer(db *sql.DB, input ServerInput) (int, error) {
	// 동일한 hops와 인프라 ID를 가진 서버가 이미 존재하는지 확인
	existingServer, err := GetServerByHopsAndInfraID(db, input.Hops, input.InfraID)
	if err == nil {
		// 서버가 이미 존재하면 타입만 업데이트
		if err := UpdateServer(db, existingServer.ID, input); err != nil {
			return 0, err
		}
		return existingServer.ID, nil
	} else if err != sql.ErrNoRows {
		// sql.ErrNoRows가 아닌 다른 오류가 발생한 경우
		return 0, err
	}

	// 서버가 존재하지 않으면 새로 생성
	// 필요한 컬럼만 명시하고 나머지는 기본값 사용
	query := `
		INSERT INTO servers (server_name, hops, type, infra_id) 
		VALUES (?, ?, ?, ?)
	`

	// 프론트엔드에서 전송한 값을 그대로 사용
	result, err := db.Exec(query,
		input.ServerName,
		input.Hops,
		input.Type,
		input.InfraID,
	)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	return int(id), err
}

// UpdateServer 서버 업데이트
func UpdateServer(db *sql.DB, id int, input ServerInput) error {
	// 기존 서버 데이터를 가져와서 빈 필드만 업데이트
	existingServer, err := GetServerByID(db, id)
	if err != nil {
		return err
	}

	// 입력값에서 빈 값이 아닌 필드만 업데이트
	serverName := input.ServerName
	if serverName == "" {
		serverName = existingServer.ServerName
	}

	hops := input.Hops
	if hops == "" {
		hops = existingServer.Hops
	}

	typeValue := input.Type
	if typeValue == "" {
		typeValue = existingServer.Type
	}

	joinCommand := input.JoinCommand
	if joinCommand == "" {
		joinCommand = existingServer.JoinCommand
	}

	certificateKey := input.CertificateKey
	if certificateKey == "" {
		certificateKey = existingServer.CertificateKey
	}

	// 인프라 ID는 0이 아닐 때만 업데이트 (0은 기본값으로 간주)
	infraID := input.InfraID
	if infraID == 0 {
		infraID = existingServer.InfraID
	}

	// 필요한 컬럼만 업데이트
	query := `
		UPDATE servers 
		SET server_name = ?, hops = ?, type = ?, infra_id = ?, join_command = ?, certificate_key = ?
		WHERE id = ?
	`

	// 기존 값과 새 값을 조합하여 업데이트
	_, err = db.Exec(query,
		serverName,
		hops,
		typeValue,
		infraID,
		joinCommand,
		certificateKey,
		id,
	)

	return err
}

// DeleteServer 서버 삭제
func DeleteServer(db *sql.DB, id int) error {
	query := `DELETE FROM servers WHERE id = ?`

	_, err := db.Exec(query, id)
	return err
}

// UpdateServerLastChecked 서버의 마지막 확인 시간 업데이트
func UpdateServerLastChecked(db *sql.DB, serverID int, lastChecked time.Time) error {
	query := "UPDATE servers SET last_checked = ? WHERE id = ?"
	_, err := db.Exec(query, lastChecked, serverID)
	return err
}

// UpdateServerHaStatus 서버의 HA 상태를 업데이트합니다.
func UpdateServerHaStatus(db *sql.DB, serverID int, haStatus string) error {
	query := "UPDATE servers SET ha = ? WHERE id = ?"

	_, err := db.Exec(query, haStatus, serverID)
	if err != nil {
		return fmt.Errorf("서버 HA 상태 업데이트 실패: %v", err)
	}

	return nil
}
