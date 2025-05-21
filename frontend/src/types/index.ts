// API 응답 타입
export interface ApiResponse<T> {
  success: boolean;
  error?: string;
  message?: string;
  [key: string]: any;
  data?: T;
}

// 서버 상태 타입
export type ServerStatus = 'running' | 'stopped' | 'inactive' | 'active' | 'maintenance' | 'preparing' | '등록' | 'checking';

// 사용자 타입
export interface User {
  id: number;
  username: string;
  email: string;
  createdAt: string;
  updatedAt: string;
}

// 서비스 타입
export interface Service {
  id: number;
  name: string;
  domain: string;
  gitlab_url: string;
  gitlab_id: string;
  gitlab_access_token: string;
  user_id: number;
  created_at: string;
  updated_at: string;
}

// 인증 관련 타입
export interface AuthState {
  isAuthenticated: boolean;
  user: User | null;
  token: string | null;
  loading: boolean;
  error: string | null;
} 