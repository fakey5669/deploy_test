// 인프라 상태 타입
export type InfraStatus = 'active' | 'inactive';

// 인프라 데이터 타입 정의
export interface InfraItem {
  id: number;
  name: string;
  type: 'kubernetes' | 'cloud' | 'baremetal' | 'docker' | 'external_kubernetes' | 'external_docker';
  info: string;
  created_at: string;
  updated_at: string;
  status?: InfraStatus; // DB에는 없고 런타임에 결정되는 필드
}

// 인프라 노드 데이터 타입 정의 (인프라 세부 설정에서 사용)
export interface InfraNodeItem {
  id: number;
  infra_id: number;  // 상위 인프라 ID (외래 키)
  name: string;
  role: string;
  status: 'active' | 'inactive';
  ip_address: string;
  os?: string;
  created_at: string;
  updated_at: string;
}

// 쿠버네티스 관련 추가 정보
export interface KubernetesConfig {
  id: number;
  infraId: number;  // 상위 인프라 ID (외래 키)
  version: string;
  networkPlugin?: string;
  storageClass?: string;
  ingressController?: string;
}

// 온프레미스 관련 추가 정보
export interface OnPremiseConfig {
  id: number;
  infraId: number;  // 상위 인프라 ID (외래 키)
  dataCenter?: string;
  rackLocation?: string;
  powerConsumption?: number;
}

// 클라우드 관련 추가 정보
export interface CloudConfig {
  id: number;
  infraId: number;  // 상위 인프라 ID (외래 키)
  provider: 'aws' | 'gcp' | 'azure' | 'other';
  region?: string;
  accountId?: string;
  billingType?: 'ondemand' | 'reserved' | 'spot';
} 