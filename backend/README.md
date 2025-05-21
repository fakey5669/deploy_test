backend/
  ├── cmd/                   # 실행 가능한 애플리케이션 진입점
  │   └── server/            # 서버 애플리케이션
  │       └── main.go        # 메인 애플리케이션 코드
  ├── internal/              # 내부 패키지 (외부에서 가져다 쓸 수 없음)
  │   ├── api/               # API 핸들러와 라우트
  │   │   ├── routes.go      # API 라우트 설정
  │   │   └── service_handler.go # 서비스 관련 API 핸들러
  │   ├── config/            # 설정 관련 코드
  │   ├── db/                # 데이터베이스 관련 코드
  │   │   ├── db.go          # 데이터베이스 연결 관리
  │   │   └── service.go     # 서비스 모델 및 DB 작업
  │   ├── middleware/        # 미들웨어 코드
  │   ├── service/           # 비즈니스 로직 서비스
  │   └── utils/             # 유틸리티 함수
  │       ├── ssh_utils.go   # SSH 유틸리티 래퍼
  │       └── example_usage.go # SSH 유틸리티 사용 예제
  └── pkg/                   # 외부에서 가져다 쓸 수 있는 패키지
      └── ssh/               # SSH 핵심 기능
          ├── ssh.go         # SSH 구현 코드
          └── README.md      # SSH 패키지 사용 문서