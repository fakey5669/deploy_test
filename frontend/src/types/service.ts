// 서비스 데이터 타입 정의
export interface Service {
  id: number;
  name: string;
  status: string;
  domain: string;
  namespace?: string;
  gitlab_url: string | null;
  gitlab_id?: string | null;
  gitlab_password?: string | null; // GitLab 계정 비밀번호
  gitlab_token?: string | null; // GitLab Private Token
  gitlab_branch?: string | null;
  infra_id?: number | null;
  infraName?: string;
  user_id?: number;
  created_at: string;
  updated_at: string;
  loadingStatus?: boolean; // 상태 조회 로딩 여부
  namespaceStatus?: string; // 네임스페이스 상태 (Active, Not Found 등)
  podsStatus?: Array<{ name: string; status: string; ready: boolean; restarts: number }>; // 파드 상태 목록
  runningPods?: number; // 실행 중인 파드 수
  totalPods?: number; // 전체 파드 수
}

// 서비스 그룹 타입 정의
export interface ServiceGroup {
  id: string;
  name: string;
  description: string;
  created_at: string;
  updated_at: string;
}

// 쿠버네티스 상태 인터페이스
export interface KubernetesStatus {
  namespace: {
    name: string;
    status: string;
  };
  pods: {
    name: string;
    status: string;
    ready: boolean;
    restarts: number;
  }[];
}

// 서비스 운영 상태 인터페이스
export interface ServiceStatus {
  status: 'running' | 'stopped' | 'error';
  message?: string;
}

// 서비스 생성 요청 데이터
export interface CreateServiceRequest {
  name: string;
  status: string;
  domain: string;
  namespace?: string;
  gitlab_url?: string | null;
  gitlab_id?: string | null;
  gitlab_password?: string | null;
  gitlab_token?: string | null;
  gitlab_branch?: string | null;
  infra_id?: number | null;
  user_id?: number;
}

// 서비스 업데이트 요청 데이터
export interface UpdateServiceRequest {
  name?: string;
  status?: string;
  domain?: string;
  namespace?: string;
  gitlab_url?: string | null;
  gitlab_id?: string | null;
  gitlab_password?: string | null;
  gitlab_token?: string | null;
  gitlab_branch?: string | null;
  infra_id?: number | null;
  user_id?: number;
} 