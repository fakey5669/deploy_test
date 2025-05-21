# K8sControl API 서버 구조 개편 PRD

## 1. 개요

K8sControl 백엔드 API 서버의 구조를 기능별 API 구현에서 인프라 유형별(쿠버네티스, 온프레미스, 클라우드) 통합 API로 개편하고자 합니다. 주요 기능은 SSH 접속을 통한 명령어 실행을 기반으로 구현하며, 각 액션에 따라 적절한 명령어를 가져와 실행하는 방식으로 동작합니다.

## 2. 현재 구조

현재 API 서버는 다음과 같은 구조로 구현되어 있습니다:

- `routes.go`에서 기능별로 API 엔드포인트를 정의
  - 서비스 관련 API (`/api/v1/services/*`)
  - 인프라 관련 API (`/api/v1/infra/*`)
  - 서버(노드) 관련 API (`/api/v1/server/*`)
- 각 기능별 핸들러가 별도의 파일로 구현되어 있음
  - `service_handler.go`
  - `infra_handler.go`
  - `server_handler.go`
- SSH 접속 및 명령어 실행은 `pkg/ssh` 패키지를 통해 처리

## 3. 변경 목표

API 서버 구조를 다음과 같이 개편하고자 합니다:

### 3.1 인프라 유형별 단일 API 엔드포인트

기존의 기능별 API 구조에서 인프라 유형별 통합 API로 변경:

- `/api/v1/kubernetes` - 쿠버네티스 관련 모든 작업 처리
- `/api/v1/onpremise` - 온프레미스 환경 관련 모든 작업 처리
- `/api/v1/cloud` - 클라우드 환경 관련 모든 작업 처리

### 3.2 액션 기반 명령어 실행 시스템

각 API는 `action` 파라미터를 받아 해당 액션에 따른 적절한 명령어를 결정하고 실행:

```
POST /api/v1/kubernetes
{
  "action": "installCluster",
  "parameters": { ... },
  "target": { ... }
}
```

### 3.3 공통 SSH 실행 모듈

모든 API에서 공통으로 사용할 SSH 명령어 실행 모듈 구현:

- 명령어 템플릿 관리
- SSH 연결 및 명령어 실행
- 결과 처리 및 오류 핸들링

## 4. 상세 구현 계획

### 4.1 디렉토리 구조

```
backend/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── api/
│   │   ├── routes.go
│   │   ├── kubernetes_handler.go
│   │   ├── onpremise_handler.go
│   │   └── cloud_handler.go
│   ├── command/
│   │   ├── command_manager.go
│   │   ├── kubernetes_commands.go
│   │   ├── onpremise_commands.go
│   │   └── cloud_commands.go
│   └── utils/
│       ├── ssh_utils.go
│       └── common_utils.go
└── pkg/
    └── ssh/
        └── ssh.go
```

### 4.2 핵심 모듈 정의

#### 4.2.1 CommandManager

명령어 관리 및 실행을 담당하는 중앙 모듈:

```go
// CommandManager는 액션에 따른 명령어 실행을 관리합니다
type CommandManager struct {
    sshUtils  *utils.SSHUtils
    commandMap map[string]CommandTemplate
}

// CommandTemplate은 액션별 명령어 템플릿을 정의합니다
type CommandTemplate struct {
    Commands []string
    PrepareFunc func(params map[string]interface{}) ([]string, error)
    ValidateFunc func(params map[string]interface{}) error
}
```

#### 4.2.2 API 핸들러

각 인프라 유형별 핸들러:

```go
// KubernetesHandler는 쿠버네티스 관련 액션을 처리합니다
type KubernetesHandler struct {
    cmdManager *command.CommandManager
    db *sql.DB
}

// OnPremiseHandler는 온프레미스 환경 관련 액션을 처리합니다
type OnPremiseHandler struct {
    cmdManager *command.CommandManager
    db *sql.DB
}

// CloudHandler는 클라우드 환경 관련 액션을 처리합니다
type CloudHandler struct {
    cmdManager *command.CommandManager
    db *sql.DB
}
```

### 4.3 API 요청/응답 형식

#### 4.3.1 요청 형식

```json
{
  "action": "string",
  "parameters": {
    "key1": "value1",
    "key2": "value2"
  },
  "target": {
    "hops": [
      {
        "host": "string",
        "port": 0,
        "username": "string",
        "password": "string"
      }
    ]
  }
}
```

#### 4.3.2 응답 형식

```json
{
  "success": true,
  "result": {
    "commandResults": [
      {
        "command": "string",
        "output": "string",
        "error": "string",
        "exitCode": 0
      }
    ],
    "data": {}
  },
  "error": null
}
```

## 5. 액션 정의

### 5.1 쿠버네티스 액션 (`/api/v1/kubernetes`)

현재 구현된 기능들을 기반으로 쿠버네티스 액션을 다음과 같이 정의합니다:

- **클러스터 관리**
  - `installLoadBalancer` - 쿠버네티스용 HAProxy 로드 밸런서 설치 (기존 `infra.InstallLoadBalancer`)
  - `installFirstMaster` - 첫 번째 마스터 노드 설치 및 클러스터 초기화 (기존 `infra.InstallFirstMaster`)
  - `joinMaster` - 추가 마스터 노드를 클러스터에 조인 (기존 `infra.JoinMaster`)
  - `joinWorker` - 워커 노드를 클러스터에 조인 (기존 `infra.JoinWorker`)
  - `getNodeStatus` - 노드 상태 확인 (기존 `server.GetServerStatus`)
  - `removeNode` - 노드를 클러스터에서 제거

- **서비스 관리**
  - `getServices` - 모든 서비스 조회 (기존 `services.GetServices`)
  - `getServiceById` - ID로 서비스 조회 (기존 `services.GetServiceById`)
  - `createService` - 새 서비스 생성 (기존 `services.CreateService`)
  - `updateService` - 서비스 업데이트 (기존 `services.UpdateService`)
  - `deleteService` - 서비스 삭제 (기존 `services.DeleteService`)
  - `deployService` - 서비스 배포 (주석 처리된 `services.DeployService`)
  - `restartService` - 서비스 재시작 (주석 처리된 `services.RestartService`)
  - `stopService` - 서비스 중지 (주석 처리된 `services.StopService`)

- **인프라 관리**
  - `getInfras` - 모든 인프라 조회 (기존 `infra.GetInfras`)
  - `getInfraById` - ID로 인프라 조회 (기존 `infra.GetInfraById`)
  - `createInfra` - 새 인프라 생성 (기존 `infra.CreateInfra`)
  - `updateInfra` - 인프라 업데이트 (기존 `infra.UpdateInfra`)
  - `deleteInfra` - 인프라 삭제 (기존 `infra.DeleteInfra`)

- **서버/노드 관리**
  - `getServers` - 모든 서버 조회 (기존 `server.GetServers`)
  - `getServerById` - ID로 서버 조회 (기존 `server.GetServerById`)
  - `createServer` - 새 서버 생성 (기존 `server.CreateServer`)
  - `updateServer` - 서버 업데이트 (기존 `server.UpdateServer`)
  - `deleteServer` - 서버 삭제 (기존 `server.DeleteServer`)
  - `restartServer` - 서버 재시작 (주석 처리된 `server.RestartServer`)
  - `startServer` - 서버 시작 (주석 처리된 `server.StartServer`)
  - `stopServer` - 서버 중지 (주석 처리된 `server.StopServer`)

### 5.2 온프레미스 액션 (`/api/v1/onpremise`)

온프레미스 환경 관련 액션은 향후 구현 예정입니다. 현재는 구조만 정의합니다.

### 5.3 클라우드 액션 (`/api/v1/cloud`)

클라우드 환경 관련 액션은 향후 구현 예정입니다. 현재는 구조만 정의합니다.

## 6. 구현 로드맵

1. 명령어 관리 모듈 구현 (`CommandManager`)
2. 쿠버네티스 핸들러 구현 및 기존 기능 마이그레이션
3. 공통 유틸리티 개선 및 최적화
4. 테스트 및 문서화
5. (향후) 온프레미스 핸들러 구현
6. (향후) 클라우드 핸들러 구현

## 7. 기대 효과

- 코드 중복 감소 및 유지보수성 향상
- 새로운 액션 추가가 용이
- 명령어 템플릿 관리의 중앙화
- 인프라 유형별 기능 확장성 개선
- 에러 핸들링 일관성 향상
- API 구조의 직관성 향상
