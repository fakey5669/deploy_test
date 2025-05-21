package db

import (
	"database/sql"
	"errors"
	"log"
	"time"
)

// ServerInfo 서버 정보 구조체
type ServerInfo struct {
	ServerName     string    `json:"server_name"`     // 서버 이름
	Hops           string    `json:"hops"`            // SSH 홉 설정 (JSON 문자열)
	JoinCommand    string    `json:"join_command"`    // 조인 명령어
	CertificateKey string    `json:"certificate_key"` // 인증서 키
	HA             string    `json:"ha"`              // HA 프록시 설정 상태 (Y/N)
	UpdatedAt      time.Time `json:"updated_at"`      // 마지막 업데이트 시간
}

type Infra struct {
	Id        int       `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Info      string    `json:"info"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GetServerInfo 서버 ID로 서버 정보를 조회
func GetServerInfo(db *sql.DB, serverID int) (*ServerInfo, error) {
	var serverInfo ServerInfo

	// NULL 값을 처리하기 위한 변수 선언
	var serverName string
	var hops string
	var joinCommand sql.NullString
	var certificateKey sql.NullString
	var ha string
	var updatedAt time.Time

	query := "SELECT server_name, hops, join_command, certificate_key, ha, updated_at FROM servers WHERE id = ?"
	log.Printf("Executing query: %s with id: %d", query, serverID)

	err := db.QueryRow(query, serverID).Scan(&serverName, &hops, &joinCommand, &certificateKey, &ha, &updatedAt)
	if err != nil {
		log.Println("Query error:", err)
		if err == sql.ErrNoRows {
			return nil, errors.New("서버 정보를 찾을 수 없습니다")
		}
		return nil, errors.New("서버 정보를 가져오는 중 오류가 발생했습니다")
	}

	// NULL 값을 안전하게 처리
	serverInfo.ServerName = serverName
	serverInfo.Hops = hops
	serverInfo.JoinCommand = ""
	if joinCommand.Valid {
		serverInfo.JoinCommand = joinCommand.String
	}
	serverInfo.CertificateKey = ""
	if certificateKey.Valid {
		serverInfo.CertificateKey = certificateKey.String
	}
	serverInfo.HA = ha
	serverInfo.UpdatedAt = updatedAt

	return &serverInfo, nil
}

// GetMasterInfo 인프라 ID와 제외할 서버 ID를 기반으로 메인 마스터 노드 정보를 조회
func GetMasterInfo(db *sql.DB, infraID int, excludeServerID int) (*ServerInfo, error) {
	var serverID int

	// 메인 마스터 노드 ID 찾기 (join_command와 certificate_key가 있는 노드)
	query := `
		SELECT id 
		FROM servers 
		WHERE infra_id = ? 
		AND id != ? 
		AND join_command IS NOT NULL 
		AND certificate_key IS NOT NULL 
		AND type LIKE '%master%'
		LIMIT 1
	`
	log.Printf("Executing query: %s with infra_id: %d, exclude_id: %d", query, infraID, excludeServerID)

	err := db.QueryRow(query, infraID, excludeServerID).Scan(&serverID)
	if err != nil {
		log.Println("Query error:", err)
		if err == sql.ErrNoRows {
			return nil, errors.New("메인 마스터 노드를 찾을 수 없습니다")
		}
		return nil, errors.New("메인 마스터 노드를 찾는 중 오류가 발생했습니다")
	}

	// 찾은 ID로 서버 정보 조회
	return GetServerInfo(db, serverID)
}

// UpdateServerHAStatus 서버 ID로 HA 상태를 Y로 업데이트
func UpdateServerHAStatus(db *sql.DB, serverID int) error {
	query := "UPDATE servers SET ha = 'Y' WHERE id = ?"
	log.Printf("Executing query: %s with id: %d", query, serverID)

	result, err := db.Exec(query, serverID)
	if err != nil {
		log.Println("Update error:", err)
		return errors.New("서버 HA 상태를 업데이트하는 중 오류가 발생했습니다")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Println("Error getting rows affected:", err)
		return errors.New("업데이트 결과를 확인하는 중 오류가 발생했습니다")
	}

	if rowsAffected == 0 {
		return errors.New("해당 ID의 서버를 찾을 수 없습니다")
	}

	return nil
}

// UpdateServerJoinCommand 서버의 join 명령어와 인증서 키를 업데이트합니다
func UpdateServerJoinCommand(db *sql.DB, serverID int, joinCommand, certificateKey string) error {
	query := `UPDATE servers SET join_command = ?, certificate_key = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	log.Printf("Executing query: %s with join_command: %s, certificate_key: %s, id: %d",
		query, joinCommand, certificateKey, serverID)

	result, err := db.Exec(query, joinCommand, certificateKey, serverID)
	if err != nil {
		log.Println("Update error:", err)
		return errors.New("서버 join 명령어 업데이트 중 오류가 발생했습니다")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Println("Error getting rows affected:", err)
		return errors.New("업데이트 결과를 확인하는 중 오류가 발생했습니다")
	}

	if rowsAffected == 0 {
		return errors.New("해당 ID의 서버를 찾을 수 없습니다")
	}

	return nil
}

func GetAllInfras(db *sql.DB) ([]Infra, error) {
	query := `
		SELECT id, name, type, info, created_at, updated_at 
		FROM infras
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var infras []Infra
	for rows.Next() {
		var infra Infra
		err := rows.Scan(
			&infra.Id,
			&infra.Name,
			&infra.Type,
			&infra.Info,
			&infra.CreatedAt,
			&infra.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		infras = append(infras, infra)
	}

	return infras, nil
}

func GetInfraById(db *sql.DB, id int) (Infra, error) {
	query := `
		SELECT id, name, type, info, created_at, updated_at
		FROM infras
		Where id = ?
	`

	var infra Infra
	err := db.QueryRow(query, id).Scan(
		&infra.Id,
		&infra.Name,
		&infra.Type,
		&infra.Info,
		&infra.CreatedAt,
		&infra.UpdatedAt,
	)

	return infra, err
}

func CreateInfra(db *sql.DB, infra Infra) (int, error) {
	query := `
		INSERT INTO infras (name, type, info) 
		values (?, ?, ?)
	`

	result, err := db.Exec(query, infra.Name, infra.Type, infra.Info)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	return int(id), err

}

func UpdateInfra(db *sql.DB, infra Infra) error {
	query := `
		UPDATE infras 
		SET name = ?, type = ?, info = ?
		WHERE id = ?
	`
	_, err := db.Exec(query, infra.Name, infra.Type, infra.Info, infra.Id)

	return err
}

func DeleteInfra(db *sql.DB, id int) error {
	query := `DELETE FROM infras WHERE id = ?`

	_, err := db.Exec(query, id)
	return err
}

func DeleteWorker(db *sql.DB, id int) error {
	query := `DELETE FROM servers WHERE id = ?`

	_, err := db.Exec(query, id)
	return err
}

func DeleteMaster(db *sql.DB, id int) error {
	query := `DELETE FROM servers WHERE id = ?`

	_, err := db.Exec(query, id)
	return err
}
