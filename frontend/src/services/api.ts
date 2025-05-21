import axios, { AxiosInstance, InternalAxiosRequestConfig } from 'axios';
import { ServerStatus, ApiResponse } from '../types';

const API_URL = process.env.REACT_APP_API_URL || 'http://localhost:8080/api/v1';

// API 클라이언트 생성
const apiClient: AxiosInstance = axios.create({
  baseURL: API_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

// 인터셉터 설정 (필요한 경우)
apiClient.interceptors.request.use(
  (config: InternalAxiosRequestConfig): InternalAxiosRequestConfig => {
    // 요청 전에 수행할 작업
    const token = localStorage.getItem('token');
    if (token && config.headers) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// API 함수들
export const api = {
  // 헬스 체크
  checkHealth: () => apiClient.get<ServerStatus>('/health'),
  
  // 서비스 API (통합 형식)
  service: {
    // 단일 엔드포인트로 모든 요청 처리
    request: <T>(action: string, parameters: any) => 
      apiClient.post<ApiResponse<T>>('/service', {
        action,
        parameters
      })
  },
  
  // 통합 쿠버네티스 API
  kubernetes: {
    // 단일 엔드포인트로 모든 요청 처리
    request: <T>(action: string, parameters: any) => 
      apiClient.post<ApiResponse<T>>('/kubernetes', {
        action,
        parameters
      })
  },
  
  // 통합 도커 API
  docker: {
    // 단일 엔드포인트로 모든 요청 처리
    request: <T>(action: string, parameters: any) => 
      apiClient.post<ApiResponse<T>>('/docker', {
        action,
        parameters
      })
  }
};

export default api; 