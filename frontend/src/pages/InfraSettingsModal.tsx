import React, { useState, useEffect, ReactNode } from 'react';
import { Modal, Form, Input, Select, Button, Spin, message, Tabs, Space, Typography, Tag } from 'antd';
import { InfraItem, InfraStatus } from '../types/infra';
import api from '../services/api';

const { TabPane } = Tabs;
const { Text } = Typography;

interface InfraSettingsModalProps {
  visible: boolean;
  infraItem: InfraItem | null;
  onClose: () => void;
  onSave: (infraItem: InfraItem) => void;
  children?: ReactNode;
}

const InfraSettingsModal: React.FC<InfraSettingsModalProps> = ({
  visible,
  infraItem,
  onClose,
  onSave,
  children
}) => {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState<boolean>(false);
  const [activeTab, setActiveTab] = useState<string>('general');

  useEffect(() => {
    if (visible && infraItem) {
      form.setFieldsValue({
        name: infraItem.name,
        type: infraItem.type,
        info: infraItem.info,
        status: infraItem.status || 'inactive'
      });
    }
  }, [visible, infraItem, form]);

  const handleSave = async () => {
    try {
      const values = await form.validateFields();
      
      if (!infraItem) return;
      
      setLoading(true);
      
      // 업데이트된 인프라 항목 (상태 제외)
      const updatedInfra: InfraItem = {
        ...infraItem,
        name: values.name,
        type: values.type,
        info: values.info,
      };
      
      // 실제 API 호출로 인프라 업데이트 - 통합 API 사용
      await api.kubernetes.request('updateInfra', {
        id: updatedInfra.id,
        name: updatedInfra.name,
        type: updatedInfra.type,
        info: updatedInfra.info
      });
      
      setLoading(false);
      onSave(updatedInfra);
      message.success('인프라 설정이 저장되었습니다');
      onClose();
    } catch (error) {
      console.error('설정 저장 중 오류 발생:', error);
      message.error('설정 저장 중 오류가 발생했습니다');
      setLoading(false);
    }
  };

  if (!infraItem) return null;

  return (
    <Modal
      title={
        <Space>
          {`${infraItem.name}`}
          <Tag color={infraItem.status === 'active' ? 'success' : 'error'}>
            {infraItem.status === 'active' ? '활성' : '비활성'}
          </Tag>
        </Space>
      }
      open={visible}
      onCancel={onClose}
      footer={[
        <Button key="cancel" onClick={onClose}>
          취소
        </Button>,
        <Button
          key="save"
          type="primary"
          loading={loading}
          onClick={handleSave}
        >
          저장
        </Button>
      ]}
      width={600}
    >
      <Spin spinning={loading}>
        <Tabs activeKey={activeTab} onChange={setActiveTab}>
          <TabPane tab="기본 정보" key="general">
            <Form
              form={form}
              layout="vertical"
            >
              <Form.Item
                label="이름"
                name="name"
                rules={[{ required: true, message: '인프라 이름을 입력해주세요' }]}
              >
                <Input placeholder="인프라 이름" />
              </Form.Item>
              
              <Form.Item
                label="유형"
                name="type"
                rules={[{ required: true, message: '인프라 유형을 선택해주세요' }]}
              >
                <Select>
                  <Select.Option value="kubernetes">쿠버네티스</Select.Option>
                  <Select.Option value="baremetal">베어메탈</Select.Option>
                  <Select.Option value="docker">도커</Select.Option>
                  <Select.Option value="cloud">클라우드</Select.Option>
                  <Select.Option value="external_kubernetes">외부 쿠버네티스</Select.Option>
                  <Select.Option value="external_docker">외부 도커</Select.Option>
                </Select>
              </Form.Item>
              
              <Form.Item
                label="구성 정보"
                name="info"
                rules={[{ required: true, message: '구성 정보를 입력해주세요' }]}
              >
                <Input.TextArea rows={5} placeholder="인프라 구성에 대한 상세 정보를 입력하세요" />
              </Form.Item>
            </Form>
          </TabPane>
          
          <TabPane tab="접근 정보" key="access">
            <Space direction="vertical" style={{ width: '100%' }}>
              <Text type="secondary">
                이 섹션에서는 인프라에 접근하기 위한 정보를 구성할 수 있습니다.
                추후 실제 API와 연동하여 구현할 예정입니다.
              </Text>
              
              {/* 접근 정보 폼 필드들은 추후 추가 */}
              <Form layout="vertical">
                <Form.Item label="접근 URL">
                  <Input placeholder="https://example.com" disabled />
                </Form.Item>
                
                <Form.Item label="인증 토큰">
                  <Input.Password disabled />
                </Form.Item>
              </Form>
            </Space>
          </TabPane>
          
          <TabPane tab="모니터링" key="monitoring">
            <Space direction="vertical" style={{ width: '100%' }}>
              <Text type="secondary">
                모니터링 설정은 추후 구현 예정입니다.
              </Text>
            </Space>
          </TabPane>
          
          {children && <TabPane tab="상세 설정" key="details">{children}</TabPane>}
        </Tabs>
      </Spin>
    </Modal>
  );
};

export default InfraSettingsModal; 