import api from '../../services/api';
import { ApiResponse } from '../../types';

// 도커 서버 정보 조회
export const getDockerServer = async (infraId: number) => {
  try {
    const response = await api.docker.request<ApiResponse<{
      server: any
    }>>("getDockerServer", {
      infra_id: infraId
    });
    
    if (!response.data.success) {
      throw new Error(response.data.error || "도커 서버 정보를 가져오는데 실패했습니다.");
    }
    
    return response.data;
  } catch (error) {
    console.error('도커 서버 정보 조회 실패:', error);
    // 서버 조회 실패 시 빈 데이터 반환
    return {
      success: true,
      server: null
    };
  }
};

// 도커 서버 생성
export const createDockerServer = async (data: {
  name: string;
  infra_id: number;
  ip: string;
  port?: number;
  status?: string;
  hops?: Array<{
    host: string;
    port: number;
  }>;
}) => {
  try {
    const response = await api.docker.request<ApiResponse<{
      server: any;
      id: number;
      message: string;
    }>>("createDockerServer", data);
    
    if (!response.data.success) {
      throw new Error(response.data.error || "도커 서버 생성에 실패했습니다.");
    }
    
    return response.data;
  } catch (error) {
    console.error('도커 서버 생성 실패:', error);
    throw error;
  }
};

// 도커 정보 조회
export const getDockerInfo = async (serverId: number) => {
  try {
    const response = await api.docker.request<ApiResponse<{
      info: any,
      server: any
    }>>("getDockerInfo", {
      server_id: serverId
    });
        
    if (!response.data.success) {
      throw new Error(response.data.error || "도커 정보를 가져오는데 실패했습니다.");
    }
    
    return response.data;
  } catch (error) {
    console.error('도커 정보 조회 실패:', error);
    throw error;
  }
};

// 컨테이너 목록 조회
export const getContainers = async (serverId: number, authInfo?: {
  username?: string;
  password?: string;
  hops?: Array<{
    host: string;
    port: number;
    username: string;
    password: string;
  }>;
}) => {
  try {
    console.log(`[디버그] 도커 서버 ID ${serverId}의 컨테이너 목록 요청`);
    console.log(`[디버그] authInfo:`, JSON.stringify(authInfo, null, 2));
    
    const requestData: any = {
      server_id: serverId
    };
    
    // 인증 정보가 제공된 경우 hops만 요청에 포함
    if (authInfo && authInfo.hops) {
      requestData.hops = authInfo.hops;
      console.log(`[디버그] hops 데이터:`, JSON.stringify(authInfo.hops, null, 2));
    }
    
    console.log(`[디버그] 최종 요청 데이터:`, JSON.stringify(requestData, null, 2));
    
    const response = await api.docker.request<ApiResponse<{
      containers: Array<{
        id: string;
        image: string;
        status: string;
        name: string;
        ports: string;
        size: string;
        created: string;
      }>;
      images: Array<{
        repository: string;
        tag: string;
        size: string;
        created: string;
      }>;
      networks: Array<{
        id: string;
        name: string;
        driver: string;
        scope: string;
      }>;
      volumes: Array<{
        name: string;
        driver: string;
        size: string;
      }>;
      container_count: number;
      image_count: number;
    }>>("getContainers", requestData);
    
    console.log(`[디버그] 도커 컨테이너 목록 응답:`, JSON.stringify(response.data, null, 2));
    
    if (!response.data.success) {
      console.error(`[디버그] 응답 오류:`, response.data.error);
      throw new Error(response.data.error || "컨테이너 목록을 가져오는데 실패했습니다.");
    }
    
    // 컨테이너 배열이 없는 경우 빈 배열로 처리
    const containerList = response.data.containers || [];
    console.log(`[디버그] 컨테이너 수:`, containerList.length);
    
    // 백엔드에서 받은 컨테이너 데이터를 프론트엔드에서 사용할 형식으로 변환
    const containers = containerList.map((container: {
      id: string;
      image: string;
      status: string;
      name: string;
      ports: string;
      size: string;
      created: string;
    }) => {
      // 상태 문자열에서 실제 상태 추출 (예: "Up 3 hours" => "running")
      let statusText = container.status.toLowerCase();
      let status: 'running' | 'stopped' | 'paused' | 'exited' = 'stopped';
      
      if (statusText.includes('up')) {
        status = 'running';
      } else if (statusText.includes('paused')) {
        status = 'paused';
      } else if (statusText.includes('exited')) {
        status = 'exited';
      }
      
      // 포트 정보를 배열로 파싱
      const ports = container.ports
        ? container.ports.split(',').map((p: string) => p.trim()).filter((p: string) => p !== '')
        : [];
        
      return {
        id: container.id,
        name: container.name,
        image: container.image,
        status,
        created: container.created,
        ports,
        size: container.size
      };
    });
    
    // images, networks, volumes 등 다른 정보도 반환
    return {
      success: response.data.success,
      containers,
      images: response.data.images || [],
      networks: response.data.networks || [],
      volumes: response.data.volumes || [],
      container_count: response.data.container_count || containers.length,
      image_count: response.data.image_count || (response.data.images ? response.data.images.length : 0)
    };
  } catch (error) {
    console.error('컨테이너 목록 조회 실패:', error);
    throw error;
  }
};

// 도커 서버 업데이트
export const updateDockerServer = async (data: {
  id: number,
  name?: string,
  ip?: string,
  port?: number,
  status?: string
}) => {
  try {
    const response = await api.docker.request<ApiResponse<{
      server: any
    }>>("updateDockerServer", data);
        
    if (!response.data.success) {
      throw new Error(response.data.error || "도커 서버 업데이트에 실패했습니다.");
    }
    
    return response.data;
  } catch (error) {
    console.error('도커 서버 업데이트 실패:', error);
    throw error;
  }
};

// 컨테이너 시작
export const startContainer = async (serverId: number, containerId: string) => {
  try {
    const response = await api.docker.request<ApiResponse<{
      message: string
    }>>("startContainer", {
      server_id: serverId,
      container_id: containerId
    });
        
    if (!response.data.success) {
      throw new Error(response.data.error || "컨테이너 시작에 실패했습니다.");
    }
    
    return response.data;
  } catch (error) {
    console.error('컨테이너 시작 실패:', error);
    throw error;
  }
};

// 컨테이너 중지
export const stopContainer = async (serverId: number, containerId: string, authInfo?: {
  username?: string;
  password?: string;
  hops?: Array<{
    host: string;
    port: number;
    username: string;
    password: string;
  }>;
}) => {
  try {
    const requestData: any = {
      server_id: serverId,
      container_id: containerId,
      action_type: 'stop'
    };
    
    // 인증 정보가 제공된 경우 요청에 포함
    if (authInfo) {
      if (authInfo.username) requestData.username = authInfo.username;
      if (authInfo.password) requestData.password = authInfo.password;
      if (authInfo.hops) requestData.hops = authInfo.hops;
    }
    
    const response = await api.docker.request<ApiResponse<{
      message: string;
      logs?: any[];
      container_id: string;
      action_type: string;
    }>>("controlContainer", requestData);
        
    if (!response.data.success) {
      throw new Error(response.data.error || "컨테이너 중지에 실패했습니다.");
    }
    
    return response.data;
  } catch (error) {
    console.error('컨테이너 중지 실패:', error);
    throw error;
  }
};

// 컨테이너 재시작
export const restartContainer = async (serverId: number, containerId: string, authInfo?: {
  username?: string;
  password?: string;
  hops?: Array<{
    host: string;
    port: number;
    username: string;
    password: string;
  }>;
}) => {
  try {
    const requestData: any = {
      server_id: serverId,
      container_id: containerId,
      action_type: 'restart'
    };
    
    // 인증 정보가 제공된 경우 요청에 포함
    if (authInfo) {
      if (authInfo.username) requestData.username = authInfo.username;
      if (authInfo.password) requestData.password = authInfo.password;
      if (authInfo.hops) requestData.hops = authInfo.hops;
    }
    
    const response = await api.docker.request<ApiResponse<{
      message: string;
      logs?: any[];
      container_id: string;
      action_type: string;
    }>>("controlContainer", requestData);
        
    if (!response.data.success) {
      throw new Error(response.data.error || "컨테이너 재시작에 실패했습니다.");
    }
    
    return response.data;
  } catch (error) {
    console.error('컨테이너 재시작 실패:', error);
    throw error;
  }
};

// 컨테이너 삭제
export const removeContainer = async (serverId: number, containerId: string) => {
  try {
    const response = await api.docker.request<ApiResponse<{
      message: string
    }>>("removeContainer", {
      server_id: serverId,
      container_id: containerId
    });
        
    if (!response.data.success) {
      throw new Error(response.data.error || "컨테이너 삭제에 실패했습니다.");
    }
    
    return response.data;
  } catch (error) {
    console.error('컨테이너 삭제 실패:', error);
    throw error;
  }
};

// 도커 서버 상태 확인
export const checkDockerServerStatus = async (serverId: number, authInfo?: {
  username?: string;
  password?: string;
  hops?: Array<{
    host: string;
    port: number;
    username: string;
    password: string;
  }>;
}) => {
  try {    
    const requestData: any = {
      server_id: serverId
    };
    
    // 인증 정보가 제공된 경우 요청에 포함
    if (authInfo) {
      if (authInfo.username) requestData.username = authInfo.username;
      if (authInfo.password) requestData.password = authInfo.password;
      if (authInfo.hops) requestData.hops = authInfo.hops;
    }
    
    const response = await api.docker.request<ApiResponse<{
      status: {
        installed: boolean;
        running: boolean;
      };
      lastChecked: string;
    }>>("checkDockerServerStatus", requestData);
        
    if (!response.data.success) {
      throw new Error(response.data.error || "도커 서버 상태 확인에 실패했습니다.");
    }
    
    // 새로운 응답 구조에서 status를 변환하여 기존 인터페이스와 호환되도록 함
    let status: 'active' | 'inactive' | 'uninstalled' = 'uninstalled';
    
    if (response.data.status) {
      if (response.data.status.installed && response.data.status.running) {
        status = 'active';
      } else if (response.data.status.installed) {
        status = 'inactive';
      } else {
        status = 'uninstalled';
      }
    }
    
    // 반환 객체에 lastChecked 명시적 포함
    return {
      ...response.data,
      status,
      lastChecked: response.data.lastChecked,
      message: status === 'active' 
        ? '도커 서버가 활성 상태입니다.' 
        : status === 'inactive' 
          ? '도커가 설치되었지만 실행 중이 아닙니다.'
          : '도커가 설치되지 않았습니다.'
    };
  } catch (error) {
    console.error('도커 서버 상태 확인 실패:', error);
    throw error;
  }
};

// 도커 설치
export const installDocker = async (serverId: number, authInfo?: {
  username?: string;
  password?: string;
  hops?: Array<{
    host: string;
    port: number;
    username: string;
    password: string;
  }>;
}) => {
  try {    
    const requestData: any = {
      id: serverId  // server_id 대신 id 키 사용 (백엔드 호환성)
    };
    
    // 인증 정보가 제공된 경우 요청에 포함
    if (authInfo) {
      if (authInfo.username) requestData.username = authInfo.username;
      if (authInfo.password) requestData.password = authInfo.password;
      if (authInfo.hops) requestData.hops = authInfo.hops;
    }
    
    const response = await api.docker.request<ApiResponse<{
      message: string;
    }>>("installDocker", requestData);
        
    if (!response.data.success) {
      throw new Error(response.data.error || "도커 설치에 실패했습니다.");
    }
    
    return response.data;
  } catch (error) {
    console.error('도커 설치 실패:', error);
    throw error;
  }
};

// 도커 컴포즈 배포
export const createDockerContainer = async (data: {
  id: number;
  repo_url: string;
  hops: Array<{
    host: string;
    port: number;
    username: string;
    password: string;
  }>;
  branch?: string;
  username_repo?: string;
  password_repo?: string;
  compose_path?: string;
  compose_project?: string;
  force_recreate?: boolean;
  docker_registry?: string;
  docker_username?: string;
  docker_password?: string;
}) => {
  try {
    const response = await api.docker.request<ApiResponse<{
      success: boolean;
      message: string;
      containers?: {
        running: string[];
        exited: string[];
        other_state: string[];
      };
    }>>("createContainer", data);
    
    if (!response.data.success) {
      throw new Error(response.data.error || "도커 컴포즈 배포에 실패했습니다.");
    }
    
    return response.data;
  } catch (error) {
    console.error('도커 컴포즈 배포 실패:', error);
    throw error;
  }
};

// 도커 제거
export const uninstallDocker = async (serverId: number, authInfo: {
  username: string;
  password: string;
  hops: Array<{
    host: string;
    port: number;
    username: string;
    password: string;
  }>;
}) => {
  try {
    const requestData: any = {
      server_id: serverId
    };
    
    // 인증 정보가 제공된 경우 요청에 포함
    if (authInfo) {
      if (authInfo.username) requestData.username = authInfo.username;
      if (authInfo.password) requestData.password = authInfo.password;
      if (authInfo.hops) requestData.hops = authInfo.hops;
    }
    
    const response = await api.docker.request<ApiResponse<{
      message: string;
      logs: string[];
    }>>("uninstallDocker", requestData);
        
    if (!response.data.success) {
      throw new Error(response.data.error || "도커 제거에 실패했습니다.");
    }
    
    return response.data;
  } catch (error) {
    console.error('도커 제거 실패:', error);
    throw error;
  }
};

// 외부 도커 인프라 가져오기
export const importDockerInfra = async (data: {
  name: string;
  type: string;
  info: string;
  host: string;
  port: number;
  username: string;
  password: string;
  hops?: Array<{
    host: string;
    port: number;
    username: string;
    password: string;
  }>;
}) => {
  try {
    // hops 구성 (기본값으로 host, port, username, password 사용)
    const hops = data.hops || [{
      host: data.host,
      port: data.port,
      username: data.username,
      password: data.password
    }];

    const requestData = {
      name: data.name,
      type: data.type,
      info: data.info,
      hops: hops
    };
    
    const response = await api.docker.request<ApiResponse<{
      infra_id: number;
      server_name: string;
      registered_services: string[];
      service_groups: any;
    }>>("importDockerInfra", requestData);
    
    if (!response.data.success) {
      throw new Error(response.data.error || "외부 도커 인프라 가져오기에 실패했습니다.");
    }
    
    return response.data;
  } catch (error) {
    console.error('외부 도커 인프라 가져오기 실패:', error);
    throw error;
  }
}; 