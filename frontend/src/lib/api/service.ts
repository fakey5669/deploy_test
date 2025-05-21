import { 
  Service, 
  CreateServiceRequest, 
  UpdateServiceRequest, 
  ServiceStatus,
  KubernetesStatus
} from '../../types/service';
import api from '../../services/api';
import { ApiResponse } from '../../types';

// 모든 서비스 가져오기
export const getServices = async (): Promise<Service[]> => {
  try {
    const response = await api.service.request<Service[]>('getServices', {});
    return response.data?.data || [];
  } catch (error) {
    console.error('서비스 목록 가져오기 실패:', error);
    throw error;
  }
};

// 특정 서비스 가져오기
export const getService = async (serviceId: string | number): Promise<Service> => {
  try {
    const id = typeof serviceId === 'string' ? parseInt(serviceId, 10) : serviceId;
    const response = await api.service.request<Service>('getServiceById', { id });
    if (!response.data) {
      throw new Error(`서비스 ID ${id}를 찾을 수 없습니다.`);
    }
    return response.data?.data!;
  } catch (error) {
    console.error(`서비스 ID ${serviceId} 가져오기 실패:`, error);
    throw error;
  }
};

// 서비스 생성
export const createService = async (data: Omit<Service, 'id' | 'created_at' | 'updated_at'>): Promise<Service> => {
  try {
    const response = await api.service.request<Service>('createService', data);
    if (!response.data) {
      throw new Error('서비스 생성 응답에 데이터가 없습니다.');
    }
    return response.data?.data!;
  } catch (error) {
    console.error('서비스 생성 실패:', error);
    throw error;
  }
};

// 서비스 업데이트
export const updateService = async (serviceId: string | number, data: Omit<Service, 'id' | 'created_at' | 'updated_at'>): Promise<Service> => {
  try {
    const id = typeof serviceId === 'string' ? parseInt(serviceId, 10) : serviceId;
    const response = await api.service.request<Service>('updateService', {
      id,
      ...data
    });
    if (!response.data) {
      throw new Error(`서비스 ID ${id} 업데이트 응답에 데이터가 없습니다.`);
    }
    return response.data?.data!;
  } catch (error) {
    console.error(`서비스 ID ${serviceId} 업데이트 실패:`, error);
    throw error;
  }
};

// 서비스 삭제
export const deleteService = async (serviceId: string | number): Promise<void> => {
  try {
    const id = typeof serviceId === 'string' ? parseInt(serviceId, 10) : serviceId;
    await api.service.request('deleteService', { id });
  } catch (error) {
    console.error(`서비스 ID ${serviceId} 삭제 실패:`, error);
    throw error;
  }
};

// 서비스 상태 변경
export const changeServiceStatus = async (
  serviceId: string | number, 
  action: 'active' | 'inactive'
): Promise<Service> => {
  try {
    const id = typeof serviceId === 'string' ? parseInt(serviceId, 10) : serviceId;
    // API에 상태 변경 엔드포인트가 없으므로 일반 업데이트를 사용
    const response = await api.service.request<Service>('updateService', { 
      id, 
      status: action 
    });
    if (!response.data) {
      throw new Error(`서비스 ID ${id} 상태 변경 응답에 데이터가 없습니다.`);
    }
    return response.data?.data!;
  } catch (error) {
    console.error(`서비스 ID ${serviceId} 상태 변경 실패:`, error);
    throw error;
  }
};

// 서비스 운영 상태 가져오기
export const getServiceStatus = async (serviceId: string | number): Promise<{
  id: number, 
  name: string, 
  gitlab_url: string, 
  namespace: string, 
  kubernetesStatus: KubernetesStatus
}> => {
  try {
    const id = typeof serviceId === 'string' ? parseInt(serviceId, 10) : serviceId;
    const response = await api.service.request<{
      id: number, 
      name: string, 
      gitlab_url: string, 
      namespace: string, 
      kubernetesStatus: KubernetesStatus
    }>('getServiceStatus', { id });
    
    if (!response.data?.data) {
      throw new Error(`서비스 ID ${id} 상태 조회 응답에 데이터가 없습니다.`);
    }
    
    return response.data.data;
  } catch (error) {
    console.error(`서비스 ID ${serviceId} 운영 상태 가져오기 실패:`, error);
    throw error;
  }
};

// 서비스 배포
export const deployService = async (serviceId: string | number): Promise<{ id: number, status: string }> => {
  try {
    const id = typeof serviceId === 'string' ? parseInt(serviceId, 10) : serviceId;
    const response = await api.service.request<{ id: number, status: string }>('deployService', { id });
    
    if (!response.data?.data) {
      throw new Error(`서비스 ID ${id} 배포 응답에 데이터가 없습니다.`);
    }
    
    return response.data.data;
  } catch (error) {
    console.error(`서비스 ID ${serviceId} 배포 실패:`, error);
    throw error;
  }
};

// 서비스 재시작
export const restartService = async (serviceId: string | number): Promise<{ id: number, status: string }> => {
  try {
    const id = typeof serviceId === 'string' ? parseInt(serviceId, 10) : serviceId;
    const response = await api.service.request<{ id: number, status: string }>('restartService', { id });
    
    if (!response.data?.data) {
      throw new Error(`서비스 ID ${id} 재시작 응답에 데이터가 없습니다.`);
    }
    
    return response.data.data;
  } catch (error) {
    console.error(`서비스 ID ${serviceId} 재시작 실패:`, error);
    throw error;
  }
};

// 서비스 중지
export const stopService = async (serviceId: string | number): Promise<{ id: number, status: string }> => {
  try {
    const id = typeof serviceId === 'string' ? parseInt(serviceId, 10) : serviceId;
    const response = await api.service.request<{ id: number, status: string }>('stopService', { id });
    
    if (!response.data?.data) {
      throw new Error(`서비스 ID ${id} 중지 응답에 데이터가 없습니다.`);
    }
    
    return response.data.data;
  } catch (error) {
    console.error(`서비스 ID ${serviceId} 중지 실패:`, error);
    throw error;
  }
};

// 서비스 제거
export const removeService = async (serviceId: string | number): Promise<{ id: number, status: string }> => {
  try {
    const id = typeof serviceId === 'string' ? parseInt(serviceId, 10) : serviceId;
    const response = await api.service.request<{ id: number, status: string }>('removeService', { id });
    
    if (!response.data?.data) {
      throw new Error(`서비스 ID ${id} 제거 응답에 데이터가 없습니다.`);
    }
    
    return response.data.data;
  } catch (error) {
    console.error(`서비스 ID ${serviceId} 제거 실패:`, error);
    throw error;
  }
};

/**
 * 서비스의 도커 파일 존재 여부 및 내용을 조회합니다.
 * @param serviceId 서비스 ID
 * @returns 도커 파일 정보
 */
export const getDockerFiles = async (serviceId: string | number) => {
  try {
    const id = typeof serviceId === 'string' ? parseInt(serviceId, 10) : serviceId;
    
    const params: any = { id };
    
    const response = await api.service.request('getDockerFiles', params);
    
    return {
      success: true,
      hasDockerfile: response.data?.hasDockerfile || false,
      hasDockerCompose: response.data?.hasDockerCompose || false,
      dockerfileContent: response.data?.dockerfileContent || '',
      dockerComposeContent: response.data?.dockerComposeContent || '',
    };
  } catch (error) {
    console.error(`서비스 ID ${serviceId} 도커 파일 정보 가져오기 실패:`, error);
    return {
      success: false,
      error: "도커 파일 정보를 가져오는 중 오류가 발생했습니다."
    };
  }
};

/**
 * 서비스의 Dockerfile 내용을 조회합니다.
 * @param serviceId 서비스 ID
 * @returns Dockerfile 내용
 */
export const getDockerfile = async (serviceId: string | number) => {
  try {
    const id = typeof serviceId === 'string' ? parseInt(serviceId, 10) : serviceId;
    
    const params: any = { id };
    
    const response = await api.service.request('getDockerfile', params);
    
    return {
      success: true,
      content: response.data?.content || '',
    };
  } catch (error) {
    console.error(`서비스 ID ${serviceId} Dockerfile 가져오기 실패:`, error);
    return {
      success: false,
      error: "Dockerfile을 가져오는 중 오류가 발생했습니다."
    };
  }
};

/**
 * 서비스의 Docker Compose 파일 내용을 조회합니다.
 * @param serviceId 서비스 ID
 * @returns Docker Compose 파일 내용
 */
export const getDockerCompose = async (serviceId: string | number) => {
  try {
    const id = typeof serviceId === 'string' ? parseInt(serviceId, 10) : serviceId;
    
    const params: any = { id };
    
    const response = await api.service.request('getDockerCompose', params);
    
    return {
      success: true,
      content: response.data?.content || '',
    };
  } catch (error) {
    console.error(`서비스 ID ${serviceId} Docker Compose 가져오기 실패:`, error);
    return {
      success: false,
      error: "Docker Compose 파일을 가져오는 중 오류가 발생했습니다."
    };
  }
};

/**
 * 서비스의 Dockerfile을 저장합니다.
 * @param serviceId 서비스 ID
 * @param content Dockerfile 내용
 * @param commitMessage 저장 시 남길 커밋 메시지
 * @returns 저장 결과
 */
export const saveDockerfile = async (
  serviceId: string | number, 
  content: string, 
  commitMessage: string = "Update Dockerfile"
) => {
  try {
    const id = typeof serviceId === 'string' ? parseInt(serviceId, 10) : serviceId;
    
    const params: any = { 
      id, 
      content,
      commitMessage
    };
    
    const response = await api.service.request('saveDockerfile', params);
    
    return {
      success: true,
    };
  } catch (error) {
    console.error(`서비스 ID ${serviceId} Dockerfile 저장 실패:`, error);
    return {
      success: false,
      error: "Dockerfile을 저장하는 중 오류가 발생했습니다."
    };
  }
};

/**
 * 서비스의 Docker Compose 파일을 저장합니다.
 * @param serviceId 서비스 ID
 * @param content Docker Compose 파일 내용
 * @param commitMessage 저장 시 남길 커밋 메시지
 * @returns 저장 결과
 */
export const saveDockerCompose = async (
  serviceId: string | number, 
  content: string, 
  commitMessage: string = "Update docker-compose.yml"
) => {
  try {
    const id = typeof serviceId === 'string' ? parseInt(serviceId, 10) : serviceId;
    
    const params: any = { 
      id, 
      content,
      commitMessage
    };
    
    const response = await api.service.request('saveDockerCompose', params);
    
    return {
      success: true,
    };
  } catch (error) {
    console.error(`서비스 ID ${serviceId} Docker Compose 저장 실패:`, error);
    return {
      success: false,
      error: "Docker Compose 파일을 저장하는 중 오류가 발생했습니다."
    };
  }
};
