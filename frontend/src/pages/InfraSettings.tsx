'use client';

import React, { useState, useEffect } from 'react';
import { Card, Table, Button, Typography, Space, Tag, Tabs, Form, Input, Select, Divider, Empty, Spin, Row, Col, List, message, Statistic, Modal, InputNumber, Tooltip, Result } from 'antd';
import { CloudServerOutlined, DatabaseOutlined, GlobalOutlined, ReloadOutlined, PlusOutlined, SettingOutlined, MinusCircleOutlined, FilterOutlined, CheckCircleOutlined, CloseCircleOutlined, ExclamationCircleOutlined, ClusterOutlined, ApiOutlined, InfoCircleOutlined, ClockCircleOutlined, PlayCircleOutlined, UserOutlined, LockOutlined, PoweroffOutlined, DeleteOutlined, SyncOutlined, DesktopOutlined, ContainerOutlined, CloudOutlined } from '@ant-design/icons';
import InfraSettingsModal from './InfraSettingsModal';
import { UserGroupInfo, UserInfo } from '../types/user';
import { InfraItem, InfraStatus } from '../types/infra';
import './InfraSettings.css';
import { ServerInput, ServerStatus } from '../types/server';
import { useParams, useNavigate } from 'react-router-dom';
import api from '../services/api';
import * as kubernetesApi from '../lib/api/kubernetes';
import { ServerStatus as ApiServerStatus } from '../types';
import InfraKubernetesSetting from '../components/infra/InfraKubernetesSetting';
import InfraCloudSetting from '../components/infra/InfraCloudSetting';
import InfraBaremetalSetting from '../components/infra/InfraBaremetalSetting';
import InfraDockerSetting from '../components/infra/InfraDockerSetting';

const { Title, Text } = Typography;
const { TabPane } = Tabs;
const { Option } = Select;

// 노드 정보 타입 정의
interface Node {
  id: string;
  nodeType: 'master' | 'worker' | 'ha' | string;
  ip: string;
  port: string;
  status: ServerStatus;
  server_name?: string;
  join_command?: string;
  certificate_key?: string;
  last_checked?: string; // 최근 상태 조회 시간
}

// 샘플 데이터는 API 호출로 대체하므로 삭제

interface InfraSettingsProps {
  userInfo: UserInfo;
  groupInfo: UserGroupInfo;
}

const InfraSettings: React.FC<InfraSettingsProps> = ({ userInfo, groupInfo }) => {
  const [infraData, setInfraData] = useState<(InfraItem & { nodes?: Node[] })[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [selectedInfraId, setSelectedInfraId] = useState<number | null>(null);
  const [isSettingsModalVisible, setIsSettingsModalVisible] = useState<boolean>(false);
  const [selectedInfra, setSelectedInfra] = useState<InfraItem | null>(null);
  const [messageApi, contextHolder] = message.useMessage();
  const { infraId } = useParams<{ infraId?: string }>();
  const navigate = useNavigate();

  // 인프라 데이터 가져오기
  const fetchInfraData = async () => {
    setLoading(true);
    try {
      // 모든 인프라 조회 - lib/api 모듈 함수로 변경
      const response = await kubernetesApi.getInfras();
      const data = response?.infras || [];
      
      // 각 인프라의 상태 정보 가져오기
      const dataWithStatus = await Promise.all(
        data.map(async (infra: InfraItem) => {
          try {
            // getInfraStatus가 없으므로 getInfraById로 대체
            const statusResponse = await kubernetesApi.getInfraById(infra.id);
            return {
              ...infra,
              status: statusResponse?.infra?.status || 'inactive'
            };
          } catch (error) {
            // 상태 조회 실패 시 기본값 사용
            console.error(`인프라 ID ${infra.id} 상태 조회 실패:`, error);
            return {
              ...infra,
              status: 'inactive' as InfraStatus
            };
          }
        })
      );
      
      // 각 인프라의 노드(서버) 정보 가져오기
      const enhancedData = await Promise.all(
        dataWithStatus.map(async (infra: InfraItem & { status?: InfraStatus }) => {
          if (infra.type === 'kubernetes') {
            try {
              // 서버 목록 가져오기 - lib/api 모듈 함수로 변경
              const response = await kubernetesApi.getServers(infra.id);
              const servers = response?.servers || [];
          
              // 노드 데이터 변환
              const nodesData = servers.map((server: any) => {
                // hops 데이터 파싱
                let ip = "";
                let port = "";
                
                try {
                  if (server.hops) {
                    const hopsData = JSON.parse(server.hops);
                    if (hopsData && hopsData.length > 0) {
                      ip = hopsData[0].host || "";
                      port = hopsData[0].port ? hopsData[0].port.toString() : "";
                    }
                  }
                } catch (error) {
                  console.error('hops 데이터 파싱 오류:', error);
                }
                
                // 노드 유형 처리 - 타입을 분리하여 처리
                let nodeType: 'master' | 'worker' | 'ha' = 'worker'; // 기본값 (UI 표시용)
                if (server.type) {
                  // 콤마로 구분된 타입을 배열로 변환
                  const types = server.type.split(',').map((t: string) => t.trim());
                  
                  // UI에 표시하기 위한 nodeType은 타입을 그대로 저장
                  if (types.includes('ha') && types.includes('master')) {
                    // ha와 master 모두 포함된 경우, 원래 문자열을 그대로 유지하여 양쪽 탭에 표시될 수 있도록 함
                    return {
                      id: `${server.id}`,
                      nodeType: server.type, // 원래의 타입 문자열 그대로 사용 (예: 'ha,master')
                      ip: ip,
                      port: port,
                      status: server.last_checked ? 'preparing' : '등록' as ServerStatus, // status 필드가 DB에 없으므로 last_checked 기준으로만 초기 상태 설정
                      server_name: server.server_name,
                      join_command: server.join_command,
                      certificate_key: server.certificate_key,
                      last_checked: server.last_checked,
                      hops: server.hops
                    };
                  }
                  
                  // 단일 타입인 경우 기존 로직 유지
                  if (types.includes('ha')) {
                    nodeType = 'ha';
                  } else if (types.includes('master')) {
                    nodeType = 'master';
                  } else if (types.includes('worker')) {
                    nodeType = 'worker';
                  }
                }
                
                return {
                  id: `${server.id}`,
                  nodeType: nodeType,
                  ip: ip,
                  port: port,
                  status: server.last_checked ? 'preparing' : '등록' as ServerStatus, // status 필드가 DB에 없으므로 last_checked 기준으로만 초기 상태 설정
                  server_name: server.server_name,
                  join_command: server.join_command,
                  certificate_key: server.certificate_key,
                  last_checked: server.last_checked,
                  hops: server.hops
                };
              });
              
              // 인프라 객체 생성
              const infraWithNodes = {
                ...infra,
                nodes: nodesData
              };
              
              // last_checked가 있는 노드에 대해 상태 확인 API 호출
              // 실제 상태 조회는 컴포넌트에서 노드 데이터가 설정된 후에 수행
              return infraWithNodes;
            } catch (error) {
              console.error(`인프라 ID ${infra.id}의 서버 정보 조회 실패:`, error);
              return infra;
            }
          }
          return infra;
        })
      ) as (InfraItem & { nodes?: Node[] })[];
      
      // 데이터 설정
      setInfraData(enhancedData);
      
      // 자동 상태 조회를 제거하고, 사용자가 직접 조회 버튼을 클릭하도록 변경
      // (인증 정보 입력이 필요하기 때문)
      
    } catch (error) {
      console.error('인프라 데이터 로딩 중 오류 발생:', error);
      messageApi.error('인프라 데이터를 불러오는데 실패했습니다.');
      setInfraData([]);
    } finally {
      setLoading(false);
    }
  };

  // 컴포넌트 마운트 시 데이터 가져오기
  useEffect(() => {
    fetchInfraData();
  }, []);

  // URL에서 infraId 파라미터를 사용하여 선택된 인프라 설정
  useEffect(() => {
    if (infraId && infraData.length > 0) {
      const id = parseInt(infraId, 10);
      if (!isNaN(id)) {
        // URL에서 받은 인프라 ID로 상태 설정 (선택만 하고 서버 데이터는 가져오지 않음)
        setSelectedInfraId(id);
        // 선택된 인프라 찾기
        const infra = infraData.find(item => item.id === id);
        if (infra) {
          setSelectedInfra(infra);
          // 서버 정보 가져오기
          handleInfraSelect(id);
        }
      }
    }
  }, [infraId, infraData.length]);

  // 인프라 새로고침
  const handleRefresh = async () => {
    try {
      if (selectedInfraId) {
        // 선택된 인프라가 있는 경우, 해당 인프라 정보만 새로고침
        setLoading(true);
        
        // 전체 인프라 목록 유지를 위해 모든 데이터 가져오기
        await fetchInfraData();
        
        messageApi.success('인프라 정보가 새로고침되었습니다.');
      } else {
        // 선택된 인프라가 없는 경우 전체 목록 새로고침
        await fetchInfraData();
        messageApi.success('인프라 목록이 새로고침되었습니다.');
      }
    } catch (error) {
      console.error('인프라 새로고침 중 오류 발생:', error);
      messageApi.error('인프라 새로고침에 실패했습니다.');
    }
  };

  // 인프라 선택
  const handleInfraSelect = async (id: number) => {
    // 선택된 인프라 ID가 현재와 같으면 아무 작업도 하지 않음
    if (id === selectedInfraId) return;
    
    // URL 업데이트 (히스토리에 추가)
    if (id) {
      navigate(`/settings/${id}`);
    } else {
      navigate('/settings');
    }
    
    setSelectedInfraId(id);
    
    // 선택된 인프라가 있는 경우
    if (id) {
      try {
        setLoading(true);
        
        // 선택된 인프라의 데이터 가져오기
        const statusResponse = await kubernetesApi.getInfraById(id);
        const infraWithStatus = {
          ...infraData.find(item => item.id === id)!,
          status: statusResponse?.infra?.status || 'inactive'
        };
        
        // 서버 목록 가져오기
        const response = await kubernetesApi.getServers(id);
        const servers = response?.servers || [];
        
        // 노드 데이터 변환
        const nodesData = servers.map((server: any) => {
          // hops 데이터 파싱
          let ip = "";
          let port = "";
          
          try {
            if (server.hops) {
              const hopsData = JSON.parse(server.hops);
              if (hopsData && hopsData.length > 0) {
                ip = hopsData[0].host || "";
                port = hopsData[0].port ? hopsData[0].port.toString() : "";
              }
            }
          } catch (error) {
            console.error('hops 데이터 파싱 오류:', error);
          }
          
          // 노드 유형 처리
          let nodeType: 'master' | 'worker' | 'ha' = 'worker'; // 기본값
          if (server.type) {
            const types = server.type.split(',').map((t: string) => t.trim());
            
            if (types.includes('ha') && types.includes('master')) {
              return {
                id: `${server.id}`,
                nodeType: server.type,
                ip: ip,
                port: port,
                status: server.last_checked ? 'preparing' : '등록' as ServerStatus,
                server_name: server.server_name,
                join_command: server.join_command,
                certificate_key: server.certificate_key,
                last_checked: server.last_checked,
                hops: server.hops
              };
            }
            
            if (types.includes('ha')) {
              nodeType = 'ha';
            } else if (types.includes('master')) {
              nodeType = 'master';
            } else if (types.includes('worker')) {
              nodeType = 'worker';
            }
          }
          
          return {
            id: `${server.id}`,
            nodeType: nodeType,
            ip: ip,
            port: port,
            status: server.last_checked ? 'preparing' : '등록' as ServerStatus,
            server_name: server.server_name,
            join_command: server.join_command,
            certificate_key: server.certificate_key,
            last_checked: server.last_checked,
            hops: server.hops
          };
        });
        
        // 업데이트된 인프라 객체
        const updatedInfra = {
          ...infraWithStatus,
          nodes: nodesData
        };
        
        // 인프라 데이터 업데이트
        const updatedInfraData = infraData.map(item => {
          if (item.id === id) {
            return updatedInfra;
          }
          return item;
        });
        
        setInfraData(updatedInfraData);
        // 선택된 인프라 상태 설정 (renderSettingsByType 함수에서 사용)
        setSelectedInfra(updatedInfra);
      } catch (error) {
        console.error(`인프라 ID ${id}의 정보 로딩 중 오류 발생:`, error);
        messageApi.error('인프라 정보를 불러오는데 실패했습니다.');
      } finally {
        setLoading(false);
      }
    } else {
      // id가 없는 경우 (선택 해제)
      setSelectedInfra(null);
    }
  };

  // 인프라 설정 모달 열기
  const showSettingsModal = (infra: InfraItem) => {
    setSelectedInfra(infra);
    setIsSettingsModalVisible(true);
  };

  // 인프라 설정 모달 닫기
  const handleSettingsCancel = () => {
    setIsSettingsModalVisible(false);
    setSelectedInfra(null);
  };

  // 인프라 설정 저장
  const handleSaveSettings = async (updatedInfra: InfraItem) => {
    setLoading(true);
    try {
      // 인프라 설정 업데이트 - lib/api 모듈 함수로 변경
      await kubernetesApi.updateInfra(updatedInfra.id, {
        name: updatedInfra.name,
        type: updatedInfra.type,
        info: updatedInfra.info // info 필드 사용
      });
      
      messageApi.success('인프라 설정이 저장되었습니다.');
      await fetchInfraData(); // 데이터 새로고침
      setIsSettingsModalVisible(false);
      setSelectedInfra(null);
    } catch (error) {
      console.error('인프라 설정 저장 중 오류 발생:', error);
      messageApi.error('인프라 설정 저장에 실패했습니다.');
    } finally {
      setLoading(false);
    }
  };

  // 인프라 타입에 따른 아이콘 반환
  const getTypeIcon = (type: string) => {
    switch (type) {
      case 'kubernetes':
        return <CloudServerOutlined style={{ color: '#1890ff' }} />;
      case 'baremetal':
        return <DatabaseOutlined style={{ color: '#52c41a' }} />;
      case 'docker':
        return <CloudServerOutlined style={{ color: '#f5222d' }} />;
      case 'cloud':
        return <GlobalOutlined style={{ color: '#722ed1' }} />;
      default:
        return <CloudServerOutlined />;
    }
  };

  // 상태에 따른 태그 색상 반환
  const getStatusColor = (status?: string) => {
    switch (status) {
      case 'active':
        return 'success';
      case 'inactive':
        return 'default';
      default:
        return 'default';
    }
  };

  // 인프라 타입에 따른 컴포넌트 렌더링
  const renderInfraComponent = (infra: InfraItem & { nodes?: Node[] }) => {
    switch(infra.type) {
      case 'kubernetes':
        return <InfraKubernetesSetting infra={infra} showSettingsModal={showSettingsModal} />;
      case 'baremetal':
        return <InfraBaremetalSetting infra={infra} showSettingsModal={showSettingsModal} />;
      case 'docker':
        return <InfraDockerSetting infra={infra} showSettingsModal={showSettingsModal} />;
      case 'cloud':
        return <InfraCloudSetting infra={infra} showSettingsModal={showSettingsModal} />;
      default:
        return <Empty description="지원되지 않는 인프라 유형입니다" />;
    }
  };

  // 인프라 타입별 설정 컴포넌트 렌더링
  const renderSettingsByType = () => {
    if (!selectedInfra) return null;

    switch (selectedInfra.type) {
      case 'kubernetes':
        return <InfraKubernetesSetting infra={selectedInfra} showSettingsModal={showSettingsModal} />;
      case 'baremetal':
        return <InfraBaremetalSetting infra={selectedInfra} showSettingsModal={showSettingsModal} />;
      case 'docker':
        return <InfraDockerSetting infra={selectedInfra} showSettingsModal={showSettingsModal} />;
      case 'cloud':
        return <InfraCloudSetting infra={selectedInfra} showSettingsModal={showSettingsModal} />;
      case 'external_kubernetes':
        return <InfraKubernetesSetting infra={selectedInfra} showSettingsModal={showSettingsModal} isExternal={true} />;
      case 'external_docker':
        return <InfraDockerSetting infra={selectedInfra} showSettingsModal={showSettingsModal} isExternal={true} />;
      default:
        return (
          <Result
            status="warning"
            title={`지원되지 않는 인프라 유형입니다: ${selectedInfra.type}`}
            subTitle="인프라 유형에 맞는 설정 화면을 불러올 수 없습니다."
          />
        );
    }
  };

  // 인프라 타입별 제목 생성
  const getInfraTypeTitle = (type: string): string => {
    switch (type) {
      case 'kubernetes':
        return '쿠버네티스 인프라 설정';
      case 'baremetal':
        return '베어메탈 인프라 설정';
      case 'docker':
        return '도커 인프라 설정';
      case 'cloud':
        return '클라우드 인프라 설정';
      case 'external_kubernetes':
        return '외부 쿠버네티스 인프라 설정';
      case 'external_docker':
        return '외부 도커 인프라 설정';
      default:
        return '인프라 설정';
    }
  };

  // 인프라 타입별 아이콘 생성
  const getInfraTypeIcon = (type: string): React.ReactNode => {
    switch (type) {
      case 'kubernetes':
        return <ClusterOutlined style={{ color: '#1890ff' }} />;
      case 'baremetal':
        return <DesktopOutlined style={{ color: '#fa8c16' }} />;
      case 'docker':
        return <ContainerOutlined style={{ color: '#52c41a' }} />;
      case 'cloud':
        return <CloudOutlined style={{ color: '#722ed1' }} />;
      case 'external_kubernetes':
        return <ClusterOutlined style={{ color: '#1890ff' }} />;
      case 'external_docker':
        return <ContainerOutlined style={{ color: '#52c41a' }} />;
      default:
        return <SettingOutlined />;
    }
  };

  return (
    <div className="infra-settings-container">
      {contextHolder}
      <div className="infra-settings-wrapper">
        <div className="infra-title">
          <Space align="center" size={12}>
            <CloudServerOutlined style={{ fontSize: '24px', color: '#1890ff' }} />
            <Typography.Title level={4} style={{ margin: 0, fontWeight: 600 }}>인프라 설정</Typography.Title>
          </Space>
          <Button
            type="default"
            icon={<ReloadOutlined spin={loading} />}
            onClick={handleRefresh}
            className="btn-refresh"
          >
            새로고침
          </Button>
        </div>

        <Card bordered={false}>
          {loading ? (
            <div className="loading-container">
              <Spin size="large" tip="인프라 데이터 로딩 중..." />
            </div>
          ) : (
            <>
              <div className="infra-selector">
                <Card bordered={false} className="infra-selector-card">
                  <Row gutter={16} align="middle">
                    <Col span={4}>
                      <Text strong style={{ fontSize: '16px' }}>인프라 선택</Text>
                    </Col>
                    <Col span={20}>
                      <Select
                        placeholder="설정할 인프라를 선택하세요"
                        style={{ width: '100%' }}
                        onChange={handleInfraSelect}
                        value={selectedInfraId}
                        loading={loading}
                        allowClear
                        showSearch
                        optionFilterProp="children"
                        className="infra-select"
                      >
                        {infraData.map(item => (
                          <Option key={item.id} value={item.id}>
                            <Space>
                              {getTypeIcon(item.type)}
                              <span className="infra-option-name">{item.name}</span>
                              {/* <Tag color={getStatusColor(item.status)}>
                                {item.status === 'active' ? '활성' : '비활성'}
                              </Tag> */}
                            </Space>
                          </Option>
                        ))}
                      </Select>
                    </Col>
                  </Row>
                </Card>
              </div>
              {/* 상세 정보 섹션 제목 */}
              {selectedInfra && (
                <div className={`infra-detail-section infra-detail-${selectedInfra.type}`}>
                  <Typography.Title level={5} style={{ margin: 0 }}>
                    <Space>
                      {getInfraTypeIcon(selectedInfra.type)}
                      {selectedInfra.name} 상세 정보
                      {/* <Tag color={getStatusColor(selectedInfra.status)}>
                        {selectedInfra.status === 'active' ? '활성' : '비활성'}
                      </Tag> */}
                    </Space>
                  </Typography.Title>

                  {/* 선택된 인프라 정보 표시 */}
                  {loading ? (
                    <div className="loading-container">
                      <Spin size="large" tip="인프라 데이터 로딩 중..." />
                    </div>
                  ) : (
                    renderSettingsByType()
                  )}
                </div>
              )}
            </>
          )}
        </Card>
      </div>

      {selectedInfra && (
        <InfraSettingsModal
          visible={isSettingsModalVisible}
          infraItem={selectedInfra}
          onClose={handleSettingsCancel}
          onSave={handleSaveSettings}
        />
      )}
    </div>
  );
};

export default InfraSettings; 