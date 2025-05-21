import api from '../../services/api';
import { ServerStatus, ApiResponse } from '../../types';
import axios from 'axios';

// 서버 상태 확인
export const getNodeStatus = async (data: {
  id: number,
  type: string,
  hops: Array<{
    host: string,
    port: string | number,
    username: string,
    password: string
  }>
}) => {
  try {
    const response = await api.kubernetes.request<ApiResponse<{
      status: {
        installed: boolean,
        running: boolean,
        isMaster?: boolean,
        isWorker?: boolean
      },
      lastChecked: string,
      message?: string
    }>>('getNodeStatus', {
      server_id: data.id,
      type: data.type,
      hops: data.hops
    });
    
    if (!response.data || !response.data.success) {
      throw new Error(response.data?.error || '응답 데이터가 없습니다');
    }
    
    // 응답 구조 그대로 반환
    return {
      status: {
        installed: response.data.status?.installed || false,
        running: response.data.status?.running || false
      },
      lastChecked: response.data.lastChecked || '',
      isMaster: response.data.status?.isMaster,
      isWorker: response.data.status?.isWorker
    };
  } catch (error) {
    console.error('노드 상태 확인 실패:', error);
    throw error;
  }
};

// 로드밸런서(HA) 설치
export const installLoadBalancer = async (data: {
  id: number,
  hops: Array<{
    host: string,
    port: string | number,
    username: string,
    password: string
  }>
}) => {
  try {
    const response = await api.kubernetes.request<{
      message: string,
      ha_status: string,
      commandResults: Array<any>
    }>('installLoadBalancer', {
      server_id: data.id,
      hops: data.hops
    });
    
    return response.data;
  } catch (error) {
    console.error('로드밸런서 설치 실패:', error);
    throw error;
  }
};

// 첫 번째 마스터 노드 설치
export const installFirstMaster = async (data: {
  id: number,
  hops: Array<{
    host: string,
    port: string | number,
    username: string,
    password: string
  }>,
  lb_hops?: Array<{
    host: string,
    port: string | number,
    username: string,
    password: string
  }>,
  password?: string,
  lb_password?: string,
}) => {
  try {
    const response = await api.kubernetes.request<{
      success: boolean,
      message: string,
      details?: any,
      note?: string
    }>('installFirstMaster', {
      server_id: data.id,
      hops: data.hops,
      lb_hops: data.lb_hops,
      password: data.password,
      lb_password: data.lb_password
    });
    
    return response.data;
  } catch (error) {
    console.error('첫 번째 마스터 노드 설치 실패:', error);
    throw error;
  }
};

// 마스터 노드 조인
export const joinMaster = async (data: {
  id: number,
  hops: Array<{
    host: string,
    port: string | number,
    username: string,
    password: string
  }>,
  lb_hops: Array<{
    host: string,
    port: string | number,
    username: string,
    password: string
  }>,
  password: string,
  lb_password: string,
  main_id: number
}) => {
  try {
    const response = await api.kubernetes.request<{
      message: string,
      commandResults: Array<any>
    }>('joinMaster', {
      server_id: data.id,
      hops: data.hops,
      lb_hops: data.lb_hops,
      password: data.password,
      lb_password: data.lb_password,
      main_id: data.main_id
    });
    
    return response.data;
  } catch (error) {
    console.error('마스터 노드 조인 실패:', error);
    throw error;
  }
};

// 워커 노드 조인
export const joinWorker = async (data: {
  id: number,
  hops: Array<{
    host: string,
    port: string | number,
    username: string,
    password: string
  }>,
  password: string,
  main_id: number
}) => {
  try {
    const response = await api.kubernetes.request<{
      success: boolean,
      message: string,
      details?: any,
      note?: string
    }>('joinWorker', {
      server_id: data.id,
      hops: data.hops,
      password: data.password,
      main_id: data.main_id
    });
    
    return response.data;
  } catch (error) {
    console.error('워커 노드 조인 실패:', error);
    throw error;
  }
};

// 노드 제거
export const removeNode = async (data: {
  id: number,
  hops: Array<{
    host: string,
    port: string | number,
    username: string,
    password: string
  }>,
  nodeName: string
}) => {
  try {
    const response = await api.kubernetes.request<{
      message: string,
      commandResults: Array<any>
    }>('removeNode', {
      server_id: data.id,
      hops: data.hops,
      nodeName: data.nodeName
    });
    
    return response.data;
  } catch (error) {
    console.error('노드 제거 실패:', error);
    throw error;
  }
};

// 서버 시작
export const startServer = async (data: {
  id: number,
  hops: Array<{
    host: string,
    port: string | number,
    username: string,
    password: string
  }>
}) => {
  try {
    const response = await api.kubernetes.request<{
      message: string,
      commandResults: Array<any>,
      lastChecked: string
    }>('startServer', {
      server_id: data.id,
      hops: data.hops
    });
    
    return response.data;
  } catch (error) {
    console.error('서버 시작 실패:', error);
    throw error;
  }
};

// 서버 중지
export const stopServer = async (data: {
  id: number,
  hops: Array<{
    host: string,
    port: string | number,
    username: string,
    password: string
  }>
}) => {
  try {
    const response = await api.kubernetes.request<{
      message: string,
      commandResults: Array<any>,
      lastChecked: string
    }>('stopServer', {
      server_id: data.id,
      hops: data.hops
    });
    
    return response.data;
  } catch (error) {
    console.error('서버 중지 실패:', error);
    throw error;
  }
};

// 서버 재시작
export const restartServer = async (data: {
  id: number,
  hops: Array<{
    host: string,
    port: string | number,
    username: string,
    password: string
  }>
}) => {
  try {
    const response = await api.kubernetes.request<{
      message: string,
      commandResults: Array<any>,
      lastChecked: string
    }>('restartServer', {
      server_id: data.id,
      hops: data.hops
    });
    
    return response.data;
  } catch (error) {
    console.error('서버 재시작 실패:', error);
    throw error;
  }
};

// 인프라 목록 가져오기
export const getInfras = async () => {
  try {
    const response = await api.kubernetes.request<{
      infras: Array<any>
    }>('getInfras', {});
    
    return response.data;
  } catch (error) {
    console.error('인프라 목록 가져오기 실패:', error);
    throw error;
  }
};

// 특정 인프라 가져오기
export const getInfraById = async (id: number) => {
  try {
    const response = await api.kubernetes.request<{
      infra: any
    }>('getInfraById', {
      id: id
    });
    
    return response.data;
  } catch (error) {
    console.error(`인프라 ID ${id} 가져오기 실패:`, error);
    throw error;
  }
};

// 인프라 생성
export const createInfra = async (data: {
  name: string,  // 인프라 이름
  type: string,  // 인프라 유형 (kubernetes, baremetal, docker, cloud, external_kubernetes, external_docker)
  info: string // 인프라 구성 정보
}) => {
  try {
    const response = await api.kubernetes.request<{
      infra: any
    }>('createInfra', {
      name: data.name,
      type: data.type,  // 인프라 유형은 반드시 지정해야 함
      info: data.info
    });
    
    return response.data;
  } catch (error) {
    console.error('인프라 생성 실패:', error);
    throw error;
  }
};

// 인프라 업데이트
export const updateInfra = async (id: number, data: {
  name?: string,  // 인프라 이름
  type?: string,  // 인프라 유형 (kubernetes, baremetal, docker, cloud, external_kubernetes, external_docker)
  info?: string // 인프라 구성 정보
}) => {
  try {
    const response = await api.kubernetes.request<{
      infra: any
    }>('updateInfra', {
      id: id,
      name: data.name,
      type: data.type,  // 인프라 유형은 반드시 지정해야 함
      info: data.info
    });
    
    return response.data;
  } catch (error) {
    console.error(`인프라 ID ${id} 업데이트 실패:`, error);
    throw error;
  }
};

// 인프라 삭제
export const deleteInfra = async (id: number) => {
  try {
    await api.kubernetes.request<{
      message: string
    }>('deleteInfra', {
      id: id
    });
  } catch (error) {
    console.error(`인프라 ID ${id} 삭제 실패:`, error);
    throw error;
  }
};

// 서버 목록 가져오기
export const getServers = async (infraId: number) => {
  try {    
    const response = await api.kubernetes.request<{
      servers: Array<any>
    }>('getServers', {
      infra_id: infraId
    });
        
    // 응답이 있지만 서버가 없는 경우 빈 배열 반환 확인
    if (!response.data?.servers) {
      console.log('[디버그] 서버 데이터가 없습니다');
      return { servers: [] };
    }
    
    return response.data;
  } catch (error) {
    console.error('서버 목록 가져오기 실패:', error);
    throw error;
  }
};

// 특정 서버 가져오기
export const getServerById = async (id: number) => {
  try {
    const response = await api.kubernetes.request<{
      server: any
    }>('getServerById', {
      id: id
    });
    
    return response.data;
  } catch (error) {
    console.error(`서버 ID ${id} 가져오기 실패:`, error);
    throw error;
  }
};

// 서버 생성
export const createServer = async (data: {
  name: string,
  infra_id: number,
  type: string,
  ip: string,
  port: number,
  status: ServerStatus,
  hops?: Array<{
    host: string,
    port: number
  }>,
  join_command?: string,
  certificate_key?: string
}) => {
  try {
    const response = await api.kubernetes.request<{
      server: any
    }>('createServer', {
      name: data.name,
      infra_id: data.infra_id,
      type: data.type,
      ip: data.ip,
      port: data.port,
      status: data.status,
      hops: data.hops,
      join_command: data.join_command,
      certificate_key: data.certificate_key
    });
    
    return response.data;
  } catch (error) {
    console.error('서버 생성 실패:', error);
    throw error;
  }
};

// 서버 업데이트
export const updateServer = async (id: number, data: {
  name?: string,
  infra_id?: number,
  type?: string,
  hops?: any,
  join_command?: string,
  certificate_key?: string
}) => {
  try {
    const response = await api.kubernetes.request<{
      server: any
    }>('updateServer', {
      id: id,
      ...data
    });
    
    return response.data;
  } catch (error) {
    console.error(`서버 ID ${id} 업데이트 실패:`, error);
    throw error;
  }
};

// 서버 삭제
export const deleteServer = async (id: number) => {
  try {
    await api.kubernetes.request<{
      message: string
    }>('deleteServer', {
      id: id
    });
  } catch (error) {
    console.error(`서버 ID ${id} 삭제 실패:`, error);
    throw error;
  }
};

// 워커 노드 삭제
export const deleteWorker = async (data: {
  id: number,
  main_id: number,
  password: string,
  main_password: string,
  hops: Array<{
    host: string,
    port: string,
    username: string,
    password: string
  }>,
  main_hops: Array<{
    host: string,
    port: string,
    username: string,
    password: string
  }>
}) => {
  try {
    const response = await api.kubernetes.request<{
      success: boolean,
      message: string,
      details: {
        masterNodeOperations: string,
        workerNodeCleanup: string
      }
    }>('deleteWorker', {
      server_id: data.id,
      main_id: data.main_id,
      password: data.password,
      main_password: data.main_password,
      hops: data.hops,
      main_hops: data.main_hops
    });
    
    return response.data;
  } catch (error) {
    console.error('워커 노드 삭제 실패:', error);
    throw error;
  }
};

// 마스터 노드 삭제
export const deleteMaster = async (data: {
  id: number,
  password: string,
  lb_password?: string,
  main_password?: string,
  hops: Array<{
    host: string,
    port: string,
    username: string,
    password: string
  }>,
  lb_hops?: Array<{
    host: string,
    port: string,
    username: string,
    password: string
  }>,
  main_hops?: Array<{
    host: string,
    port: string,
    username: string,
    password: string
  }>
}) => {
  try {
    const response = await api.kubernetes.request<{
      success: boolean,
      message: string,
      details: {
        mainMasterCleanup: string,
        masterNodeCleanup: string,
        manualSteps: string,
        isMainMaster: boolean,
        otherMasterCount: number,
        autoCleanupPerformed: boolean,
        logFileLocation: string
      }
    }>('deleteMaster', {
      server_id: data.id,
      password: data.password,
      lb_password: data.lb_password,
      main_password: data.main_password,
      hops: data.hops,
      lb_hops: data.lb_hops,
      main_hops: data.main_hops
    });
    
    return response.data;
  } catch (error) {
    console.error('마스터 노드 삭제 실패:', error);
    throw error;
  }
};

export const getNamespaceAndPodStatus = async (data: {
  id: number,
  namespace: string,
  hops: Array<{
    host: string,
    port: string | number,
    username: string,
    password: string
  }>
}) => {
  try {
    const response = await api.kubernetes.request<any>('getNamespaceAndPodStatus', {
      id: data.id,
      namespace: data.namespace,
      hops: data.hops
    });
    
    return response.data;
  } catch (error) {
    console.error('네임스페이스 및 파드 상태 조회 실패:', error);
    throw error;
  }
};

/**
 * 쿠버네티스 배포 함수 
 * @param data 배포에 필요한 데이터
 * @returns 배포 결과
 */
export const deployKubernetes = async (data: {
  id: number,                   // 서버 ID
  repo_url: string,             // 저장소 URL
  namespace: string,            // 네임스페이스
  hops: Array<{                 // SSH 연결 정보
    host: string,
    port: number | string,
    username: string,
    password: string
  }>,
  branch?: string,              // 브랜치 (기본값: main)
  username_repo?: string,       // 저장소 인증 사용자 이름 (선택적)
  password_repo?: string        // 저장소 인증 비밀번호 (선택적)
}) => {
  try {
    const response = await api.kubernetes.request<any>('deployKubernetes', {
      id: data.id,
      repo_url: data.repo_url,
      namespace: data.namespace,
      hops: data.hops,
      branch: data.branch || 'main',
      username_repo: data.username_repo,
      password_repo: data.password_repo
    });
    
    return response.data;
  } catch (error) {
    console.error('쿠버네티스 배포 실패:', error);
    throw error;
  }
};

// 네임스페이스 삭제 API
export const deleteNamespace = async (params: {
  id: number;
  namespace: string;
  hops: Array<{
    host: string;
    port: number;
    username: string;
    password: string;
  }>;
}): Promise<{
  success: boolean;
  message?: string;
  error?: string;
  logs?: any[];
}> => {
  try {
    const response = await api.kubernetes.request<{
      success: boolean;
      message?: string;
      error?: string;
      logs?: any[];
    }>('deleteNamespace', params);
    
    if (!response.data) {
      throw new Error('네임스페이스 삭제 응답이 없습니다.');
    }
    
    return response.data.data || response.data;
  } catch (error) {
    console.error('네임스페이스 삭제 중 오류 발생:', error);
    if (error instanceof Error) {
      return {
        success: false,
        error: error.message
      };
    }
    return {
      success: false,
      error: '네임스페이스 삭제 중 알 수 없는 오류가 발생했습니다.'
    };
  }
};

// 파드 로그 조회 API
export const getPodLogs = async (params: {
  id: number;
  namespace: string;
  pod_name: string;
  lines?: number;
  hops: Array<{
    host: string;
    port: number;
    username: string;
    password: string;
  }>;
}): Promise<{
  success: boolean;
  logs?: string;
  error?: string;
  pod_exists?: boolean;
}> => {
  try {
    const response = await api.kubernetes.request<{
      success: boolean;
      logs?: string;
      error?: string;
      pod_exists?: boolean;
    }>('getPodLogs', params);
    
    if (!response.data) {
      throw new Error('파드 로그 조회 응답이 없습니다.');
    }
    
    return response.data.data || response.data;
  } catch (error) {
    console.error('파드 로그 조회 중 오류 발생:', error);
    if (error instanceof Error) {
      return {
        success: false,
        error: error.message
      };
    }
    return {
      success: false,
      error: '파드 로그 조회 중 알 수 없는 오류가 발생했습니다.'
    };
  }
};

// 파드 재시작
export const restartPod = async (params: {
  id: number;
  namespace: string;
  pod_name: string;
  hops: Array<{
    host: string;
    port: number;
    username: string;
    password: string;
  }>;
}): Promise<{
  success: boolean;
  message?: string;
  error?: string;
  logs?: any[];
}> => {
  try {
    const response = await api.kubernetes.request<{
      success: boolean;
      message?: string;
      error?: string;
      logs?: any[];
    }>('restartPod', {
      server_id: params.id,
      namespace: params.namespace,
      pod_name: params.pod_name,
      hops: params.hops
    });
    
    return response.data;
  } catch (error) {
    console.error('파드 재시작 실패:', error);
    return {
      success: false,
      error: error instanceof Error ? error.message : '파드 재시작에 실패했습니다.'
    };
  }
};

// 파드 삭제
export const deletePod = async (params: {
  id: number;
  namespace: string;
  pod_name: string;
  hops: Array<{
    host: string;
    port: number;
    username: string;
    password: string;
  }>;
}): Promise<{
  success: boolean;
  message?: string;
  error?: string;
  logs?: any[];
}> => {
  try {
    const response = await api.kubernetes.request<{
      success: boolean;
      message?: string;
      error?: string;
      logs?: any[];
    }>('deletePod', {
      server_id: params.id,
      namespace: params.namespace,
      pod_name: params.pod_name,
      hops: params.hops
    });
    
    return response.data;
  } catch (error) {
    console.error('파드 삭제 실패:', error);
    return {
      success: false,
      error: error instanceof Error ? error.message : '파드 삭제에 실패했습니다.'
    };
  }
};

// 쿠버네티스 인프라 가져오기
export const importKubernetesInfra = async (data: {
  name: string,
  type: string,
  info: string,
  hops: Array<{
    host: string,
    port: number | string,
    username: string,
    password: string
  }>
}) => {
  try {
    const response = await api.kubernetes.request<{
      success: boolean;
      infra?: any;
      message?: string;
      error?: string;
    }>('importKubernetesInfra', {
      name: data.name,
      type: data.type,
      info: data.info,
      hops: data.hops
    });
    
    return response.data;
  } catch (error) {
    console.error('쿠버네티스 인프라 가져오기 실패:', error);
    throw error;
  }
};

// 외부 쿠버네티스 노드 통계 계산
export const calculateNodes = async (data: {
  id: number,
  hops: Array<{
    host: string,
    port: string | number,
    username: string,
    password: string
  }>
}) => {
  try {
    const response = await api.kubernetes.request<{
      success: boolean,
      message: string,
      nodes: {
        total: number,
        master: number,
        worker: number,
        list: Array<any>
      }
    }>('calculateNodes', {
      server_id: data.id,
      hops: data.hops
    });
    
    if (!response.data || !response.data.success) {
      throw new Error(response.data?.error || '응답 데이터가 없습니다');
    }
    
    return response.data;
  } catch (error) {
    console.error('쿠버네티스 노드 통계 계산 실패:', error);
    throw error;
  }
};

// 서버 리소스 계산 API
export const calculateResources = async (params: { id: number; hops: Array<{
  host: string;
  port: number;
  username: string;
  password: string;
}> }) => {
  try {
    const response = await api.kubernetes.request('calculateResources', params);
    return response.data;
  } catch (error) {
    console.error('서버 리소스 계산 API 호출 오류:', error);
    throw error;
  }
};
