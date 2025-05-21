// 서버 상태 타입
export type ServerStatus = 'running' | 'stopped' | 'maintenance' | 'preparing' | '등록' | 'checking';

// 서버 타입 (여러 값 가능)
export type ServerType = 'master' | 'worker' | 'ha' | string;

// 서버 데이터 타입 정의
export interface ServerItem {
  id: number;
  server_name?: string;
  hops: string;
  join_command: string;
  certificate_key: string;
  type: string; // 쉼표로 구분된 여러 타입 값 (예: "master,ha")
  infra_id: number;
  ha: string; // 'Y' 또는 'N'
  created_at: string;
  updated_at: string;
  status?: ServerStatus; // 프론트엔드에서만 사용하는 런타임 상태
  last_checked?: string; // 마지막 상태 확인 시간
}

// 서버 생성/업데이트 입력 데이터
export interface ServerInput {
  server_name?: string;
  hops?: string;
  ip?: string;
  port?: number;
  join_command?: string;
  certificate_key?: string;
  type: string; // 쉼표로 구분된 여러 타입 값 (예: "master,ha")
  infra_id: number;
  ha?: string; // 'Y' 또는 'N'
  status?: ServerStatus; // 서버 상태
}

// DB에 저장되는 서버 타입 (status 필드 제외)
export type ServerDbItem = Omit<ServerItem, 'status'>; 