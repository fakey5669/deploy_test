import React, { useState, useEffect } from 'react';
import { Modal, Form, Input, Space, Button, Divider, Row, Col, Tooltip, Tag, Select, message } from 'antd';
import { 
  CloseOutlined, 
  PlusOutlined, 
  EditOutlined, 
  AppstoreOutlined, 
  GlobalOutlined, 
  GithubOutlined, 
  LinkOutlined, 
  UserOutlined, 
  KeyOutlined, 
  SaveOutlined,
  CloudServerOutlined,
  BranchesOutlined
} from '@ant-design/icons';
import { Service } from '../../types/service';
import { InfraItem } from '../../types/infra';
import api from '../../services/api';

const { Option } = Select;

interface ServiceFormModalProps {
  visible: boolean;
  loading: boolean;
  onCancel: () => void;
  onSubmit: (values: any) => void;
  form: any;
  isEditMode?: boolean;
  currentService?: Service | null;
}

const ServiceFormModal: React.FC<ServiceFormModalProps> = ({
  visible,
  loading,
  onCancel,
  onSubmit,
  form,
  isEditMode = false,
  currentService = null
}) => {
  // 인프라 목록 상태
  const [infraList, setInfraList] = useState<InfraItem[]>([]);
  const [infraLoading, setInfraLoading] = useState<boolean>(false);
  const [messageApi] = message.useMessage();
  
  // 인프라 목록 가져오기
  useEffect(() => {
    if (visible) {
      fetchInfraList();
    }
  }, [visible]);
  
  const fetchInfraList = async () => {
    try {
      setInfraLoading(true);
      
      // API에서 인프라 목록 가져오기
      const response = await api.kubernetes.request<{ infras: InfraItem[], success: boolean }>('getInfras', {});
      
      if (response.data?.success && response.data.infras) {
        // API 응답에서 인프라 목록 추출
        const infras = response.data.infras;

        setInfraList(infras);
        
        // 인프라가 없는 경우 메시지 표시
        if (infras.length === 0) {
          messageApi.warning('사용 가능한 쿠버네티스 인프라가 없습니다. 먼저 인프라를 생성해주세요.');
        }
      } else {
        // 응답 데이터가 없는 경우
        setInfraList([]);
        messageApi.warning('사용 가능한 인프라가 없습니다.');
      }
    } catch (error) {
      console.error('인프라 목록 가져오기 실패:', error);
      messageApi.error('인프라 목록을 불러오는데 실패했습니다.');
      setInfraList([]);
    } finally {
      setInfraLoading(false);
    }
  };
  
  const handleGitLabUrlChange = () => {
    // GitLab URL 변경 시 처리 로직
    console.log('GitLab URL changed');
  };
  
  return (
    <Modal
      title={
        <Space>
          {isEditMode ? (
            <EditOutlined style={{ color: '#1890ff' }} />
          ) : (
            <PlusOutlined style={{ color: '#52c41a' }} />
          )}
          <span style={{ fontWeight: 'bold' }}>{isEditMode ? '서비스 편집' : '새 서비스 생성'}</span>
          {isEditMode && currentService && (
            <Tag color="blue">{currentService.name}</Tag>
          )}
        </Space>
      }
      open={visible}
      onCancel={onCancel}
      footer={null}
      width={700}
      centered
      destroyOnClose
      className="service-modal"
    >
      <Divider style={{ margin: '0 0 24px 0' }} />
      
      <Form
        form={form}
        layout="vertical"
        onFinish={onSubmit}
        requiredMark="optional"
        style={{ padding: '0 10px' }}
      >
        <Row gutter={24}>
          <Col span={24}>
            <Form.Item
              name="name"
              label={
                <Space>
                  <AppstoreOutlined />
                  <span>서비스 이름</span>
                </Space>
              }
              rules={[{ required: true, message: '서비스 이름을 입력해주세요' }]}
            >
              <Input prefix={<AppstoreOutlined style={{ color: '#bfbfbf' }} />} placeholder="서비스 이름 입력" />
            </Form.Item>
          </Col>
          
          <Col span={24}>
            <Form.Item
              name="domain"
              label={
                <Space>
                  <GlobalOutlined />
                  <span>도메인</span>
                </Space>
              }
            >
              <Input 
                prefix={<GlobalOutlined style={{ color: '#bfbfbf' }} />} 
                placeholder="example.com" 
              />
            </Form.Item>
          </Col>
          
          <Col span={24}>
            <Form.Item
              name="namespace"
              label={
                <Space>
                  <AppstoreOutlined />
                  <span>네임스페이스</span>
                </Space>
              }
              rules={[
                { required: true, message: '네임스페이스를 입력해주세요' },
                { pattern: /^[a-zA-Z0-9-]+$/, message: '영문, 숫자, 하이픈(-)만 입력 가능합니다' }
              ]}
              tooltip="서비스를 배포할 쿠버네티스 네임스페이스를 입력하세요 (영문, 숫자, 하이픈만 사용 가능)"
            >
              <Input 
                prefix={<AppstoreOutlined style={{ color: '#bfbfbf' }} />} 
                placeholder="네임스페이스 입력 (영문, 숫자, 하이픈만 사용 가능)" 
              />
            </Form.Item>
          </Col>
        </Row>
        
        <Divider orientation="left">
          <Space>
            <CloudServerOutlined />
            <span>인프라 설정</span>
          </Space>
        </Divider>
        
        <Row gutter={24}>
          <Col span={24}>
            <Form.Item
              name="infra_id"
              label={
                <Space>
                  <CloudServerOutlined />
                  <span>인프라 선택</span>
                </Space>
              }
              rules={[{ required: true, message: '인프라를 선택해주세요' }]}
              tooltip="서비스를 배포할 인프라를 선택하세요"
            >
              <Select
                placeholder="인프라 선택"
                loading={infraLoading}
                showSearch
                optionFilterProp="children"
                filterOption={(input, option) =>
                  (option?.children as unknown as string)?.toLowerCase?.()?.includes?.(input.toLowerCase())
                }
              >
                {infraList.map(infra => (
                  <Option key={infra.id} value={infra.id}>
                    {infra.name} ({infra.type})
                  </Option>
                ))}
              </Select>
            </Form.Item>
          </Col>
        </Row>
        
        <Divider orientation="left">
          <Space>
            <GithubOutlined />
            <span>GitLab 설정 (코드 저장소)</span>
          </Space>
        </Divider>
        
        <div style={{ marginBottom: '15px', color: '#666', fontSize: '13px' }}>
          Private Token은 GitLab 개인 설정 페이지에서 생성할 수 있습니다. 'API' 및 'read_repository' 권한이 필요합니다.
        </div>
        
        <Row gutter={24}>
          <Col span={24}>
            <Form.Item
              name="gitlab_url"
              label={
                <Space>
                  <GithubOutlined />
                  <span>GitLab 저장소 URL</span>
                </Space>
              }
              tooltip="GitLab 저장소 URL을 입력하세요 (예: https://gitlab.com/username/repository)"
            >
              <Input 
                placeholder="https://gitlab.com/username/repository" 
                prefix={<LinkOutlined style={{ color: '#bfbfbf' }} />}
                onChange={handleGitLabUrlChange}
              />
            </Form.Item>
          </Col>
          
          <Col span={12}>
            <Form.Item
              name="gitlab_id"
              label={
                <Space>
                  <UserOutlined />
                  <span>GitLab 사용자명 (선택)</span>
                </Space>
              }
              tooltip="GitLab 사용자명을 입력하세요. Private Token을 사용하는 경우 선택사항입니다."
            >
              <Input 
                placeholder="GitLab 사용자명" 
                prefix={<UserOutlined style={{ color: '#bfbfbf' }} />}
              />
            </Form.Item>
          </Col>
          
          <Col span={12}>
            <Form.Item
              name="gitlab_password"
              label={
                <Space>
                  <KeyOutlined />
                  <span>GitLab 비밀번호</span>
                </Space>
              }
              tooltip="GitLab 계정 비밀번호를 입력하세요"
            >
              <Input.Password 
                placeholder="GitLab 비밀번호" 
                prefix={<KeyOutlined style={{ color: '#bfbfbf' }} />}
              />
            </Form.Item>
          </Col>
          
          <Col span={12}>
            <Form.Item
              name="gitlab_token"
              label={
                <Space>
                  <KeyOutlined />
                  <span>GitLab Private Token</span>
                </Space>
              }
              tooltip="GitLab의 개인 접근 토큰(Personal Access Token)을 입력하세요. API 및 저장소 접근 권한이 필요합니다."
            >
              <Input.Password 
                placeholder="glpat-xxxxxxxxxxxxxxxxxx" 
                prefix={<KeyOutlined style={{ color: '#bfbfbf' }} />}
              />
            </Form.Item>
          </Col>
          
          <Col span={24}>
            <Form.Item
              name="gitlab_branch"
              label={
                <Space>
                  <BranchesOutlined />
                  <span>GitLab 브랜치</span>
                </Space>
              }
              tooltip="배포할 GitLab 브랜치를 입력하세요 (기본값: main)"
            >
              <Input 
                placeholder="main" 
                prefix={<BranchesOutlined style={{ color: '#bfbfbf' }} />}
              />
            </Form.Item>
          </Col>
        </Row>
        
        <Divider style={{ margin: '24px 0' }} />
        
        <Row justify="end" gutter={16}>
          <Col>
            <Button 
              icon={<CloseOutlined />} 
              onClick={onCancel}
            >
              취소
            </Button>
          </Col>
          <Col>
            <Button 
              type="primary" 
              htmlType="submit" 
              icon={isEditMode ? <SaveOutlined /> : <PlusOutlined />}
              loading={loading}
            >
              {isEditMode ? '저장' : '생성'}
            </Button>
          </Col>
        </Row>
      </Form>
    </Modal>
  );
};

export default ServiceFormModal; 