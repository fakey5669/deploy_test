'use client';

import React, { useState, useEffect } from 'react';
import { Form, message, Button, Modal, Input, Space, Typography, Descriptions, Spin, Table, Tag, Badge, Switch, Select } from 'antd';
import { LinkOutlined, UserOutlined, KeyOutlined, CopyOutlined, CloudServerOutlined, BranchesOutlined } from '@ant-design/icons';
import { Service, KubernetesStatus } from '../types/service';
import './Services.css';
import api from '../services/api';
import * as serviceApi from '../lib/api/service';
import * as kubernetesApi from '../lib/api/kubernetes';
import * as dockerApi from '../lib/api/docker';

// 컴포넌트 임포트
import ServiceTag from '../components/services/ServiceTag';
import ServiceCounter from '../components/services/ServiceCounter';
import ServiceActions from '../components/services/ServiceActions';
import ServiceTable from '../components/services/ServiceTable';
import ServiceFormModal from '../components/services/ServiceFormModal';
import ServiceDetailModal from '../components/services/ServiceDetailModal';
import DockerSettingsModal from '../components/services/DockerSettingsModal';

// API URL (실제로는 환경 변수에서 가져옴)
const API_BASE_URL = process.env.REACT_APP_API_URL || 'http://localhost:8080/api';

const { Text } = Typography;
const { Option } = Select;

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

// hops에서 추출한 호스트 정보 인터페이스
interface HopInfo {
  host: string;
  port: number;
}

// 인터페이스 정의 부분에 도커 컨테이너 인터페이스 추가
interface DockerContainer {
  name: string;
  status: string;
  image: string;
  created: string;
  ports: string;
}

// 추가: 외부 서비스 조회를 위한 인터페이스
interface ExternalServiceFetchParams {
  infraType: string;
  hops: Array<{
    host: string;
    port: number;
    username: string;
    password: string;
  }>;
}

const Services: React.FC = () => {
  // 상태 관리
  const [services, setServices] = useState<Service[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [searchTerm, setSearchTerm] = useState<string>('');
  const [modalVisible, setModalVisible] = useState<boolean>(false);
  const [detailModalVisible, setDetailModalVisible] = useState<boolean>(false);
  const [dockerSettingsModalVisible, setDockerSettingsModalVisible] = useState<boolean>(false);
  const [editMode, setEditMode] = useState<boolean>(false);
  const [currentService, setCurrentService] = useState<Service | null>(null);
  const [messageApi, contextHolder] = message.useMessage();
  const [form] = Form.useForm();
  const [currentPage, setCurrentPage] = useState<number>(1);
  const [pageSize, setPageSize] = useState<number>(10);
  const [gitlabModalVisible, setGitlabModalVisible] = useState<boolean>(false);
  const [selectedServiceName, setSelectedServiceName] = useState<string>('');
  const [selectedGitlabUrl, setSelectedGitlabUrl] = useState<string>('');
  const [statusModalVisible, setStatusModalVisible] = useState<boolean>(false);
  const [infraModalVisible, setInfraModalVisible] = useState<boolean>(false);
  const [selectedInfraId, setSelectedInfraId] = useState<number | null>(null);
  const [selectedInfraInfo, setSelectedInfraInfo] = useState<any>(null);
  const [serviceStatus, setServiceStatus] = useState<{
    id: number, 
    name: string, 
    gitlab_url: string, 
    namespace: string, 
    kubernetesStatus: KubernetesStatus
  } | null>(null);
  const [authModalVisible, setAuthModalVisible] = useState<boolean>(false);
  const [selectedServer, setSelectedServer] = useState<Server | null>(null);
  const [authForm] = Form.useForm();
  const [authModalType, setAuthModalType] = useState<'status' | 'deploy'>('status');
  const [infraTypes] = useState<string[]>(['kubernetes', 'docker']);

  // 서비스 목록 불러오기
  const fetchServicesData = async () => {
    try {
      setLoading(true);
      // lib/api 모듈 함수로 변경
      const services = await serviceApi.getServices();
      
      // 인프라 정보가 있는 서비스에 대해 인프라 이름 가져오기
      const servicesWithInfraNames = await Promise.all((services || []).map(async (service) => {
        if (service.infra_id) {
          try {
            // 인프라 정보 가져오기
            const response = await api.kubernetes.request('getInfraById', { id: service.infra_id });
            if (response.data?.success && response.data.infra) {
              return {
                ...service,
                status: 'registered', // 최초 상태를 '등록'으로 설정
                infraName: response.data.infra.name,
                loadingStatus: false  // 상태 조회 로딩 상태 추가
              };
            }
          } catch (error) {
            console.error(`인프라 정보 불러오기 실패 (ID: ${service.infra_id}):`, error);
          }
        }
        // 인프라 정보가 없는 경우에도 상태를 '등록'으로 설정
        return {
          ...service,
          status: 'registered',
          loadingStatus: false  // 상태 조회 로딩 상태 추가
        };
      }));
      
      setServices(servicesWithInfraNames || []);
    } catch (error) {
      console.error('Error fetching services:', error);
      messageApi.error('서비스 목록을 불러오는데 실패했습니다.');
      setServices([]);
    } finally {
      setLoading(false);
    }
  };

  // 컴포넌트 마운트 시 데이터 가져오기
  useEffect(() => {
    fetchServicesData();
  }, []);

  // 서비스 상태 변경
  const handleStatusChange = async (serviceId: string | number, action: 'active' | 'inactive') => {
    try {
      setLoading(true);
      
      // 데이터베이스에 status 필드가 없으므로 API 호출은 생략하고 프론트엔드에서만 상태 변경
      // await serviceApi.changeServiceStatus(serviceId, action); 
      
      // 프론트엔드 상태 업데이트
      setServices(prevServices => 
        prevServices.map(service => 
          service.id === serviceId 
            ? { ...service, status: action } 
            : service
        )
      );
      
      messageApi.success(`서비스가 ${action === 'active' ? '활성화' : '비활성화'}되었습니다.`);
    } catch (error) {
      console.error('서비스 상태 변경 중 오류가 발생했습니다:', error);
      messageApi.error('서비스 상태 변경에 실패했습니다.');
    } finally {
      setLoading(false);
    }
  };

  // 서비스 삭제
  const handleDelete = async (serviceId: string | number) => {
    try {
      // 확인 모달 표시
      Modal.confirm({
        title: '서비스 삭제',
        content: '정말로 이 서비스를 삭제하시겠습니까? 네임스페이스와 모든 관련 자원이 영구적으로 삭제됩니다.',
        okText: '삭제',
        okType: 'danger',
        cancelText: '취소',
        onOk: async () => {
          // 삭제 진행
          setLoading(true);
          
          // 서비스 정보 가져오기
          const targetService = services.find(s => s.id === serviceId);
          
          if (!targetService) {
            messageApi.error('서비스 정보를 찾을 수 없습니다.');
            setLoading(false);
            return;
          }
          
          // 인프라 ID가 없는 경우 - DB에서만 삭제
          if (!targetService.infra_id) {
            messageApi.info('이 서비스에 연결된 인프라가 없습니다. 서비스 정보만 삭제합니다.');
            await serviceApi.deleteService(serviceId);
            await fetchServicesData();
            messageApi.success('서비스가 삭제되었습니다.');
            setLoading(false);
            return;
          }
          
          // 인프라에서 마스터 노드 찾기
          try {
            // 인프라 정보 가져오기
            const infraResponse = await kubernetesApi.getInfraById(targetService.infra_id);
            if (!infraResponse.infra) {
              messageApi.info('인프라 정보를 가져올 수 없습니다. 서비스 정보만 삭제합니다.');
              await serviceApi.deleteService(serviceId);
              await fetchServicesData();
              messageApi.success('서비스가 삭제되었습니다.');
              setLoading(false);
              return;
            }
            
            // 인프라의 서버 정보 가져오기
            const serversResponse = await kubernetesApi.getServers(targetService.infra_id);
            if (!serversResponse.servers || serversResponse.servers.length === 0) {
              messageApi.info('인프라에 연결된 서버가 없습니다. 서비스 정보만 삭제합니다.');
              await serviceApi.deleteService(serviceId);
              await fetchServicesData();
              messageApi.success('서비스가 삭제되었습니다.');
              setLoading(false);
              return;
            }
            
            // 마스터 노드 찾기 (인프라 유형별로 다른 방식으로 찾음)
            let masterServer;
            if (infraResponse.infra.type === 'kubernetes' || infraResponse.infra.type === 'external_kubernetes') {
              // 쿠버네티스 인프라인 경우
              masterServer = serversResponse.servers.find((server: Server) => 
                server.type.includes('master') && 
                server.join_command && 
                server.certificate_key
              );
              
              // 외부 쿠버네티스의 경우 첫 번째 서버를 사용
              if (infraResponse.infra.type === 'external_kubernetes' && !masterServer && serversResponse.servers.length > 0) {
                masterServer = serversResponse.servers[0];
              }
            } else if (infraResponse.infra.type === 'docker' || infraResponse.infra.type === 'external_docker') {
              // 도커는 첫 번째 서버를 마스터로 사용할 수 있음
              masterServer = serversResponse.servers.find((server: Server) => server.type === 'master');
              if (!masterServer && serversResponse.servers.length > 0) {
                masterServer = serversResponse.servers[0];
              }
            } else {
              // 기타 인프라의 경우
              masterServer = serversResponse.servers.find((server: Server) => server.type === 'master');
            }
            
            if (!masterServer) {
              messageApi.info('마스터 노드를 찾을 수 없습니다. 서비스 정보만 삭제합니다.');
              await serviceApi.deleteService(serviceId);
              await fetchServicesData();
              messageApi.success('서비스가 삭제되었습니다.');
              setLoading(false);
              return;
            }
            
            // 인증 정보 입력 모달 표시
            setCurrentService(targetService);
            setSelectedServer(masterServer);
            
            // 사용자 입력을 기다리기 위해 Promise 생성
            const authPromise = new Promise<{username: string, password: string}>((resolve, reject) => {
              // 인증 정보 모달 표시 함수 생성
              Modal.confirm({
                title: '서버 인증 정보 입력',
                icon: <KeyOutlined />,
                content: (
                  <Form layout="vertical">
                    <Form.Item
                      label="사용자 이름"
                      required
                      tooltip="서버 접속에 사용할 SSH 사용자 이름"
                    >
                      <Input 
                        id="username" 
                        placeholder="SSH 사용자 이름" 
                        onChange={(e) => {
                          (document.getElementById('username') as HTMLInputElement).value = e.target.value;
                        }}
                      />
                    </Form.Item>
                    <Form.Item
                      label="비밀번호"
                      required
                      tooltip="서버 접속에 사용할 SSH 비밀번호"
                    >
                      <Input.Password 
                        id="password" 
                        placeholder="SSH 비밀번호"
                        onChange={(e) => {
                          (document.getElementById('password') as HTMLInputElement).value = e.target.value;
                        }}
                      />
                    </Form.Item>
                  </Form>
                ),
                okText: '인증 정보 확인',
                okType: 'primary',
                cancelText: '취소',
                onOk() {
                  const username = (document.getElementById('username') as HTMLInputElement)?.value;
                  const password = (document.getElementById('password') as HTMLInputElement)?.value;
                  
                  if (!username || !password) {
                    messageApi.error('사용자 이름과 비밀번호를 모두 입력해주세요.');
                    reject(new Error('인증 정보 누락'));
                    return;
                  }
                  
                  resolve({ username, password });
                },
                onCancel() {
                  reject(new Error('사용자가 취소함'));
                },
              });
            });
            
            try {
              // 사용자가 인증 정보를 입력할 때까지 대기
              const { username, password } = await authPromise;
              
              // hops 문자열에서 호스트 정보 파싱
              let hopInfo: HopInfo = { host: '', port: 22 };
              try {
                const hopsData = JSON.parse(masterServer.hops);
                if (Array.isArray(hopsData) && hopsData.length > 0) {
                  hopInfo = hopsData[0];
                }
              } catch (err) {
                console.error('Hops 정보 파싱 오류:', err);
                messageApi.error('서버 연결 정보를 파싱할 수 없습니다.');
                setLoading(false);
                return;
              }
              
              // hops 배열 생성
              const hops = [{
                host: hopInfo.host,
                port: hopInfo.port,
                username: username,
                password: password
              }];
              
              // 인프라 유형별로 다른 처리
              if (infraResponse.infra.type === 'kubernetes') {
                // 1. 먼저 네임스페이스 존재 여부 확인
                messageApi.loading('네임스페이스 상태 확인 중...');
                
                const statusResponse = await kubernetesApi.getNamespaceAndPodStatus({
                  id: Number(masterServer.id),
                  namespace: targetService.namespace || '',
                  hops: hops
                });
                
                if (statusResponse.success) {
                  if (statusResponse.namespace_exists) {
                    // 1-1. 네임스페이스가 존재하는 경우 (배포됨) - 네임스페이스 삭제 후 서비스 삭제
                    messageApi.loading('네임스페이스 삭제 중...');
                    
                    const deleteResponse = await kubernetesApi.deleteNamespace({
                      id: Number(masterServer.id),
                      namespace: targetService.namespace || '',
                      hops: hops
                    });
                    
                    if (deleteResponse.success) {
                      // 네임스페이스 삭제 성공 후 서비스 정보 삭제
                      await serviceApi.deleteService(serviceId);
                      await fetchServicesData();
                      messageApi.success('서비스 및 네임스페이스가 성공적으로 삭제되었습니다.');
                    } else {
                      messageApi.error(deleteResponse.error || '네임스페이스 삭제에 실패했습니다.');
                    }
                  } else {
                    // 1-2. 네임스페이스가 존재하지 않는 경우 (배포 안됨) - DB에서만 삭제
                    messageApi.info('이 서비스는 쿠버네티스에 배포되지 않았습니다. 서비스 정보만 삭제합니다.');
                    await serviceApi.deleteService(serviceId);
                    await fetchServicesData();
                    messageApi.success('서비스 정보가 삭제되었습니다.');
                  }
                } else {
                  // 상태 확인 실패 시 오류 메시지 표시 후 서비스 정보만 삭제 여부 확인
                  Modal.confirm({
                    title: '서비스 상태 확인 실패',
                    content: '서비스 상태 확인에 실패했습니다. 서비스 정보만 삭제하시겠습니까?',
                    okText: '서비스 정보 삭제',
                    cancelText: '취소',
                    onOk: async () => {
                      await serviceApi.deleteService(serviceId);
                      await fetchServicesData();
                      messageApi.success('서비스 정보가 삭제되었습니다.');
                    }
                  });
                }
              } else if (infraResponse.infra.type === 'docker') {
                // 2. 도커 컨테이너 존재 여부 확인
                messageApi.loading('도커 컨테이너 상태 확인 중...');
                
                const containersResponse = await api.docker.request('getContainers', {
                  id: masterServer.id,
                  hops: hops,
                  compose_project: targetService.name
                });
                
                if (containersResponse.data?.success) {
                  const containers = containersResponse.data.containers || [];
                  
                  if (containers.length > 0) {
                    // 2-1. 컨테이너가 존재하는 경우 (배포됨) - 컨테이너 삭제 후 서비스 삭제
                    messageApi.loading('도커 컨테이너 삭제 중...');
                    
                    const removeResponse = await api.docker.request('removeContainer', {
                      server_id: masterServer.id,
                      hops: hops,
                      repo_url: targetService.gitlab_url || '',
                      branch: targetService.gitlab_branch || 'main',
                      username_repo: targetService.gitlab_id || '',
                      password_repo: targetService.gitlab_password || ''
                    });
                    
                    if (removeResponse.data?.success) {
                      // 컨테이너 삭제 성공 후 서비스 정보 삭제
                      await serviceApi.deleteService(serviceId);
                      await fetchServicesData();
                      messageApi.success('서비스 및 도커 컨테이너가 성공적으로 삭제되었습니다.');
                    } else {
                      messageApi.error(removeResponse.data?.error || '도커 컨테이너 삭제에 실패했습니다.');
                    }
                  } else {
                    // 2-2. 컨테이너가 존재하지 않는 경우 (배포 안됨) - DB에서만 삭제
                    messageApi.info('이 서비스는 도커에 배포되지 않았습니다. 서비스 정보만 삭제합니다.');
                    await serviceApi.deleteService(serviceId);
                    await fetchServicesData();
                    messageApi.success('서비스 정보가 삭제되었습니다.');
                  }
                } else {
                  // 상태 확인 실패 시 오류 메시지 표시 후 서비스 정보만 삭제 여부 확인
                  Modal.confirm({
                    title: '서비스 상태 확인 실패',
                    content: '도커 컨테이너 상태 확인에 실패했습니다. 서비스 정보만 삭제하시겠습니까?',
                    okText: '서비스 정보 삭제',
                    cancelText: '취소',
                    onOk: async () => {
                      await serviceApi.deleteService(serviceId);
                      await fetchServicesData();
                      messageApi.success('서비스 정보가 삭제되었습니다.');
                    }
                  });
                }
              } else {
                // 3. 기타 인프라 유형에 대한 처리 (현재는 DB에서만 삭제)
                messageApi.info(`${infraResponse.infra.type} 인프라 유형은 지원되지 않습니다. 서비스 정보만 삭제합니다.`);
                await serviceApi.deleteService(serviceId);
                await fetchServicesData();
                messageApi.success('서비스 정보가 삭제되었습니다.');
              }
            } catch (error) {
              if (error instanceof Error && error.message === '사용자가 취소함') {
                messageApi.info('서비스 삭제가 취소되었습니다.');
              } else {
                console.error('인증 정보 처리 중 오류 발생:', error);
                messageApi.error('인증 정보 처리 중 오류가 발생했습니다.');
              }
            }
          } catch (error) {
            console.error('인프라 정보 조회 중 오류 발생:', error);
            messageApi.error('인프라 정보 조회 중 오류가 발생했습니다.');
            
            // 오류 발생 시 서비스 정보만 삭제할지 확인
            Modal.confirm({
              title: '서비스 정보만 삭제',
              content: '인프라 정보 조회 중 오류가 발생했습니다. 서비스 정보만 삭제하시겠습니까?',
              okText: '서비스 정보 삭제',
              cancelText: '취소',
              onOk: async () => {
                await serviceApi.deleteService(serviceId);
                await fetchServicesData();
                messageApi.success('서비스 정보가 삭제되었습니다.');
              }
            });
          }
          
          setLoading(false);
        },
        onCancel() {
          messageApi.info('서비스 삭제가 취소되었습니다.');
        },
      });
    } catch (error) {
      console.error('서비스 삭제 중 오류가 발생했습니다:', error);
      messageApi.error('서비스 삭제에 실패했습니다.');
      setLoading(false);
    }
  };

  // 모달 열기 (생성)
  const showCreateModal = () => {
    setEditMode(false);
    setCurrentService(null);
    form.resetFields();
    setModalVisible(true);
  };

  // 모달 열기 (수정)
  const showEditModal = (service: Service) => {
    setEditMode(true);
    setCurrentService(service);
    form.setFieldsValue({
      name: service.name,
      domain: service.domain,
      namespace: service.namespace,
      gitlab_url: service.gitlab_url || '',
      gitlab_id: service.gitlab_id || '',
      gitlab_password: service.gitlab_password || '',
      gitlab_branch: service.gitlab_branch || 'main',
      gitlab_token: service.gitlab_token || null,
      infra_id: service.infra_id || null,
      status: service.status
    });
    setModalVisible(true);
  };

  // 상세보기 모달 열기
  const showDetailModal = (service: Service) => {
    setCurrentService(service);
    setDetailModalVisible(true);
  };

  // 모달 닫기
  const handleCancel = () => {
    setModalVisible(false);
    form.resetFields();
  };

  // 상세보기 모달 닫기
  const handleDetailCancel = () => {
    setDetailModalVisible(false);
    setCurrentService(null);
  };

  // 폼 제출 처리
  const handleSubmit = async (values: any) => {
    try {
      setLoading(true);

      // API 호출을 위한 데이터 준비
      // 참고: 타입 정의에는 status가 필요하지만 서버 DB에는 없으므로 포함시켜서 전송
      // (서버에서는 이 필드를 무시)
      const serviceData = {
        name: values.name,
        status: editMode ? (values.status || 'inactive') : 'inactive', // 타입 정의 호환을 위해 포함
        domain: values.domain,
        namespace: values.namespace, // 네임스페이스를 필수값으로 설정
        gitlab_url: values.gitlab_url || null,
        gitlab_id: values.gitlab_id || null,
        gitlab_password: values.gitlab_password || null,
        gitlab_branch: values.gitlab_branch || 'main',
        gitlab_token: values.gitlab_token || null,
        infra_id: values.infra_id || null,
        user_id: 1 // 기본 사용자 ID 설정
      };

      if (editMode && currentService) {
        // 수정 모드 - lib/api 모듈 함수로 변경
        await serviceApi.updateService(currentService.id, serviceData);
        
        // 프론트엔드 상태 업데이트 (화면에 표시하기 위함)
        setServices(prevServices => 
          prevServices.map(service => {
            if (service.id === currentService.id) {
              // 기존 서비스 정보(...service)와 폼에서 받은 새 정보(...serviceData)를 병합하여 업데이트
              // infraName 등 폼에 없는 기존 정보는 유지됨
              return { 
                ...service, 
                ...serviceData 
              };
            }
            // 수정 대상이 아닌 서비스는 그대로 반환
            return service;
          })
        );
        
        messageApi.success('서비스가 업데이트되었습니다.');
      } else {
        // 생성 모드 - lib/api 모듈 함수로 변경
        await serviceApi.createService(serviceData);
        
        // 새 서비스 추가 후 목록 갱신
        await fetchServicesData();
        messageApi.success('새 서비스가 생성되었습니다.');
      }
      
      setModalVisible(false);
      form.resetFields();
    } catch (error) {
      console.error('서비스 저장 중 오류가 발생했습니다:', error);
      
      if (error instanceof Error) {
        // Form 유효성 검사 오류인지 확인
        if ('errorFields' in error) {
          messageApi.error('필수 입력 항목을 모두 채워주세요.');
        } else {
          messageApi.error('서비스 저장에 실패했습니다.');
        }
      } else {
        messageApi.error('서비스 저장에 실패했습니다.');
      }
    } finally {
      setLoading(false);
    }
  };

  // 인증 정보 제출
  const handleAuthSubmit = async (values: any) => {
    try {
      // 서버와 서비스 정보가 없으면 처리하지 않음
      if (!selectedServer || !currentService) {
        messageApi.error('서버 또는 서비스 정보가 없습니다.');
        return;
      }

      // 인증 모달 타입에 따라 다른 함수 호출
      if (authModalType === 'status') {
        await handleAuthSubmitStatus(values, selectedServer, currentService);
      } else if (authModalType === 'deploy') {
        await handleAuthSubmitDeploy(values, selectedServer, currentService);
      }
    } catch (error) {
      console.error('인증 처리 중 오류 발생:', error);
      messageApi.error('인증 처리 중 오류가 발생했습니다.');
    }
  };

  // 필터링된 서비스 목록
  const filteredServices = (services || []).filter(service =>
    service.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
    (service.domain && service.domain.toLowerCase().includes(searchTerm.toLowerCase())) ||
    (service.gitlab_url && service.gitlab_url.toLowerCase().includes(searchTerm.toLowerCase()))
  );

  // 서비스 통계
  const totalServices = filteredServices.length;
  
  // 상태가 'active'인 서비스만 카운트
  const activeServices = filteredServices.filter(service => service.status === 'active').length;
  
  // 상태가 'inactive'인 서비스만 카운트 (모든 서비스가 inactive로 설정되어 있으므로 전체 - 활성)
  const inactiveServices = totalServices - activeServices;

  // 페이지네이션 처리
  const totalPages = Math.ceil(filteredServices.length / pageSize);
  const startIndex = (currentPage - 1) * pageSize;
  const endIndex = startIndex + pageSize;
  const currentPageData = filteredServices.slice(startIndex, endIndex);

  // 페이지네이션 핸들러
  const handlePageChange = (page: number) => {
    setCurrentPage(page);
  };

  // 페이지 크기 변경 핸들러
  const handlePageSizeChange = (current: number, size: number) => {
    setPageSize(size);
    setCurrentPage(1);
  };

  // 검색어 변경 핸들러
  const handleSearch = (value: string) => {
    setSearchTerm(value);
    setCurrentPage(1); // 검색 시 첫 페이지로 이동
  };

  // 목록 새로고침 핸들러
  const handleRefresh = () => {
    fetchServicesData();
    messageApi.success('서비스 목록이 새로고침되었습니다.');
  };

  // 서비스 리로드 핸들러
  const handleReload = async (serviceId: string | number) => {
    try {
      setLoading(true);
      // 특정 서비스만 새로고침하는 로직을 추가할 수 있습니다
      // 현재는 전체 목록을 가져옵니다
      await fetchServicesData();
      messageApi.success('서비스가 새로고침되었습니다.');
    } catch (error) {
      console.error('서비스 새로고침 중 오류가 발생했습니다:', error);
      messageApi.error('서비스 새로고침에 실패했습니다.');
    } finally {
      setLoading(false);
    }
  };

  // GitLab 정보 모달 열기
  const showGitlabModal = (service: Service) => {
    setCurrentService(service);
    setSelectedServiceName(service.name);
    setSelectedGitlabUrl(service.gitlab_url || '');
    setGitlabModalVisible(true);
  };

  // 서비스 배포 처리
  const handleDeploy = async (service: Service) => {
    try {
      if (!service.infra_id) {
        messageApi.error('서비스에 연결된 인프라가 없습니다. 먼저 인프라를 연결해주세요.');
        return;
      }
      
      // 인프라 정보 가져오기
      const infraResponse = await kubernetesApi.getInfraById(service.infra_id);
      if (!infraResponse.infra) {
        messageApi.error('인프라 정보를 가져올 수 없습니다.');
        return;
      }
      
      // 인프라 유형에 따라 처리
      if (infraResponse.infra.type === 'kubernetes' || infraResponse.infra.type === 'external_kubernetes') {
        // 쿠버네티스 인프라인 경우

        // 서비스에 GitLab URL이 있는지 확인
        if (!service.gitlab_url) {
          messageApi.error('서비스에 GitLab 저장소 URL이 연결되어 있지 않습니다.');
          return;
        }

        // 서비스의 네임스페이스 확인
        if (!service.namespace) {
          messageApi.error('서비스의 네임스페이스가 설정되어 있지 않습니다.');
          return;
        }

        // 인프라의 서버 정보 가져오기
        const serversResponse = await kubernetesApi.getServers(service.infra_id);
        if (!serversResponse.servers || serversResponse.servers.length === 0) {
          messageApi.error('인프라에 연결된 서버가 없습니다.');
          return;
        }

        // 마스터 노드 찾기
        let masterServer;
        if (infraResponse.infra.type === 'kubernetes') {
          masterServer = serversResponse.servers.find((server: Server) => 
          server.type.includes('master') && 
          server.join_command && 
          server.certificate_key
        );
        } else if (infraResponse.infra.type === 'external_kubernetes') {
          // 외부 쿠버네티스의 경우 첫 번째 서버를 사용
          masterServer = serversResponse.servers[0];
        }

        if (!masterServer) {
          messageApi.error('마스터 노드를 찾을 수 없습니다.');
          return;
        }
        
        // 폼 초기화
        authForm.resetFields();
        
        // 현재 서비스 설정과 서버 설정
        setCurrentService(service);
        setSelectedServer(masterServer);
        
        // 배포 모드로 플래그 설정
        setAuthModalType('deploy');
        setAuthModalVisible(true);
        
      } else if (infraResponse.infra.type === 'docker' || infraResponse.infra.type === 'external_docker') {
        // 도커 인프라의 경우
        console.log('[도커 배포] 도커 컨테이너 배포 시작:', service.name);
        
        // 도커 서버 정보 가져오기 (docker API 사용)
        const serverResponse = await dockerApi.getDockerServer(service.infra_id!);
        if (!serverResponse.success || !serverResponse.server) {
          messageApi.error('도커 서버 정보를 가져오는데 실패했습니다.');
          setServices(prevServices => 
            prevServices.map(s => 
              s.id === service.id 
                ? { ...s, loadingStatus: false } 
                : s
            )
          );
          return;
        }
        
        // 도커 컴포즈 설정이 필요한지 확인
        if (!service.gitlab_url) {
          messageApi.error('도커 배포를 위한 GitLab URL이 설정되어 있지 않습니다.');
          setServices(prevServices => 
            prevServices.map(s => 
              s.id === service.id 
                ? { ...s, loadingStatus: false } 
                : s
            )
          );
          return;
        }
        
        // 폼 초기화
        authForm.resetFields();
        
        // 현재 서비스 설정과 서버 설정
        setCurrentService(service);
        setSelectedServer(serverResponse.server);
        
        // 배포 모드로 플래그 설정
        setAuthModalType('deploy');
        setAuthModalVisible(true);
      } else {
        // 기타 인프라 유형에 대한 처리
        messageApi.info(`${infraResponse.infra.type} 인프라 유형은 아직 지원되지 않습니다.`);
        setServices(prevServices => 
          prevServices.map(s => 
            s.id === service.id 
              ? { ...s, loadingStatus: false } 
              : s
          )
        );
      }
    } catch (error) {
      console.error('서비스 배포 중 오류가 발생:', error);
      messageApi.error('서비스 배포 중 오류가 발생했습니다.');
      
      // 배포 실패 상태로 업데이트
      if (service) {
        setServices(prevServices => 
          prevServices.map(s => 
            s.id === service.id 
              ? { ...s, loadingStatus: false } 
              : s
          )
        );
      }
    }
  };

  // 서비스 재시작 처리
  const handleRestart = async (service: Service) => {
    try {
      setLoading(true);
      // 통합 API 사용 (kubernetes → service)
      await serviceApi.restartService(service.id);
      
      // 재시작이 시작되었다는 메시지와 함께 상태 확인 안내
      messageApi.success(
        '서비스 재시작이 시작되었습니다. 잠시 후 상태 확인 버튼을 클릭하여 서비스 상태를 확인하세요.'
      );
      
      // 상태는 즉시 변경하지 않음 - 상태 확인 버튼을 통해 확인하도록 유도
    } catch (error) {
      console.error('서비스 재시작 중 오류가 발생했습니다:', error);
      messageApi.error('서비스 재시작에 실패했습니다.');
    } finally {
      setLoading(false);
    }
  };

  // 서비스 중지 처리
  const handleStop = async (service: Service) => {
    try {
      setLoading(true);
      // 통합 API 사용 (kubernetes → service)
      await serviceApi.stopService(service.id);
      
      // 중지가 시작되었다는 메시지와 함께 상태 확인 안내
      messageApi.success(
        '서비스 중지가 시작되었습니다. 잠시 후 상태 확인 버튼을 클릭하여 서비스 상태를 확인하세요.'
      );
      
      // 상태는 즉시 변경하지 않음 - 상태 확인 버튼을 통해 확인하도록 유도
    } catch (error) {
      console.error('서비스 중지 중 오류가 발생했습니다:', error);
      messageApi.error('서비스 중지에 실패했습니다.');
    } finally {
      setLoading(false);
    }
  };

  // 서비스 제거 처리
  const handleRemove = async (service: Service) => {
    try {
      setLoading(true);
      // 통합 API 사용 (kubernetes → service)
      await serviceApi.removeService(service.id);
      messageApi.success('서비스가 제거되었습니다.');
      
      // 제거 후 서비스 목록 새로고침
      await fetchServicesData();
    } catch (error) {
      console.error('서비스 제거 중 오류가 발생했습니다:', error);
      messageApi.error('서비스 제거에 실패했습니다.');
    } finally {
      setLoading(false);
    }
  };

  // 서버 인증 정보 입력 모달 열기
  const showAuthModal = (server: Server) => {
    setSelectedServer(server);
    setAuthModalVisible(true);
  };

  // 서버 인증 정보 입력 모달 닫기
  const handleAuthCancel = () => {
    setAuthModalVisible(false);
    setSelectedServer(null);
    setCurrentService(null);
    setAuthModalType('status');
    authForm.resetFields();
  };

  // 서비스 상태 조회
  const handleServiceStatus = async (service: Service) => {
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

      // 인프라의 서버 정보 가져오기
      const serversResponse = await kubernetesApi.getServers(service.infra_id);
      if (!serversResponse.servers || serversResponse.servers.length === 0) {
        messageApi.error('인프라에 연결된 서버가 없습니다.');
        return;
      }

      // 인프라 유형에 따른 마스터 노드 찾기
      let masterServer;

      // 인프라 유형에 따라 다르게 처리
      if (infraResponse.infra.type === 'kubernetes' || infraResponse.infra.type === 'external_kubernetes') {
        // 쿠버네티스 인프라인 경우
        masterServer = serversResponse.servers.find((server: Server) => 
          server.type.includes('master') && 
          server.join_command && 
          server.certificate_key
        );
        
        // 외부 쿠버네티스의 경우 첫 번째 서버를 사용
        if (infraResponse.infra.type === 'external_kubernetes' && !masterServer && serversResponse.servers.length > 0) {
          masterServer = serversResponse.servers[0];
        }
      } else if (infraResponse.infra.type === 'docker' || infraResponse.infra.type === 'external_docker') {
        // 도커는 첫 번째 서버를 마스터로 사용할 수 있음
        masterServer = serversResponse.servers.find((server: Server) => server.type === 'master');
        if (!masterServer && serversResponse.servers.length > 0) {
          masterServer = serversResponse.servers[0];
        }
      } else {
        // 기타 인프라의 경우
        masterServer = serversResponse.servers.find((server: Server) => server.type === 'master');
      }

      if (!masterServer) {
        messageApi.error('마스터 노드를 찾을 수 없습니다.');
        return;
      }
      
      // 서버 정보 설정 및 인증 모달 표시
      setCurrentService(service);
      setSelectedServer(masterServer);
      setAuthModalType('status');
      setAuthModalVisible(true);
    } catch (error) {
      console.error('서비스 상태 조회 준비 중 오류 발생:', error);
      messageApi.error('서비스 상태 조회 준비 중 오류가 발생했습니다.');
    }
  };
  
  // 상태 확인용 인증 제출 처리
  const handleAuthSubmitStatus = async (
    values: any, 
    server: Server = selectedServer!, 
    service: Service = currentService!
  ) => {
    try {
      if (!server || !service) {
        messageApi.error('서버 정보가 없습니다.');
        return;
      }

      // 테이블 상태 컬럼 업데이트 - 조회 중 표시
      setServices(prevServices => 
        prevServices.map(s => 
          s.id === service.id 
            ? { ...s, loadingStatus: true } 
            : s
        )
      );
      
      // hops 문자열에서 호스트 정보 파싱
      let hopInfo: HopInfo = { host: '', port: 22 };
      try {
        const hopsData = JSON.parse(server.hops);
        if (Array.isArray(hopsData) && hopsData.length > 0) {
          hopInfo = hopsData[0];
        }
      } catch (err) {
        console.error('Hops 정보 파싱 오류:', err);
        // 조회 실패 상태로 업데이트
        setServices(prevServices => 
          prevServices.map(s => 
            s.id === service.id 
              ? { ...s, loadingStatus: false } 
              : s
          )
        );
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

      // 인프라 정보 가져오기
      const infraResponse = await kubernetesApi.getInfraById(service.infra_id!);
      if (!infraResponse.infra) {
        messageApi.error('인프라 정보를 가져올 수 없습니다.');
        setServices(prevServices => 
          prevServices.map(s => 
            s.id === service.id 
              ? { ...s, loadingStatus: false } 
              : s
          )
        );
        return;
      }

      // 모달 닫기 (API 호출 전에 미리 닫기)
      setAuthModalVisible(false);

      if (infraResponse.infra.type === 'kubernetes' || infraResponse.infra.type === 'external_kubernetes') {
        // 쿠버네티스 인프라의 경우
        // 서비스 상태 조회 API 호출
        const response = await kubernetesApi.getNamespaceAndPodStatus({
          id: Number(server.id),
          namespace: service?.namespace || '',
          hops: hops
        });
        
        if (response.success) {
          const namespaceStatus = response.namespace_exists ? 'Active' : 'Not Found';
          const podsStatus = (response.pods || []).map((pod: { name: string; status: string; restarts: string }) => ({
            name: pod.name,
            status: pod.status,
            ready: pod.status === 'Running',
            restarts: parseInt(pod.restarts, 10) || 0
          }));
          
          const runningPods = podsStatus.filter((pod: { status: string }) => pod.status === 'Running').length;
          const totalPods = podsStatus.length;
          
          // 서비스 상태를 업데이트 (namespaceStatus와 pods 정보 포함)
          setServices(prevServices => 
            prevServices.map(s => 
              s.id === service.id 
                ? { 
                    ...s, 
                    loadingStatus: false,
                    namespaceStatus: namespaceStatus, 
                    podsStatus: podsStatus,
                    runningPods: runningPods,
                    totalPods: totalPods,
                    // 실행 중인 파드가 있으면 active, 없으면 inactive
                    status: runningPods > 0 ? 'active' : 'inactive'
                  } 
                : s
            )
          );
          
          messageApi.success(`네임스페이스 상태: ${namespaceStatus}, 파드: ${runningPods}/${totalPods} 실행 중`);
        } else {
          // 조회 실패 상태로 업데이트
          setServices(prevServices => 
            prevServices.map(s => 
              s.id === service.id 
                ? { ...s, loadingStatus: false } 
                : s
            )
          );
          messageApi.error(response.error || '서비스 상태 확인에 실패했습니다.');
        }
      } else if (infraResponse.infra.type === 'docker' || infraResponse.infra.type === 'external_docker') {
        // 도커 인프라의 경우
        setLoading(true);
        
        // 1. 먼저 도커 서버 상태 확인 API 호출
        const serverStatusResponse = await api.docker.request<{
          success: boolean;
          status: {
            installed: boolean;
            running: boolean;
          };
          lastChecked: string;
          error?: string;
        }>('checkDockerServerStatus', {
          server_id: server.id,
          hops: hops
        });
        
        if (!serverStatusResponse.data?.success) {
          messageApi.error(serverStatusResponse.data?.error || '도커 서버 상태 확인에 실패했습니다.');
          setServices(prevServices => 
            prevServices.map(s => 
              s.id === service.id 
                ? { ...s, loadingStatus: false } 
                : s
            )
          );
          setLoading(false);
          return;
        }
        
        // 서버가 실행 중이 아니면 오류 메시지 표시
        if (!serverStatusResponse.data.status.running) {
          messageApi.error('도커 서비스가 실행 중이 아닙니다. 도커 서비스를 시작해주세요.');
          setServices(prevServices => 
            prevServices.map(s => 
              s.id === service.id 
                ? { ...s, loadingStatus: false } 
                : s
            )
          );
          setLoading(false);
          return;
        }
        
        // 2. 도커 서비스가 실행 중이면 컨테이너 정보 조회
        const containersResponse = await api.docker.request<{
          success: boolean;
          containers: Array<{
            name: string;
            status: string;
            image: string;
            created: string;
            ports: string;
          }>;
          error?: string;
        }>('getContainers', {
          id: server.id,
          hops: hops,
          compose_project: service.name // 프로젝트/서비스 이름으로 필터링
        });
        
        if (!containersResponse.data?.success) {
          messageApi.error(containersResponse.data?.error || '도커 컨테이너 조회에 실패했습니다.');
          setServices(prevServices => 
            prevServices.map(s => 
              s.id === service.id 
                ? { ...s, loadingStatus: false } 
                : s
            )
          );
          setLoading(false);
          return;
        }
        
        // 3. 컨테이너 정보로 상태 업데이트
        const containers = containersResponse.data.containers || [];
        const runningContainers = containers.filter((container: DockerContainer) => container.status.includes('Up')).length;
        
        // 서비스 상태를 업데이트
        setServices(prevServices => 
          prevServices.map(s => 
            s.id === service.id 
              ? { 
                  ...s, 
                  loadingStatus: false,
                  namespaceStatus: containers.length > 0 ? 'Active' : 'Inactive',
                  runningPods: runningContainers,
                  totalPods: containers.length,
                  status: runningContainers > 0 ? 'active' : 'inactive'
                } 
              : s
          )
        );
        
        // 성공 메시지 표시
        messageApi.success('도커 서버 및 컨테이너 상태를 성공적으로 가져왔습니다.');
        setLoading(false);
      } else {
        // 기타 인프라 유형에 대한 처리
        messageApi.info(`${infraResponse.infra.type} 인프라 유형은 아직 지원되지 않습니다.`);
        setServices(prevServices => 
          prevServices.map(s => 
            s.id === service.id 
              ? { ...s, loadingStatus: false } 
              : s
          )
        );
      }
    } catch (error) {
      console.error('서비스 상태 확인 중 오류 발생:', error);
      messageApi.error('서비스 상태 확인 중 오류가 발생했습니다.');
      
      // 조회 실패 상태로 업데이트
      if (service) {
        setServices(prevServices => 
          prevServices.map(s => 
            s.id === service.id 
              ? { ...s, loadingStatus: false } 
              : s
          )
        );
      }
      
      // 오류 발생 시에도 모달 닫기
      setAuthModalVisible(false);
    }
  };

  // 배포용 인증 제출 처리  
  const handleAuthSubmitDeploy = async (
    values: any, 
    server: Server = selectedServer!, 
    service: Service = currentService!
  ) => {
    try {
      if (!server || !service) {
        messageApi.error('서버 또는 서비스 정보가 없습니다.');
        return;
      }
      
      // 테이블 상태 컬럼 업데이트 - 배포 중 표시
      setServices(prevServices => 
        prevServices.map(s => 
          s.id === service.id 
            ? { ...s, loadingStatus: true } 
            : s
        )
      );
      
      // hops 문자열에서 호스트 정보 파싱
      let hopInfo: HopInfo = { host: '', port: 22 };
      try {
        const hopsData = JSON.parse(server.hops);
        if (Array.isArray(hopsData) && hopsData.length > 0) {
          hopInfo = hopsData[0];
        }
      } catch (err) {
        console.error('Hops 정보 파싱 오류:', err);
        // 배포 실패 상태로 업데이트
        setServices(prevServices => 
          prevServices.map(s => 
            s.id === service.id 
              ? { ...s, loadingStatus: false } 
              : s
          )
        );
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
      
      // 인프라 정보 가져오기
      const infraResponse = await kubernetesApi.getInfraById(service.infra_id!);
      if (!infraResponse.infra) {
        messageApi.error('인프라 정보를 가져올 수 없습니다.');
        setServices(prevServices => 
          prevServices.map(s => 
            s.id === service.id 
              ? { ...s, loadingStatus: false } 
              : s
          )
        );
        return;
      }

      // 인증 모달 닫기 (API 호출 전에 미리 닫기)
      setAuthModalVisible(false);
      
      if (infraResponse.infra.type === 'kubernetes' || infraResponse.infra.type === 'external_kubernetes') {
        // 쿠버네티스 인프라의 경우
        // 배포에 필요한 데이터 준비
        const deployData = {
          id: Number(server.id),
          repo_url: service.gitlab_url || '',
          namespace: service.namespace || '',
          hops: hops,
          branch: service.gitlab_branch || 'main',
          username_repo: service.gitlab_id || undefined, 
          password_repo: service.gitlab_password || undefined
        };
        
        // 쿠버네티스 배포 API 호출
        const response = await kubernetesApi.deployKubernetes(deployData);
        
        // 배포 완료 상태로 업데이트
        setServices(prevServices => 
          prevServices.map(s => 
            s.id === service.id 
              ? { ...s, loadingStatus: false } 
              : s
          )
        );
        
        if (response.success) {
          // 배포 성공
          messageApi.success(
            '쿠버네티스 서비스 배포가 시작되었습니다. 잠시 후 상태 확인 버튼을 클릭하여 서비스 상태를 확인하세요.'
          );
        } else {
          // 배포 실패
          messageApi.error(response.error || '쿠버네티스 배포에 실패했습니다.');
        }
      } else if (infraResponse.infra.type === 'docker' || infraResponse.infra.type === 'external_docker') {
        // 도커 인프라의 경우
        console.log('[도커 배포] 도커 컨테이너 배포 시작:', service.name);
        
        // 도커 컴포즈 설정이 필요한지 확인
        if (!service.gitlab_url) {
          messageApi.error('도커 배포를 위한 GitLab URL이 설정되어 있지 않습니다.');
          setServices(prevServices => 
            prevServices.map(s => 
              s.id === service.id 
                ? { ...s, loadingStatus: false } 
                : s
            )
          );
          return;
        }
        
        // 모달을 통해 추가 정보 입력 받기
        const dockerDeployInfoPromise = new Promise<{
          compose_path: string;
          compose_project: string;
          force_recreate: boolean;
          docker_registry: string;
          docker_username: string;
          docker_password: string;
        }>((resolve, reject) => {
          // 모달에서 사용할 폼 생성
          Modal.confirm({
            title: '도커 배포 설정',
            icon: <CloudServerOutlined />,
            width: 600,
            content: (
              <Form 
                layout="vertical"
                initialValues={{
                  compose_path: 'docker-compose.yml',
                  compose_project: service.name,
                  force_recreate: false,
                  docker_registry: '',
                  docker_username: '',
                  docker_password: ''
                }}
              >
                <Form.Item
                  name="compose_path"
                  label="docker-compose.yml 파일 경로"
                  tooltip="저장소 내 docker-compose.yml 파일 경로 (기본값: docker-compose.yml)"
                >
                  <Input id="compose_path" defaultValue="docker-compose.yml" placeholder="예: docker-compose.yml 또는 ./services/docker-compose.yml" />
                </Form.Item>
                <Form.Item
                  name="compose_project"
                  label="컴포즈 프로젝트 이름"
                  tooltip="컴포즈 프로젝트 이름 (기본값: 서비스 이름)"
                >
                  <Input id="compose_project" defaultValue={service.name} placeholder="프로젝트 이름" />
                </Form.Item>
                <Form.Item
                  name="force_recreate"
                  label="컨테이너 강제 재생성"
                  tooltip="기존 컨테이너가 있으면 삭제하고 새로 생성"
                  valuePropName="checked"
                >
                  <Switch id="force_recreate" />
                </Form.Item>
                
                <Typography.Title level={5}>도커 레지스트리 설정 (선택사항)</Typography.Title>
                <Form.Item
                  name="docker_registry"
                  label="도커 레지스트리 URL"
                  tooltip="프라이빗 도커 레지스트리 URL (예: harbor.mipllab.com)"
                >
                  <Input id="docker_registry" placeholder="예: harbor.mipllab.com" />
                </Form.Item>
                <Form.Item
                  name="docker_username"
                  label="레지스트리 사용자 이름"
                >
                  <Input id="docker_username" placeholder="도커 레지스트리 사용자 이름" />
                </Form.Item>
                <Form.Item
                  name="docker_password"
                  label="레지스트리 비밀번호"
                >
                  <Input.Password id="docker_password" placeholder="도커 레지스트리 비밀번호" />
                </Form.Item>
              </Form>
            ),
            okText: '배포',
            cancelText: '취소',
            onOk() {
              // DOM에서 값을 가져옴
              const composePath = (document.getElementById('compose_path') as HTMLInputElement)?.value || 'docker-compose.yml';
              const composeProject = (document.getElementById('compose_project') as HTMLInputElement)?.value || service.name;
              const forceRecreate = (document.getElementById('force_recreate') as HTMLInputElement)?.checked || false;
              const dockerRegistry = (document.getElementById('docker_registry') as HTMLInputElement)?.value || '';
              const dockerUsername = (document.getElementById('docker_username') as HTMLInputElement)?.value || '';
              const dockerPassword = (document.getElementById('docker_password') as HTMLInputElement)?.value || '';
              
              resolve({
                compose_path: composePath,
                compose_project: composeProject,
                force_recreate: forceRecreate,
                docker_registry: dockerRegistry,
                docker_username: dockerUsername,
                docker_password: dockerPassword
              });
            },
            onCancel() {
              reject(new Error('사용자가 취소함'));
            }
          });
        });
        
        try {
          // 사용자가 모달에서 입력한 정보 대기
          const dockerDeployInfo = await dockerDeployInfoPromise;
          
          // 도커 배포 API 호출에 필요한 데이터 준비
          const deployData = {
            id: Number(server.id),
            repo_url: service.gitlab_url,
            hops: hops,
            branch: service.gitlab_branch || 'main',
            username_repo: service.gitlab_id || undefined,
            password_repo: service.gitlab_password || undefined,
            // 사용자가 입력한 추가 정보
            compose_path: dockerDeployInfo.compose_path,
            compose_project: dockerDeployInfo.compose_project,
            force_recreate: dockerDeployInfo.force_recreate,
            docker_registry: dockerDeployInfo.docker_registry || undefined,
            docker_username: dockerDeployInfo.docker_username || undefined,
            docker_password: dockerDeployInfo.docker_password || undefined
          };
          
          messageApi.loading('도커 컨테이너 배포 중...');
          const response = await dockerApi.createDockerContainer(deployData);
          
          // 배포 결과 확인
          if (response.data?.success) {
            messageApi.success(response.data.message || '서비스가 성공적으로 배포되었습니다.');
            setAuthModalVisible(false);
            setAuthModalType('status');
            setDockerSettingsModalVisible(false);
            setServices(prevServices => 
              prevServices.map(s => 
                s.id === service.id 
                  ? { ...s, loadingStatus: false } 
                  : s
              )
            );
          } else {
            const errorMessage = response.data?.error || 
                                response.data?.message || 
                                '서비스 배포에 실패했습니다.';
            messageApi.error(errorMessage);
          }
        } catch (error) {
          console.error('도커 배포 정보 입력 취소:', error);
          messageApi.info('도커 배포가 취소되었습니다.');
          // 배포 취소 상태로 업데이트
          setServices(prevServices => 
            prevServices.map(s => 
              s.id === service.id 
                ? { ...s, loadingStatus: false } 
                : s
            )
          );
        }
      } else {
        // 기타 인프라 유형에 대한 처리
        messageApi.info(`${infraResponse.infra.type} 인프라 유형은 아직 지원되지 않습니다.`);
        setServices(prevServices => 
          prevServices.map(s => 
            s.id === service.id 
              ? { ...s, loadingStatus: false } 
              : s
          )
        );
      }
    } catch (error) {
      console.error('서비스 배포 중 오류가 발생:', error);
      messageApi.error('서비스 배포 중 오류가 발생했습니다.');
      
      // 배포 실패 상태로 업데이트
      if (service) {
        setServices(prevServices => 
          prevServices.map(s => 
            s.id === service.id 
              ? { ...s, loadingStatus: false } 
              : s
          )
        );
      }
      
      // 오류 발생 시에도 모달 닫기
      setAuthModalVisible(false);
    }
  };

  // 서비스 상세 모달 표시
  const showDetailModalWithStatus = (service: Service, status: any) => {
    setCurrentService(service);
    setDetailModalVisible(true);
    
    // 서비스 컴포넌트에 상태 전달을 위한 방법 (필요에 따라 수정)
    // 예: 이벤트 발생 또는 전역 상태 업데이트
    
    // EventBus 또는 Context API 또는 Redux 등을 사용할 수 있음
    // 간단한 예제로 CustomEvent 사용
    const event = new CustomEvent('serviceStatusUpdate', { 
      detail: { 
        serviceId: service.id,
        status
      } 
    });
    document.dispatchEvent(event);
  };

  // 인프라 정보 조회 핸들러
  const handleInfraClick = async (service: Service) => {
    if (!service.infra_id) {
      messageApi.warning('이 서비스에 연결된 인프라가 없습니다.');
      return;
    }
    
    try {
      setLoading(true);
      setCurrentService(service);
      setSelectedInfraId(service.infra_id);
      
      // 인프라 API를 통해 인프라 정보 조회
      // console.log(`[디버그] 인프라 ID ${service.infra_id} 정보 조회`);
      const response = await api.kubernetes.request('getInfraById', { id: service.infra_id });
      
      if (response.data?.success && response.data.infra) {
        setSelectedInfraInfo(response.data.infra);
        
        // 인프라에 속한 서버 정보 조회
        try {
          // console.log(`[디버그] 인프라 ID ${service.infra_id}에 속한 서버 정보 조회`);
          const serversResponse = await api.kubernetes.request('getServers', { infra_id: service.infra_id });
          
          if (serversResponse.data?.success && serversResponse.data.servers) {
            // console.log(`[디버그] 인프라 ID ${service.infra_id}의 서버 목록:`, serversResponse.data.servers);
            
            // 인프라 ID로 서버 필터링하여 인프라 정보에 추가
            const filteredServers = serversResponse.data.servers.filter((server: any) => server.infra_id === service.infra_id);
            // console.log(`[디버그] 필터링된 서버 목록:`, filteredServers);
            
            // 인프라 정보에 서버 정보 추가
            setSelectedInfraInfo({
              ...response.data.infra,
              servers: filteredServers
            });
          } else {
            console.log(`[디버그] 서버 정보가 없거나 요청 실패`);
            // 서버 정보가 없는 경우 빈 배열로 설정
            setSelectedInfraInfo({
              ...response.data.infra,
              servers: []
            });
          }
        } catch (serverError) {
          console.error('서버 정보 조회 중 오류가 발생했습니다:', serverError);
          // 서버 정보 조회 실패해도 인프라 정보는 표시
          setSelectedInfraInfo({
            ...response.data.infra,
            servers: []
          });
        }
        
        setInfraModalVisible(true);
      } else {
        messageApi.warning('인프라 정보를 가져올 수 없습니다.');
      }
    } catch (error) {
      console.error('인프라 정보 조회 중 오류가 발생했습니다:', error);
      messageApi.error('인프라 정보 조회에 실패했습니다.');
    } finally {
      setLoading(false);
    }
  };

  // 서비스 추가 처리
  const handleAddService = async () => {
    try {
      const values = await form.validateFields();
      
      messageApi.loading('새 서비스를 추가 중입니다...');
      
      // API 클라이언트 사용
      await serviceApi.createService({
        name: values.name,
        domain: values.domain,
        gitlab_url: values.gitlab_url || null,
        gitlab_id: values.gitlab_id || null,
        gitlab_password: values.gitlab_password || null,
        infra_id: values.infra_id || null,
        status: 'inactive', // 기본 상태는 비활성화
        user_id: values.user_id || null
      });
      
      messageApi.success('서비스가 등록되었습니다.');
      fetchServicesData(); // 서비스 목록 새로고침
    } catch (error) {
      console.error('서비스 추가 중 오류 발생:', error);
      messageApi.error('서비스 추가에 실패했습니다.');
    }
  };

  // Gitlab 연동 취소 처리
  const handleDisconnectGitlab = async () => {
    if (!currentService) return;
    
    try {
      setLoading(true);
      
      // 기존 서비스 정보에서 Gitlab 관련 필드만 null로 변경
      const updatedService = {
        ...currentService,
        gitlab_url: null,
        gitlab_id: null,
        gitlab_password: null,
        gitlab_branch: null
      };
      
      // API 호출 (서비스 업데이트)
      await serviceApi.updateService(currentService.id, {
        name: updatedService.name,
        domain: updatedService.domain,
        namespace: updatedService.namespace,
        status: updatedService.status,
        gitlab_url: null,
        gitlab_id: null,
        gitlab_password: null,
        gitlab_branch: null
      });
      
      // 프론트엔드 상태 업데이트
      setServices(prevServices => 
        prevServices.map(service => 
          service.id === currentService.id 
            ? { ...service, gitlab_url: null, gitlab_id: null, gitlab_password: null, gitlab_branch: null } 
            : service
        )
      );
      
      messageApi.success('Gitlab 연동이 해제되었습니다.');
      setDetailModalVisible(false); // 상세보기 모달 닫기
    } catch (error) {
      console.error('Gitlab 연동 해제 중 오류가 발생했습니다:', error);
      messageApi.error('Gitlab 연동 해제에 실패했습니다.');
    } finally {
      setLoading(false);
    }
  };

  // 날짜 포맷팅 함수
  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
  };

  // 도커 설정 모달 열기
  const showDockerSettingsModal = (service: Service) => {
    setCurrentService(service);
    setDockerSettingsModalVisible(true);
  };

  // 도커 설정 모달 닫기
  const handleDockerSettingsCancel = () => {
    setDockerSettingsModalVisible(false);
  };

  return (
    <div className="services-container">
      {contextHolder}
      <div className="service-wrapper">
        <ServiceTag />
        
        <ServiceCounter 
          total={totalServices} 
          active={activeServices} 
          inactive={inactiveServices} 
        />
        
        <ServiceActions 
          onSearch={handleSearch}
          searchTerm={searchTerm}
          onRefresh={handleRefresh}
          onCreate={showCreateModal}
        />
        
        <ServiceTable 
          services={currentPageData}
          loading={loading}
          pagination={{
            current: currentPage,
            pageSize: pageSize,
            total: filteredServices.length,
            onChange: handlePageChange,
            onShowSizeChange: handlePageSizeChange
          }}
          onView={showDetailModal}
          onEdit={showEditModal}
          onDelete={handleDelete}
          onGitlabClick={showGitlabModal}
          onDeploy={handleDeploy}
          onRestart={handleRestart}
          onStop={handleStop}
          onCheckStatus={handleServiceStatus}
          onInfraClick={handleInfraClick}
          onDockerSettings={showDockerSettingsModal}
        />
      </div>

      <ServiceFormModal
        visible={modalVisible}
        loading={loading}
        onCancel={handleCancel}
        onSubmit={handleSubmit}
        form={form}
        isEditMode={editMode}
        currentService={currentService}
      />

      <ServiceDetailModal 
        visible={detailModalVisible}
        service={currentService}
        onCancel={handleDetailCancel}
        onEdit={showEditModal}
      />

      <DockerSettingsModal
        visible={dockerSettingsModalVisible}
        service={currentService}
        onCancel={handleDockerSettingsCancel}
      />

      <Modal
        title={`${selectedServiceName} GitLab 저장소`}
        open={gitlabModalVisible}
        onCancel={() => {
          setGitlabModalVisible(false);
          if (!editMode) setCurrentService(null);
        }}
        footer={[
          <Button key="close" onClick={() => {
            setGitlabModalVisible(false);
            if (!editMode) setCurrentService(null);
          }}>
            닫기
          </Button>,
          <Button 
            key="visit" 
            type="primary" 
            icon={<LinkOutlined />}
            onClick={() => window.open(selectedGitlabUrl, '_blank')}
            disabled={!selectedGitlabUrl}
          >
            방문하기
          </Button>
        ]}
        className="gitlab-modal"
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <Text strong>GitLab URL:</Text>
          <Input 
            value={selectedGitlabUrl || ''} 
            readOnly 
            addonAfter={
              <CopyOutlined 
                onClick={() => {
                  navigator.clipboard.writeText(selectedGitlabUrl || '');
                  messageApi.success('GitLab URL이 클립보드에 복사되었습니다.');
                }}
                style={{ cursor: 'pointer' }}
              />
            }
          />
          
          <Text strong style={{ marginTop: 16 }}>GitLab ID:</Text>
          <Input 
            value={(currentService && currentService.gitlab_id) || ''} 
            readOnly 
            prefix={<UserOutlined style={{ color: '#bfbfbf' }} />}
          />
          
          <Text strong style={{ marginTop: 16 }}>GitLab 비밀번호:</Text>
          <Input.Password 
            value={(currentService && currentService.gitlab_password) || ''} 
            readOnly 
            prefix={<KeyOutlined style={{ color: '#bfbfbf' }} />}
          />
          
          <Text strong style={{ marginTop: 16 }}>GitLab Private Token:</Text>
          <Input.Password 
            value={(currentService && currentService.gitlab_token) || ''} 
            readOnly 
            prefix={<KeyOutlined style={{ color: '#bfbfbf' }} />}
          />
          
          <Text strong style={{ marginTop: 16 }}>GitLab 브랜치:</Text>
          <Input 
            value={(currentService && currentService.gitlab_branch) || 'main'} 
            readOnly 
            prefix={<BranchesOutlined style={{ color: '#bfbfbf' }} />}
          />
          
          <div style={{ marginTop: 16 }}>
            <Text type="secondary">
              이 저장소에는 서비스 소스 코드, 배포 구성 및 문서가 포함되어 있습니다.
            </Text>
          </div>
        </Space>
      </Modal>
      
      {/* 인프라 정보 모달 */}
      <Modal
        title={
          <Space>
            <CloudServerOutlined style={{ color: '#1890ff' }} />
            <span>{currentService?.name || '서비스'} 인프라 정보</span>
          </Space>
        }
        open={infraModalVisible}
        onCancel={() => {
          setInfraModalVisible(false);
          setSelectedInfraInfo(null);
        }}
        footer={[
          <Button key="close" onClick={() => {
            setInfraModalVisible(false);
            setSelectedInfraInfo(null);
          }}>
            닫기
          </Button>
        ]}
        width={700}
        className="infra-modal"
      >
        {selectedInfraInfo ? (
          <div className="infra-detail">
            <Descriptions bordered column={1} size="small">
              <Descriptions.Item label="인프라 이름">
                {selectedInfraInfo.name}
              </Descriptions.Item>
              <Descriptions.Item label="인프라 유형">
                {selectedInfraInfo.type === 'kubernetes' ? '쿠버네티스' : 
                 selectedInfraInfo.type === 'baremetal' ? '베어메탈' : 
                 selectedInfraInfo.type === 'docker' ? '도커' : 
                 selectedInfraInfo.type === 'cloud' ? '클라우드' : 
                 selectedInfraInfo.type === 'external_kubernetes' ? '외부 쿠버네티스' :
                 selectedInfraInfo.type === 'external_docker' ? '외부 도커' :
                 selectedInfraInfo.type}
              </Descriptions.Item>
              <Descriptions.Item label="구성 정보">
                {selectedInfraInfo.info || <Text type="secondary">추가 정보 없음</Text>}
              </Descriptions.Item>
              <Descriptions.Item label="생성일">
                {formatDate(selectedInfraInfo.created_at)}
              </Descriptions.Item>
              <Descriptions.Item label="최종 업데이트">
                {formatDate(selectedInfraInfo.updated_at)}
              </Descriptions.Item>
            </Descriptions>
            
            {/* 서버 정보 섹션 추가 */}
            {selectedInfraInfo.servers && selectedInfraInfo.servers.length > 0 && (
              <div style={{ marginTop: '20px' }}>
                <Typography.Title level={5}>서버 목록</Typography.Title>
                <Table 
                  dataSource={selectedInfraInfo.servers}
                  rowKey="id"
                  size="small"
                  pagination={false}
                  bordered
                  columns={[
                    {
                      title: '서버명',
                      dataIndex: 'server_name',
                      key: 'server_name',
                      render: (text) => text || <Text type="secondary">이름 없음</Text>
                    },
                    {
                      title: '유형',
                      dataIndex: 'type',
                      key: 'type',
                      render: (type) => {
                        if (type.includes('master')) return <Tag color="blue">Master</Tag>;
                        if (type.includes('worker')) return <Tag color="green">Worker</Tag>;
                        if (type.includes('ha')) return <Tag color="purple">HA</Tag>;
                        return <Tag>{type}</Tag>;
                      }
                    }
                  ]}
                />
              </div>
            )}
            
            <div style={{ marginTop: 16 }}>
              <Text type="secondary">
                이 인프라는 쿠버네티스 서비스를 실행하기 위한 기반 환경을 제공합니다.
              </Text>
            </div>
          </div>
        ) : (
          <div style={{ textAlign: 'center', padding: '20px' }}>
            <Spin />
            <div style={{ marginTop: '10px' }}>인프라 정보를 불러오는 중...</div>
          </div>
        )}
      </Modal>

      {/* 서버 인증 정보 입력 모달 */}
      <Modal
        title={authModalType === 'status' ? '서비스 상태 확인 - 서버 인증 정보' : '서비스 배포 - 서버 인증 정보'}
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
            <Button type="primary" htmlType="submit" loading={loading}>
              {authModalType === 'status' ? '상태 확인' : '배포 시작'}
            </Button>
            <Button onClick={handleAuthCancel} style={{ marginLeft: 8 }}>
              취소
            </Button>
          </Form.Item>
        </Form>
      </Modal>

      {/* 서비스 상태 모달 */}
      <Modal
        title={`${serviceStatus?.name || '서비스'} 상태 정보`}
        open={statusModalVisible}
        onCancel={() => setStatusModalVisible(false)}
        footer={[
          <Button key="close" onClick={() => setStatusModalVisible(false)}>
            닫기
          </Button>
        ]}
        width={700}
      >
        {serviceStatus ? (
          <div>
            <Descriptions bordered column={1} size="small">
              <Descriptions.Item label="서비스 이름">
                {serviceStatus.name}
              </Descriptions.Item>
              <Descriptions.Item label="네임스페이스">
                {serviceStatus.namespace}
                {serviceStatus.kubernetesStatus?.namespace?.status && (
                  <Tag 
                    color={serviceStatus.kubernetesStatus.namespace.status === 'Active' ? 'green' : 'red'}
                    style={{ marginLeft: 10 }}
                  >
                    {serviceStatus.kubernetesStatus.namespace.status}
                  </Tag>
                )}
              </Descriptions.Item>
            </Descriptions>
            
            {serviceStatus.kubernetesStatus?.pods && serviceStatus.kubernetesStatus.pods.length > 0 ? (
              <div style={{ marginTop: 20 }}>
                <Typography.Title level={5}>파드 목록</Typography.Title>
                <Table 
                  dataSource={serviceStatus.kubernetesStatus.pods}
                  rowKey="name"
                  pagination={false}
                  bordered
                  size="small"
                  columns={[
                    {
                      title: '파드 이름',
                      dataIndex: 'name',
                      key: 'name',
                    },
                    {
                      title: '상태',
                      dataIndex: 'status',
                      key: 'status',
                      render: (status) => (
                        <Tag color={status === 'Running' ? 'green' : 
                                    status === 'Pending' ? 'orange' : 
                                    status === 'Error' || status === 'Failed' ? 'red' : 
                                    'default'}>
                          {status}
                        </Tag>
                      )
                    },
                    {
                      title: '준비 상태',
                      dataIndex: 'ready',
                      key: 'ready',
                      render: (ready) => (
                        <Badge status={ready ? "success" : "error"} 
                               text={ready ? "준비됨" : "준비 안됨"} />
                      )
                    },
                    {
                      title: '재시작 횟수',
                      dataIndex: 'restarts',
                      key: 'restarts',
                    }
                  ]}
                />
              </div>
            ) : (
              <div style={{ marginTop: 20, textAlign: 'center' }}>
                <Typography.Text type="secondary">
                  이 네임스페이스에 실행 중인 파드가 없습니다.
                </Typography.Text>
              </div>
            )}
          </div>
        ) : (
          <div style={{ textAlign: 'center', padding: '20px' }}>
            <Spin />
            <div style={{ marginTop: '10px' }}>서비스 상태 정보를 불러오는 중...</div>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default Services; 