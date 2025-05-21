'use client';

import React, { useState } from 'react';
import { Button, Typography, Space, Divider, List, Card, Row, Col, message, Modal, Input, Form } from 'antd';
import { SettingOutlined, ReloadOutlined, PoweroffOutlined, UserOutlined, LockOutlined } from '@ant-design/icons';
import { InfraItem } from '../../types/infra';
import { ServerStatus, ServerInput } from '../../types/server';
import api from '../../services/api';

const { Text } = Typography;

interface InfraBaremetalSettingProps {
  infra: InfraItem;
  showSettingsModal: (infra: InfraItem) => void;
}

// 서버 정보 인터페이스를 ServerInput을 확장하여 정의
interface ServerInfo extends Omit<ServerInput, 'type' | 'infra_id'> {
  ip: string;
  os: string;
  cpu: string;
  memory: string;
  disk: string;
  status: ServerStatus;
}

// 서버 재시작 모달 컴포넌트
const RestartServerModal: React.FC<{
  visible: boolean;
  onClose: () => void;
  onConfirm: (username: string, password: string) => void;
  loading: boolean;
}> = ({ visible, onClose, onConfirm, loading }) => {
  const [form] = Form.useForm();

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      onConfirm(values.username, values.password);
      form.resetFields();
    } catch (error) {
      console.error('폼 유효성 검사 중 오류 발생:', error);
    }
  };

  return (
    <Modal
      title="서버 재시작 인증"
      open={visible}
      onCancel={onClose}
      onOk={handleSubmit}
      okText="재시작"
      cancelText="취소"
      confirmLoading={loading}
    >
      <Typography.Paragraph>
        서버를 재시작하기 위한 인증 정보를 입력해주세요.
      </Typography.Paragraph>
      
      <Form form={form} layout="vertical">
        <Form.Item
          name="username"
          label="사용자 이름"
          rules={[{ required: true, message: '사용자 이름을 입력해주세요' }]}
        >
          <Input prefix={<UserOutlined />} placeholder="예: root" />
        </Form.Item>
        
        <Form.Item
          name="password"
          label="비밀번호"
          rules={[{ required: true, message: '비밀번호를 입력해주세요' }]}
        >
          <Input.Password prefix={<LockOutlined />} placeholder="서버 접속 비밀번호" />
        </Form.Item>
      </Form>
    </Modal>
  );
};

const InfraBaremetalSetting: React.FC<InfraBaremetalSettingProps> = ({ infra, showSettingsModal }) => {
  const [messageApi, contextHolder] = message.useMessage();
  const [loading, setLoading] = useState(false);
  const [serverInfo, setServerInfo] = useState<ServerInfo>({
    ip: '192.168.1.100',
    os: 'CentOS 8',
    cpu: '12코어',
    memory: '64GB',
    disk: '1TB SSD',
    status: '등록'
  });
  const [isRestartModalVisible, setIsRestartModalVisible] = useState(false);
  const [restartLoading, setRestartLoading] = useState(false);

  // 서버 정보 새로고침
  const handleRefresh = async () => {
    try {
      setLoading(true);
      // API 호출하여 서버 정보 가져오기
      const response = await api.kubernetes.request('getServerById', { id: infra.id });
      
      // 서버 정보 변환 및 타입 안전성 보장
      const serverData = response.data;
      const updatedServerInfo: ServerInfo = {
        ...serverInfo,
        status: serverData.status || '등록',
        // 서버 데이터가 없는 경우 기존 값 유지
        ip: (serverData as any).ip || serverInfo.ip,
        os: (serverData as any).os || serverInfo.os,
        cpu: (serverData as any).cpu || serverInfo.cpu,
        memory: (serverData as any).memory || serverInfo.memory,
        disk: (serverData as any).disk || serverInfo.disk
      };
      
      setServerInfo(updatedServerInfo);
      messageApi.success('서버 정보가 새로고침되었습니다.');
    } catch (error) {
      console.error('서버 정보 조회 중 오류 발생:', error);
      messageApi.error('서버 정보 조회에 실패했습니다.');
    } finally {
      setLoading(false);
    }
  };

  // 서버 재시작
  const handleRestart = async (username: string, password: string) => {
    try {
      setRestartLoading(true);
      const hopsData = [{
        host: serverInfo.ip,
        port: '22',
        username,
        password
      }];
      await api.kubernetes.request('restartServer', {
        id: infra.id,
        hops: hopsData
      });
      messageApi.success('서버가 재시작되었습니다.');
      setIsRestartModalVisible(false);
      handleRefresh();
    } catch (error) {
      console.error('서버 재시작 중 오류 발생:', error);
      messageApi.error('서버 재시작에 실패했습니다.');
    } finally {
      setRestartLoading(false);
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

  return (
    <>
      {contextHolder}
      <div className="infra-content-wrapper">
        <Row gutter={[16, 16]}>
          <Col span={24}>
            <Card 
              title="기본 정보" 
              extra={
                <Button 
                  icon={<ReloadOutlined spin={loading} />} 
                  onClick={handleRefresh}
                >
                  새로고침
                </Button>
              }
            >
              <List size="small">
                <List.Item>
                  <Text strong>서버 정보: </Text>
                  <Text>{infra.info || '-'}</Text>
                </List.Item>
                <List.Item>
                  <Text strong>생성일: </Text>
                  <Text>{formatDate(infra.created_at)}</Text>
                </List.Item>
                <List.Item>
                  <Text strong>최종 업데이트: </Text>
                  <Text>{formatDate(infra.updated_at)}</Text>
                </List.Item>
              </List>
            </Card>
          </Col>

          <Col span={24}>
            <Card title="서버 상세 정보">
              <List size="small">
                <List.Item>
                  <Text strong>IP 주소: </Text>
                  <Text>{serverInfo.ip}</Text>
                </List.Item>
                <List.Item>
                  <Text strong>운영체제: </Text>
                  <Text>{serverInfo.os}</Text>
                </List.Item>
                <List.Item>
                  <Text strong>CPU: </Text>
                  <Text>{serverInfo.cpu}</Text>
                </List.Item>
                <List.Item>
                  <Text strong>메모리: </Text>
                  <Text>{serverInfo.memory}</Text>
                </List.Item>
                <List.Item>
                  <Text strong>디스크: </Text>
                  <Text>{serverInfo.disk}</Text>
                </List.Item>
                <List.Item>
                  <Text strong>상태: </Text>
                  <Text>{serverInfo.status}</Text>
                </List.Item>
              </List>
              
              <div style={{ marginTop: 16, textAlign: 'right' }}>
                <Space>
                  <Button 
                    onClick={() => setIsRestartModalVisible(true)}
                    icon={<PoweroffOutlined />}
                  >
                    서버 재시작
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
            </Card>
          </Col>
        </Row>
      </div>

      <RestartServerModal
        visible={isRestartModalVisible}
        onClose={() => setIsRestartModalVisible(false)}
        onConfirm={handleRestart}
        loading={restartLoading}
      />
    </>
  );
};

export default InfraBaremetalSetting; 