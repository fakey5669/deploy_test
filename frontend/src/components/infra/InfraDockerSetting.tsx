'use client';

import React, { useState, useEffect } from 'react';
import { Button, Typography, Space, Divider, List, Card, Row, Col, message, Modal, Input, Form, Table, Tag, Tooltip, Statistic, Spin, Empty } from 'antd';
import { SettingOutlined, ReloadOutlined, PlayCircleOutlined, PauseCircleOutlined, DeleteOutlined, InfoCircleOutlined, ClockCircleOutlined, CodeOutlined, CloudServerOutlined, PlusOutlined, GlobalOutlined, ApiOutlined, ContainerOutlined, FileImageOutlined, AppstoreOutlined, SyncOutlined, DownloadOutlined, DashboardOutlined } from '@ant-design/icons';
import { InfraItem } from '../../types/infra';
import api from '../../services/api';
import * as kubernetesApi from '../../lib/api/kubernetes';
import * as dockerApi from '../../lib/api/docker';

const { Text, Title } = Typography;

interface InfraDockerSettingProps {
  infra: InfraItem;
  showSettingsModal: (infra: InfraItem) => void;
  isExternal?: boolean; // 추가
}

// 도커 컨테이너 인터페이스
interface DockerContainer {
  id: string;
  name: string;
  image: string;
  status: 'running' | 'stopped' | 'paused' | 'exited';
  created: string;
  ports: string[];
  size: string;
}

// 도커 정보 인터페이스
interface DockerInfo {
  version: string;
  apiVersion: string;
  totalContainers: number;
  runningContainers: number;
  stoppedContainers: number;
  volumes: number;
  images: number;
  os: string;
  arch: string;
  memory: string;
  cpus: number;
  networks: { name: string; driver: string; scope: string }[];
  imageList: { repository: string; tag: string; size: string; created: string }[];
  volumeList: { name: string; driver: string; size: string }[];
}

// 도커 서버 상태 타입 정의
type DockerServerStatus = 'active' | 'inactive' | 'uninstalled';

// 도커 서버 정보 인터페이스
interface DockerServer {
  id: number;
  name?: string;
  server_name?: string;
  ip?: string;
  port?: string;
  status: DockerServerStatus;
  hops?: string | any[];
  lastChecked?: string;
  created_at?: string;
  updated_at?: string;
}

// 인증 목적 상수 정의
type AuthPurpose = 'install' | 'status' | 'uninstall';

// 서버 리소스 정보 인터페이스 추가
interface ServerResource {
  host_info: {
    hostname: string;
    os: string;
    kernel: string;
  };
  cpu: {
    model: string;
    cores: string;
    usage_percent: string;
  };
  memory: {
    total_mb: string;
    used_mb: string;
    free_mb: string;
    usage_percent: string;
  };
  disk: {
    root_total: string;
    root_used: string;
    root_free: string;
    root_usage_percent: string;
  };
}

const InfraDockerSetting: React.FC<InfraDockerSettingProps> = ({ infra, showSettingsModal }) => {
  const [messageApi, contextHolder] = message.useMessage();
  const [loading, setLoading] = useState(false);
  const [dockerInfo, setDockerInfo] = useState<DockerInfo>({
    version: '20.10.14',
    apiVersion: '1.41',
    totalContainers: 0,
    runningContainers: 0,
    stoppedContainers: 0,
    volumes: 0,
    images: 0,
    os: 'Linux',
    arch: 'x86_64',
    memory: '16GB',
    cpus: 8,
    networks: [],
    imageList: [],
    volumeList: []
  });
  const [containers, setContainers] = useState<DockerContainer[]>([]);
  const [selectedContainer, setSelectedContainer] = useState<DockerContainer | null>(null);
  const [isContainerActionModalVisible, setIsContainerActionModalVisible] = useState(false);
  const [containerAction, setContainerAction] = useState<'start' | 'stop' | 'restart' | 'delete' | null>(null);
  const [containerLoading, setContainerLoading] = useState(false);
  const [dockerServer, setDockerServer] = useState<DockerServer | null>(null);
  const [serverLoading, setServerLoading] = useState(false);
  const [isServerRegisterModalVisible, setIsServerRegisterModalVisible] = useState(false);
  const [serverForm] = Form.useForm();
  const [serverRegisterLoading, setServerRegisterLoading] = useState(false);
  const [checkingServerStatus, setCheckingServerStatus] = useState(false);
  const [installingDocker, setInstallingDocker] = useState(false);
  const [isAuthModalVisible, setIsAuthModalVisible] = useState(false);
  const [authForm] = Form.useForm();
  const [authLoading, setAuthLoading] = useState(false);
  const [authPurpose, setAuthPurpose] = useState<AuthPurpose>('install');
  const [uninstallingDocker, setUninstallingDocker] = useState(false);
  const [isUninstallModalVisible, setIsUninstallModalVisible] = useState(false);
  const [serverResource, setServerResource] = useState<ServerResource | null>(null);
  const [resourceLoading, setResourceLoading] = useState(false);

  // 컨테이너 목록 로드
  const loadContainers = async (serverId: number, authInfo?: {
    username?: string;
    password?: string;
    hops?: Array<{
      host: string;
      port: number;
      username: string;
      password: string;
    }>;
  }) => {
    if (!serverId) {
      setContainers([]);
      return;
    }
    
    try {
      setLoading(true);
      const response = await dockerApi.getContainers(serverId, authInfo ? { hops: authInfo.hops } : undefined);
      
      if (response.success) {
        // 컨테이너 목록 설정 (null 체크 추가)
        const containerList = Array.isArray(response.containers) ? response.containers : [];
        setContainers(containerList);
        
        // 도커 정보 업데이트 (null 체크 추가)
        setDockerInfo(prevInfo => ({
          ...prevInfo,
          totalContainers: response.container_count || containerList.length,
          runningContainers: containerList.filter(c => c.status === 'running').length,
          stoppedContainers: containerList.filter(c => c.status !== 'running').length,
          images: response.image_count || 0,
          volumes: Array.isArray(response.volumes) ? response.volumes.length : 0,
          networks: response.networks || [],
          imageList: response.images || [],
          volumeList: response.volumes || []
        }));
        
        // 컨테이너가 없는 경우 메시지 표시
        if (containerList.length === 0) {
          console.log('컨테이너가 없습니다.');
          message.info('서버에 실행 중인 도커 컨테이너가 없습니다.');
        }
      } else {
        setContainers([]);
        if ('error' in response && typeof response.error === 'string') {
          message.error(response.error || '컨테이너 목록을 가져오지 못했습니다.');
        } else {
          message.error('컨테이너 목록을 가져오지 못했습니다.');
        }
      }
    } catch (error) {
      console.error('컨테이너 목록 로드 실패:', error);
      message.error('컨테이너 목록을 불러오는데 실패했습니다.');
      setContainers([]);
    } finally {
      setLoading(false);
    }
  };

  // 도커 서버 정보 로드
  const loadDockerServerInfo = async () => {
    try {
      setServerLoading(true);
      setLoading(true);
      const response = await dockerApi.getDockerServer(infra.id);
      console.log("도커 서버 응답:", response);
      
      if (response.success && response.server) {
        // hops에서 IP 주소와 포트 추출
        let ip = '-';
        let port = '22';
        
        try {
          if (response.server.hops) {
            const hopsData = typeof response.server.hops === 'string' 
              ? JSON.parse(response.server.hops) 
              : response.server.hops;
            
            if (Array.isArray(hopsData) && hopsData.length > 0) {
              ip = hopsData[0].host || '-';
              port = hopsData[0].port ? String(hopsData[0].port) : '22';
            }
          }
        } catch (error) {
          console.error('hops 데이터 파싱 오류:', error);
        }
        
        // 서버 정보 구조 확인 및 필드 매핑
        const serverInfo: DockerServer = {
          ...response.server,
          name: response.server.server_name || response.server.name || '-',
          status: response.server.status || 'uninstalled', // 상태가 없으면 '미설치'로 설정
          ip: ip,
          port: port
        };
        console.log("설정할 도커 서버 정보:", serverInfo);
        setDockerServer(serverInfo);
        
        // 서버가 있으면 도커 정보도 로드 (active 상태인 경우만)
        if (serverInfo.status === 'active') {
          try {
            // 도커 정보 조회 API 호출 (getContainers 대신 getDockerInfo 사용)
            const dockerInfoResponse = await dockerApi.getDockerInfo(response.server.id);
            if (dockerInfoResponse.success && dockerInfoResponse.info) {
              // 도커 상세 정보 설정
              const dockerSystemInfo = dockerInfoResponse.info;
              
              // 도커 정보 업데이트
              setDockerInfo({
                version: dockerSystemInfo.version || '20.10.14',
                apiVersion: dockerSystemInfo.api_version || '1.41',
                totalContainers: dockerSystemInfo.containers || 0,
                runningContainers: dockerSystemInfo.running_containers || 0,
                stoppedContainers: dockerSystemInfo.stopped_containers || 0,
                volumes: dockerSystemInfo.volumes_count || 0,
                images: dockerSystemInfo.images_count || 0,
                os: dockerSystemInfo.os || 'Linux',
                arch: dockerSystemInfo.architecture || 'x86_64',
                memory: dockerSystemInfo.memory || '16GB',
                cpus: dockerSystemInfo.cpus || 8,
                networks: dockerSystemInfo.networks || [],
                imageList: dockerSystemInfo.images || [],
                volumeList: dockerSystemInfo.volumes || []
              });
              
              // 컨테이너 목록도 함께 로드
              if (dockerSystemInfo.containers_list) {
                // 컨테이너 데이터 변환
                const containers = dockerSystemInfo.containers_list.map((container: any) => {
                  // 상태 문자열에서 실제 상태 추출
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
                
                setContainers(containers);
              } else {
                // 컨테이너 목록이 없으면 별도로 조회
          await loadContainers(response.server.id);
              }
            } else {
              // getDockerInfo 실패시 기존 컨테이너 로드 방식으로 폴백
              await loadContainers(response.server.id);
            }
          } catch (error) {
            console.error("도커 정보 로드 실패:", error);
            // 오류 발생시 기존 컨테이너 로드 방식으로 폴백
            await loadContainers(response.server.id);
          }
        } else {
          setContainers([]);
        }
      } else {
        // 서버가 없는 경우 컨테이너 목록 초기화
        setContainers([]);
        setDockerServer(null);
        console.log("도커 서버가 등록되지 않았습니다.");
      }
    } catch (error) {
      message.error("도커 서버 정보를 불러오는데 실패했습니다.");
      console.error("도커 서버 정보 로드 오류:", error);
      setDockerServer(null);
    } finally {
      setLoading(false);
      setServerLoading(false);
    }
  };

  // 컴포넌트 마운트 시 데이터 로드
  useEffect(() => {
    loadDockerServerInfo();
  }, [infra.id]);

  // 컨테이너 작업 모달 표시
  const showContainerActionModal = (container: DockerContainer, action: 'start' | 'stop' | 'restart' | 'delete') => {
    setSelectedContainer(container);
    setContainerAction(action);
    setIsContainerActionModalVisible(true);
  };

  // 컨테이너 작업 모달 닫기
  const handleContainerActionCancel = () => {
    setIsContainerActionModalVisible(false);
    setSelectedContainer(null);
    setContainerAction(null);
  };

  // 컨테이너 작업 실행
  const handleContainerAction = async () => {
    if (!selectedContainer || !containerAction || !dockerServer) {
      return;
    }

    try {
      setContainerLoading(true);
      let successMessage = '';
      let response;

      // 도커 API를 사용하여 컨테이너 작업 실행
      switch (containerAction) {
        case 'start':
          response = await dockerApi.startContainer(dockerServer.id, selectedContainer.id);
          successMessage = `컨테이너 ${selectedContainer.name}가 시작되었습니다.`;
          break;
        case 'stop':
          response = await dockerApi.stopContainer(dockerServer.id, selectedContainer.id);
          successMessage = `컨테이너 ${selectedContainer.name}가 중지되었습니다.`;
          break;
        case 'restart':
          response = await dockerApi.restartContainer(dockerServer.id, selectedContainer.id);
          successMessage = `컨테이너 ${selectedContainer.name}가 재시작되었습니다.`;
          break;
        case 'delete':
          response = await dockerApi.removeContainer(dockerServer.id, selectedContainer.id);
          successMessage = `컨테이너 ${selectedContainer.name}가 삭제되었습니다.`;
          break;
      }

      if (response && response.success) {
        messageApi.success(successMessage);
      } else {
        throw new Error((response && response.error) || '작업 실패');
      }

      // 컨테이너 목록 다시 로드
      await loadDockerServerInfo();
      setIsContainerActionModalVisible(false);
    } catch (error) {
      console.error(`컨테이너 ${containerAction} 작업 중 오류 발생:`, error);
      messageApi.error(`컨테이너 작업에 실패했습니다.`);
    } finally {
      setContainerLoading(false);
      setSelectedContainer(null);
      setContainerAction(null);
    }
  };

  // 컨테이너 상태에 따른 태그 색상 반환
  const getStatusColor = (status: string) => {
    switch (status) {
      case 'running':
        return 'success';
      case 'stopped':
        return 'default';
      case 'paused':
        return 'warning';
      case 'exited':
        return 'error';
      default:
        return 'default';
    }
  };

  // 컨테이너 상태 텍스트 반환
  const getStatusText = (status: string) => {
    switch (status) {
      case 'running':
        return '실행 중';
      case 'stopped':
        return '중지됨';
      case 'paused':
        return '일시 정지';
      case 'exited':
        return '종료됨';
      default:
        return status;
    }
  };

  // 날짜 포맷팅 함수
  const formatDate = (dateString?: string): string => {
    if (!dateString) return '-';
    
    try {
      // KST 시간대를 UTC로 변환
      const date = new Date(dateString.replace('KST', ''));
      if (isNaN(date.getTime())) return '-';
      
      return new Intl.DateTimeFormat('ko-KR', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false
      }).format(date);
    } catch (error) {
      console.error('날짜 형식 변환 오류:', error);
      return '-';
    }
  };

  // 컨테이너 테이블 컬럼 정의
  const columns = [
    {
      title: '상태',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: string) => (
        <Tag color={getStatusColor(status)} style={{ minWidth: '60px', textAlign: 'center', fontWeight: 500 }}>
          {getStatusText(status)}
        </Tag>
      )
    },
    {
      title: '이름',
      dataIndex: 'name',
      key: 'name',
      render: (text: string) => <Text strong style={{ fontSize: '14px' }}>{text}</Text>
    },
    {
      title: '이미지',
      dataIndex: 'image',
      key: 'image',
      render: (text: string) => <Text style={{ fontSize: '14px', color: '#666' }}>{text}</Text>
    },
    {
      title: '포트',
      dataIndex: 'ports',
      key: 'ports',
      render: (ports: string[]) => (
        <span>
          {ports.map(port => (
            <Tag key={port} color="blue" style={{ margin: '2px' }}>{port}</Tag>
          ))}
        </span>
      )
    },
    {
      title: '크기',
      dataIndex: 'size',
      key: 'size',
      render: (text: string) => <Text style={{ fontSize: '14px', color: '#666' }}>{text}</Text>
    },
    {
      title: '생성일',
      dataIndex: 'created',
      key: 'created',
      render: (text: string) => <Text style={{ fontSize: '14px', color: '#666' }}>{formatDate(text)}</Text>
    },
    // {
    //   title: '작업',
    //   key: 'action',
    //   width: 120,
    //   render: (_: unknown, record: DockerContainer) => (
    //     <Space size="small">
    //       {record.status !== 'running' && (
    //         <Tooltip title="시작">
    //           <Button
    //             type="text"
    //             icon={<PlayCircleOutlined style={{ color: '#52c41a' }} />}
    //             onClick={() => showContainerActionModal(record, 'start')}
    //             size="small"
    //           />
    //         </Tooltip>
    //       )}
    //       {record.status === 'running' && (
    //         <>
    //           <Tooltip title="중지">
    //             <Button
    //               type="text"
    //               icon={<PauseCircleOutlined style={{ color: '#faad14' }} />}
    //               onClick={() => showContainerActionModal(record, 'stop')}
    //               size="small"
    //             />
    //           </Tooltip>
    //           <Tooltip title="재시작">
    //             <Button
    //               type="text"
    //               icon={<ReloadOutlined style={{ color: '#1890ff' }} />}
    //               onClick={() => showContainerActionModal(record, 'restart')}
    //               size="small"
    //             />
    //           </Tooltip>
    //         </>
    //       )}
    //       <Tooltip title="삭제">
    //         <Button
    //           type="text"
    //           danger
    //           icon={<DeleteOutlined />}
    //           onClick={() => showContainerActionModal(record, 'delete')}
    //           size="small"
    //         />
    //       </Tooltip>
    //     </Space>
    //   )
    // }
  ];

  // 컨테이너 작업 모달 제목 반환
  const getActionModalTitle = () => {
    if (!selectedContainer || !containerAction) return '';
    
    switch (containerAction) {
      case 'start':
        return `컨테이너 시작: ${selectedContainer.name}`;
      case 'stop':
        return `컨테이너 중지: ${selectedContainer.name}`;
      case 'restart':
        return `컨테이너 재시작: ${selectedContainer.name}`;
      case 'delete':
        return `컨테이너 삭제: ${selectedContainer.name}`;
      default:
        return '';
    }
  };

  // 컨테이너 작업 모달 메시지 반환
  const getActionModalMessage = () => {
    if (!selectedContainer || !containerAction) return '';
    
    switch (containerAction) {
      case 'start':
        return `'${selectedContainer.name}' 컨테이너를 시작하시겠습니까?`;
      case 'stop':
        return `'${selectedContainer.name}' 컨테이너를 중지하시겠습니까?`;
      case 'restart':
        return `'${selectedContainer.name}' 컨테이너를 재시작하시겠습니까?`;
      case 'delete':
        return `'${selectedContainer.name}' 컨테이너를 삭제하시겠습니까? 이 작업은 되돌릴 수 없습니다.`;
      default:
        return '';
    }
  };

  // 서버 등록 모달 표시
  const showServerRegisterModal = () => {
    serverForm.resetFields();
    setIsServerRegisterModalVisible(true);
  };

  // 서버 등록 모달 닫기
  const handleServerRegisterCancel = () => {
    setIsServerRegisterModalVisible(false);
  };

  // 서버 등록 처리
  const handleServerRegister = async () => {
    try {
      const values = await serverForm.validateFields();
      setServerRegisterLoading(true);

      // hops 데이터 생성 - 객체 배열 형태로
      const hopsData = [
        {
          host: values.ip,
          port: parseInt(values.port.toString())
        }
      ];

      // 도커 서버 생성 API 호출
      await dockerApi.createDockerServer({
        name: values.name,
        infra_id: infra.id,
        ip: values.ip,
        port: parseInt(values.port || "22"),
        status: "inactive", // 초기 상태는 비활성
        hops: hopsData
      });

      messageApi.success('도커 서버가 등록되었습니다.');
      setIsServerRegisterModalVisible(false);
      
      // 서버 정보 다시 로드
      loadDockerServerInfo();
    } catch (error) {
      console.error('도커 서버 등록 중 오류 발생:', error);
      
      if (error instanceof Error && 'errorFields' in error) {
        messageApi.error('필수 입력 항목을 모두 채워주세요.');
      } else {
        messageApi.error('도커 서버 등록에 실패했습니다.');
      }
    } finally {
      setServerRegisterLoading(false);
    }
  };

  // 상태 확인 버튼 클릭 처리
  const checkServerStatus = async () => {
    if (!dockerServer || !dockerServer.id) {
      messageApi.error('서버 정보가 없습니다.');
      return;
    }

    // 인증 모달 표시 (상태 조회 목적)
    setAuthPurpose('status');
    setIsAuthModalVisible(true);
  };
  
  // 상태 확인 후 도커 정보 새로고침
  const refreshDockerInfo = async (authInfo: {
    username: string;
    password: string;
    hops: Array<{
      host: string;
      port: number;
      username: string;
      password: string;
    }>;
  }) => {
    if (!dockerServer || !dockerServer.id) {
      messageApi.error('서버 정보가 없습니다.');
      return;
    }
    
    try {
      setLoading(true);
      
      // 도커 정보 조회 API 호출
      const dockerInfoResponse = await dockerApi.getDockerInfo(dockerServer.id);
      if (dockerInfoResponse.success && dockerInfoResponse.info) {
        // 도커 상세 정보 설정
        const dockerSystemInfo = dockerInfoResponse.info;
        
        // 도커 정보 업데이트
        setDockerInfo({
          version: dockerSystemInfo.version || '20.10.14',
          apiVersion: dockerSystemInfo.api_version || '1.41',
          totalContainers: dockerSystemInfo.containers || 0,
          runningContainers: dockerSystemInfo.running_containers || 0,
          stoppedContainers: dockerSystemInfo.stopped_containers || 0,
          volumes: dockerSystemInfo.volumes_count || 0,
          images: dockerSystemInfo.images_count || 0,
          os: dockerSystemInfo.os || 'Linux',
          arch: dockerSystemInfo.architecture || 'x86_64',
          memory: dockerSystemInfo.memory || '16GB',
          cpus: dockerSystemInfo.cpus || 8,
          networks: dockerSystemInfo.networks || [],
          imageList: dockerSystemInfo.images || [],
          volumeList: dockerSystemInfo.volumes || []
        });
        
        // 컨테이너 목록도 함께 로드
        if (dockerSystemInfo.containers_list) {
          // 컨테이너 데이터 변환
          const containers = dockerSystemInfo.containers_list.map((container: any) => {
            // 상태 문자열에서 실제 상태 추출
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
          
          setContainers(containers);
        } else {
          // 컨테이너 목록이 없으면 별도로 조회
          await loadContainers(dockerServer.id, authInfo);
        }
      } else {
        // getDockerInfo 실패시 기존 컨테이너 로드 방식으로 폴백
        await loadContainers(dockerServer.id, authInfo);
      }
    } catch (error) {
      console.error("도커 정보 새로고침 실패:", error);
      messageApi.error("도커 정보를 새로고침하는데 실패했습니다.");
      // 오류 발생시 기존 컨테이너 로드 방식으로 폴백
      await loadContainers(dockerServer.id, authInfo);
    } finally {
      setLoading(false);
    }
  };

  // 도커 설치
  const installDocker = async () => {
    if (!dockerServer || !dockerServer.id) {
      messageApi.error('서버 정보가 없습니다.');
      return;
    }

    // 인증 모달 표시 (설치 목적)
    setAuthPurpose('install');
    setIsAuthModalVisible(true);
  };

  // 도커 제거 확인 모달 표시
  const showUninstallModal = () => {
    setIsUninstallModalVisible(true);
  };

  // 도커 제거 확인 모달 닫기
  const handleUninstallCancel = () => {
    setIsUninstallModalVisible(false);
  };

  // 도커 제거 함수
  const uninstallDocker = async () => {
    if (!dockerServer || !dockerServer.id) {
      messageApi.error('서버 정보가 없습니다.');
      return;
    }

    // 인증 모달 표시 (제거 목적)
    setAuthPurpose('uninstall');
    setIsAuthModalVisible(true);
    setIsUninstallModalVisible(false);
  };

  // 서버 리소스 가져오기 함수 추가
  const getServerResource = async (authInfo: {
    username: string;
    password: string;
    hops: Array<{
      host: string;
      port: number;
      username: string;
      password: string;
    }>;
  }) => {
    if (!dockerServer || !dockerServer.id) {
      messageApi.error('서버 정보가 없습니다.');
      return;
    }

    try {
      setResourceLoading(true);
      
      // 리소스 계산 API 호출
      const response = await kubernetesApi.calculateResources({
        id: infra.id,
        hops: authInfo.hops
      });
      
      if (response && response.success) {
        // 리소스 데이터 저장
        const resourceData: ServerResource = {
          host_info: response.host_info,
          cpu: response.cpu,
          memory: response.memory,
          disk: response.disk
        };
        setServerResource(resourceData);
        
        // 성공 메시지
        messageApi.success('서버 리소스 정보를 성공적으로 가져왔습니다.');
      } else {
        messageApi.error('서버 리소스 조회에 실패했습니다.');
      }
    } catch (error) {
      console.error('서버 리소스 조회 오류:', error);
      messageApi.error('서버 리소스 정보를 가져오는데 실패했습니다.');
    } finally {
      setResourceLoading(false);
    }
  };

  // 인증 정보를 사용한 작업 실행을 수정하여 리소스 조회 추가
  const handleAuthSubmit = async () => {
    if (!dockerServer || !dockerServer.id) {
      messageApi.error('서버 정보가 없습니다.');
      return;
    }

    try {
      const values = await authForm.validateFields();
      setAuthLoading(true);

      // 이미 저장된 hops 정보 가져오기
      let hopsData = [];
      try {
        if (dockerServer.hops) {
          const parsedHops = typeof dockerServer.hops === 'string' 
            ? JSON.parse(dockerServer.hops) 
            : dockerServer.hops;
          
          // hops 정보가 있으면 사용자 이름과 비밀번호 업데이트
          if (Array.isArray(parsedHops) && parsedHops.length > 0) {
            hopsData = parsedHops.map(hop => ({
              ...hop,
              username: values.username,
              password: values.password
            }));
          }
        }
      } catch (error) {
        console.error('hops 데이터 파싱 오류:', error);
      }

      // hops 데이터가 비어있으면 서버 IP와 포트로 생성
      if (hopsData.length === 0 && dockerServer.ip) {
        hopsData = [{
          host: dockerServer.ip,
          port: parseInt(dockerServer.port || "22"),
          username: values.username,
          password: values.password
        }];
      }

      // 인증 목적에 따라 다른 작업 수행
      if (authPurpose === 'install') {
        // 도커 설치 API 호출
        setInstallingDocker(true);
        
        // API 호출은 백그라운드로 진행하고 바로 모달을 닫음
        dockerApi.installDocker(dockerServer.id, {
          username: values.username,
          password: values.password,
          hops: hopsData
        }).catch(error => {
          console.error('도커 설치 중 오류 발생:', error);
          messageApi.error('도커 설치 중 오류가 발생했습니다.');
        });
        
        // 즉시 성공 메시지 표시 및 모달 닫기
        messageApi.success('도커 설치가 백그라운드에서 진행 중입니다. 잠시 후 상태를 확인해주세요.');
        setIsAuthModalVisible(false);
          
          // 서버 상태 업데이트
          setDockerServer({
            ...dockerServer,
          status: 'inactive' // 설치 중 상태는 비활성
          });

        setInstallingDocker(false);
      } else if (authPurpose === 'status') {
        // 도커 상태 확인 API 호출
        setCheckingServerStatus(true);
        const response = await dockerApi.checkDockerServerStatus(dockerServer.id, {
          username: values.username,
          password: values.password,
          hops: hopsData
        });
        
        if (response.success) {
          // 서버 상태 업데이트 (lastChecked가 undefined일 경우 현재 시간 사용)
          const lastChecked = response.lastChecked || new Date().toISOString();
          
          setDockerServer({
            ...dockerServer,
            status: response.status || 'uninstalled',
            lastChecked: lastChecked
          });
          
          // 상태에 따른 메시지 표시
          if (response.status === 'active') {
            messageApi.success('도커 서버가 활성 상태입니다.');
            // 활성 상태인 경우 도커 정보와 컨테이너 목록 로드 (refreshDockerInfo 호출)
            await refreshDockerInfo({
              username: values.username,
              password: values.password,
              hops: hopsData
            });
            
            // 서버 리소스 정보도 함께 가져오기
            await getServerResource({
              username: values.username,
              password: values.password,
              hops: hopsData
            });
          } else if (response.status === 'inactive') {
            messageApi.warning('도커가 설치되었지만 실행 중이 아닙니다.');
          } else {
            messageApi.warning('도커가 설치되어 있지 않습니다.');
          }
          
          // 인증 모달 닫기
          setIsAuthModalVisible(false);
        } else {
          throw new Error(response.error || '서버 상태 조회에 실패했습니다.');
        }
        setCheckingServerStatus(false);
      } else if (authPurpose === 'uninstall') {
        // 도커 제거 API 호출
        setUninstallingDocker(true);
        
        // API 호출은 백그라운드로 진행하고 바로 모달을 닫음
        dockerApi.uninstallDocker(dockerServer.id, {
          username: values.username,
          password: values.password,
          hops: hopsData
        }).catch(error => {
          console.error('도커 제거 중 오류 발생:', error);
          messageApi.error('도커 제거 중 오류가 발생했습니다.');
        });
        
        // 즉시 성공 메시지 표시 및 모달 닫기
        messageApi.success('도커 제거가 백그라운드에서 진행 중입니다. 잠시 후 상태를 확인해주세요.');
        setIsAuthModalVisible(false);
          
          // 서버 상태 업데이트
          setDockerServer({
            ...dockerServer,
            status: 'uninstalled'
          });

        setUninstallingDocker(false);
      }
    } catch (error) {
      console.error('인증 처리 오류:', error);
      if (authPurpose === 'install') {
        messageApi.error('도커 설치에 실패했습니다.');
        setInstallingDocker(false);
      } else if (authPurpose === 'uninstall') {
        messageApi.error('도커 제거에 실패했습니다.');
        setUninstallingDocker(false);
      } else {
        messageApi.error('서버 상태 조회에 실패했습니다.');
        setCheckingServerStatus(false);
      }
    } finally {
      setAuthLoading(false);
    }
  };

  // 인증 모달 취소
  const handleAuthCancel = () => {
    setIsAuthModalVisible(false);
    authForm.resetFields();
    
    // 진행 중인 작업 상태 초기화
    if (authPurpose === 'install') {
      setInstallingDocker(false);
    } else if (authPurpose === 'status') {
      setCheckingServerStatus(false);
    } else if (authPurpose === 'uninstall') {
      setUninstallingDocker(false);
    }
  };

  // 인증 모달 제목 반환
  const getAuthModalTitle = () => {
    switch (authPurpose) {
      case 'install':
        return '도커 설치를 위한 서버 인증 정보';
      case 'status':
        return '서버 상태 확인을 위한 인증 정보';
      case 'uninstall':
        return '도커 제거를 위한 서버 인증 정보';
      default:
        return '서버 인증 정보';
    }
  };

  // 인증 모달 확인 버튼 텍스트 반환
  const getAuthModalOkText = () => {
    switch (authPurpose) {
      case 'install':
        return '설치 시작';
      case 'status':
        return '상태 확인';
      case 'uninstall':
        return '제거 시작';
      default:
        return '확인';
    }
  };

  // 서버 상태에 따른 버튼 렌더링
  const renderServerActionButtons = () => {
    if (!dockerServer) return null;

    return (
      <Space>
        <Button
          onClick={checkServerStatus}
          loading={checkingServerStatus}
          icon={<SyncOutlined />}
        >
          상태 확인
        </Button>
        
        {dockerServer.status === 'uninstalled' && (
          <Button
            type="primary"
            onClick={installDocker}
            loading={installingDocker}
            icon={<DownloadOutlined />}
          >
            도커 설치
          </Button>
        )}

        {dockerServer.status === 'active' && (
          <Button
            danger
            onClick={showUninstallModal}
            loading={uninstallingDocker}
            icon={<DeleteOutlined />}
          >
            도커 제거
          </Button>
        )}
      </Space>
    );
  };

  // 서버 상태에 따른 태그 색상 및 텍스트
  const getServerStatusTag = () => {
    if (!dockerServer) return null;

    let color = '';
    let text = '';

    switch (dockerServer.status) {
      case 'active':
        color = 'success';
        text = '활성';
        break;
      case 'inactive':
        color = 'warning';
        text = '비활성';
        break;
      case 'uninstalled':
        color = 'default';
        text = '미설치';
        break;
      default:
        color = 'default';
        text = '알 수 없음';
    }

    return (
      <div>
        <Tag color={color} style={{ fontSize: '14px', padding: '4px 8px' }}>{text}</Tag>
        {dockerServer.lastChecked && (
          <div style={{ fontSize: '12px', color: '#888', marginTop: '4px' }}>
            마지막 확인: {formatDate(dockerServer.lastChecked)}
          </div>
        )}
      </div>
    );
  };

  // 리소스 카드 렌더링 함수 추가
  const renderResourceCards = () => {
    if (!serverResource) return null;

    return (
      <div className="resource-cards" style={{ marginTop: '24px' }}>
        <Divider orientation="left">
          <span style={{ fontSize: '16px', fontWeight: 600 }}>서버 리소스 정보</span>
        </Divider>
        <Row gutter={[24, 24]}>
          <Col span={24}>
            <Card 
              size="small"
              title={<span style={{ fontSize: '15px', fontWeight: 600 }}>시스템 정보</span>}
              bodyStyle={{ padding: '16px' }}
              style={{ marginBottom: '16px' }}
            >
              <Row gutter={[32, 16]}>
                <Col span={8}>
                  <Statistic 
                    title={<span style={{ fontSize: '14px', color: '#666' }}>호스트명</span>}
                    value={serverResource.host_info.hostname} 
                    valueStyle={{ fontSize: '16px', fontWeight: 500 }}
                  />
                </Col>
                <Col span={8}>
                  <Statistic 
                    title={<span style={{ fontSize: '14px', color: '#666' }}>운영체제</span>}
                    value={serverResource.host_info.os} 
                    valueStyle={{ fontSize: '16px', fontWeight: 500 }}
                  />
                </Col>
                <Col span={8}>
                  <Statistic 
                    title={<span style={{ fontSize: '14px', color: '#666' }}>커널</span>}
                    value={serverResource.host_info.kernel} 
                    valueStyle={{ fontSize: '16px', fontWeight: 500 }}
                  />
                </Col>
              </Row>
            </Card>
          </Col>
          
          <Col span={8}>
            <Card 
              size="small"
              title={<span style={{ fontSize: '15px', fontWeight: 600 }}>CPU</span>}
              bodyStyle={{ padding: '16px' }}
            >
              <Row gutter={[16, 16]}>
                <Col span={12}>
                  <Statistic 
                    title={<span style={{ fontSize: '14px', color: '#666' }}>코어</span>}
                    value={serverResource.cpu.cores} 
                    valueStyle={{ fontSize: '16px', fontWeight: 500 }}
                  />
                </Col>
                <Col span={12}>
                  <Statistic 
                    title={<span style={{ fontSize: '14px', color: '#666' }}>사용량</span>}
                    value={serverResource.cpu.usage_percent} 
                    suffix="%" 
                    valueStyle={{ 
                      color: parseInt(serverResource.cpu.usage_percent) > 80 ? '#cf1322' : 
                             parseInt(serverResource.cpu.usage_percent) > 60 ? '#faad14' : '#3f8600',
                      fontSize: '16px',
                      fontWeight: 500
                    }}
                  />
                </Col>
                <Col span={24} style={{ marginTop: '8px' }}>
                  <div style={{ fontSize: '14px', color: '#666' }}>모델</div>
                  <div style={{ fontSize: '14px', fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {serverResource.cpu.model}
                  </div>
                </Col>
              </Row>
            </Card>
          </Col>
          
          <Col span={8}>
            <Card 
              size="small"
              title={<span style={{ fontSize: '15px', fontWeight: 600 }}>메모리</span>}
              bodyStyle={{ padding: '16px' }}
            >
              <Row gutter={[16, 16]}>
                <Col span={12}>
                  <Statistic 
                    title={<span style={{ fontSize: '14px', color: '#666' }}>전체</span>}
                    value={`${Math.round(parseInt(serverResource.memory.total_mb) / 1024)} GB`} 
                    valueStyle={{ fontSize: '16px', fontWeight: 500 }}
                  />
                </Col>
                <Col span={12}>
                  <Statistic 
                    title={<span style={{ fontSize: '14px', color: '#666' }}>사용 중</span>}
                    value={`${Math.round(parseInt(serverResource.memory.used_mb) / 1024 * 10) / 10} GB`} 
                    valueStyle={{ fontSize: '16px', fontWeight: 500 }}
                  />
                </Col>
                <Col span={24} style={{ marginTop: '8px' }}>
                  <Statistic 
                    title={<span style={{ fontSize: '14px', color: '#666' }}>사용량</span>}
                    value={serverResource.memory.usage_percent} 
                    suffix="%" 
                    valueStyle={{ 
                      color: parseInt(serverResource.memory.usage_percent) > 80 ? '#cf1322' : 
                             parseInt(serverResource.memory.usage_percent) > 60 ? '#faad14' : '#3f8600',
                      fontSize: '16px',
                      fontWeight: 500
                    }}
                  />
                </Col>
              </Row>
            </Card>
          </Col>
          
          <Col span={8}>
            <Card 
              size="small"
              title={<span style={{ fontSize: '15px', fontWeight: 600 }}>디스크</span>}
              bodyStyle={{ padding: '16px' }}
            >
              <Row gutter={[16, 16]}>
                <Col span={12}>
                  <Statistic 
                    title={<span style={{ fontSize: '14px', color: '#666' }}>전체</span>}
                    value={serverResource.disk.root_total} 
                    valueStyle={{ fontSize: '16px', fontWeight: 500 }}
                  />
                </Col>
                <Col span={12}>
                  <Statistic 
                    title={<span style={{ fontSize: '14px', color: '#666' }}>사용 중</span>}
                    value={serverResource.disk.root_used} 
                    valueStyle={{ fontSize: '16px', fontWeight: 500 }}
                  />
                </Col>
                <Col span={24} style={{ marginTop: '8px' }}>
                  <Statistic 
                    title={<span style={{ fontSize: '14px', color: '#666' }}>사용량</span>}
                    value={serverResource.disk.root_usage_percent} 
                    suffix="%" 
                    valueStyle={{ 
                      color: parseInt(serverResource.disk.root_usage_percent) > 80 ? '#cf1322' : 
                             parseInt(serverResource.disk.root_usage_percent) > 60 ? '#faad14' : '#3f8600',
                      fontSize: '16px',
                      fontWeight: 500
                    }}
                  />
                </Col>
              </Row>
            </Card>
          </Col>
        </Row>
      </div>
    );
  };

  return (
    <>
      {contextHolder}
      <div className="infra-content-wrapper">
        <Row gutter={[16, 16]}>          
          <Col span={24}>
            <Card 
              title={<span style={{ fontSize: '16px', fontWeight: 600 }}>도커 호스트 서버</span>} 
              bordered={false}
              bodyStyle={{ padding: '16px' }}
              extra={dockerServer && renderServerActionButtons()}
            >
              {serverLoading ? (
                <div className="loading-container">
                  <Spin size="large" tip="서버 정보 로딩 중..." />
                </div>
              ) : dockerServer ? (
                <>
                  <Row gutter={[32, 16]}>
                    <Col span={6}>
                      <Statistic 
                        title={<span style={{ fontSize: '14px', color: '#666' }}>서버 이름</span>}
                        value={dockerServer.name || dockerServer.server_name || '-'} 
                        valueStyle={{ fontSize: '18px', fontWeight: 500 }}
                      />
                    </Col>
                    <Col span={6}>
                      <Statistic 
                        title={<span style={{ fontSize: '14px', color: '#666' }}>IP 주소</span>}
                        value={dockerServer.ip || '-'} 
                        valueStyle={{ fontSize: '18px', fontWeight: 500 }}
                      />
                    </Col>
                    <Col span={6}>
                      <Statistic 
                        title={<span style={{ fontSize: '14px', color: '#666' }}>포트</span>}
                        value={dockerServer.port || '22'} 
                        valueStyle={{ fontSize: '18px', fontWeight: 500 }}
                      />
                    </Col>
                    <Col span={6}>
                      <div>
                        <div style={{ fontSize: '14px', color: '#666', marginBottom: '8px' }}>상태</div>
                        {getServerStatusTag()}
                      </div>
                    </Col>
                  </Row>
                  
                  {/* 서버 리소스 정보 표시 */}
                  {resourceLoading ? (
                    <div style={{ textAlign: 'center', padding: '20px' }}>
                      <Spin size="small" />
                      <div style={{ marginTop: '10px' }}>리소스 정보 로딩 중...</div>
                    </div>
                  ) : serverResource && (
                    renderResourceCards()
                  )}
                </>
              ) : (
                <Empty 
                  description="등록된 도커 서버가 없습니다." 
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                >
                  <Button 
                    type="primary" 
                    onClick={showServerRegisterModal}
                    icon={<PlusOutlined />}
                  >
                    도커 서버 등록
                  </Button>
                </Empty>
              )}
            </Card>
          </Col>
          
          {/* 도커 서버가 있는 경우에만 도커 정보와 컨테이너 목록을 표시 */}
          {dockerServer && dockerServer.status === 'active' && (
            <>
              <Col span={24}>
                <Card 
                  title={<span style={{ fontSize: '16px', fontWeight: 600 }}>도커 정보</span>} 
                  bordered={false}
                  bodyStyle={{ padding: '16px' }}
                  // extra={
                  //   <Button 
                  //     icon={<ReloadOutlined />} 
                  //     onClick={() => dockerServer && dockerServer.status === 'active' && loadContainers(dockerServer.id)}
                  //     loading={loading}
                  //   >
                  //     새로고침
                  //   </Button>
                  // }
                >
                  <Row gutter={[32, 16]}>
                    <Col span={6}>
                      <Statistic 
                        title={<span style={{ fontSize: '14px', color: '#666' }}>도커 버전</span>}
                        value={dockerInfo.version} 
                        valueStyle={{ fontSize: '18px', fontWeight: 500 }}
                      />
                    </Col>
                    <Col span={6}>
                      <Statistic 
                        title={<span style={{ fontSize: '14px', color: '#666' }}>컨테이너</span>}
                        value={dockerInfo.totalContainers} 
                        suffix={<span style={{ fontSize: '14px', color: '#52c41a' }}>{dockerInfo.runningContainers > 0 ? `실행 중: ${dockerInfo.runningContainers}` : ''}</span>}
                        valueStyle={{ fontSize: '18px', fontWeight: 500 }}
                      />
                    </Col>
                    <Col span={6}>
                      <Statistic 
                        title={<span style={{ fontSize: '14px', color: '#666' }}>이미지</span>}
                        value={dockerInfo.images} 
                        valueStyle={{ fontSize: '18px', fontWeight: 500 }}
                      />
                    </Col>
                    <Col span={6}>
                      <Statistic 
                        title={<span style={{ fontSize: '14px', color: '#666' }}>볼륨</span>}
                        value={dockerInfo.volumes || 0} 
                        valueStyle={{ fontSize: '18px', fontWeight: 500 }}
                      />
                    </Col>
                  </Row>
                </Card>
              </Col>

              <Col span={24}>
                <Card 
                  title={<span style={{ fontSize: '16px', fontWeight: 600 }}>컨테이너 목록</span>}
                  bordered={false}
                  bodyStyle={{ padding: '16px' }}
                >
                  <Table 
                    dataSource={containers || []} 
                    columns={columns} 
                    rowKey="id"
                    pagination={{ pageSize: 10 }}
                    loading={loading}
                    size="middle"
                    className="container-table"
                    rowClassName={() => "container-table-row"}
                    style={{ borderRadius: '8px', overflow: 'hidden' }}
                  />
                </Card>
              </Col>

              {/* 이미지 목록 */}
              <Col span={24}>
                <Card 
                  title={<span style={{ fontSize: '16px', fontWeight: 600 }}>이미지 목록</span>}
                  bordered={false}
                  bodyStyle={{ padding: '16px' }}
                >
                  <Table 
                    dataSource={Array.isArray(dockerInfo.imageList) ? dockerInfo.imageList : []} 
                    columns={[
                      {
                        title: '이미지',
                        dataIndex: 'repository',
                        key: 'repository',
                        render: (text: string) => <Text strong style={{ fontSize: '14px' }}>{text}</Text>
                      },
                      {
                        title: '태그',
                        dataIndex: 'tag',
                        key: 'tag',
                        render: (text: string) => <Text style={{ fontSize: '14px', color: '#666' }}>{text}</Text>
                      },
                      {
                        title: '크기',
                        dataIndex: 'size',
                        key: 'size',
                        render: (text: string) => <Text style={{ fontSize: '14px', color: '#666' }}>{text}</Text>
                      },
                      {
                        title: '생성일',
                        dataIndex: 'created',
                        key: 'created',
                        render: (text: string) => <Text style={{ fontSize: '14px', color: '#666' }}>{text}</Text>
                      }
                    ]}
                    rowKey={(record) => `${record.repository}-${record.tag}-${record.size}-${record.created}`}
                    pagination={{ pageSize: 10 }}
                    loading={loading}
                    size="middle"
                  />
                </Card>
              </Col>

              {/* 네트워크 목록 */}
              <Col span={24}>
                <Card 
                  title={<span style={{ fontSize: '16px', fontWeight: 600 }}>네트워크 목록</span>}
                  bordered={false}
                  bodyStyle={{ padding: '16px' }}
                >
                  <Table 
                    dataSource={Array.isArray(dockerInfo.networks) ? dockerInfo.networks : []} 
                    columns={[
                      {
                        title: '이름',
                        dataIndex: 'name',
                        key: 'name',
                        render: (text: string) => <Text strong style={{ fontSize: '14px' }}>{text}</Text>
                      },
                      {
                        title: '드라이버',
                        dataIndex: 'driver',
                        key: 'driver',
                        render: (text: string) => <Text style={{ fontSize: '14px', color: '#666' }}>{text}</Text>
                      },
                      {
                        title: '범위',
                        dataIndex: 'scope',
                        key: 'scope',
                        render: (text: string) => <Text style={{ fontSize: '14px', color: '#666' }}>{text}</Text>
                      }
                    ]}
                    rowKey={(record, index) => record.name || `network-${index}`}
                    pagination={{ pageSize: 10 }}
                    loading={loading}
                    size="middle"
                  />
                </Card>
              </Col>

              {/* 볼륨 목록 */}
              <Col span={24}>
                <Card 
                  title={<span style={{ fontSize: '16px', fontWeight: 600 }}>볼륨 목록</span>}
                  bordered={false}
                  bodyStyle={{ padding: '16px' }}
                >
                  <Table 
                    dataSource={Array.isArray(dockerInfo.volumeList) ? dockerInfo.volumeList : []} 
                    columns={[
                      {
                        title: '이름',
                        dataIndex: 'name',
                        key: 'name',
                        render: (text: string) => <Text strong style={{ fontSize: '14px' }}>{text}</Text>
                      },
                      {
                        title: '드라이버',
                        dataIndex: 'driver',
                        key: 'driver',
                        render: (text: string) => <Text style={{ fontSize: '14px', color: '#666' }}>{text}</Text>
                      },
                      {
                        title: '크기',
                        dataIndex: 'size',
                        key: 'size',
                        render: (text: string) => <Text style={{ fontSize: '14px', color: '#666' }}>{text}</Text>
                      }
                    ]}
                    rowKey={(record) => record.name || Math.random().toString()}
                    pagination={{ pageSize: 10 }}
                    loading={loading}
                    size="middle"
                  />
                </Card>
              </Col>
            </>
          )}
        </Row>
      </div>

      {/* 컨테이너 작업 모달 */}
      <Modal
        title={getActionModalTitle()}
        open={isContainerActionModalVisible}
        onCancel={handleContainerActionCancel}
        onOk={handleContainerAction}
        okText={containerAction === 'delete' ? '삭제' : '확인'}
        cancelText="취소"
        confirmLoading={containerLoading}
        maskClosable={!containerLoading}
      >
        <p>{getActionModalMessage()}</p>
        {containerAction === 'delete' && (
          <div style={{ marginTop: 16 }}>
            <Text type="danger" strong>
              경고: 이 작업은 컨테이너의 모든 데이터를 삭제합니다. 볼륨에 저장되지 않은 데이터는 복구할 수 없습니다.
            </Text>
          </div>
        )}
      </Modal>

      {/* 서버 등록 모달 */}
      <Modal
        title="도커 서버 등록"
        open={isServerRegisterModalVisible}
        onCancel={handleServerRegisterCancel}
        onOk={handleServerRegister}
        okText="등록"
        cancelText="취소"
        confirmLoading={serverRegisterLoading}
        maskClosable={!serverRegisterLoading}
      >
        <Form
          form={serverForm}
          layout="vertical"
        >
          <Form.Item
            name="name"
            label="서버 이름"
            rules={[{ required: true, message: '서버 이름을 입력해주세요' }]}
          >
            <Input placeholder="서버 이름을 입력하세요" />
          </Form.Item>
          <Form.Item
            name="ip"
            label="IP 주소"
            rules={[{ required: true, message: 'IP 주소를 입력해주세요' }]}
          >
            <Input placeholder="예: 192.168.0.100" />
          </Form.Item>
          <Form.Item
            name="port"
            label="SSH 포트"
            initialValue="22"
          >
            <Input placeholder="기본값: 22" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 인증 모달 추가 */}
      <Modal
        title={getAuthModalTitle()}
        open={isAuthModalVisible}
        onCancel={handleAuthCancel}
        onOk={handleAuthSubmit}
        okText={getAuthModalOkText()}
        cancelText="취소"
        confirmLoading={authLoading}
        maskClosable={!authLoading}
      >
        <p>{authPurpose === 'install' ? '도커를 설치하기 위한 서버 접속 정보를 입력해주세요.' : '서버 상태를 확인하기 위한 접속 정보를 입력해주세요.'}</p>
        <Form
          form={authForm}
          layout="vertical"
        >
          <Form.Item
            name="username"
            label="SSH 사용자 이름"
            rules={[{ required: true, message: 'SSH 사용자 이름을 입력해주세요' }]}
          >
            <Input placeholder="예: root, ubuntu" />
          </Form.Item>
          <Form.Item
            name="password"
            label="SSH 비밀번호"
            rules={[{ required: true, message: 'SSH 비밀번호를 입력해주세요' }]}
          >
            <Input.Password placeholder="SSH 접속 비밀번호" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 도커 제거 확인 모달 */}
      <Modal
        title="도커 제거 확인"
        open={isUninstallModalVisible}
        onCancel={handleUninstallCancel}
        onOk={uninstallDocker}
        okText="제거 시작"
        cancelText="취소"
        okButtonProps={{ danger: true }}
      >
        <p>도커를 제거하시겠습니까? 이 작업은 되돌릴 수 없습니다.</p>
        <p>다음 작업이 수행됩니다:</p>
        <ul>
          <li>모든 컨테이너 중지 및 삭제</li>
          <li>모든 이미지 삭제</li>
          <li>모든 볼륨 삭제</li>
          <li>모든 네트워크 삭제 (기본 네트워크 제외)</li>
          <li>도커 서비스 중지 및 제거</li>
        </ul>
        <Text type="danger" strong>
          경고: 이 작업은 모든 도커 데이터를 삭제합니다. 복구할 수 없습니다.
        </Text>
      </Modal>
    </>
  );
};

export default InfraDockerSetting; 