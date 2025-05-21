'use client';

import React, { useState, useEffect } from 'react';
import { Table, Card, Button, Typography, Space, Tag, Tooltip, Empty, Spin, Modal, Form, Input, Select, InputNumber, message } from 'antd';
import { CloudServerOutlined, DatabaseOutlined, GlobalOutlined, ReloadOutlined, PlusOutlined, SettingOutlined } from '@ant-design/icons';
import { UserInfo, UserGroupInfo } from '../types/user';
import { InfraItem, InfraStatus } from '../types/infra';
import InfraSettingsModal from './InfraSettingsModal';
import './InfraManage.css';
import { useNavigate } from 'react-router-dom';
import * as kubernetesApi from '../lib/api/kubernetes';
import * as dockerApi from '../lib/api/docker';

const { Title, Text } = Typography;

interface InfraManageProps {
  userInfo: UserInfo;
  groupInfo: UserGroupInfo;
}

const InfraManage: React.FC<InfraManageProps> = ({ userInfo, groupInfo }) => {
  const [infraData, setInfraData] = useState<InfraItem[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [refreshing, setRefreshing] = useState<boolean>(false);
  const [isAddModalVisible, setIsAddModalVisible] = useState<boolean>(false);
  const [isSettingsModalVisible, setIsSettingsModalVisible] = useState<boolean>(false);
  const [selectedInfra, setSelectedInfra] = useState<InfraItem | null>(null);
  const [form] = Form.useForm();
  const [messageApi, contextHolder] = message.useMessage();
  const navigate = useNavigate();
  const [isImportModalVisible, setIsImportModalVisible] = useState<boolean>(false);
  const [importForm] = Form.useForm();

  // 인프라 데이터 가져오기
  const fetchInfraData = async () => {
    try {
      setLoading(true);
      // lib/api 모듈 함수로 변경
      const response = await kubernetesApi.getInfras();
      const data = response?.infras || [];
      
      // 상태 정보 업데이트 - 각 인프라의 상태를 별도 API로 조회
      const updatedData = await Promise.all(
        data.map(async (infra: InfraItem) => {
          try {
            // lib/api 모듈 함수로 변경
            const statusResponse = await kubernetesApi.getInfraById(infra.id);
            return {
              ...infra,
              status: statusResponse?.infra?.status || 'inactive'
            };
          } catch (error) {
            // 상태 조회 실패 시 기본값 사용
            console.error(`인프라 ID ${infra.id} 상태 조회 실패:`, error);
            const defaultStatus: InfraStatus = 'inactive';
            return {
              ...infra,
              status: defaultStatus
            };
          }
        })
      );
      
      setInfraData(updatedData);
    } catch (error) {
      console.error('인프라 데이터 로딩 중 오류 발생:', error);
      messageApi.error('인프라 데이터를 불러오는데 실패했습니다.');
      // 오류 발생 시 빈 배열로 초기화
      setInfraData([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchInfraData();
  }, []);

  // 인프라 새로고침
  const handleRefresh = async () => {
    try {
      setRefreshing(true);
      await fetchInfraData();
      messageApi.success('인프라 정보가 새로고침되었습니다.');
    } catch (error) {
      console.error('인프라 새로고침 중 오류 발생:', error);
      messageApi.error('인프라 정보 새로고침에 실패했습니다.');
    } finally {
      setRefreshing(false);
    }
  };

  // 인프라 추가 모달 열기
  const showAddModal = () => {
    form.resetFields();
    // 기본값 설정
    form.setFieldsValue({
      type: 'kubernetes'
    });
    setIsAddModalVisible(true);
  };

  // 인프라 추가/수정 모달 닫기
  const handleCancel = () => {
    setIsAddModalVisible(false);
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
  
  // 인프라 추가
  const handleAddInfra = async () => {
    try {
      const values = await form.validateFields();
      setLoading(true);
      
      // lib/api 모듈 함수로 변경
      await kubernetesApi.createInfra({
        name: values.name,
        type: values.type,
        info: values.info || ''
      });
      
      messageApi.success('새 인프라가 추가되었습니다.');
      setIsAddModalVisible(false);
      form.resetFields();
      await fetchInfraData();
    } catch (error) {
      if (error instanceof Error) {
        console.error('인프라 추가 중 오류 발생:', error);
        
        // Form 유효성 검사 오류인지 확인
        if ('errorFields' in error) {
          messageApi.error('필수 입력 항목을 모두 채워주세요.');
        } else {
          messageApi.error('인프라 추가에 실패했습니다.');
        }
      } else {
        messageApi.error('인프라 추가에 실패했습니다.');
      }
      
      setLoading(false);
    } finally {
      setLoading(false);
    }
  };

  // 인프라 삭제
  const handleDelete = async (infraId: number) => {
    try {
      setLoading(true);
      // lib/api 모듈 함수로 변경
      await kubernetesApi.deleteInfra(infraId);
      messageApi.success('인프라가 삭제되었습니다.');
      await fetchInfraData();
    } catch (error) {
      console.error('인프라 삭제 중 오류 발생:', error);
      messageApi.error('인프라 삭제에 실패했습니다.');
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

  // 인프라 상태에 따른 태그 색상 반환
  const getStatusColor = (status?: string) => {
    switch (status) {
      case 'active':
        return 'success';
      case 'inactive':
        return 'error';
      default:
        return 'default';
    }
  };

  // 인프라 상태 텍스트 반환
  const getStatusText = (status?: string) => {
    switch (status) {
      case 'active':
        return '활성';
      case 'inactive':
        return '비활성';
      default:
        return '알 수 없음';
    }
  };

  // 인프라 설정 저장
  const handleSaveSettings = (updatedInfra: InfraItem) => {
    // 인프라 데이터를 다시 불러와 UI 업데이트
    fetchInfraData();
    // 설정 모달 닫기
    setIsSettingsModalVisible(false);
    setSelectedInfra(null);
  };

  // 날짜 포맷팅 함수 추가
  const formatDate = (dateString?: string): string => {
    if (!dateString) return '-';
    
    try {
      const date = new Date(dateString);
      if (isNaN(date.getTime())) return '-';
      
      // YYYY-MM-DD 형식으로 변환
      return date.toISOString().split('T')[0];
    } catch (error) {
      console.error('날짜 형식 변환 오류:', error);
      return '-';
    }
  };

  // 인프라 설정 페이지로 이동
  const navigateToSettings = (infraId: number) => {
    navigate(`/settings/${infraId}`);
  };

  // 인프라 타입별 한글 이름 매핑
  const infraTypeNames: Record<string, string> = {
    'kubernetes': '쿠버네티스',
    'baremetal': '베어메탈',
    'docker': '도커',
    'cloud': '클라우드',
    'external_kubernetes': '외부 쿠버네티스',
    'external_docker': '외부 도커'
  };

  // 인프라 임포트 모달 열기
  const showImportModal = () => {
    importForm.resetFields();
    // 기본값 설정
    importForm.setFieldsValue({
      type: 'external_kubernetes',
      port: 22
    });
    setIsImportModalVisible(true);
  };

  // 인프라 임포트 모달 닫기
  const handleImportCancel = () => {
    setIsImportModalVisible(false);
  };

  // 인프라 임포트 처리
  const handleImportInfra = async () => {
    try {
      const values = await importForm.validateFields();
      setLoading(true);
      
      // hops 정보 구성
      const hops = [{
        host: values.host,
        port: Number(values.port) || 22,
        username: values.username,
        password: values.password
      }];
      
      if (values.type === 'external_kubernetes') {
        // 외부 쿠버네티스 API 호출
        const response = await kubernetesApi.importKubernetesInfra({
          name: values.name,
          type: values.type,
          info: values.info || '',
          hops: hops
        });
        
        if (response.success) {
          messageApi.success('인프라를 성공적으로 가져왔습니다.');
          setIsImportModalVisible(false);
          importForm.resetFields();
          await fetchInfraData();
        } else {
          messageApi.error(response.error || '인프라 가져오기에 실패했습니다.');
        }
      } else if (values.type === 'external_docker') {
        // 외부 도커 API 호출
        const response = await dockerApi.importDockerInfra({
          name: values.name,
          type: values.type,
          info: values.info || '',
          host: values.host,
          port: Number(values.port) || 22,
          username: values.username,
          password: values.password,
          hops: hops
        });
        
        if (response.success) {
          messageApi.success('외부 도커 인프라를 성공적으로 가져왔습니다.');
          if (response.registered_services && response.registered_services.length > 0) {
            messageApi.info(`${response.registered_services.length}개의 서비스가 자동으로 등록되었습니다.`);
          }
          setIsImportModalVisible(false);
          importForm.resetFields();
          await fetchInfraData();
        } else {
          messageApi.error(response.error || '외부 도커 인프라 가져오기에 실패했습니다.');
        }
      } else {
        messageApi.error('지원되지 않는 인프라 유형입니다.');
      }
    } catch (error) {
      if (error instanceof Error) {
        console.error('인프라 가져오기 중 오류 발생:', error);
        
        // Form 유효성 검사 오류인지 확인
        if ('errorFields' in error) {
          messageApi.error('필수 입력 항목을 모두 채워주세요.');
        } else {
          messageApi.error('인프라 가져오기에 실패했습니다.');
        }
      } else {
        messageApi.error('인프라 가져오기에 실패했습니다.');
      }
    } finally {
      setLoading(false);
    }
  };

  // 테이블 컬럼 정의
  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 80
    },
    {
      title: '인프라 이름',
      dataIndex: 'name',
      key: 'name',
      width: 220,
      render: (text: string, record: InfraItem) => (
        <Space>
          {getTypeIcon(record.type)}
          <Text 
            strong 
            style={{ cursor: 'pointer', color: '#1890ff' }}
            onClick={() => navigateToSettings(record.id)}
          >
            {text}
          </Text>
        </Space>
      )
    },
    {
      title: '유형',
      dataIndex: 'type',
      key: 'type',
      width: 180,
      render: (type: string) => {
        return infraTypeNames[type] || type;
      }
    },
    {
      title: '구성 정보',
      dataIndex: 'info',
      key: 'info',
      width: 200,
      ellipsis: true,
    },
    // {
    //   title: '상태',
    //   dataIndex: 'status',
    //   key: 'status',
    //   width: 100,
    //   render: (status: string) => (
    //     <Tag color={getStatusColor(status)}>
    //       {getStatusText(status)}
    //     </Tag>
    //   )
    // },
    {
      title: '생성일',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 140,
      render: (text: string) => formatDate(text)
    },
    {
      title: '최종 업데이트',
      dataIndex: 'updated_at',
      key: 'updated_at',
      width: 140,
      render: (text: string) => formatDate(text)
    },
    {
      title: '작업',
      key: 'action',
      width: 100,
      render: (text: string, record: InfraItem) => (
        <div className="action-cell">
          <Tooltip title="인프라 설정">
            <Button
              type="text"
              icon={<SettingOutlined style={{ fontSize: '16px' }} />}
              onClick={() => showSettingsModal(record)}
              size="small"
              className="action-btn edit"
            />
          </Tooltip>
        </div>
      )
    }
  ];

  return (
    <div className="infra-manage-container">
      {contextHolder}
      <div className="infra-manage-wrapper">
        <div className="infra-title">
          <Space align="center" size={12}>
            <CloudServerOutlined style={{ fontSize: '24px', color: '#1890ff' }} />
            <Typography.Title level={4} style={{ margin: 0, fontWeight: 600 }}>인프라 관리</Typography.Title>
          </Space>
          <Space>
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={showAddModal}
              className="btn-add"
            >
              인프라 추가
            </Button>
            <Button
              icon={<CloudServerOutlined />}
              onClick={showImportModal}
              className="btn-import"
            >
              인프라 가져오기
            </Button>
            <Button
              type="default"
              icon={<ReloadOutlined spin={refreshing} />}
              onClick={handleRefresh}
              className="btn-refresh"
            >
              새로고침
            </Button>
          </Space>
        </div>
      
        <Card bordered={false}>
          {loading ? (
            <div className="loading-container">
              <Spin size="large" tip="인프라 데이터 로딩 중..." />
            </div>
          ) : infraData.length > 0 ? (
            <Table 
              columns={columns} 
              dataSource={infraData} 
              rowKey="id" 
              pagination={{ pageSize: 10 }}
              bordered={false}
            />
          ) : (
            <Empty description="인프라 데이터가 없습니다." />
          )}
        </Card>
      </div>

      {/* 인프라 추가 모달 */}
      <Modal
        title="인프라 추가"
        open={isAddModalVisible}
        onOk={() => handleAddInfra()}
        onCancel={handleCancel}
        okText="추가"
        cancelText="취소"
      >
        <Form
          form={form}
          layout="vertical"
          name="addInfraForm"
        >
          <Form.Item
            name="name"
            label="인프라 이름"
            rules={[{ required: true, message: '인프라 이름을 입력해주세요' }]}
          >
            <Input placeholder="인프라 이름을 입력하세요" />
          </Form.Item>
          <Form.Item
            name="type"
            label="인프라 유형"
            rules={[{ required: true, message: '인프라 유형을 선택해주세요' }]}
          >
            <Select placeholder="인프라 유형을 선택하세요">
              <Select.Option value="kubernetes">쿠버네티스</Select.Option>
              <Select.Option value="baremetal">베어메탈</Select.Option>
              <Select.Option value="docker">도커</Select.Option>
              <Select.Option value="cloud">클라우드</Select.Option>
              <Select.Option value="external_kubernetes">외부 쿠버네티스</Select.Option>
              <Select.Option value="external_docker">외부 도커</Select.Option>
            </Select>
          </Form.Item>
          
          <Form.Item
            name="info"
            label="구성 정보"
            rules={[{ required: true, message: '구성 정보를 입력해주세요' }]}
          >
            <Input.TextArea 
              placeholder="인프라 구성에 대한 상세 정보를 입력하세요" 
              rows={5} 
            />
          </Form.Item>
        </Form>
      </Modal>

      {/* 인프라 설정 모달 */}
      <InfraSettingsModal
        visible={isSettingsModalVisible}
        infraItem={selectedInfra}
        onClose={handleSettingsCancel}
        onSave={handleSaveSettings}
      />

      {/* 인프라 임포트 모달 */}
      <Modal
        title="인프라 가져오기"
        open={isImportModalVisible}
        onOk={() => handleImportInfra()}
        onCancel={handleImportCancel}
        okText="가져오기"
        cancelText="취소"
      >
        <Form
          form={importForm}
          layout="vertical"
          name="importInfraForm"
        >
          <Form.Item
            name="name"
            label="인프라 이름"
            rules={[{ required: true, message: '인프라 이름을 입력해주세요' }]}
          >
            <Input placeholder="인프라 이름을 입력하세요" />
          </Form.Item>
          <Form.Item
            name="type"
            label="인프라 유형"
            rules={[{ required: true, message: '인프라 유형을 선택해주세요' }]}
          >
            <Select placeholder="인프라 유형을 선택하세요">
              <Select.Option value="external_kubernetes">외부 쿠버네티스</Select.Option>
              <Select.Option value="external_docker">외부 도커</Select.Option>
            </Select>
          </Form.Item>
          
          <Form.Item
            name="info"
            label="구성 정보"
            rules={[{ required: true, message: '구성 정보를 입력해주세요' }]}
          >
            <Input.TextArea 
              placeholder="인프라 구성에 대한 상세 정보를 입력하세요" 
              rows={2} 
            />
          </Form.Item>
          
          <Typography.Title level={5}>서버 연결 정보</Typography.Title>
          
          <Form.Item
            name="host"
            label="호스트"
            rules={[{ required: true, message: '호스트 주소를 입력해주세요' }]}
          >
            <Input placeholder="예: 192.168.0.1 또는 example.com" />
          </Form.Item>
          
          <Form.Item
            name="port"
            label="SSH 포트"
            initialValue={22}
            rules={[{ required: true, message: 'SSH 포트를 입력해주세요' }]}
          >
            <InputNumber placeholder="예: 22" min={1} max={65535} style={{ width: '100%' }} />
          </Form.Item>
          
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
        </Form>
      </Modal>
    </div>
  );
};

export default InfraManage; 