'use client';

import React, { useState, useEffect } from 'react';
import { Button, Typography, Space, Divider, List, Card, Row, Col, message, Modal, Input, Form } from 'antd';
import { SettingOutlined, ReloadOutlined, UserOutlined, LockOutlined } from '@ant-design/icons';
import { InfraItem } from '../../types/infra';
import { ServerStatus } from '../../types/server';
import api from '../../services/api';

const { Text } = Typography;

interface InfraCloudSettingProps {
  infra: InfraItem;
  showSettingsModal: (infra: InfraItem) => void;
}

const InfraCloudSetting: React.FC<InfraCloudSettingProps> = ({ infra, showSettingsModal }) => {
  const [loading, setLoading] = useState(false);
  const [serverInfo, setServerInfo] = useState<any>(null);
  const [isRestartModalVisible, setIsRestartModalVisible] = useState(false);
  const [messageApi, contextHolder] = message.useMessage();

  // 서버 정보 새로고침
  const handleRefresh = async () => {
    try {
      setLoading(true);
      const response = await api.kubernetes.request('getServerById', { id: infra.id });
      setServerInfo(response.data);
      messageApi.success('인스턴스 정보가 업데이트되었습니다.');
    } catch (error) {
      console.error('인스턴스 정보 조회 실패:', error);
      messageApi.error('인스턴스 정보 조회에 실패했습니다.');
    } finally {
      setLoading(false);
    }
  };

  // 인스턴스 재시작
  const handleRestart = async (username: string, password: string) => {
    try {
      setLoading(true);
      const hopsData = [{
        host: serverInfo?.ip || '',
        port: serverInfo?.port || '22',
        username,
        password
      }];
      await api.kubernetes.request('restartServer', {
        id: infra.id,
        hops: hopsData
      });
      messageApi.success('인스턴스가 재시작되었습니다.');
      handleRefresh();
    } catch (error) {
      console.error('인스턴스 재시작 실패:', error);
      messageApi.error('인스턴스 재시작에 실패했습니다.');
    } finally {
      setLoading(false);
      setIsRestartModalVisible(false);
    }
  };

  // 날짜 포맷팅 함수
  const formatDate = (dateString?: string): string => {
    if (!dateString) return '-';
    try {
      const date = new Date(dateString);
      if (isNaN(date.getTime())) return '-';
      return date.toISOString().split('T')[0];
    } catch (error) {
      console.error('날짜 형식 변환 오류:', error);
      return '-';
    }
  };

  useEffect(() => {
    handleRefresh();
  }, [infra.id]);

  return (
    <div className="infra-content-wrapper">
      {contextHolder}
      <div style={{ marginBottom: 16 }}>
        <Space>
          <Text strong>클라우드 정보: </Text>
          <Text>{infra.info || '-'}</Text>
          <Button
            icon={<ReloadOutlined spin={loading} />}
            onClick={handleRefresh}
            size="small"
          />
        </Space>
      </div>
      <div style={{ marginBottom: 16 }}>
        <Text strong>생성일: </Text>
        <Text>{formatDate(infra.created_at)}</Text>
      </div>
      <div style={{ marginBottom: 16 }}>
        <Text strong>최종 업데이트: </Text>
        <Text>{formatDate(infra.updated_at)}</Text>
      </div>

      <Divider orientation="left">클라우드 상세 정보</Divider>
      <Row gutter={[16, 16]}>
        <Col span={12}>
          <Card size="small" title="인스턴스 정보" loading={loading}>
            <List
              size="small"
              dataSource={[
                { label: '유형', value: serverInfo?.instance_type || '-' },
                { label: 'vCPU', value: serverInfo?.vcpu || '-' },
                { label: '메모리', value: serverInfo?.memory || '-' },
                { label: '스토리지', value: serverInfo?.storage || '-' },
                { label: '상태', value: serverInfo?.status || '-' }
              ]}
              renderItem={item => (
                <List.Item>
                  <Text strong>{item.label}: </Text>
                  <Text>{item.value}</Text>
                </List.Item>
              )}
            />
          </Card>
        </Col>
        <Col span={12}>
          <Card size="small" title="네트워크 정보" loading={loading}>
            <List
              size="small"
              dataSource={[
                { label: '리전', value: serverInfo?.region || '-' },
                { label: '가용 영역', value: serverInfo?.availability_zone || '-' },
                { label: 'VPC', value: serverInfo?.vpc || '-' },
                { label: '서브넷', value: serverInfo?.subnet || '-' }
              ]}
              renderItem={item => (
                <List.Item>
                  <Text strong>{item.label}: </Text>
                  <Text>{item.value}</Text>
                </List.Item>
              )}
            />
          </Card>
        </Col>
      </Row>

      <div style={{ marginTop: 16, textAlign: 'right' }}>
        <Space>
          <Button 
            type="default" 
            onClick={() => setIsRestartModalVisible(true)}
            loading={loading}
          >
            인스턴스 재시작
          </Button>
          <Button 
            type="primary" 
            icon={<SettingOutlined />} 
            onClick={() => showSettingsModal(infra)}
          >
            설정
          </Button>
        </Space>
      </div>

      <Modal
        title="인스턴스 재시작"
        open={isRestartModalVisible}
        onCancel={() => setIsRestartModalVisible(false)}
        footer={null}
      >
        <Form onFinish={(values: any) => handleRestart(values.username, values.password)}>
          <Form.Item
            name="username"
            label="사용자 이름"
            rules={[{ required: true, message: '사용자 이름을 입력하세요' }]}
          >
            <Input prefix={<UserOutlined />} />
          </Form.Item>
          <Form.Item
            name="password"
            label="비밀번호"
            rules={[{ required: true, message: '비밀번호를 입력하세요' }]}
          >
            <Input.Password prefix={<LockOutlined />} />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button onClick={() => setIsRestartModalVisible(false)}>
                취소
              </Button>
              <Button type="primary" htmlType="submit" loading={loading}>
                재시작
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default InfraCloudSetting; 