package db

import (
	"database/sql"
	"encoding/json"
)

// Service 모델 정의
type Service struct {
	ID             int64          `json:"id"`
	Name           string         `json:"name"`
	Domain         sql.NullString `json:"domain"`
	Namespace      sql.NullString `json:"namespace"`
	GitlabURL      sql.NullString `json:"gitlab_url"`
	GitlabID       sql.NullString `json:"gitlab_id"`
	GitlabPassword sql.NullString `json:"gitlab_password"`
	GitlabToken    sql.NullString `json:"gitlab_token"`
	GitlabBranch   sql.NullString `json:"gitlab_branch"`
	UserID         int64          `json:"user_id"`
	InfraID        sql.NullInt64  `json:"infra_id"`
	CreatedAt      string         `json:"created_at"`
	UpdatedAt      string         `json:"updated_at"`
}

// MarshalJSON은 Service 구조체의 JSON 직렬화를 위한 메서드입니다.
// sql.NullString 필드를 일반 string으로 변환합니다.
func (s Service) MarshalJSON() ([]byte, error) {
	type Alias Service // 재귀 호출 방지를 위한 타입 별칭

	// 일반 문자열로 변환된 필드를 가진 임시 구조체
	return json.Marshal(&struct {
		Domain         string `json:"domain"`
		Namespace      string `json:"namespace"`
		GitlabURL      string `json:"gitlab_url"`
		GitlabID       string `json:"gitlab_id"`
		GitlabPassword string `json:"gitlab_password"`
		GitlabToken    string `json:"gitlab_token"`
		GitlabBranch   string `json:"gitlab_branch"`
		InfraID        int64  `json:"infra_id,omitempty"`
		*Alias
	}{
		Domain:         stringFromNullString(s.Domain),
		Namespace:      stringFromNullString(s.Namespace),
		GitlabURL:      stringFromNullString(s.GitlabURL),
		GitlabID:       stringFromNullString(s.GitlabID),
		GitlabPassword: stringFromNullString(s.GitlabPassword),
		GitlabToken:    stringFromNullString(s.GitlabToken),
		GitlabBranch:   stringFromNullString(s.GitlabBranch),
		InfraID:        nullInt64ToInt64(s.InfraID),
		Alias:          (*Alias)(&s),
	})
}

// stringFromNullString은 sql.NullString을 일반 string으로 변환합니다.
func stringFromNullString(n sql.NullString) string {
	if n.Valid {
		return n.String
	}
	return ""
}

// nullInt64ToInt64는 sql.NullInt64를 일반 int64로 변환합니다.
func nullInt64ToInt64(n sql.NullInt64) int64 {
	if n.Valid {
		return n.Int64
	}
	return 0
}

// GetAllServices 모든 서비스 조회
func GetAllServices(db *sql.DB) ([]Service, error) {
	query := `
		SELECT id, name, domain, namespace, gitlab_url, gitlab_id, gitlab_password, gitlab_token, gitlab_branch, user_id, infra_id, created_at, updated_at 
		FROM services
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []Service
	for rows.Next() {
		var service Service
		err := rows.Scan(
			&service.ID,
			&service.Name,
			&service.Domain,
			&service.Namespace,
			&service.GitlabURL,
			&service.GitlabID,
			&service.GitlabPassword,
			&service.GitlabToken,
			&service.GitlabBranch,
			&service.UserID,
			&service.InfraID,
			&service.CreatedAt,
			&service.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}

	return services, nil
}

// GetServiceByID ID로 서비스 조회
func GetServiceByID(db *sql.DB, id int) (Service, error) {
	query := `
		SELECT id, name, domain, namespace, gitlab_url, gitlab_id, gitlab_password, gitlab_token, gitlab_branch, user_id, infra_id, created_at, updated_at 
		FROM services 
		WHERE id = ?
	`

	var service Service
	err := db.QueryRow(query, id).Scan(
		&service.ID,
		&service.Name,
		&service.Domain,
		&service.Namespace,
		&service.GitlabURL,
		&service.GitlabID,
		&service.GitlabPassword,
		&service.GitlabToken,
		&service.GitlabBranch,
		&service.UserID,
		&service.InfraID,
		&service.CreatedAt,
		&service.UpdatedAt,
	)

	return service, err
}

// CreateService 새 서비스 생성
func CreateService(db *sql.DB, service Service) (int, error) {
	query := `
		INSERT INTO services (name, domain, namespace, gitlab_url, gitlab_id, gitlab_password, gitlab_token, gitlab_branch, user_id, infra_id) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := db.Exec(query,
		service.Name,
		service.Domain,
		service.Namespace,
		service.GitlabURL,
		service.GitlabID,
		service.GitlabPassword,
		service.GitlabToken,
		service.GitlabBranch,
		service.UserID,
		service.InfraID,
	)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	return int(id), err
}

// UpdateService 서비스 업데이트
func UpdateService(db *sql.DB, service Service) error {
	query := `		UPDATE services 
		SET name = ?, domain = ?, namespace = ?, gitlab_url = ?, gitlab_id = ?, gitlab_password = ?, gitlab_token = ?, gitlab_branch = ?, user_id = ?, infra_id = ? 
		WHERE id = ?
	`

	_, err := db.Exec(query,
		service.Name,
		service.Domain,
		service.Namespace,
		service.GitlabURL,
		service.GitlabID,
		service.GitlabPassword,
		service.GitlabToken,
		service.GitlabBranch,
		service.UserID,
		service.InfraID,
		service.ID,
	)

	return err
}

// DeleteService 서비스 삭제
func DeleteService(db *sql.DB, id int) error {
	query := `DELETE FROM services WHERE id = ?`

	_, err := db.Exec(query, id)
	return err
}
