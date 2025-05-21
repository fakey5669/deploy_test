import React, { useEffect, useState } from 'react';
import { Modal, Button, Divider, Badge, Card, Table, Tag, Space, Typography, Row, Col, Descriptions, Tooltip, message, Form, Input, Spin } from 'antd';
import { InfoCircleOutlined, CloudOutlined, CheckCircleOutlined, CloseCircleOutlined, EditOutlined, DeleteOutlined, AppstoreOutlined, GithubOutlined, GlobalOutlined, LinkOutlined, UserOutlined, KeyOutlined, CloudServerOutlined, ReloadOutlined, RocketOutlined, StopOutlined, SyncOutlined, FileTextOutlined, PlayCircleOutlined } from '@ant-design/icons';
import { Service } from '../../types/service';
import { InfraItem } from '../../types/infra';
import api from '../../services/api';
import * as serviceApi from '../../lib/api/service';
import * as kubernetesApi from '../../lib/api/kubernetes';

const { Text } = Typography;

interface ServiceDetailModalProps {
  visible: boolean;
  service: Service | null;
  onCancel: () => void;
  onEdit: (service: Service) => void;
}

// 가상의 데이터 구조 (실제 API와 연동 필요)
interface PodInfo {
  name: string;
  status: string;
  ready: boolean;
  restarts: number;
}

interface NamespaceInfo {
  name: string;
  status: string;
}

interface ServiceOperationStatus {
  namespace: NamespaceInfo;
  pods: PodInfo[];
}

// hops에서 추출한 호스트 정보 인터페이스
interface HopInfo {
  host: string;
  port: number;
}

interface Server {
  id: number;
  server_name?: string;
  type: string;
  hops: string; // JSON 문자열 형태로 된 호스트 정보
  join_command?: string;
  certificate_key?: string;
  ha?: string;
  last_checked?: string;
}

// 도커 컨테이너 정보용 인터페이스 (확장된 PodInfo)
interface DockerPodInfo extends PodInfo {
  id?: string;
  image?: string;
  created?: string;
  ports?: string;
  size?: string;
}

// 도커 서버 상태 정보 인터페이스
interface DockerServerStatus {
  installed: boolean; 
  running: boolean;
}

// 도커 상태 인터페이스
interface DockerStatus {
  serverStatus: DockerServerStatus;
  lastChecked: string;
  namespace: NamespaceInfo;
  pods: DockerPodInfo[];
}

// 전역 캐시 변수 추가
let globalServiceStatusCache: Record<number, {
  operationStatus: ServiceOperationStatus | DockerStatus | null;
  infraInfo: InfraItem | null;
  selectedServer: Server | null;
}> = {};

const ServiceDetailModal: React.FC<ServiceDetailModalProps> = ({
  visible,
  service,
  onCancel,
  onEdit
}) => {
  // 상태 관리
  const [loading, setLoading] = useState<boolean>(false);
  const [statusLoading, setStatusLoading] = useState<boolean>(false);
  const [operationStatus, setOperationStatus] = useState<ServiceOperationStatus | DockerStatus | null>(null);
  const [infraInfo, setInfraInfo] = useState<InfraItem | null>(null);
  const [infraLoading, setInfraLoading] = useState<boolean>(false);
  const [messageApi, contextHolder] = message.useMessage();
  const [selectedServer, setSelectedServer] = useState<Server | null>(null);
  const [authModalVisible, setAuthModalVisible] = useState<boolean>(false);
  const [authForm] = Form.useForm();
  const [podLogModalVisible, setPodLogModalVisible] = useState<boolean>(false);
  const [currentPod, setCurrentPod] = useState<{name: string} | null>(null);
  const [podLogs, setPodLogs] = useState<string>('');
  const [podLogsLoading, setPodLogsLoading] = useState<boolean>(false);
  const [authPurpose, setAuthPurpose] = useState<'status' | 'logs'>('status');

  // 모달이 열릴 때 캐시된 상태 복원
  useEffect(() => {
    if (service) {
      const cachedData = globalServiceStatusCache[service.id];
      if (cachedData) {
        setOperationStatus(cachedData.operationStatus);
        setInfraInfo(cachedData.infraInfo);
        setSelectedServer(cachedData.selectedServer);
      } else {
        // 캐시된 데이터가 없으면 초기화
        setOperationStatus(null);
        setInfraInfo(null);
        setSelectedServer(null);
        if (service.infra_id) {
          fetchInfraInfo(service.infra_id);
        }
      }
    }
  }, [service]);

  // 상태 조회 후 캐시 업데이트
  const updateStatusCache = (serviceId: number, newStatus: ServiceOperationStatus | DockerStatus | null) => {
    globalServiceStatusCache = {
      ...globalServiceStatusCache,
      [serviceId]: {
        ...globalServiceStatusCache[serviceId],
        operationStatus: newStatus
      }
    };
  };

  // 인프라 정보 조회 후 캐시 업데이트
  const updateInfraCache = (serviceId: number, newInfraInfo: InfraItem | null) => {
    globalServiceStatusCache = {
      ...globalServiceStatusCache,
      [serviceId]: {
        ...globalServiceStatusCache[serviceId],
        infraInfo: newInfraInfo
      }
    };
  };

  // 서버 선택 후 캐시 업데이트
  const updateServerCache = (serviceId: number, newServer: Server | null) => {
    globalServiceStatusCache = {
      ...globalServiceStatusCache,
      [serviceId]: {
        ...globalServiceStatusCache[serviceId],
        selectedServer: newServer
      }
    };
  };

  // 서비스 운영 상태 가져오기 (실제로는 API 호출)
  useEffect(() => {
    if (visible && service) {
      // 운영 상태는 수동으로 가져오도록 변경
      // fetchOperationStatus();
      if (service.infra_id) {
        fetchInfraInfo(service.infra_id);
      }
    }
  }, [visible, service]);

  // 상태 확인 버튼 클릭 핸들러
  const handleStatusCheck = async () => {
    if (!service) return;
    
    try {
      if (!service.infra_id) {
        messageApi.error('서비스에 연결된 인프라가 없습니다.');
        return;
      }

      // 인프라 정보 가져오기
      const infraResponse = await kubernetesApi.getInfraById(service.infra_id);
      if (!infraResponse.infra) {
        messageApi.error('인프라 정보를 가져올 수 없습니다.');
        return;
      }

      // 인프라 정보 설정
      setInfraInfo(infraResponse.infra);
      updateInfraCache(service.id, infraResponse.infra);

      // 인프라 유형 확인
      const infraType = infraResponse.infra.type?.toLowerCase();
      if (!infraType || (
        infraType !== 'kubernetes' && 
        infraType !== 'docker' && 
        infraType !== 'external_kubernetes' && 
        infraType !== 'external_docker'
      )) {
        messageApi.error('지원되지 않는 인프라 유형입니다. (kubernetes, docker, external_kubernetes, external_docker만 지원)');
        return;
      }

      // 인프라의 서버 정보 가져오기
      const serversResponse = await kubernetesApi.getServers(service.infra_id);
      if (!serversResponse.servers || serversResponse.servers.length === 0) {
        messageApi.error('인프라에 연결된 서버가 없습니다.');
        return;
      }

      // 인프라 유형에 따른 마스터 노드 찾기
      let masterServer;
      if (infraType === 'kubernetes') {
        masterServer = serversResponse.servers.find((server: Server) => 
          server.type.includes('master') && 
          server.join_command && 
          server.certificate_key
        );
      } else if (infraType === 'external_kubernetes') {
        // 외부 쿠버네티스는 서버가 하나만 등록되어 있으므로 첫 번째 서버 사용
        masterServer = serversResponse.servers[0];
      } else if (infraType === 'docker' || infraType === 'external_docker') {
        masterServer = serversResponse.servers[0];
      }

      if (!masterServer) {
        messageApi.error('마스터 노드를 찾을 수 없습니다.');
        return;
      }
      
      setSelectedServer(masterServer);
      updateServerCache(service.id, masterServer);
      setAuthModalVisible(true);
      setAuthPurpose('status');
    } catch (error) {
      console.error('서버 정보 가져오기 중 오류 발생:', error);
      messageApi.error('서버 정보를 가져오는 중 오류가 발생했습니다.');
    }
  };

  // 인프라 정보 가져오기
  const fetchInfraInfo = async (infraId: number) => {
    if (!service) return;
    
    setInfraLoading(true);
    try {
      const response = await api.kubernetes.request<{ infra: InfraItem, success: boolean }>('getInfraById', { id: infraId });
      
      if (response.data?.success && response.data.infra) {
        updateInfraCache(service.id, response.data.infra);
      } else {
        updateInfraCache(service.id, null);
      }
    } catch (error) {
      console.error('인프라 정보 가져오기 실패:', error);
      updateInfraCache(service.id, null);
    } finally {
      setInfraLoading(false);
    }
  };

  // 인증 모달 취소
  const handleAuthCancel = () => {
    setAuthModalVisible(false);
    setAuthPurpose('status');
    setSelectedServer(null);
    authForm.resetFields();
  };

  // 파드 로그 조회 함수
  const handleViewPodLogs = async (podName: string) => {
    if (!service || !service.namespace) {
      messageApi.error('서비스 정보가 없습니다.');
      return;
    }

    try {
      if (!selectedServer) {
        // 서버 정보가 없으면 인증 모달 표시
        setCurrentPod({ name: podName });
        setAuthPurpose('logs');
        setAuthModalVisible(true);
        return;
      }
      
      setPodLogsLoading(true);
      setPodLogModalVisible(true);
      setCurrentPod({ name: podName });
      setPodLogs('로그를 불러오는 중...');
      
      // hops 문자열에서 호스트 정보 파싱
      let hopInfo: HopInfo = { host: '', port: 22 };
      try {
        const hopsData = JSON.parse(selectedServer.hops);
        if (Array.isArray(hopsData) && hopsData.length > 0) {
          hopInfo = hopsData[0];
        }
      } catch (err) {
        console.error('Hops 정보 파싱 오류:', err);
        messageApi.error('서버 연결 정보를 파싱할 수 없습니다.');
        setPodLogsLoading(false);
        return;
      }
      
      // Kubernetes API 호출을 위한 hops 배열 생성
      const hops = [{
        host: hopInfo.host,
        port: hopInfo.port,
        username: authForm.getFieldValue('username'),
        password: authForm.getFieldValue('password')
      }];
      
      // 인프라 유형에 따라 다른 API 호출
      if (infraInfo?.type === 'kubernetes' || infraInfo?.type === 'external_kubernetes') {
        console.log(`[LOG] 쿠버네티스 파드 로그 조회 시작 - 파드: ${podName}, 네임스페이스: ${service.namespace}`);
        
        // 쿠버네티스 파드 로그 조회 API 호출
        const response = await kubernetesApi.getPodLogs({
          id: Number(selectedServer.id),
          namespace: service.namespace,
          pod_name: podName,
          lines: 100, // 최대 100줄까지 조회
          hops: hops
        });
        
        if (response.success) {
          console.log(`[LOG] 파드 로그 조회 성공 - 데이터 길이: ${response.logs?.length || 0}바이트`);
          setPodLogs(response.logs || '로그가 없습니다.');
        } else {
          console.error('[LOG] 로그 조회 실패:', response.error);
          
          let errorMessage = '로그를 불러오는데 실패했습니다.';
          let detailError = '';
          
          // 오류 메시지 분석
          if (response.error) {
            // stderr 및 errlog 부분 추출
            const stderrMatch = response.error.match(/stderr: (.*?)(?:, errlog:|$)/);
            const errlogMatch = response.error.match(/errlog: (.*?)$/);
            
            if (stderrMatch && stderrMatch[1] && stderrMatch[1].trim() !== '') {
              detailError += `\n\nSTDERR: ${stderrMatch[1]}`;
            }
            
            if (errlogMatch && errlogMatch[1] && errlogMatch[1].trim() !== '' && errlogMatch[1] !== 'No error log') {
              detailError += `\n\nERRLOG: ${errlogMatch[1]}`;
            }
            
            // 특정 오류 메시지에 대한 사용자 친화적인 메시지
            if (response.error.includes('Process exited with status 2')) {
              errorMessage = '권한 부족 또는 명령어 오류로 로그를 조회할 수 없습니다. 다음을 확인하세요:';
              detailError += `\n\n1. SSH 사용자에게 적절한 권한이 있는지
                              2. kubectl 명령어가 서버에 설치되어 있는지
                              3. 사용자에게 sudo 권한이 있는지
                              4. 패스워드가 정확한지`;
            } else if (response.error.includes('not found')) {
              errorMessage = `파드 '${podName}'를 찾을 수 없습니다. 파드가 실행 중인지 확인하세요.`;
            } else if (response.error.includes('Connection refused') || response.error.includes('timeout')) {
              errorMessage = '서버 연결에 실패했습니다. 서버가 실행 중이고 네트워크 연결이 가능한지 확인하세요.';
            } else if (response.error.includes('Authentication failed')) {
              errorMessage = '인증에 실패했습니다. 사용자 이름과 비밀번호를 확인하세요.';
            }
          }
          
          const fullErrorMessage = `${errorMessage}${detailError}\n\n오류 상세: ${response.error || '알 수 없는 오류'}`;
          console.log('[LOG] 에러 메시지:', fullErrorMessage);
          
          setPodLogs(fullErrorMessage);
          messageApi.error({
            content: errorMessage,
            duration: 5
          });
        }
      } else if (infraInfo?.type === 'docker' || infraInfo?.type === 'external_docker') {
        console.log(`[LOG] 도커 컨테이너 로그 조회 시작 - 컨테이너: ${podName}`);
        
        // 도커 컨테이너 로그 조회 API 호출
        const response = await api.docker.request('getDockerLogs', {
          container_id: podName,
          hops: hops,
          lines: 100 // 최대 100줄까지 조회
        });
        
        if (response.data?.success) {
          console.log(`[LOG] 컨테이너 로그 조회 성공 - 데이터 길이: ${response.data.logs?.length || 0}바이트`);
          setPodLogs(response.data.logs || '로그가 없습니다.');
        } else {
          console.error('[LOG] 도커 로그 조회 실패:', response.data?.error);
          
          const errorMessage = response.data?.error 
            ? `도커 컨테이너 로그를 불러오는데 실패했습니다: ${response.data.error}` 
            : '도커 컨테이너 로그를 불러오는데 실패했습니다.';
          
          setPodLogs(errorMessage);
          messageApi.error({
            content: '도커 컨테이너 로그를 불러오는데 실패했습니다.',
            duration: 5
          });
        }
      } else {
        const errorMessage = `지원되지 않는 인프라 유형(${infraInfo?.type || '알 수 없음'})입니다.`;
        setPodLogs(errorMessage);
        messageApi.error({
          content: errorMessage,
          duration: 5
        });
      }
      
      setPodLogsLoading(false);
    } catch (error) {
      console.error('[LOG] 로그 조회 예외 발생:', error);
      messageApi.error('로그 조회 중 오류가 발생했습니다.');
      setPodLogs('로그를 불러오는데 실패했습니다. 네트워크 오류가 발생했습니다.');
      setPodLogsLoading(false);
    }
  };

  // 인증 정보 제출 처리
  const handleAuthSubmit = async (values: any) => {
    if (!service) {
      messageApi.error('서비스 정보가 없습니다.');
      return;
    }

    try {
      if (authPurpose === 'status') {
        setStatusLoading(true);
        
        if (!selectedServer) {
          messageApi.error('서버 정보가 없습니다.');
          return;
        }

        // 인프라 유형 확인
        const infraType = infraInfo?.type?.toLowerCase();
        if (!infraType || (
          infraType !== 'kubernetes' && 
          infraType !== 'docker' && 
          infraType !== 'external_kubernetes' && 
          infraType !== 'external_docker'
        )) {
          messageApi.error('지원되지 않는 인프라 유형입니다. (kubernetes, docker, external_kubernetes, external_docker만 지원)');
          return;
        }

        // hops 문자열에서 호스트 정보 파싱
        let hopInfo: HopInfo = { host: '', port: 22 };
        try {
          const hopsData = JSON.parse(selectedServer.hops);
          if (Array.isArray(hopsData) && hopsData.length > 0) {
            hopInfo = hopsData[0];
          }
        } catch (err) {
          console.error('Hops 정보 파싱 오류:', err);
          messageApi.error('서버 연결 정보를 파싱할 수 없습니다.');
          return;
        }

        // hops 배열 생성
        const hops = [{
          host: hopInfo.host,
          port: hopInfo.port,
          username: values.username,
          password: values.password
        }];

        if (infraType === 'kubernetes' || infraType === 'external_kubernetes') {
          // 쿠버네티스 상태 조회 로직
          const response = await kubernetesApi.getNamespaceAndPodStatus({
            id: Number(selectedServer.id),
            namespace: String(service?.namespace || ''),
            hops: hops
          });

          if (response.success) {
            setOperationStatus({
              namespace: {
                name: response.namespace || service?.namespace || '',
                status: response.namespace_exists ? 'Active' : 'Not Found'
              },
              pods: (response.pods || []).map((pod: { name: string; status: string; restarts: string }) => ({
                name: pod.name,
                status: pod.status,
                ready: pod.status === 'Running',
                restarts: parseInt(pod.restarts, 10) || 0
              }))
            });
            updateStatusCache(service.id, operationStatus);
            setAuthModalVisible(false);
            messageApi.success('서비스 상태를 성공적으로 가져왔습니다.');
          } else {
            messageApi.error(response.error || '서비스 상태 확인에 실패했습니다.');
          }
        } else if (infraType === 'docker' || infraType === 'external_docker') {
          // 도커 상태 조회 로직
          const response = await api.docker.request('getContainers', {
            id: selectedServer.id,
            hops: hops,
            compose_project: service?.name
          });

          if (response.data?.success) {
            const containers = response.data.containers || [];
            setOperationStatus({
              namespace: {
                name: service?.name || '',
                status: containers.length > 0 ? 'Active' : 'Inactive'
              },
              pods: containers.map((container: any) => ({
                id: container.id || '',
                name: container.name,
                status: container.status.includes('Up') ? 'Running' : container.status,
                ready: container.status.includes('Up'),
                restarts: 0,
                image: container.image,
                created: container.created,
                ports: container.ports,
                size: container.size || '-'
              }))
            });
            updateStatusCache(service.id, operationStatus);
            setAuthModalVisible(false);
            messageApi.success('도커 컨테이너 상태를 성공적으로 가져왔습니다.');
          } else {
            messageApi.error(response.data?.error || '도커 컨테이너 상태 확인에 실패했습니다.');
          }
        }
      } else if (authPurpose === 'logs' && currentPod) {
        // 로그 조회 처리
        setAuthModalVisible(false);
        
        // 인증 정보와 함께 로그 조회 함수 호출
        handleViewPodLogs(currentPod.name);
      }
    } catch (error) {
      console.error('인증 처리 중 오류 발생:', error);
      messageApi.error('인증 처리 중 오류가 발생했습니다.');
    } finally {
      if (authPurpose === 'status') {
        setStatusLoading(false);
      }
    }
  };

  // 날짜 포맷팅
  const formatDate = (dateString: string) => {
    try {
      // KST 시간대를 UTC로 변환
      const date = new Date(dateString.replace('KST', '+0900'));
      return date.toLocaleString('ko-KR', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false
      });
    } catch (error) {
      console.error('날짜 파싱 오류:', error);
      return dateString; // 파싱 실패 시 원본 문자열 반환
    }
  };

  // 쿠버네티스 포드 재시작 함수
  const handleRestartPod = async (podName: string) => {
    if (!selectedServer) {
      messageApi.error('서버 정보가 없습니다.');
      return;
    }

    try {
      // hops 문자열에서 호스트 정보 파싱
      let hopInfo: HopInfo = { host: '', port: 22 };
      try {
        const hopsData = JSON.parse(selectedServer.hops);
        if (Array.isArray(hopsData) && hopsData.length > 0) {
          hopInfo = hopsData[0];
        }
      } catch (err) {
        console.error('Hops 정보 파싱 오류:', err);
        messageApi.error('서버 연결 정보를 파싱할 수 없습니다.');
        return;
      }

      // hops 배열 생성
      const hops = [{
        host: hopInfo.host,
        port: hopInfo.port,
        username: authForm.getFieldValue('username'),
        password: authForm.getFieldValue('password')
      }];

      // 쿠버네티스 포드 재시작 API 호출
      setStatusLoading(true);
      const response = await kubernetesApi.restartPod({
        id: Number(selectedServer.id),
        namespace: service?.name || 'default',
        pod_name: podName,
        hops: hops
      });

      if (response.success) {
        messageApi.success(`포드가 재시작되었습니다.`);
        
        // 약간의 지연 후 상태 업데이트
        setTimeout(async () => {
          await refreshPodStatus(selectedServer.id, hops);
        }, 2000);
      } else {
        messageApi.error(response.error || '포드 재시작에 실패했습니다.');
        setStatusLoading(false);
      }
    } catch (error) {
      console.error('포드 재시작 중 오류 발생:', error);
      messageApi.error('포드 재시작 중 오류가 발생했습니다.');
      setStatusLoading(false);
    }
  };

  // 쿠버네티스 포드 상태 새로고침 함수
  const refreshPodStatus = async (serverId: number | string, hops: any[]) => {
    try {
      const namespace = service?.namespace || 'default';
      const response = await kubernetesApi.getNamespaceAndPodStatus({
        id: Number(serverId),
        namespace: namespace,
        hops: hops
      });
      
      if (response.success) {
        const namespaceName = response.namespace || namespace;
        
        setOperationStatus({
          namespace: {
            name: namespaceName,
            status: response.namespace_exists ? 'Active' : 'Not Found'
          },
          pods: (response.pods || []).map((pod: { name: string; status: string; restarts: string }) => ({
            id: pod.name || '',
            name: pod.name,
            status: pod.status,
            ready: pod.status === 'Running',
            restarts: parseInt(pod.restarts, 10) || 0,
            image: '',
            created: '',
            ports: [],
            size: '-'
          }))
        });
        
        // 캐시 업데이트
        if (service) {
          updateStatusCache(service.id, operationStatus);
        }
      }
    } catch (error) {
      console.error('포드 상태 업데이트 중 오류 발생:', error);
    } finally {
      setStatusLoading(false);
    }
  };

  // 컨테이너 또는 포드 재시작 처리
  const handleRestart = (id: string) => {
    if (infraInfo?.type === 'kubernetes' || infraInfo?.type === 'external_kubernetes') {
      handleRestartPod(id);
    } else {
      handleRestartContainer(id);
    }
  };

  // 도커 컨테이너 중지 함수
  const handleStopContainer = async (containerId: string) => {
    if (!selectedServer) {
      messageApi.error('서버 정보가 없습니다.');
      return;
    }

    try {
      // hops 문자열에서 호스트 정보 파싱
      let hopInfo: HopInfo = { host: '', port: 22 };
      try {
        const hopsData = JSON.parse(selectedServer.hops);
        if (Array.isArray(hopsData) && hopsData.length > 0) {
          hopInfo = hopsData[0];
        }
      } catch (err) {
        console.error('Hops 정보 파싱 오류:', err);
        messageApi.error('서버 연결 정보를 파싱할 수 없습니다.');
        return;
      }

      // hops 배열 생성
      const hops = [{
        host: hopInfo.host,
        port: hopInfo.port,
        username: authForm.getFieldValue('username'),
        password: authForm.getFieldValue('password')
      }];

      // 도커 컨테이너 중지 API 호출
      setStatusLoading(true);
      const response = await api.docker.request('controlContainer', {
        server_id: selectedServer.id,
        container_id: containerId,
        action_type: 'stop',
        hops: hops
      });

      if (response.data?.success) {
        messageApi.success(`컨테이너가 중지되었습니다.`);
        
        // 상태 새로고침
        await refreshContainerStatus(selectedServer.id, hops);
      } else {
        messageApi.error(response.data?.error || '컨테이너 중지에 실패했습니다.');
      }
    } catch (error) {
      console.error('컨테이너 중지 중 오류 발생:', error);
      messageApi.error('컨테이너 중지 중 오류가 발생했습니다.');
    } finally {
      setStatusLoading(false);
    }
  };
  
  // 컨테이너 상태 새로고침 함수
  const refreshContainerStatus = async (serverId: number, hops: any[]) => {
    try {
      // 도커 컨테이너 상태 조회 API 호출
      const statusResponse = await api.docker.request('getContainers', {
        id: serverId,
        hops: hops,
        compose_project: service?.name
      });
      
      if (statusResponse.data?.success) {
        const containers = statusResponse.data.containers || [];
        
        setOperationStatus({
          namespace: {
            name: service?.name || '',
            status: containers.length > 0 ? 'Active' : 'Inactive'
          },
          pods: containers.map((container: any) => ({
            id: container.id || '',
            name: container.name,
            status: container.status.includes('Up') ? 'Running' : container.status,
            ready: container.status.includes('Up'),
            restarts: 0,
            image: container.image,
            created: container.created,
            ports: container.ports,
            size: container.size || '-'
          }))
        });
        
        // 캐시 업데이트
        if (service) {
          updateStatusCache(service.id, operationStatus);
        }
      }
    } catch (error) {
      console.error('컨테이너 상태 조회 중 오류 발생:', error);
      throw error;
    }
  };
  
  // 컨테이너 삭제 함수
  const handleDeleteContainer = async (containerId: string) => {
    if (!selectedServer) {
      messageApi.error('서버 정보가 없습니다.');
      return;
    }

    try {
      // hops 문자열에서 호스트 정보 파싱
      let hopInfo: HopInfo = { host: '', port: 22 };
      try {
        const hopsData = JSON.parse(selectedServer.hops);
        if (Array.isArray(hopsData) && hopsData.length > 0) {
          hopInfo = hopsData[0];
        }
      } catch (err) {
        console.error('Hops 정보 파싱 오류:', err);
        messageApi.error('서버 연결 정보를 파싱할 수 없습니다.');
        return;
      }

      // hops 배열 생성
      const hops = [{
        host: hopInfo.host,
        port: hopInfo.port,
        username: authForm.getFieldValue('username'),
        password: authForm.getFieldValue('password')
      }];

      setStatusLoading(true);
      // 도커 컨테이너 삭제 API 호출
      const response = await api.docker.request('removeOneDockerContainer', {
        id: selectedServer.id,
        container_id: containerId,
        hops: hops
      });

      if (response.data?.success) {
        messageApi.success('컨테이너가 성공적으로 삭제되었습니다.');
        // 상태 새로고침
        await refreshContainerStatus(selectedServer.id, hops);
      } else {
        messageApi.error(response.data?.error || '컨테이너 삭제에 실패했습니다.');
      }
    } catch (error) {
      console.error('컨테이너 삭제 중 오류 발생:', error);
      messageApi.error('컨테이너 삭제 중 오류가 발생했습니다.');
    } finally {
      setStatusLoading(false);
    }
  };

  // 도커 컨테이너 컬럼 정의 (확장된 정보 포함)
  const dockerColumns = [
    {
      title: '컨테이너 ID',
      dataIndex: 'id',
      key: 'id',
      render: (id: string, record: DockerPodInfo) => (
        <Tooltip title={id}>
          <span>{id?.substring(0, 12) || ''}</span>
        </Tooltip>
      )
    },
    {
      title: '컨테이너 이름',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '상태',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => (
        <Tag color={status === 'Running' || status.includes('Up') ? 'success' : 'error'}>
          {status || '알 수 없음'}
        </Tag>
      )
    },
    {
      title: '이미지',
      dataIndex: 'image',
      key: 'image',
      render: (image: string) => (
        <Tooltip title={image}>
          <span>{image?.split(':')[0] || ''}</span>
        </Tooltip>
      )
    },
    {
      title: '포트',
      dataIndex: 'ports',
      key: 'ports',
    },
    {
      title: '사이즈',
      dataIndex: 'size',
      key: 'size',
    },
    {
      title: '생성일',
      dataIndex: 'created',
      key: 'created',
      render: (created: string) => created ? formatDate(created) : '-'
    },
    {
      title: '작업',
      key: 'action',
      render: (_: any, record: DockerPodInfo) => (
        <Space>
          <Button 
            type="primary" 
            size="small"
            onClick={() => handleViewPodLogs(record.id || record.name)}
            icon={<FileTextOutlined />}
          >
            로그
          </Button>

          {record.status === 'Running' || record.status.includes('Up') ? (
            <>
              <Button
                size="small"
                onClick={() => handleStopContainer(record.id || '')}
                icon={<StopOutlined />}
                loading={statusLoading}
                disabled={!record.id}
              >
                중지
              </Button>

              <Button
                size="small"
                onClick={() => handleRestart(record.id || '')}
                icon={<SyncOutlined />}
                loading={statusLoading}
                disabled={!record.id}
              >
                재시작
              </Button>
            </>
          ) : (
            <Button
              type="dashed" 
              size="small"
              onClick={() => handleRestart(record.id || '')}
              icon={<PlayCircleOutlined />}
              loading={statusLoading}
              disabled={!record.id}
            >
              시작
            </Button>
          )}

          <Button 
            danger
            size="small"
            onClick={() => {
              Modal.confirm({
                title: '컨테이너 삭제',
                content: `정말로 컨테이너 "${record.name}"를 삭제하시겠습니까?`,
                okText: '삭제',
                cancelText: '취소',
                onOk: () => handleDeleteContainer(record.id || record.name)
              });
            }}
            icon={<DeleteOutlined />}
            loading={statusLoading}
          >
            삭제
          </Button>
        </Space>
      )
    }
  ];

  // 파드 테이블 컬럼 정의
  const podColumns = [
    {
      title: '파드 이름',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '상태',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => (
        <Tag color={status === 'Running' ? 'success' : 'error'}>
          {status || '알 수 없음'}
        </Tag>
      )
    },
    {
      title: '준비 상태',
      dataIndex: 'ready',
      key: 'ready',
      render: (ready: boolean) => (
        <span>
          {ready ? 
            <CheckCircleOutlined style={{ color: '#52c41a' }} /> : 
            <CloseCircleOutlined style={{ color: '#f5222d' }} />
          }
        </span>
      )
    },
    {
      title: '재시작 횟수',
      dataIndex: 'restarts',
      key: 'restarts',
    },
    {
      title: '로그',
      key: 'logs',
      render: (_: any, record: PodInfo) => (
        <Button 
          type="primary" 
          size="small"
          onClick={() => handleViewPodLogs(record.name)}
          icon={<FileTextOutlined />}
        >
          로그 보기
        </Button>
      )
    },
    {
      title: '작업',
      key: 'action',
      render: (_: any, record: PodInfo) => (
        <Space>
          <Button
            size="small"
            onClick={() => handleRestart(record.name)}
            icon={<SyncOutlined />}
            loading={statusLoading}
          >
            재시작
          </Button>
        </Space>
      )
    }
  ];

  // 파드 유형에 따른 컬럼 선택
  const getColumnsBasedOnInfraType = () => {
    if (!infraInfo) return podColumns;
    
    switch (infraInfo.type?.toLowerCase()) {
      case 'docker':
      case 'external_docker':
        return dockerColumns;
      case 'kubernetes':
      case 'external_kubernetes':
        return podColumns;
      default:
        return podColumns;
    }
  };

  // 도커 컨테이너 재시작 함수
  const handleRestartContainer = async (containerId: string) => {
    if (!selectedServer) {
      messageApi.error('서버 정보가 없습니다.');
      return;
    }

    try {
      // hops 문자열에서 호스트 정보 파싱
      let hopInfo: HopInfo = { host: '', port: 22 };
      try {
        const hopsData = JSON.parse(selectedServer.hops);
        if (Array.isArray(hopsData) && hopsData.length > 0) {
          hopInfo = hopsData[0];
        }
      } catch (err) {
        console.error('Hops 정보 파싱 오류:', err);
        messageApi.error('서버 연결 정보를 파싱할 수 없습니다.');
        return;
      }

      // hops 배열 생성
      const hops = [{
        host: hopInfo.host,
        port: hopInfo.port,
        username: authForm.getFieldValue('username'),
        password: authForm.getFieldValue('password')
      }];

      // 도커 컨테이너 재시작 API 호출
      setStatusLoading(true);
      const response = await api.docker.request('controlContainer', {
        server_id: selectedServer.id,
        container_id: containerId,
        action_type: 'restart',
        hops: hops
      });

      if (response.data?.success) {
        messageApi.success(`컨테이너가 재시작되었습니다.`);
        
        // 상태 새로고침
        await refreshContainerStatus(selectedServer.id, hops);
      } else {
        messageApi.error(response.data?.error || '컨테이너 재시작에 실패했습니다.');
      }
    } catch (error) {
      console.error('컨테이너 재시작 중 오류 발생:', error);
      messageApi.error('컨테이너 재시작 중 오류가 발생했습니다.');
    } finally {
      setStatusLoading(false);
    }
  };

  // 로그 조회 핸들러

  // 파드/컨테이너 정보 렌더링
  const renderPodsOrContainers = () => {
    if (!operationStatus) return null;
    
    const columns = getColumnsBasedOnInfraType();
    const entityName = '파드';
    
    return (
      <>
        <div style={{ marginTop: 16 }}>
          <Text strong>{entityName} 목록</Text>
          <Tag style={{ marginLeft: '8px' }}>{operationStatus.pods.length}개</Tag>
        </div>
        
        {operationStatus.pods.length > 0 ? (
          <Table 
            dataSource={operationStatus.pods}
            columns={columns}
            rowKey="name"
            size="small"
            pagination={false}
            scroll={{ x: 'max-content' }}
            style={{ 
              marginTop: 8,
              backgroundColor: '#ffffff', 
              borderRadius: '8px', 
              boxShadow: '0 1px 2px rgba(0, 0, 0, 0.03)'
            }}
          />
        ) : (
          <Card 
            size="small"
            bordered={false}
            style={{ 
              marginTop: 8,
              backgroundColor: '#f9fafc', 
              borderRadius: '8px', 
              padding: '16px', 
              textAlign: 'center',
              boxShadow: '0 1px 2px rgba(0, 0, 0, 0.03)'
            }}
          >
            <div>
              <InfoCircleOutlined style={{ color: '#1890ff', marginRight: '8px' }} />
              실행 중인 {entityName}가 없습니다. 서비스 배포를 통해 {entityName}를 생성하세요.
            </div>
          </Card>
        )}
      </>
    );
  };

  // 기존 코드를 수정하여 도커 서버 상태 정보도 표시하도록 수정
  const renderServiceStatus = () => {
    if (!operationStatus || !service) return null;
    
    return (
      <>
        <Card 
          size="small" 
          bordered={false}
          style={{ 
            backgroundColor: '#f9fafc', 
            borderRadius: '8px', 
            padding: '16px', 
            boxShadow: '0 1px 2px rgba(0, 0, 0, 0.03)',
            marginBottom: '16px'
          }}
        >
          <div>
            <InfoItem 
              label={infraInfo?.type === 'kubernetes' || infraInfo?.type === 'external_kubernetes' ? '네임스페이스 이름' : '서비스 이름'} 
              value={operationStatus.namespace.name || service.namespace || service.name} 
            />
            <InfoItem 
              label="상태" 
              value={
                <Tag color={operationStatus.namespace.status === 'Active' ? 'success' : 'error'}>
                  {operationStatus.namespace.status}
                </Tag>
              } 
            />
          </div>
        </Card>
        
        {renderPodsOrContainers()}
      </>
    );
  };

  // 컴포넌트 마운트 시 이벤트 리스너 등록
  useEffect(() => {
    // 서비스 상태 업데이트 이벤트 리스너
    const handleServiceStatusUpdate = (event: Event) => {
      const customEvent = event as CustomEvent;
      if (
        customEvent.detail && 
        customEvent.detail.serviceId === service?.id && 
        customEvent.detail.status
      ) {
        setOperationStatus(customEvent.detail.status);
      }
    };

    // 이벤트 리스너 등록
    document.addEventListener('serviceStatusUpdate', handleServiceStatusUpdate as EventListener);

    // 컴포넌트 언마운트 시 이벤트 리스너 제거
    return () => {
      document.removeEventListener('serviceStatusUpdate', handleServiceStatusUpdate as EventListener);
    };
  }, [service]);

  // 모달이 닫힐 때 상태를 리셋하지 않음
  const handleCancel = () => {
    onCancel();
  };

  if (!service) return null;

  return (
    <>
      {contextHolder}
      <Modal
        title={
          <div style={{ 
            display: 'flex', 
            alignItems: 'center', 
            padding: '8px 0',
          }}>
            <CloudOutlined style={{ 
              color: '#1890ff', 
              fontSize: '20px', 
              marginRight: '12px' 
            }} />
            <span style={{
              fontSize: '16px',
              fontWeight: 600
            }}>서비스 상세 정보</span>
          </div>
        }
        open={visible}
        onCancel={handleCancel}
        footer={[
          <Button key="back" onClick={handleCancel}>
            닫기
          </Button>
        ]}
        width={700}
        style={{ 
          borderRadius: '12px',
          overflow: 'hidden'
        }}
      >
        <div className="service-detail-content">
          <div style={{ 
            display: 'flex', 
            alignItems: 'center', 
            padding: '8px 0'
          }}>
            <Text strong style={{ fontSize: '16px' }}>기본 정보</Text>
          </div>
          <Divider style={{ margin: '0 0 16px 0' }} />
          
          <div className="basic-info-section">
            <Card 
              bordered={false} 
              style={{ 
                backgroundColor: '#f9fafc', 
                borderRadius: '8px', 
                padding: '16px', 
                boxShadow: '0 1px 2px rgba(0, 0, 0, 0.03)' 
              }}
            >
              <InfoItem label="서비스 이름" value={service.name} />
              <InfoItem 
                label="상태" 
                value={
                  service.status === 'active' ? (
                    <Badge status="success" text="활성" />
                  ) : service.status === 'inactive' ? (
                    <Badge status="error" text="비활성" />
                  ) : (
                    <Badge status="default" text="등록" />
                  )
                } 
              />
              {infraInfo?.type === 'kubernetes' || infraInfo?.type === 'external_kubernetes' && (
                <InfoItem 
                  label="네임스페이스" 
                  value={service.namespace || '-'}
                />
              )}
              <InfoItem 
                label="GitLab URL" 
                value={
                  service.gitlab_url ? (
                    <a href={service.gitlab_url} target="_blank" rel="noopener noreferrer">
                      {service.gitlab_url}
                    </a>
                  ) : '-'
                } 
              />
              <InfoItem 
                label="인프라" 
                value={
                  infraLoading ? (
                    <span>로딩 중...</span>
                  ) : service.infra_id ? (
                    <Space>
                      <CloudServerOutlined style={{ color: '#1890ff' }} />
                      {infraInfo ? (
                        <span>
                          {infraInfo.name} 
                          <Tag color={
                            infraInfo.type === 'kubernetes' || infraInfo.type === 'external_kubernetes' ? 'blue' :
                            infraInfo.type === 'docker' || infraInfo.type === 'external_docker' ? 'green' :
                            infraInfo.type === 'baremetal' ? 'orange' :
                            infraInfo.type === 'cloud' ? 'purple' : 'default'
                          }>
                            {infraInfo.type === 'kubernetes' || infraInfo.type === 'external_kubernetes' ? '쿠버네티스' :
                             infraInfo.type === 'docker' || infraInfo.type === 'external_docker' ? '도커' :
                             infraInfo.type === 'baremetal' ? '베어메탈' :
                             infraInfo.type === 'cloud' ? '클라우드' : infraInfo.type}
                          </Tag>
                        </span>
                      ) : (
                        <span>인프라 ID: {service.infra_id}</span>
                      )}
                    </Space>
                  ) : '-'
                } 
              />
            </Card>
          </div>
          
          <div style={{ 
            display: 'flex', 
            alignItems: 'center', 
            justifyContent: 'space-between',
            marginTop: '32px',
            padding: '8px 0'
          }}>
            <Text strong style={{ fontSize: '16px' }}>
              {infraInfo?.type === 'kubernetes' || infraInfo?.type === 'external_kubernetes' ? '네임스페이스와 파드 상태' : 
               infraInfo?.type === 'docker' || infraInfo?.type === 'external_docker' ? '도커 컨테이너 상태' :
               '서비스 상태'}
            </Text>
            <Button 
              type="primary" 
              icon={<SyncOutlined />} 
              onClick={handleStatusCheck}
              loading={statusLoading}
              disabled={!service.infra_id}
            >
              상태 확인
            </Button>
          </div>
          <Divider style={{ margin: '0 0 16px 0' }} />
          
          {operationStatus ? (
            renderServiceStatus()
          ) : (
            <Card 
              size="small"
              bordered={false}
              style={{ 
                backgroundColor: '#f9fafc', 
                borderRadius: '8px', 
                padding: '16px', 
                textAlign: 'center',
                boxShadow: '0 1px 2px rgba(0, 0, 0, 0.03)'
              }}
            >
              {statusLoading ? (
                <div>서비스 상태 정보를 불러오는 중입니다...</div>
              ) : (
                <div>
                  <InfoCircleOutlined style={{ color: '#1890ff', marginRight: '8px' }} />
                  서비스 상태를 확인하려면 상태 확인 버튼을 클릭하세요.
                </div>
              )}
            </Card>
          )}
        </div>
      </Modal>

      {/* 파드 로그 모달 */}
      <Modal
        title={`파드 로그: ${currentPod?.name}`}
        open={podLogModalVisible}
        onCancel={() => setPodLogModalVisible(false)}
        width={800}
        footer={[
          <Button key="close" onClick={() => setPodLogModalVisible(false)}>
            닫기
          </Button>,
          <Button 
            key="refresh" 
            type="primary" 
            icon={<ReloadOutlined />}
            onClick={() => currentPod && handleViewPodLogs(currentPod.name)}
            loading={podLogsLoading}
            disabled={!selectedServer || !service?.namespace}
          >
            새로고침
          </Button>,
          <Button 
            key="auth" 
            type="dashed" 
            icon={<KeyOutlined />}
            onClick={() => {
              if (currentPod) {
                setPodLogModalVisible(false);
                setAuthPurpose('logs');
                authForm.resetFields();
                setAuthModalVisible(true);
              }
            }}
            disabled={!currentPod}
          >
            인증 정보 변경
          </Button>
        ]}
      >
        <Spin spinning={podLogsLoading}>
          <pre style={{ 
            maxHeight: '400px', 
            overflow: 'auto', 
            padding: '12px',
            backgroundColor: '#000',
            color: '#fff',
            borderRadius: '4px',
            fontSize: '13px',
            lineHeight: '1.5'
          }}>
            {podLogs}
          </pre>
        </Spin>
      </Modal>

      {/* 인증 모달 */}
      <Modal
        title={authPurpose === 'status' ? "서버 인증 정보 입력" : "로그 조회를 위한 인증 정보 입력"}
        open={authModalVisible}
        onCancel={handleAuthCancel}
        footer={null}
      >
        <Form
          form={authForm}
          layout="vertical"
          onFinish={handleAuthSubmit}
          initialValues={{
            username: '',
            password: ''
          }}
        >
          <Descriptions bordered size="small" column={1} style={{ marginBottom: 16 }}>
            <Descriptions.Item label="서버 이름">
              {selectedServer?.server_name || '마스터 노드'}
            </Descriptions.Item>
            <Descriptions.Item label="호스트">
              {selectedServer && (() => {
                try {
                  const hopsData = JSON.parse(selectedServer.hops);
                  return hopsData[0]?.host || '';
                } catch (e) {
                  return '';
                }
              })()}
            </Descriptions.Item>
            <Descriptions.Item label="포트">
              {selectedServer && (() => {
                try {
                  const hopsData = JSON.parse(selectedServer.hops);
                  return hopsData[0]?.port || 22;
                } catch (e) {
                  return 22;
                }
              })()}
            </Descriptions.Item>
            <Descriptions.Item label="유형">
              <Tag color="blue">{selectedServer?.type || 'master'}</Tag>
            </Descriptions.Item>
          </Descriptions>
          
          <Form.Item
            name="username"
            label="사용자 이름"
            rules={[{ required: true, message: '사용자 이름을 입력해주세요' }]}
          >
            <Input placeholder="SSH 사용자 이름" />
          </Form.Item>
          
          <Form.Item
            name="password"
            label="비밀번호"
            rules={[{ required: true, message: '비밀번호를 입력해주세요' }]}
          >
            <Input.Password placeholder="SSH 비밀번호" />
          </Form.Item>
          
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={statusLoading}>
              {authPurpose === 'status' ? '상태 확인' : '로그 조회'}
            </Button>
            <Button onClick={handleAuthCancel} style={{ marginLeft: 8 }}>
              취소
            </Button>
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
};

interface InfoItemProps {
  label: string;
  value: React.ReactNode;
}

const InfoItem: React.FC<InfoItemProps> = ({ label, value }) => {
  return (
    <div style={{ 
      display: 'flex', 
      marginBottom: '12px',
      alignItems: 'center'
    }}>
      <span style={{ 
        color: '#666', 
        fontWeight: 500,
        width: '120px',
        flexShrink: 0
      }}>
        {label}:
      </span>
      <span style={{ 
        flex: 1, 
        color: '#333',
        overflow: 'hidden',
        textOverflow: 'ellipsis'
      }}>
        {value}
      </span>
    </div>
  );
};

export default ServiceDetailModal; 