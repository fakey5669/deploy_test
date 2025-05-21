'use client';

import React, { useState, useEffect } from 'react';
import { Card, Table, Button, Typography, Space, Tag, Tabs, Form, Input, Select, Divider, Empty, Spin, Row, Col, List, message, Statistic, Modal, InputNumber, Tooltip, Popconfirm, Alert } from 'antd';
import { CloudServerOutlined, DatabaseOutlined, GlobalOutlined, ReloadOutlined, PlusOutlined, SettingOutlined, MinusCircleOutlined, FilterOutlined, CheckCircleOutlined, CloseCircleOutlined, ExclamationCircleOutlined, ClusterOutlined, ApiOutlined, InfoCircleOutlined, ClockCircleOutlined, PlayCircleOutlined, UserOutlined, LockOutlined, PoweroffOutlined, DeleteOutlined, SyncOutlined, SearchOutlined, ToolOutlined, DashboardOutlined } from '@ant-design/icons';
import { InfraItem } from '../../types/infra';
import { ServerInput } from '../../types/server';
import api from '../../services/api';
import * as kubernetesApi from '../../lib/api/kubernetes';
import { 
  getNodeStatus, 
  installLoadBalancer, 
  installFirstMaster, 
  joinMaster, 
  joinWorker, 
  removeNode,
  startServer,
  stopServer,
  restartServer,
  deleteWorker
} from '../../lib/api/kubernetes';

const { Title, Text } = Typography;
const { TabPane } = Tabs;
const { Option } = Select;

type NodeType = 'master' | 'worker' | 'ha' | string;
type ServerStatus = 'running' | 'stopped' | 'maintenance' | 'preparing' | '등록' | 'checking';

interface Node {
  id: string;
  nodeType: NodeType;
  ip: string;
  port: string;
  server_name?: string;
  join_command?: string;
  certificate_key?: string;
  last_checked?: string;
  status: ServerStatus;
  hops?: string;
  updated_at?: string;
  ha?: string; // Added ha field
}

// 노드 추가 모달 컴포넌트
const AddNodeModal: React.FC<{
  visible: boolean;
  infraId: number;
  onClose: () => void;
  onAdd: (node: Omit<Node, 'id'>) => void;
  initialNodeType: 'ha' | 'master' | 'worker';
}> = ({ visible, infraId, onClose, onAdd, initialNodeType }) => {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (visible) {
      form.resetFields();
    }
  }, [visible, form]);

  const handleSubmit = async () => {
    try {
      setLoading(true);
      const values = await form.validateFields();
      
      // HA 노드가 아닌 경우 서버 이름은 필수
      if (initialNodeType === 'master' || initialNodeType === 'worker') {
        if (!values.server_name) {
          message.error('마스터/워커 노드는 서버 이름을 반드시 입력해야 합니다.');
          setLoading(false);
          return;
        }
      }
      
      let newNode: Omit<Node, 'id'> = {
        nodeType: initialNodeType,
        ip: values.ip,
        port: values.port.toString(),
        status: '등록' as ServerStatus
      };
      
      // HA 노드가 아닌 경우에만 서버 이름 설정
      if (initialNodeType !== 'ha') {
        // 사용자가 입력한 이름 그대로 사용
        newNode.server_name = values.server_name;
      }
      
      onAdd(newNode);
      form.resetFields();
      onClose();
    } catch (error) {
      console.error('노드 추가 중 오류 발생:', error);
    } finally {
      setLoading(false);
    }
  };

  const renderModalTitle = () => {
    let title = '';
    let icon = null;
    
    switch(initialNodeType) {
      case 'ha':
        title = 'HA 노드 추가';
        icon = <ApiOutlined />;
        break;
      case 'master':
        title = '마스터 노드 추가';
        icon = <ClusterOutlined />;
        break;
      case 'worker':
        title = '워커 노드 추가';
        icon = <CloudServerOutlined />;
        break;
    }
    
    return <Space>{icon} {title}</Space>;
  };

  return (
    <Modal
      title={renderModalTitle()}
      open={visible}
      onCancel={onClose}
      confirmLoading={loading}
      onOk={handleSubmit}
      okText="추가"
      cancelText="취소"
    >
      <Form
        form={form}
        layout="vertical"
      >
        {(initialNodeType === 'master' || initialNodeType === 'worker') && (
          <Form.Item
            name="server_name"
            label="서버 이름"
            rules={[
              { required: true, message: '서버 이름을 입력해주세요' }
            ]}
          >
            <Input placeholder="서버 이름을 입력하세요" />
          </Form.Item>
        )}
        
        <Form.Item
          name="ip"
          label="IP 주소"
          rules={[
            { required: true, message: 'IP 주소를 입력해주세요' },
            { pattern: /^(\d{1,3}\.){3}\d{1,3}$/, message: '올바른 IP 주소 형식을 입력해주세요' }
          ]}
        >
          <Input placeholder="예: 192.168.1.100" />
        </Form.Item>
        
        <Form.Item
          name="port"
          label="SSH 포트"
          rules={[{ required: true, message: '포트를 입력해주세요' }]}
        >
          <InputNumber min={1} max={65535} style={{ width: '100%' }} />
        </Form.Item>
      </Form>
    </Modal>
  );
};

// 서버 구축 모달 컴포넌트
const BuildServerModal: React.FC<{
  visible: boolean;
  onClose: () => void;
  onConfirm: (username: string, password: string) => void;
  loading: boolean;
  node: Node | null;
}> = ({ visible, onClose, onConfirm, loading, node }) => {
  const [form] = Form.useForm();
  
  useEffect(() => {
    if (visible) {
      form.resetFields();
    }
  }, [visible, form]);

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      onConfirm(values.username, values.password);
    } catch (error) {
      console.error('폼 유효성 검사 중 오류 발생:', error);
    }
  };

  // 노드 상태에 따라 모달 제목 변경
  const getModalTitle = () => {
    if (!node) return "서버 인증";
    
    if (node.status === 'running') {
      return "서버 재시작 인증";
    } else if (node.status === 'stopped') {
      return "서버 시작 인증";
    } else {
      return "서버 구축 인증";
    }
  };

  // 노드 상태에 따라 확인 버튼 텍스트 변경
  const getOkButtonText = () => {
    if (!node) return "확인";
    
    if (node.status === 'running') {
      return "재시작";
    } else if (node.status === 'stopped') {
      return "시작";
    } else {
      return "구축 시작";
    }
  };

  return (
    <Modal
      title={getModalTitle()}
      open={visible}
      onCancel={onClose}
      confirmLoading={loading}
      onOk={handleSubmit}
      okText={getOkButtonText()}
      cancelText="취소"
    >
      <Typography.Paragraph>
        서버 <strong>{node?.server_name || node?.ip}</strong>에 접속하기 위한 인증 정보를 입력해주세요.
      </Typography.Paragraph>
      
      <Form
        form={form}
        layout="vertical"
      >
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

// 서버 상태 조회 모달 컴포넌트 추가
const CheckStatusModal: React.FC<{
  visible: boolean;
  onClose: () => void;
  onConfirm: (username: string, password: string) => void;
  loading: boolean;
  node: Node | null;
}> = ({ visible, onClose, onConfirm, loading, node }) => {
  const [form] = Form.useForm();
  
  useEffect(() => {
    if (visible) {
      form.resetFields();
    }
  }, [visible, form]);

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      onConfirm(values.username, values.password);
    } catch (error) {
      console.error('폼 유효성 검사 중 오류 발생:', error);
    }
  };

  return (
    <Modal
      title="서버 상태 조회"
      open={visible}
      onCancel={onClose}
      confirmLoading={loading}
      onOk={handleSubmit}
      okText="상태 조회"
      cancelText="취소"
    >
      <Typography.Paragraph>
        서버 <strong>{node?.server_name || node?.ip}</strong>에 접속하기 위한 인증 정보를 입력해주세요.
      </Typography.Paragraph>
      
      <Form
        form={form}
        layout="vertical"
      >
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

// HA 노드 인증 정보 모달 컴포넌트 추가
const HACredentialsModal: React.FC<{
  visible: boolean;
  onClose: () => void;
  onConfirm: (username: string, password: string) => void;
  loading: boolean;
  nodes: Node[];
}> = ({ visible, onClose, onConfirm, loading, nodes }) => {
  const [form] = Form.useForm();
  
  useEffect(() => {
    if (visible) {
      form.resetFields();
    }
  }, [visible, form]);

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      onConfirm(values.username, values.password);
    } catch (error) {
      console.error('폼 유효성 검사 중 오류 발생:', error);
    }
  };

  return (
    <Modal
      title="HA 노드 인증 정보"
      open={visible}
      onCancel={onClose}
      confirmLoading={loading}
      onOk={handleSubmit}
      okText="확인"
      cancelText="취소"
    >
      <Typography.Paragraph>
        HA 노드에 접속하기 위한 인증 정보를 입력해주세요. ({nodes.length}개의 HA 노드에 동일한 인증 정보가 사용됩니다)
      </Typography.Paragraph>
      
      <Form
        form={form}
        layout="vertical"
      >
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

// DeleteWorkerModal 컴포넌트 수정
const DeleteWorkerModal: React.FC<{
  visible: boolean;
  onClose: () => void;
  onConfirm: (username: string, password: string, mainUsername: string, mainPassword: string) => void;
  loading: boolean;
  node: Node | null;
}> = ({ visible, onClose, onConfirm, loading, node }) => {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [mainUsername, setMainUsername] = useState('');
  const [mainPassword, setMainPassword] = useState('');
  
  useEffect(() => {
    if (visible) {
      setUsername('');
      setPassword('');
      setMainUsername('');
      setMainPassword('');
    }
  }, [visible]);

  const handleSubmit = async () => {
    if (!username) {
      message.error('워커 노드 사용자 이름을 입력해주세요.');
      return;
    }
    
    if (!password) {
      message.error('워커 노드 비밀번호를 입력해주세요.');
      return;
    }
    
    if (!mainUsername) {
      message.error('마스터 노드 사용자 이름을 입력해주세요.');
      return;
    }
    
    if (!mainPassword) {
      message.error('마스터 노드 비밀번호를 입력해주세요.');
      return;
    }
    
    onConfirm(username, password, mainUsername, mainPassword);
  };

  return (
    <Modal
      title={`워커 노드 삭제: ${node?.server_name || node?.ip}`}
      open={visible}
      onOk={handleSubmit}
      onCancel={onClose}
      confirmLoading={loading}
      okButtonProps={{ danger: true }}
      okText="삭제"
      cancelText="취소"
    >
      <Alert
        message="주의"
        description="워커 노드를 삭제하면 Kubernetes 클러스터에서 해당 노드가 제거됩니다."
        type="warning"
        showIcon
        style={{ marginBottom: 16 }}
      />
      <Form layout="vertical">
        <Form.Item label="워커 노드 사용자 이름" required>
          <Input
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="워커 노드 사용자 이름"
          />
        </Form.Item>
        <Form.Item label="워커 노드 비밀번호" required>
          <Input.Password
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="워커 노드 비밀번호"
          />
        </Form.Item>
        <Form.Item label="마스터 노드 사용자 이름" required>
          <Input
            value={mainUsername}
            onChange={(e) => setMainUsername(e.target.value)}
            placeholder="마스터 노드 사용자 이름"
          />
        </Form.Item>
        <Form.Item label="마스터 노드 비밀번호" required>
          <Input.Password
            value={mainPassword}
            onChange={(e) => setMainPassword(e.target.value)}
            placeholder="마스터 노드 비밀번호"
          />
        </Form.Item>
      </Form>
    </Modal>
  );
};

// DeleteMasterModal 컴포넌트 수정
const DeleteMasterModal: React.FC<{
  visible: boolean;
  onClose: () => void;
  onConfirm: (username: string, password: string, mainUsername: string, mainPassword: string, lbUsername?: string, lbPassword?: string) => void;
  loading: boolean;
  node: Node | null;
  nodes: Node[];
}> = ({ visible, onClose, onConfirm, loading, node, nodes }) => {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [mainUsername, setMainUsername] = useState('');
  const [mainPassword, setMainPassword] = useState('');
  const [lbUsername, setLbUsername] = useState('');
  const [lbPassword, setLbPassword] = useState('');
  
  useEffect(() => {
    if (visible) {
      setUsername('');
      setPassword('');
      setMainUsername('');
      setMainPassword('');
      setLbUsername('');
      setLbPassword('');
    }
  }, [visible]);

  // 메인 마스터 노드인지 확인
  const isMainMaster = !!(node?.join_command && node?.certificate_key);
  
  // HA 노드가 있는지 확인
  const haNodes = nodes.filter((n: Node) => n.nodeType === 'ha' || (typeof n.nodeType === 'string' && n.nodeType.includes('ha')));
  const hasHA = haNodes.length > 0;
  
  const handleSubmit = async () => {
    if (!username) {
      message.error('마스터 노드 사용자 이름을 입력해주세요.');
      return;
    }

    if (!password) {
      message.error('마스터 노드 비밀번호를 입력해주세요.');
      return;
    }
    
    if (!mainUsername) {
      message.error('메인 마스터 노드 사용자 이름을 입력해주세요.');
      return;
    }

    if (!mainPassword) {
      message.error('메인 마스터 노드 비밀번호를 입력해주세요.');
      return;
    }
    
    if (hasHA) {
      if (!lbUsername) {
        message.error('HA 노드 사용자 이름을 입력해주세요.');
        return;
      }
      
      if (!lbPassword) {
        message.error('HA 노드 비밀번호를 입력해주세요.');
        return;
      }
    }
    
    onConfirm(
      username, 
      password, 
      mainUsername,
      mainPassword,
      lbUsername || undefined, 
      lbPassword || undefined
    );
  };
  
  return (
    <Modal
      title={`마스터 노드 삭제: ${node?.server_name || node?.ip}`}
      open={visible}
      onOk={handleSubmit}
      onCancel={onClose}
      confirmLoading={loading}
      okButtonProps={{ danger: true }}
      okText="삭제"
      cancelText="취소"
    >
      <Form layout="vertical">
        {isMainMaster && (
          <Alert
            message="주의"
            description="이 노드는 메인 마스터 노드입니다. 삭제하면 전체 클러스터가 제거됩니다!"
            type="warning"
            showIcon
            style={{ marginBottom: 16 }}
          />
        )}
        
        <Form.Item label="마스터 노드 사용자 이름" required>
          <Input
            value={username}
            onChange={e => setUsername(e.target.value)}
            placeholder="마스터 노드 사용자 이름"
          />
        </Form.Item>
        
        <Form.Item label="마스터 노드 비밀번호" required>
          <Input.Password
            value={password}
            onChange={e => setPassword(e.target.value)}
            placeholder="마스터 노드 비밀번호"
          />
        </Form.Item>
        
        <Form.Item label="메인 마스터 노드 사용자 이름" required>
          <Input
            value={mainUsername}
            onChange={e => setMainUsername(e.target.value)}
            placeholder="메인 마스터 노드 사용자 이름"
          />
        </Form.Item>
        
        <Form.Item label="메인 마스터 노드 비밀번호" required>
          <Input.Password
            value={mainPassword}
            onChange={e => setMainPassword(e.target.value)}
            placeholder="메인 마스터 노드 비밀번호"
          />
        </Form.Item>
        
        {hasHA && (
          <>
            <Form.Item label="HA 노드 사용자 이름" required>
              <Input
                value={lbUsername}
                onChange={e => setLbUsername(e.target.value)}
                placeholder="HA 노드 사용자 이름"
              />
            </Form.Item>
            
            <Form.Item label="HA 노드 비밀번호" required>
              <Input.Password
                value={lbPassword}
                onChange={e => setLbPassword(e.target.value)}
                placeholder="HA 노드 비밀번호"
              />
            </Form.Item>
          </>
        )}
      </Form>
    </Modal>
  );
};

// 외부 쿠버네티스 인증 모달 컴포넌트 추가
const ExternalKubeAuthModal: React.FC<{
  visible: boolean;
  onClose: () => void;
  onConfirm: (username: string, password: string) => void;
  loading: boolean;
  server: { ip: string; port: string; };
}> = ({ visible, onClose, onConfirm, loading, server }) => {
  const [form] = Form.useForm();
  
  useEffect(() => {
    if (visible) {
      form.resetFields();
    }
  }, [visible, form]);

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      onConfirm(values.username, values.password);
    } catch (error) {
      console.error('폼 유효성 검사 오류:', error);
    }
  };

  return (
    <Modal
      title="외부 쿠버네티스 인증"
      open={visible}
      onCancel={onClose}
      confirmLoading={loading}
      onOk={handleSubmit}
      okText="연결"
      cancelText="취소"
    >
      <Typography.Paragraph>
        외부 쿠버네티스 클러스터({server.ip})에 접속하기 위한 인증 정보를 입력해주세요.
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

// 서버 리소스 조회 모달 컴포넌트 추가
const ServerResourceModal: React.FC<{
  visible: boolean;
  onClose: () => void;
  resource: ServerResource | null;
  loading: boolean;
  server: { name?: string; ip: string; };
}> = ({ visible, onClose, resource, loading, server }) => {
  return (
    <Modal
      title={`서버 리소스 정보 - ${server.name || server.ip}`}
      open={visible}
      onCancel={onClose}
      footer={[
        <Button key="close" onClick={onClose}>
          닫기
        </Button>
      ]}
      width={900}
    >
      {loading ? (
        <div style={{ textAlign: 'center', padding: '30px' }}>
          <Spin size="large" />
          <div style={{ marginTop: '15px' }}>리소스 정보를 가져오는 중입니다...</div>
        </div>
      ) : resource ? (
        <div>
          <Card title="시스템 정보" style={{ marginBottom: '16px' }}>
            <Row gutter={[16, 16]}>
              <Col span={8}>
                <Statistic title="호스트명" value={resource.host_info.hostname} />
              </Col>
              <Col span={8}>
                <Statistic title="운영체제" value={resource.host_info.os} />
              </Col>
              <Col span={8}>
                <Statistic title="커널" value={resource.host_info.kernel} />
              </Col>
            </Row>
          </Card>
          
          <Card title="CPU" style={{ marginBottom: '16px' }}>
            <Row gutter={[16, 16]}>
              <Col span={16}>
                <Statistic title="모델" value={resource.cpu.model} />
              </Col>
              <Col span={4}>
                <Statistic title="코어" value={resource.cpu.cores} />
              </Col>
              <Col span={4}>
                <Statistic 
                  title="사용량" 
                  value={resource.cpu.usage_percent} 
                  suffix="%" 
                  valueStyle={{ 
                    color: parseInt(resource.cpu.usage_percent) > 80 ? '#cf1322' : 
                           parseInt(resource.cpu.usage_percent) > 60 ? '#faad14' : '#3f8600' 
                  }}
                />
              </Col>
            </Row>
          </Card>
          
          <Card title="메모리" style={{ marginBottom: '16px' }}>
            <Row gutter={[16, 16]}>
              <Col span={8}>
                <Statistic title="전체" value={`${Math.round(parseInt(resource.memory.total_mb) / 1024)} GB`} />
              </Col>
              <Col span={8}>
                <Statistic title="사용 중" value={`${Math.round(parseInt(resource.memory.used_mb) / 1024 * 10) / 10} GB`} />
              </Col>
              <Col span={8}>
                <Statistic 
                  title="사용량" 
                  value={resource.memory.usage_percent} 
                  suffix="%" 
                  valueStyle={{ 
                    color: parseInt(resource.memory.usage_percent) > 80 ? '#cf1322' : 
                           parseInt(resource.memory.usage_percent) > 60 ? '#faad14' : '#3f8600' 
                  }}
                />
              </Col>
            </Row>
          </Card>
          
          <Card title="디스크">
            <Row gutter={[16, 16]}>
              <Col span={8}>
                <Statistic title="전체" value={resource.disk.root_total} />
              </Col>
              <Col span={8}>
                <Statistic title="사용 중" value={resource.disk.root_used} />
              </Col>
              <Col span={8}>
                <Statistic 
                  title="사용량" 
                  value={resource.disk.root_usage_percent} 
                  suffix="%" 
                  valueStyle={{ 
                    color: parseInt(resource.disk.root_usage_percent) > 80 ? '#cf1322' : 
                           parseInt(resource.disk.root_usage_percent) > 60 ? '#faad14' : '#3f8600' 
                  }}
                />
              </Col>
            </Row>
          </Card>
        </div>
      ) : (
        <Empty description="리소스 정보를 가져올 수 없습니다." />
      )}
    </Modal>
  );
};

// 리소스 인증 모달 컴포넌트 추가
const ResourceAuthModal: React.FC<{
  visible: boolean;
  onClose: () => void;
  onConfirm: (username: string, password: string) => void;
  loading: boolean;
  node: Node | null;
}> = ({ visible, onClose, onConfirm, loading, node }) => {
  const [form] = Form.useForm();
  
  useEffect(() => {
    if (visible) {
      form.resetFields();
    }
  }, [visible, form]);

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      onConfirm(values.username, values.password);
    } catch (error) {
      console.error('폼 유효성 검사 오류:', error);
    }
  };

  return (
    <Modal
      title="서버 리소스 조회를 위한 인증"
      open={visible}
      onCancel={onClose}
      confirmLoading={loading}
      onOk={handleSubmit}
      okText="리소스 조회"
      cancelText="취소"
    >
      <Typography.Paragraph>
        서버 <strong>{node?.server_name || node?.ip}</strong>의 리소스를 조회하기 위한 인증 정보를 입력해주세요.
      </Typography.Paragraph>
      
      <Form
        form={form}
        layout="vertical"
      >
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

interface InfraKubernetesSettingProps {
  infra: InfraItem & { nodes?: Node[] };
  showSettingsModal: (infra: InfraItem) => void;
  isExternal?: boolean; // 추가
}

const InfraKubernetesSetting: React.FC<InfraKubernetesSettingProps> = ({ infra, showSettingsModal, isExternal = false }) => {
  const [isAddNodeModalVisible, setIsAddNodeModalVisible] = useState(false);
  const [isEditNodeModalVisible, setIsEditNodeModalVisible] = useState(false);
  const [isBuildServerModalVisible, setIsBuildServerModalVisible] = useState(false);
  const [isCheckStatusModalVisible, setIsCheckStatusModalVisible] = useState(false);
  const [isHACredentialsModalVisible, setIsHACredentialsModalVisible] = useState(false);
  const [buildingNode, setBuildingNode] = useState<Node | null>(null);
  const [checkingNode, setCheckingNode] = useState<Node | null>(null);
  const [buildingLoading, setBuildingLoading] = useState(false);
  const [checkingLoading, setCheckingLoading] = useState(false);
  const [nodes, setNodes] = useState<Node[]>(infra.nodes || []);
  const [messageApi, contextHolder] = message.useMessage();
  const [activeTab, setActiveTab] = useState<NodeType>('ha');
  const [checkingNodeId, setCheckingNodeId] = useState<string | null>(null);
  const [haCredentials, setHaCredentials] = useState<{username: string, password: string} | null>(null);
  const [pendingMasterBuild, setPendingMasterBuild] = useState<{hopsData: any, username: string, password: string} | null>(null);
  const [isCheckingAllServers, setIsCheckingAllServers] = useState(false);

  // 외부 쿠버네티스 관련 상태 추가
  const [externalAuthModalVisible, setExternalAuthModalVisible] = useState(false);
  const [externalServer, setExternalServer] = useState<{ip: string; port: string;} | null>(null);
  const [externalNodesInfo, setExternalNodesInfo] = useState<{
    total: number;
    master: number;
    worker: number;
    list: any[];
  } | null>(null);

  // 타입별 상태를 저장할 state 추가
  const [nodeTypeStatuses, setNodeTypeStatuses] = useState<{
    [nodeId: string]: {
      [type: string]: { 
        status: ServerStatus; 
        lastChecked: string;
      }
    }
  }>({});

  // 서버 인증 정보를 저장할 상태 추가
  const [serverCredentials, setServerCredentials] = useState<{
    node: Node;
    username: string;
    password: string;
  }[]>([]);
  
  const [deleteWorkerModalVisible, setDeleteWorkerModalVisible] = useState(false);
  const [deleteWorkerLoading, setDeleteWorkerLoading] = useState(false);
  const [selectedNode, setSelectedNode] = useState<Node | null>(null);
  const [deleteMasterModalVisible, setDeleteMasterModalVisible] = useState(false);
  const [deleteMasterLoading, setDeleteMasterLoading] = useState(false);

  // 리소스 조회 관련 상태 추가
  const [resourceModalVisible, setResourceModalVisible] = useState(false);
  const [resourceAuthModalVisible, setResourceAuthModalVisible] = useState(false);
  const [resourceNode, setResourceNode] = useState<Node | null>(null);
  const [resourceLoading, setResourceLoading] = useState(false);
  const [serverResource, setServerResource] = useState<ServerResource | null>(null);

  // 노드 상태에 따른 아이콘 반환 함수 수정
  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'running':
        return <CheckCircleOutlined style={{ color: '#52c41a' }} />;
      case 'stopped':
        return <CloseCircleOutlined style={{ color: '#bfbfbf' }} />;
      case 'maintenance':
        return <SyncOutlined spin style={{ color: '#faad14' }} />;
      case 'preparing':
        return <ClockCircleOutlined style={{ color: '#faad14' }} />;
      case 'checking':
        return <SyncOutlined spin style={{ color: '#1890ff' }} />;
      case '등록':
        return <InfoCircleOutlined style={{ color: '#1890ff' }} />;
      default:
        return <CheckCircleOutlined style={{ color: '#52c41a' }} />;
    }
  };

  // 노드 상태에 따른 텍스트 반환 함수 수정
  const getStatusText = (status: string) => {
    switch (status) {
      case 'running':
        return '활성';
      case 'stopped':
        return '비활성';
      case 'maintenance':
        return '작업 중';
      case 'preparing':
        return '구축 전';
      case 'checking':
        return '조회 중';
      case '등록':
        return '등록';
      default:
        return '알 수 없음';
    }
  };

  // 노드 타입에 따라 필터링 (단순화) - 노드가 타입을 포함하면 해당 탭에 표시
  const haNodes = nodes.filter(node => {
    // 노드 타입 확인 (문자열 또는 배열)
    if (typeof node.nodeType === 'string') {
      // 콤마로 구분된 타입이면 'ha'가 포함되어 있는지 확인
      if (node.nodeType.includes(',')) {
        return node.nodeType.split(',').map(t => t.trim()).includes('ha');
      }
      // 단일 타입이면 'ha'인지 확인
      return node.nodeType === 'ha';
    }
    return false;
  }) || [];
  
  const masterNodes = nodes.filter(node => {
    // 노드 타입 확인 (문자열 또는 배열)
    if (typeof node.nodeType === 'string') {
      // 콤마로 구분된 타입이면 'master'가 포함되어 있는지 확인
      if (node.nodeType.includes(',')) {
        return node.nodeType.split(',').map(t => t.trim()).includes('master');
      }
      // 단일 타입이면 'master'인지 확인
      return node.nodeType === 'master';
    }
    return false;
  }) || [];
  
  const workerNodes = nodes.filter(node => {
    // 노드 타입 확인 (문자열 또는 배열)
    if (typeof node.nodeType === 'string') {
      // 콤마로 구분된 타입이면 'worker'가 포함되어 있는지 확인
      if (node.nodeType.includes(',')) {
        return node.nodeType.split(',').map(t => t.trim()).includes('worker');
      }
      // 단일 타입이면 'worker'인지 확인
      return node.nodeType === 'worker';
    }
    return false;
  }) || [];
  
  // 단일 서버 상태 확인 함수
  const handleCheckStatusConfirm = async (username: string, password: string) => {
    if (!username || !password || !checkingNode) return;
    
    try {
      // 로딩 상태 설정
      setCheckingLoading(true);
      setCheckingNodeId(checkingNode.id);
      
      // 노드 상태를 "조회 중"으로 변경
      setNodes(prev => prev.map(node => {
        if (node.id === checkingNode.id) {
          return { ...node, status: 'checking' as ServerStatus };
        }
        return node;
      }));

      // 호스트 정보 구성
      const hopsData = [{
        host: checkingNode.ip,
        port: parseInt(checkingNode.port),
        username,
        password
      }];
      
      // 노드의 모든 타입에 대해 상태 조회
      const nodeTypes = typeof checkingNode.nodeType === 'string' && checkingNode.nodeType.includes(',')
        ? checkingNode.nodeType.split(',').map(t => t.trim())
        : [checkingNode.nodeType];
      
      const statusResults: { [key: string]: { status: ServerStatus, lastChecked: string } } = {};
      
      // 모든 타입에 대해 API 호출
      for (const type of nodeTypes) {
        try {
          const response = await kubernetesApi.getNodeStatus({
            id: parseInt(checkingNode.id),
            type: type,
            hops: hopsData
          });
          
          let nodeStatus: ServerStatus;
          const serverStatus = response.status;
          
          if (!serverStatus.installed) {
            nodeStatus = 'preparing';
          } else if (serverStatus.running) {
            nodeStatus = 'running';
          } else {
            nodeStatus = 'stopped';
          }
          
          statusResults[type] = {
            status: nodeStatus,
            lastChecked: response.lastChecked
          };
        } catch (error) {
          console.error(`[상태 조회 실패] 노드: ${checkingNode.server_name || checkingNode.ip}, 타입: ${type}`, error);
          statusResults[type] = {
            status: 'preparing' as ServerStatus,
            lastChecked: getCurrentTimeString()
          };
        }
      }

      // 타입별 상태를 state에 저장
      setNodeTypeStatuses(prev => ({
        ...prev,
        [checkingNode.id]: {
          ...prev[checkingNode.id],
          ...statusResults
        }
      }));

      // 노드 상태 업데이트 - 현재 노드가 현재 탭의 타입을 가진 경우에만 해당 타입 상태로 업데이트
      setNodes(prev => prev.map(node => {
        if (node.id === checkingNode.id) {
          // 노드가 현재 활성 탭에 해당하는 타입을 가지고 있는지 확인
          const hasActiveTabType = nodeTypes.includes(activeTab);
          
          // 현재 활성 탭의 타입을 가진 경우에만 상태 업데이트
          if (hasActiveTabType && statusResults[activeTab]) {
            return {
              ...node,
              status: statusResults[activeTab].status,
              last_checked: statusResults[activeTab].lastChecked
            };
          }
          
          // 그렇지 않으면 노드의 첫 번째 타입 상태 사용
          const firstType = nodeTypes[0];
          if (firstType && statusResults[firstType]) {
            return {
              ...node,
              status: statusResults[firstType].status,
              last_checked: statusResults[firstType].lastChecked
            };
          }
        }
        return node;
      }));

      // 인증 정보 저장
      setServerCredentials(prev => {
        const existingIndex = prev.findIndex(cred => cred.node.id === checkingNode.id);
        if (existingIndex >= 0) {
          // 기존 정보 업데이트
          const updated = [...prev];
          updated[existingIndex] = { node: checkingNode, username, password };
          return updated;
        } else {
          // 새 정보 추가
          return [...prev, { node: checkingNode, username, password }];
        }
      });

      messageApi.success('노드 상태 조회가 완료되었습니다.');
      setCheckingLoading(false);
    } catch (error) {
      console.error('노드 상태 조회 실패:', error);
      
      // 에러 발생 시 원래 상태로 복원
      setNodes(prev => prev.map(node => {
        if (node.id === checkingNode.id && node.status === 'checking') {
          return { ...node, status: '등록' as ServerStatus };
        }
        return node;
      }));
      
      messageApi.error('노드 상태 조회에 실패했습니다.');
      setCheckingLoading(false);
    } finally {
      // 모달 닫기
      setIsCheckStatusModalVisible(false);
      setCheckingNodeId(null);
    }
  };

  // 메인 마스터 노드 함수를 추가하여 updated_at 시간을 체크하는 함수
  const isCertificateValid = (updatedAt: string | undefined): boolean => {
    if (!updatedAt) return false;
    
    // 문자열을 Date 객체로 변환
    const updatedDate = new Date(updatedAt);
    const currentDate = new Date();
    
    // 두 시간 차이 계산 (밀리초)
    const timeDifference = currentDate.getTime() - updatedDate.getTime();
    
    // 2시간 = 7,200,000 밀리초
    const twoHoursInMilliseconds = 2 * 60 * 60 * 1000;
    
    // 2시간 이내인지 확인
    return timeDifference <= twoHoursInMilliseconds;
  };

  // 노드 구축 시작
  const handleStartBuild = (node: Node) => {
    // 마스터 조인 또는 워커 추가인 경우 인증서 유효시간 먼저 체크
    const isMaster = node.nodeType === 'master' || (typeof node.nodeType === 'string' && node.nodeType.includes('master'));
    const isWorker = node.nodeType === 'worker' || (typeof node.nodeType === 'string' && node.nodeType.includes('worker'));
    
    // 첫 번째 마스터인지 확인 (join_command와 certificate_key가 있는 노드가 없는 경우)
    const isFirstMaster = isMaster && !nodes.some(n => 
      n.id !== node.id && 
      (n.nodeType === 'master' || (typeof n.nodeType === 'string' && n.nodeType.includes('master'))) && 
      n.status === 'running'
    );
    
    // 마스터 조인 또는 워커 추가인 경우에만 인증서 체크
    if ((isMaster && !isFirstMaster) || isWorker) {
      // 메인 마스터 노드 찾기
      const mainMasterNode = nodes.find(n => 
        (n.nodeType === 'master' || (typeof n.nodeType === 'string' && n.nodeType.includes('master'))) && 
        n.join_command && 
        n.certificate_key
      );
      
      if (!mainMasterNode) {
        messageApi.error('메인 마스터 노드를 찾을 수 없습니다.');
        return;
      }
      
      // 인증서 유효시간 체크
      if (!isCertificateValid(mainMasterNode.updated_at)) {
        Modal.error({
          title: '인증서 만료',
          content: '메인 마스터 노드의 인증서가 만료되었습니다(2시간 이상 경과). 메인 마스터 노드의 인증서를 갱신해주세요.',
          okText: '확인'
        });
        return;
      }
    }
    
    setBuildingNode(node);
    
    // 백그라운드 작업 (마스터/워커 조인)이 아닌 경우에만 상태를 'maintenance'로 변경
    const isBackgroundBuildStart = (activeTab === 'master' || activeTab === 'worker') && node.status === 'preparing';
    if (!isBackgroundBuildStart) {
        const updatedNodes = nodes.map(n => {
        if (n.id === node.id) {
            return { ...n, status: 'maintenance' as ServerStatus };
        }
        return n;
        });
        setNodes(updatedNodes);
    }
        
    // 마스터 노드 설치 시 HA 노드 인증 정보가 필요한지 확인
    if (activeTab === 'master') {
      const haNodes = nodes.filter(n => 
        n.nodeType === 'ha' || 
        (typeof n.nodeType === 'string' && n.nodeType.includes('ha'))
      );
      
      if (haNodes.length > 0 && !haCredentials) {
        // HA 노드 인증 정보가 아직 없다면 HA 인증 모달 먼저 표시
        setIsHACredentialsModalVisible(true);
      } else {
        // HA 인증 정보가 이미 있거나 HA 노드가 없다면 바로 서버 구축 모달 표시
        setIsBuildServerModalVisible(true);
      }
    } else {
      // 마스터 노드가 아닌 경우 바로 서버 구축 모달 표시
      setIsBuildServerModalVisible(true);
    }
  };

  // 노드 제거 처리
  const handleRemoveNode = (nodeId: string) => {
    // 삭제할 노드 찾기
    const targetNode = nodes.find(node => node.id === nodeId);
    if (!targetNode) {
      messageApi.error('노드를 찾을 수 없습니다.');
      return;
    }
    
    // 노드가 구축 중이면 삭제 불가
    if (targetNode.status === 'maintenance') {
      messageApi.warning('노드가 현재 구축 중이거나 작업 중입니다. 작업이 완료된 후 삭제할 수 있습니다.');
      return;
    }

    // 노드 상태에 따라 다른 처리 - 구축 전 상태일 경우 DB에서만 삭제
    if (targetNode.status === '등록' || targetNode.status === 'preparing') {
      // 구축되지 않은 노드는 DB에서만 삭제
      Modal.confirm({
        title: '서버 삭제 확인',
        content: (
          <div>
            <p><strong>{targetNode.server_name || targetNode.ip}</strong> 서버를 삭제하시겠습니까?</p>
            <p>아직 구축되지 않은 노드입니다. 서버 정보만 삭제됩니다.</p>
          </div>
        ),
        okText: '삭제',
        cancelText: '취소',
        okButtonProps: { danger: true },
        onOk: async () => {
          try {
            messageApi.loading(`${targetNode.server_name || targetNode.ip} 서버 삭제 중...`);
            
            // 서버 삭제 API 호출
            await kubernetesApi.deleteServer(parseInt(nodeId));
            
            // UI에서 노드 제거
            const updatedNodes = nodes.filter(node => node.id !== nodeId);
            setNodes(updatedNodes);
            
            messageApi.success(`${targetNode.server_name || targetNode.ip} 서버가 삭제되었습니다.`);
          } catch (error) {
            console.error('서버 삭제 중 오류 발생:', error);
            messageApi.error('서버 삭제에 실패했습니다.');
          }
        }
      });
    } else {
      // 이미 구축된 노드의 경우 노드 유형에 따라 다른 처리
      setSelectedNode(targetNode);
      
      if (targetNode.nodeType === 'worker' || (typeof targetNode.nodeType === 'string' && targetNode.nodeType.includes('worker'))) {
        setDeleteWorkerModalVisible(true);
      } else if (targetNode.nodeType === 'master' || (typeof targetNode.nodeType === 'string' && targetNode.nodeType.includes('master'))) {
        setDeleteMasterModalVisible(true);
      } else if (targetNode.nodeType === 'ha' || (typeof targetNode.nodeType === 'string' && targetNode.nodeType.includes('ha'))) {
        messageApi.warning('HA 노드는 직접 삭제할 수 없습니다. 마스터 노드를 삭제하면 HA 노드도 함께 삭제됩니다.');
      } else {
        // 기타 노드 유형의 경우 단순 삭제
        Modal.confirm({
          title: '서버 삭제 확인',
          content: (
            <div>
              <p><strong>{targetNode.server_name || targetNode.ip}</strong> 서버를 삭제하시겠습니까?</p>
              <p>서버 정보가 완전히 삭제되며, 복구할 수 없습니다.</p>
            </div>
          ),
          okText: '삭제',
          cancelText: '취소',
          okButtonProps: { danger: true },
          onOk: async () => {
            try {
              messageApi.loading(`${targetNode.server_name || targetNode.ip} 서버 삭제 중...`);
              
              // 서버 삭제 API 호출
              await kubernetesApi.deleteServer(parseInt(nodeId));
              
              // UI에서 노드 제거
              const updatedNodes = nodes.filter(node => node.id !== nodeId);
              setNodes(updatedNodes);
              
              messageApi.success(`${targetNode.server_name || targetNode.ip} 서버가 삭제되었습니다.`);
            } catch (error) {
              console.error('서버 삭제 중 오류 발생:', error);
              messageApi.error('서버 삭제에 실패했습니다.');
            }
          }
        });
      }
    }
  };

  // 노드 상태 변경 처리
  const handleChangeNodeStatus = async (nodeId: string, newStatus: 'running' | 'stopped') => {
    try {
      const targetNode = nodes.find(node => node.id === nodeId);
      if (!targetNode) {
        messageApi.error('노드를 찾을 수 없습니다.');
        return;
      }

      // 상태 변경 중임을 표시
      const updatedNodes = nodes.map(node => {
        if (node.id === nodeId) {
          return { ...node, status: 'maintenance' as ServerStatus };
        }
        return node;
      });
      setNodes(updatedNodes);
      
      // 인증 모달 표시
      setBuildingNode(targetNode);
      setIsBuildServerModalVisible(true);
    } catch (error) {
      console.error('노드 상태 변경 준비 중 오류 발생:', error);
      messageApi.error(`노드 ${newStatus === 'running' ? '시작' : '중지'}에 실패했습니다.`);
    }
  };

  // 노드 재시작 처리
  const handleRestartNode = async (nodeId: string) => {
    try {
      const targetNode = nodes.find(node => node.id === nodeId);
      if (!targetNode) {
        messageApi.error('노드를 찾을 수 없습니다.');
        return;
      }

      // 재시작 중임을 표시
      const updatedNodes = nodes.map(node => {
        if (node.id === nodeId) {
          return { ...node, status: 'maintenance' as ServerStatus };
        }
        return node;
      });
      setNodes(updatedNodes);
      
      // 인증 모달 표시
      setBuildingNode(targetNode);
      setIsBuildServerModalVisible(true);
    } catch (error) {
      console.error('노드 재시작 준비 중 오류 발생:', error);
      messageApi.error('노드 재시작에 실패했습니다.');
    }
  };

  // 노드 상태 확인 함수 수정
  const handleCheckNodeStatus = async (nodeId: string, showMessage: boolean = true) => {
    try {
      // 현재 노드 찾기
      const targetNode = nodes.find(node => node.id === nodeId);
      if (!targetNode) {
        if (showMessage) messageApi.error('노드를 찾을 수 없습니다.');
        return;
      }

      // 단일 서버 조회 모드 설정
      setIsCheckingAllServers(false);
      // 모달 표시를 위한 상태 설정
      setCheckingNode(targetNode);
      setIsCheckStatusModalVisible(true);

    } catch (error) {
      console.error('노드 상태 확인 준비 중 오류 발생:', error);
      if (showMessage) messageApi.error('노드 상태 확인 준비에 실패했습니다.');
    }
  };

  // 모든 서버의 상태를 확인하는 함수
  const checkAllServerStatuses = async (credentials: {node: Node; username: string; password: string;}[]) => {
    try {
      setIsCheckingAllServers(true);
      
      // 모든 노드의 상태를 '조회 중'으로 설정
      setNodes(prev => prev.map(node => {
        if (credentials.some(cred => cred.node.id === node.id)) {
          return { ...node, status: 'checking' as ServerStatus };
        }
        return node;
      }));
      
      const statusPromises = credentials.map(async (cred) => {
        try {
          // 호스트 정보 구성
          const hopsData = [{
            host: cred.node.ip,
            port: parseInt(cred.node.port),
            username: cred.username,
            password: cred.password
          }];
          
          // 노드의 모든 타입에 대해 상태 조회
          const nodeTypes = typeof cred.node.nodeType === 'string' && cred.node.nodeType.includes(',')
            ? cred.node.nodeType.split(',').map(t => t.trim())
            : [cred.node.nodeType];
          
          const statusResults: { [key: string]: { status: ServerStatus, lastChecked: string } } = {};
          
          // 모든 타입에 대해 API 호출
          for (const type of nodeTypes) {
            try {
              // API 호출
              const response = await kubernetesApi.getNodeStatus({
                id: parseInt(cred.node.id),
                type: type,
                hops: hopsData
              });
              
              let nodeStatus: ServerStatus;
              const serverStatus = response.status;
              
              if (!serverStatus.installed) {
                nodeStatus = 'preparing';
              } else if (serverStatus.running) {
                nodeStatus = 'running';
              } else {
                nodeStatus = 'stopped';
              }
              
              statusResults[type] = {
                status: nodeStatus,
                lastChecked: response.lastChecked
              };
            } catch (error) {
              console.error(`[상태 조회 실패] 노드: ${cred.node.server_name || cred.node.ip}, 타입: ${type}`, error);
              statusResults[type] = {
                status: cred.node.status === 'checking' ? '등록' : cred.node.status || '등록',
                lastChecked: cred.node.last_checked || ''
              };
            }
          }
          
          // 타입별 상태를 state에 저장
          setNodeTypeStatuses(prev => ({
            ...prev,
            [cred.node.id]: {
              ...prev[cred.node.id],
              ...statusResults
            }
          }));
          
          // 노드 목록 업데이트 - 각 노드의 타입에 맞는 상태 표시
          setNodes(prev => prev.map(node => {
            if (node.id === cred.node.id) {
              // 노드가 현재 활성 탭에 해당하는 타입을 가지고 있는지 확인
              const hasActiveTabType = nodeTypes.includes(activeTab);
              
              // 현재 활성 탭의 타입을 가진 경우에만 해당 타입의 상태로 업데이트
              if (hasActiveTabType && statusResults[activeTab]) {
                return {
                  ...node,
                  status: statusResults[activeTab].status,
                  last_checked: statusResults[activeTab].lastChecked
                };
              }
              
              // 그렇지 않으면 노드의 첫 번째 타입 상태 사용
              const firstType = nodeTypes[0];
              if (firstType && statusResults[firstType]) {
                return {
                  ...node,
                  status: statusResults[firstType].status,
                  last_checked: statusResults[firstType].lastChecked
                };
              }
            }
            return node;
          }));
          
          return { nodeId: cred.node.id, success: true };
        } catch (error) {
          console.error(`서버 ${cred.node.id} 상태 조회 중 오류 발생:`, error);
          
          // 에러 발생 시 상태를 등록으로 변경
          setNodes(prev => prev.map(node => {
            if (node.id === cred.node.id && node.status === 'checking') {
              return { ...node, status: '등록' as ServerStatus };
            }
            return node;
          }));
          
          return { nodeId: cred.node.id, success: false };
        }
      });
      
      // 모든 상태 조회 요청이 완료되면 메시지 표시
      await Promise.allSettled(statusPromises);
      messageApi.success('모든 서버 상태 조회가 완료되었습니다.');
      
      // 인증 정보 초기화
      setServerCredentials([]);
    } catch (error) {
      console.error('서버 상태 조회 중 오류 발생:', error);
      messageApi.error('서버 상태 조회에 실패했습니다.');
    }
  };

  // 탭 변경 시 해당 타입의 상태로 업데이트
  useEffect(() => {
    const updatedNodes = nodes.map(node => {
      // 노드의 타입 목록 구하기
      const nodeTypes = typeof node.nodeType === 'string' && node.nodeType.includes(',')
        ? node.nodeType.split(',').map(t => t.trim())
        : [node.nodeType];
      
      // 현재 노드가 현재 탭의 타입을 포함하는지 확인
      const hasActiveTabType = nodeTypes.includes(activeTab);
      
      // 노드의 저장된 상태 정보 가져오기
      const nodeStatuses = nodeTypeStatuses[node.id];
      
      // 현재 탭에 해당하는 상태가 있고, 노드가 해당 타입을 가진 경우에만 상태 업데이트
      if (nodeStatuses && nodeStatuses[activeTab] && hasActiveTabType) {
        return {
          ...node,
          status: nodeStatuses[activeTab].status,
          last_checked: nodeStatuses[activeTab].lastChecked
        };
      }
      
      return node;
    });
    
    setNodes(updatedNodes);
  }, [activeTab, nodeTypeStatuses]);

  // InfraKubernetesSetting 컴포넌트 내부에 useEffect 추가
  useEffect(() => {
    // 페이지 최초 로드 시에만 실행되도록 수정
    const initialCheck = async () => {
      // 페이지 로드 시 노드 목록 새로고침
      await handleRefreshNodes();
      
      // 페이지 로드 시 상태 조회가 필요한 서버들 필터링
      const serversNeedingStatusCheck = nodes.filter(node => node.last_checked !== undefined && node.last_checked !== '');
      
      if (serversNeedingStatusCheck.length > 0) {
        // 전체 서버 조회 모드 설정
        setIsCheckingAllServers(true);
        // 첫 번째 서버 인증 모달 표시
        setCheckingNode(serversNeedingStatusCheck[0]);
        setIsCheckStatusModalVisible(true);
      }
    };

    initialCheck();
  }, []); // 의존성 배열을 비워서 최초 로드시에만 실행되도록 함

  // CheckStatusModal의 onFinish 핸들러 수정
  const handleAuthenticationSubmit = async (username: string, password: string) => {
    try {
      // 모달 먼저 닫기
      setIsCheckStatusModalVisible(false);
      
      if (!checkingNode) {
        messageApi.error('조회할 서버 정보가 없습니다.');
        return;
      }

      // 전체 서버 조회 모드인 경우
      if (isCheckingAllServers) {
        // 인증 정보를 기존 배열에 추가
        const updatedCredentials = [
          ...serverCredentials,
          { node: checkingNode, username, password }
        ];
        setServerCredentials(updatedCredentials);
        
        // 페이지 로드 시 상태 조회가 필요한 서버들 필터링
        const serversNeedingStatusCheck = nodes.filter(node => node.last_checked !== undefined && node.last_checked !== '');
        
        // 다음 체크할 서버 찾기 (이미 인증 정보가 수집된 서버는 제외)
        const remainingServers = serversNeedingStatusCheck.filter(
          server => !updatedCredentials.some(cred => cred.node.id === server.id)
        );
        
        if (remainingServers.length > 0) {
          // 다음 서버의 인증 모달 표시
          setTimeout(() => {
            setCheckingNode(remainingServers[0]);
            setIsCheckStatusModalVisible(true);
          }, 500); // 0.5초 딜레이를 줘서 UI가 갱신될 시간을 확보
        } else {
          // 모든 서버의 인증 정보가 수집되었으면 상태 조회 시작
          await checkAllServerStatuses(updatedCredentials);
          // 전체 서버 조회 모드 해제
          setIsCheckingAllServers(false);
        }
      } else {
        // 단일 서버 상태 조회인 경우 - handleCheckStatusConfirm 함수 직접 호출
        await handleCheckStatusConfirm(username, password);
      }
    } catch (error) {
      console.error('인증 정보 처리 중 오류 발생:', error);
      messageApi.error('서버 상태 조회에 실패했습니다.');
    }
  };

  // 노드 추가 처리
  const handleAddNode = async (values: {ip: string, port: string, server_name?: string}) => {
    try {
      if (!infra) {
        messageApi.error('인프라를 선택해주세요.');
        return;
      }
      
      // 동일한 IP와 포트를 가진 서버가 이미 존재하는지 확인
      const existingServer = nodes.find(node => 
        node.ip === values.ip && node.port === values.port.toString()
      );
      
      if (existingServer) {
        // 이미 동일한 IP, 포트의 서버가 존재하는 경우
        // 기존 타입에 현재 탭의 타입을 추가
        const existingTypes = typeof existingServer.nodeType === 'string' 
          ? existingServer.nodeType.split(',').map(t => t.trim()) 
          : [existingServer.nodeType];
        
        // 현재 탭 타입이 이미 포함되어 있는지 확인
        if (!existingTypes.includes(activeTab)) {
          // 기존 타입에 새 타입 추가
          const newTypeString = [...existingTypes, activeTab].join(',');
          
          // 서버 업데이트를 위한 데이터 준비
          const updateData: any = {
            id: parseInt(existingServer.id),
            type: newTypeString,
            infra_id: infra.id,
          };
          
          // HA가 아닌 노드 타입을 추가할 때만 server_name이 필요하면 추가
          if (activeTab !== 'ha' && values.server_name) {
            updateData.server_name = values.server_name;
          }
          
          // hops 정보 추가
          const hopsData = [{
            host: values.ip,
            port: parseInt(values.port.toString())
          }];
          updateData.hops = hopsData;
          
          // 서버 업데이트 API 호출
          await kubernetesApi.updateServer(parseInt(existingServer.id), updateData);
          
          // 성공 메시지 - HA 노드인 경우와 다른 노드인 경우 메시지 분리
          if (activeTab === 'ha') {
            messageApi.success(`기존 서버에 HA 타입이 추가되었습니다. (IP: ${existingServer.ip})`);
          } else {
            messageApi.success(`기존 서버에 ${activeTab} 타입이 추가되었습니다. (이름: ${values.server_name || existingServer.server_name || existingServer.ip})`);
          }
        } else {
          messageApi.info(`이미 ${activeTab} 타입으로 등록된 서버입니다.`);
        }
      } else {
        // 새로운 서버 등록
        // 서버 데이터 준비
        const serverData: any = {
          infra_id: infra.id,
          type: activeTab,
          ip: values.ip,
          port: parseInt(values.port),
          status: 'registered' as ServerStatus,
          hops: [{
            host: values.ip,
            port: parseInt(values.port.toString())
          }]
        };
        
        // HA 타입이 아닌 경우에만 서버 이름 포함
        if (activeTab !== 'ha') {
          // 다른 노드 타입은 사용자가 입력한 이름 그대로 사용
          if (values.server_name) {
            serverData.name = values.server_name;
          } else {
            // 이름을 입력하지 않은 경우 오류 메시지 표시
            messageApi.error('마스터/워커 노드는 서버 이름을 반드시 입력해야 합니다.');
            return;
          }
        }
        
        // hops 정보 추가
        const hopsData = [{
          host: values.ip,
          port: parseInt(values.port.toString())
        }];
        serverData.hops = hopsData;
                
        // 서버 생성 API 호출
        await kubernetesApi.createServer(serverData);
        
        // 성공 메시지 표시
        const nodeTypeText = activeTab === 'master' ? '마스터' : activeTab === 'worker' ? '워커' : 'HA';
        if (activeTab === 'ha') {
          messageApi.success(`HA 노드가 추가되었습니다. (IP: ${values.ip})`);
        } else {
          messageApi.success(`${nodeTypeText} 노드가 추가되었습니다. (이름: ${values.server_name})`);
        }
      }
      
      // 모달 닫기 및 데이터 새로고침
      setIsAddNodeModalVisible(false);
      // 노드 목록 새로고침
      handleRefreshNodes();
    } catch (error) {
      console.error('노드 추가 중 오류 발생:', error);
      messageApi.error('노드 추가에 실패했습니다. 다시 시도해주세요.');
    }
  };

  // HA 인증 정보 확인 처리
  const handleHACredentialsConfirm = (username: string, password: string) => {
    setHaCredentials({ username, password });
    setIsHACredentialsModalVisible(false);
    
    // HA 인증 정보 저장 후 서버 구축 모달 표시
    if (buildingNode) {
      setIsBuildServerModalVisible(true);
    }
  };

  // 서버 구축 확인
  const handleBuildConfirm = async (username: string, password: string) => {
    if (!buildingNode) return;

    const isMasterBuild = buildingNode.status === 'preparing' && activeTab === 'master';
    const isWorkerBuild = buildingNode.status === 'preparing' && activeTab === 'worker';
    const isBackgroundBuild = isMasterBuild || isWorkerBuild;

    try {
      // 모달 먼저 닫기
      setIsBuildServerModalVisible(false);

      // 로딩 상태 설정
      setBuildingLoading(true);

      // 마스터/워커 구축이 아닐 경우에만 상태를 'maintenance'로 변경하고 로딩 메시지 표시
      if (!isBackgroundBuild) {
        const updatedNodes = nodes.map(node => {
          if (node.id === buildingNode.id) {
            return { ...node, status: 'maintenance' as ServerStatus };
          }
          return node;
        });
        setNodes(updatedNodes);
        messageApi.loading(`${buildingNode.server_name || buildingNode.ip} 서버 작업을 시작합니다...`);
      } else {
         // 백그라운드 작업 시작 메시지 (선택적)
         messageApi.info(`${buildingNode.server_name || buildingNode.ip} 노드 ${activeTab} 구축 작업을 시작합니다...`);
      }


      try {
        let nodeStatus: ServerStatus | undefined; // Initialize as undefined
        let lastCheckedTime: string | undefined;

        // hops 데이터 구성
        const hopsData = [{
          host: buildingNode.ip,
          port: parseInt(buildingNode.port),
          username,
          password
        }];

        // 현재 액션에 따라 다른 API 호출
        if (buildingNode.status === 'stopped') {
          // 노드 시작
          messageApi.loading(`${buildingNode.server_name || buildingNode.ip} 노드 시작 중...`);
          const response = await kubernetesApi.startServer({
            id: parseInt(buildingNode.id),
            hops: hopsData
          });

          if (response && response.message) {
            nodeStatus = 'running'; // Start is synchronous, update status
            lastCheckedTime = response.lastChecked;
          }
          messageApi.success(`${buildingNode.server_name || buildingNode.ip} 노드가 시작되었습니다.`);
        }
        else if (buildingNode.status === 'running') {
          // 노드 재시작
          messageApi.loading(`${buildingNode.server_name || buildingNode.ip} 노드 재시작 중...`);
          const response = await kubernetesApi.restartServer({
            id: parseInt(buildingNode.id),
            hops: hopsData
          });

          if (response && response.message) {
            nodeStatus = 'running'; // Restart is synchronous, update status
            lastCheckedTime = response.lastChecked;
          }
          messageApi.success(`${buildingNode.server_name || buildingNode.ip} 노드가 재시작되었습니다.`);
        }
        else { // Build cases ('preparing' status)
          switch (activeTab) {
            case 'ha':
              messageApi.loading(`로드밸런서(HA) 설치 중...`);
              const haResponse = await kubernetesApi.installLoadBalancer({
                id: parseInt(buildingNode.id),
                hops: hopsData
              });
              messageApi.success(`로드밸런서(HA) 설치 완료`);

              if (haResponse && haResponse.success) {
                nodeStatus = 'maintenance'; // HA install seems synchronous enough, keep maintenance for now
                lastCheckedTime = haResponse.lastChecked || getCurrentTimeString();
                setServerCredentials(prev => {
                  const existingIndex = prev.findIndex(cred => cred.node.id === buildingNode.id);
                  if (existingIndex >= 0) {
                    const updated = [...prev];
                    updated[existingIndex] = { node: buildingNode, username, password };
                    return updated;
                  } else {
                    return [...prev, { node: buildingNode, username, password }];
                  }
                });
                messageApi.info(`설치 완료 후 '상태 확인' 버튼을 클릭하여 노드 상태를 확인해주세요.`);
              } else {
                 nodeStatus = 'preparing'; // Revert on failure
                 lastCheckedTime = getCurrentTimeString();
              }
              break;

            case 'master':
              // 첫 번째 마스터인지 확인
              let isFirstMaster = !nodes.some(node => 
                node.id !== buildingNode.id && 
                (
                  (typeof node.nodeType === 'string' && node.nodeType.includes('master')) ||
                  node.nodeType === 'master'
                ) && 
                node.status === 'running'
              );

              if (isFirstMaster) {
                // 모든 HA 노드 찾기
                const haNodes = nodes.filter(node => 
                  node.nodeType === 'ha' || 
                  (typeof node.nodeType === 'string' && node.nodeType.includes('ha'))
                );

                if (haNodes.length === 0) {
                  messageApi.error('HA 노드가 필요합니다.');
                  // 상태 복원
                  nodeStatus = 'preparing'; // 구축 전 상태로 되돌림
                  lastCheckedTime = getCurrentTimeString();
                  throw new Error('HA 노드가 필요합니다.'); // 에러를 발생시켜 finally 블록에서 로딩 해제
                }

                // HA 노드 인증 정보가 없으면 저장
                if (!haCredentials) {
                  setPendingMasterBuild({ hopsData, username, password });
                  setIsHACredentialsModalVisible(true);
                  // 로딩 상태를 여기서 바로 해제하지 않음 (모달이 닫힐 때 처리)
                  return; // 인증 모달이 뜰 때까지 대기
                }

                // HA 노드의 hops 정보 구성 (HA 인증 정보 사용)
                const lb_hops = haNodes.map(haNode => ({
                  host: haNode.ip,
                  port: parseInt(haNode.port),
                  username: haCredentials.username,
                  password: haCredentials.password
                }));
                                
                // 첫 번째 마스터 노드 설치 API 호출
                try {
                  const firstMasterResponse = await kubernetesApi.installFirstMaster({
                    id: parseInt(buildingNode.id),
                    hops: hopsData,
                    lb_hops: lb_hops,
                    password: password,
                    lb_password: haCredentials.password
                  });
                  
                  // 성공적으로 설치 시작되었을 때
                  if (firstMasterResponse.success) {
                    messageApi.success(`첫 번째 마스터 노드 설치가 백그라운드에서 시작되었습니다.`);
                    messageApi.info(`설치 완료까지 약 5-10분 정도 소요됩니다. 잠시 후 '상태 확인' 버튼을 클릭하여 진행 상황을 확인해주세요.`); // 메시지 수정
                    // nodeStatus = 'maintenance'; // <-- REMOVED
                    lastCheckedTime = getCurrentTimeString(); // Still set time if needed
                    setServerCredentials(prev => {
                      const existingIndex = prev.findIndex(cred => cred.node.id === buildingNode.id);
                      if (existingIndex >= 0) {
                        const updated = [...prev];
                        updated[existingIndex] = { node: buildingNode, username, password };
                        return updated;
                      } else {
                        return [...prev, { node: buildingNode, username, password }];
                      }
                    });
                    
                    // 메인 마스터 노드로 바로 표시하기 위해 join_command와 certificate_key 임시 설정
                    setNodes(prev => prev.map(n => {
                      if (n.id === buildingNode.id) {
                        return {
                          ...n,
                          // 임시 값 설정 - 실제 값은 아니지만 UI에 main master로 표시되게 함
                          join_command: 'installing',
                          certificate_key: 'installing',
                          updated_at: getCurrentTimeString()
                        };
                      }
                      return n;
                    }));
                                        
                  } else {
                    // 백엔드에서 요청은 성공했지만 설치 시작에 실패한 경우
                    messageApi.error(`첫 번째 마스터 노드 설치 시작 실패: ${firstMasterResponse.message || '알 수 없는 오류'}`);
                    nodeStatus = 'preparing'; // Revert status on explicit failure to start
                    lastCheckedTime = getCurrentTimeString();
                  }
                } catch (error) {
                  console.error('첫 번째 마스터 노드 설치 실패:', error);
                  messageApi.error(`첫 번째 마스터 노드 설치 중 오류가 발생했습니다.`);
                  // 노드 상태를 'preparing'으로 설정 (재시도 가능하도록)
                  nodeStatus = 'preparing'; // Revert status on error
                  lastCheckedTime = getCurrentTimeString();
                  throw error; // 에러를 다시 던져서 상위 catch 블록에서 처리하도록 함
                }
              } else { // Join Master
                // 모든 HA 노드 찾기
                const haNodes = nodes.filter(node => 
                  node.nodeType === 'ha' || 
                  (typeof node.nodeType === 'string' && node.nodeType.includes('ha'))
                );

                if (haNodes.length === 0) {
                  messageApi.error('HA 노드가 필요합니다.');
                  nodeStatus = 'preparing'; // Revert status
                  lastCheckedTime = getCurrentTimeString();
                  throw new Error('HA 노드가 필요합니다.');
                }

                // HA 노드 인증 정보가 없으면 저장
                if (!haCredentials) {
                  setPendingMasterBuild({ hopsData, username, password });
                  setIsHACredentialsModalVisible(true);
                  return;
                }

                // 모든 HA 노드의 hops 정보 구성 (HA 인증 정보 사용)
                const lb_hops = haNodes.map(haNode => ({
                  host: haNode.ip,
                  port: parseInt(haNode.port),
                  username: haCredentials.username,
                  password: haCredentials.password
                }));
                
                // 메인 마스터 노드 찾기 (join_command와 certificate_key가 있는 노드)
                const mainMasterNode = nodes.find(node => 
                  ((typeof node.nodeType === 'string' && node.nodeType.includes('master')) ||
                   node.nodeType === 'master') && 
                  node.join_command && 
                  node.certificate_key
                );
                
                if (!mainMasterNode) {
                  messageApi.error('메인 마스터 노드를 찾을 수 없습니다.');
                  nodeStatus = 'preparing'; // Revert status
                  lastCheckedTime = getCurrentTimeString();
                  throw new Error('메인 마스터 노드를 찾을 수 없습니다.');
                }
                
                messageApi.loading(`마스터 노드 추가(Join) 중...`); // Keep loading message for join
                const joinResponse = await kubernetesApi.joinMaster({
                  id: parseInt(buildingNode.id),
                  hops: hopsData,
                  lb_hops: lb_hops,
                  password: password,
                  lb_password: haCredentials.password,
                  main_id: parseInt(mainMasterNode.id) // 메인 마스터 노드 ID 추가
                });
                messageApi.success(`마스터 노드 추가(Join) 완료`);

                if (joinResponse && joinResponse.message) {
                  // nodeStatus = 'maintenance'; // <-- REMOVED
                  lastCheckedTime = getCurrentTimeString(); // Still set time if needed
                  setServerCredentials(prev => {
                    const existingIndex = prev.findIndex(cred => cred.node.id === buildingNode.id);
                    if (existingIndex >= 0) {
                      const updated = [...prev];
                      updated[existingIndex] = { node: buildingNode, username, password };
                      return updated;
                    } else {
                      return [...prev, { node: buildingNode, username, password }];
                    }
                  });
                  messageApi.info(`마스터 노드 추가 완료 후 '상태 확인' 버튼을 클릭하여 노드 상태를 확인해주세요.`);
                } else {
                  // If join API returns without message, assume preparing
                  nodeStatus = 'preparing';
                  lastCheckedTime = getCurrentTimeString();
                }
              }
              break;

            case 'worker':
              // 메인 마스터 노드 찾기 (join_command와 certificate_key가 있는 노드)
              const mainMasterNodeWorker = nodes.find(node => 
                ((typeof node.nodeType === 'string' && node.nodeType.includes('master')) ||
                 node.nodeType === 'master') && 
                node.join_command && 
                node.certificate_key
              );
              
              if (!mainMasterNodeWorker) {
                messageApi.error('메인 마스터 노드를 찾을 수 없습니다.');
                nodeStatus = 'preparing'; // Revert status
                lastCheckedTime = getCurrentTimeString();
                throw new Error('메인 마스터 노드를 찾을 수 없습니다.');
              }
              
              messageApi.loading(`워커 노드 구축 중...`); // Keep loading message for worker join
              const workerResponse = await kubernetesApi.joinWorker({
                id: parseInt(buildingNode.id),
                hops: hopsData,
                password: password,
                main_id: parseInt(mainMasterNodeWorker.id) // 메인 마스터 노드 ID 추가
              });
              messageApi.success(`워커 노드 설치 완료`);

              if (workerResponse && workerResponse.message) {
                // nodeStatus = 'maintenance'; // <-- REMOVED
                lastCheckedTime = getCurrentTimeString(); // Still set time if needed
                setServerCredentials(prev => {
                  const existingIndex = prev.findIndex(cred => cred.node.id === buildingNode.id);
                  if (existingIndex >= 0) {
                    const updated = [...prev];
                    updated[existingIndex] = { node: buildingNode, username, password };
                    return updated;
                  } else {
                    return [...prev, { node: buildingNode, username, password }];
                  }
                });
                messageApi.info(`워커 노드 설치 완료 후 '상태 확인' 버튼을 클릭하여 노드 상태를 확인해주세요.`);
              } else {
                 // If worker API returns without message, assume preparing
                 nodeStatus = 'preparing';
                 lastCheckedTime = getCurrentTimeString();
              }
              break;
          }
        }

        // 노드 상태 업데이트 (nodeStatus가 설정된 경우에만 실행됨 - start, restart, ha, failed builds)
        if (nodeStatus && lastCheckedTime) {
          const finalUpdatedNodes = nodes.map(node => {
            if (node.id === buildingNode.id) {
              return {
                ...node,
                status: nodeStatus as ServerStatus,
                last_checked: lastCheckedTime
              };
            }
            return node;
          });
          setNodes(finalUpdatedNodes);
        }

      } catch (apiError) {
        console.error('API 호출 중 오류 발생:', apiError);
        messageApi.error('작업에 실패했습니다. 서버 연결을 확인해주세요.');

        // 에러 발생 시 원래 상태로 복원 (buildingNode의 원래 상태 사용)
        const revertNodes = nodes.map(node => {
          if (node.id === buildingNode.id) {
             // Use the status from the original buildingNode state
             return { ...node, status: buildingNode.status }; 
          }
          return node;
        });
        setNodes(revertNodes);
      }

      setBuildingLoading(false);
      setBuildingNode(null);
      setPendingMasterBuild(null);
    } catch (error) {
      // Outer try-catch for modal validation errors etc.
      console.error('서버 작업 중 오류 발생:', error);
      messageApi.error('작업에 실패했습니다. 다시 시도해주세요.');
      setBuildingLoading(false);
      setBuildingNode(null);
      setPendingMasterBuild(null);
      // Revert status if it was changed earlier (only relevant for non-background builds now)
      if (buildingNode && !isBackgroundBuild) {
          const revertNodes = nodes.map(node => {
            if (node.id === buildingNode.id) {
              return { ...node, status: buildingNode.status }; 
            }
            return node;
          });
          setNodes(revertNodes);
      }
    }
  };

  // 노드 목록 새로고침
  const handleRefreshNodes = async () => {
    try {
      if (!infra) {
        messageApi.error('인프라를 선택해주세요.');
        return;
      }
      
      // 서버 목록 가져오기 API 호출
      const response = await kubernetesApi.getServers(infra.id);
      
      // response가 없거나 servers가 없는 경우 빈 배열로 처리
      if (!response || !response.servers) {
        console.log('서버 목록이 비어있거나 응답이 없습니다:', response);
        setNodes([]);
        return;
      }
      
      // serverList가 null이거나 undefined인 경우 빈 배열로 처리
      const serverList = response.servers || [];
      
      // 서버 데이터를 노드 데이터로 변환
      const refreshedNodes = serverList.map((server: any) => {
        // hops 데이터 파싱 (안전하게 처리)
        let ip = "";
        let port = "";
        
        try {
          if (server.hops) {
            const hopsData = typeof server.hops === 'string' 
              ? JSON.parse(server.hops)
              : server.hops;
              
            if (Array.isArray(hopsData) && hopsData.length > 0) {
              ip = hopsData[0].host || "";
              port = hopsData[0].port ? hopsData[0].port.toString() : "";
            }
          }
        } catch (error) {
          console.error('hops 데이터 파싱 오류:', error);
        }
      
        // 노드 유형 처리 - 타입을 분리하여 처리 (안전하게)
        let nodeType: NodeType = 'worker'; // 기본값
        
        if (server.type) {
          // 타입을 그대로 저장 (복합 타입도 유지)
          nodeType = server.type;
        }
        
        // 초기 상태 결정 (last_checked 기준)
        let initialStatus: ServerStatus = '등록';
        if (server.last_checked) {
          // 이전에 조회된 적이 있으면 'preparing'으로 설정
          initialStatus = 'preparing';
        }
        
        return {
          id: `${server.id || ''}`,
          nodeType: nodeType,
          ip: ip,
          port: port,
          status: initialStatus,
          server_name: server.server_name || '',
          join_command: server.join_command || '',
          certificate_key: server.certificate_key || '',
          last_checked: server.last_checked || '',
          hops: server.hops || '',
          updated_at: server.updated_at || '',
          ha: server.ha || 'N' // Added ha field
        };
      });
      
      // 노드 데이터 업데이트
      setNodes(refreshedNodes);    
    } catch (error) {
      console.error('노드 목록 새로고침 중 오류 발생:', error);
      messageApi.error('노드 목록 새로고침에 실패했습니다.');
      // 에러 발생 시 빈 배열로 설정
      setNodes([]);
    }
  };

  // 노드 편집 처리
  const handleEditNode = async (values: {id: string, port: string, server_name: string}) => {
    try {
      if (!infra) {
        messageApi.error('인프라를 선택해주세요.');
        return;
      }

      // 편집할 노드 찾기
      const targetNode = nodes.find(node => node.id === values.id);
      if (!targetNode) {
        messageApi.error('편집할 노드를 찾을 수 없습니다.');
        return;
      }

      // 업데이트 데이터 준비
      const updateData: any = {
        id: parseInt(values.id),
        port: parseInt(values.port)
      };

      // HA 노드가 아닌 경우에만 서버 이름 추가
      const isHA = targetNode.nodeType === 'ha' || (typeof targetNode.nodeType === 'string' && targetNode.nodeType.includes('ha'));
      if (!isHA) {
        updateData.server_name = values.server_name;
      }

      // hops 데이터 업데이트
      const hopsArray = JSON.parse(targetNode.hops || '[]');
      if (hopsArray.length > 0) {
        hopsArray[0].port = parseInt(values.port);
        updateData.hops = JSON.stringify(hopsArray);
      }

      // 서버 업데이트 API 호출 - 새로운 API 사용
      await api.kubernetes.request('updateServer', updateData);

      // 성공 메시지 표시
      messageApi.success(`노드 정보가 수정되었습니다. (${isHA ? 'HA 노드' : `이름: ${values.server_name}`})`);

      // 모달 닫기 및 데이터 새로고침
      setIsEditNodeModalVisible(false);
      // 노드 목록 새로고침
      handleRefreshNodes();
    } catch (error) {
      console.error('노드 편집 중 오류 발생:', error);
      messageApi.error('노드 편집에 실패했습니다. 다시 시도해주세요.');
    }
  };

  // timeString 생성 함수
  const getCurrentTimeString = () => {
    return new Date().toISOString();
  };

  // 노드 테이블 컬럼 정의
  const nodeColumns = [
    {
      title: '노드 ID',
      dataIndex: 'id',
      key: 'id',
      width: 80
    },
    {
      title: '서버 이름',
      dataIndex: 'server_name',
      key: 'server_name',
      width: 120
    },
    {
      title: '유형',
      dataIndex: 'nodeType',
      key: 'nodeType',
      width: 120,
      render: (nodeType: string, record: Node) => {
        // 복합 타입 처리 (콤마로 구분된 타입)
        if (typeof nodeType === 'string' && nodeType.includes(',')) {
          const types = nodeType.split(',').map(t => t.trim());
          return (
            <Space size={2} style={{ display: 'flex', flexDirection: 'row', flexWrap: 'nowrap' }}>
              {types.includes('ha') && (
                <Tag
                  style={{ 
                    backgroundColor: '#e6f4ff', 
                    color: '#1677ff',
                    border: '1px solid #91caff',
                    borderRadius: '4px',
                    padding: '0 4px',
                    fontSize: '11px',
                    width: '42px',
                    textAlign: 'center',
                    margin: '0 1px',
                    height: '20px',
                    lineHeight: '18px'
                  }}
                >
                  ha
                </Tag>
              )}
              {types.includes('master') && (
                <Tag
                  style={{ 
                    backgroundColor: '#f0f9eb', 
                    color: '#52c41a',
                    border: '1px solid #b7eb8f',
                    borderRadius: '4px',
                    padding: '0 4px',
                    fontSize: '11px',
                    width: record.join_command && record.certificate_key ? '80px' : '42px',
                    textAlign: 'center',
                    margin: '0 1px',
                    height: '20px',
                    lineHeight: '18px'
                  }}
                >
                  {record.join_command && record.certificate_key ? 'main master' : 'master'}
                </Tag>
              )}
              {types.includes('worker') && (
                <Tag
                  style={{ 
                    backgroundColor: '#fff2e8', 
                    color: '#fa541c',
                    border: '1px solid #ffbb96',
                    borderRadius: '4px',
                    padding: '0 4px',
                    fontSize: '11px',
                    width: '42px',
                    textAlign: 'center',
                    margin: '0 1px',
                    height: '20px',
                    lineHeight: '18px'
                  }}
                >
                  worker
                </Tag>
              )}
            </Space>
          );
        }
        
        // 단일 타입 처리
        // 현재 활성화된 탭에 따라 적절한 스타일 설정
        let style = {};
        let text = '';
        
        if (activeTab === 'ha' || nodeType === 'ha') {
          text = 'ha';
          style = {
            backgroundColor: '#e6f4ff', 
            color: '#1677ff',
            border: '1px solid #91caff',
            borderRadius: '4px',
            padding: '0 4px',
            fontSize: '11px',
            width: '42px',
            textAlign: 'center',
            margin: '0 auto',
            height: '20px',
            lineHeight: '18px'
          };
        } else if (activeTab === 'master' || nodeType === 'master') {
          // main master 조건 확인: join_command와 certificate_key가 있으면 main
          text = record.join_command && record.certificate_key ? 'main master' : 'master';
          style = {
            backgroundColor: '#f0f9eb', 
            color: '#52c41a',
            border: '1px solid #b7eb8f',
            borderRadius: '4px',
            padding: '0 4px',
            fontSize: '11px',
            width: record.join_command && record.certificate_key ? '80px' : '42px',
            textAlign: 'center',
            margin: '0 auto',
            height: '20px',
            lineHeight: '18px'
          };
        } else if (activeTab === 'worker' || nodeType === 'worker') {
          text = 'worker';
          style = {
            backgroundColor: '#fff2e8', 
            color: '#fa541c',
            border: '1px solid #ffbb96',
            borderRadius: '4px',
            padding: '0 4px',
            fontSize: '11px',
            width: '42px',
            textAlign: 'center',
            margin: '0 auto',
            height: '20px',
            lineHeight: '18px'
          };
        }
        
        return <Tag style={style}>{text}</Tag>;
      }
    },
    {
      title: 'IP 주소',
      dataIndex: 'ip',
      key: 'ip',
      width: 130
    },
    {
      title: '포트',
      dataIndex: 'port',
      key: 'port',
      width: 80
    },
    {
      title: '상태',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: ServerStatus) => {
        return (
          <Space>
            {getStatusIcon(status)}
            <span>{getStatusText(status)}</span>
          </Space>
        );
      }
    },
    {
      title: '최근 상태 조회',
      dataIndex: 'last_checked',
      key: 'last_checked',
      width: 150,
      render: (lastChecked: string) => {
        if (!lastChecked) return '-';
        
        try {
          const date = new Date(lastChecked);
          
          return date.toLocaleDateString('ko-KR', {
            year: 'numeric',
            month: '2-digit',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit',
            hour12: false
          }).replace(/\s+/g, ' ');
        } catch (error) {
          return lastChecked;
        }
      }
    },
    {
      title: '작업',
      key: 'action',
      width: 300,
      render: (_: any, record: Node) => renderNodeActions(record)
    }
  ];

  // 현재 노드에 대한 액션을 수행하는 함수
  const handleNodeAction = async (node: Node, action: string) => {
    // 인증 정보 찾기
    const credentials = serverCredentials.find(cred => cred.node.id === node.id);
    
    if (!credentials && action !== 'delete') {
      messageApi.error('이 노드에 대한 인증 정보가 없습니다. 먼저 상태 확인을 수행해주세요.');
      return;
    }
    
    try {
      // 호스트 연결 정보 구성
      const hopsData = credentials ? [{
        host: node.ip,
        port: node.port || '22',
        username: credentials.username,
        password: credentials.password
      }] : [];
      
      // 마스터 노드 ID 찾기 (첫 번째 마스터 노드 사용)
      const masterNode = nodes.find(n => 
        (n.nodeType === 'master' || (typeof n.nodeType === 'string' && n.nodeType.includes('master'))) && 
        n.join_command && 
        n.certificate_key
      );
      const masterCredentials = masterNode ? serverCredentials.find(cred => cred.node.id === masterNode.id) : null;
      
      // 마스터 노드 hops 데이터
      const masterHopsData = masterCredentials ? [{
        host: masterNode!.ip,
        port: masterNode!.port || '22',
        username: masterCredentials.username,
        password: masterCredentials.password
      }] : [];
      
      // 액션 시작 전 노드 상태를 'maintenance'로 변경
      const updatedNodes = nodes.map(n => {
        if (n.id === node.id) {
          return { ...n, status: 'maintenance' as ServerStatus };
        }
        return n;
      });
      setNodes(updatedNodes);
      
      // 액션에 따른 처리
      switch (action) {
        case 'start':
          // 노드 시작
          messageApi.loading(`${node.server_name || '노드'} 시작 중...`);
          const startResponse = await kubernetesApi.startServer({
            id: Number(node.id),
            hops: hopsData
          });
          
          // 상태 업데이트
          setNodes(prev => prev.map(n => {
            if (n.id === node.id) {
              return { 
                ...n, 
                status: 'running' as ServerStatus,
                last_checked: startResponse.lastChecked || getCurrentTimeString()
              };
            }
            return n;
          }));
          
          messageApi.success(`${node.server_name || '노드'} 시작이 완료되었습니다.`);
          break;
          
        case 'restart':
          // 노드 재시작
          messageApi.loading(`${node.server_name || '노드'} 재시작 중...`);
          const restartResponse = await kubernetesApi.restartServer({
            id: Number(node.id),
            hops: hopsData
          });
          
          // 상태 업데이트
          setNodes(prev => prev.map(n => {
            if (n.id === node.id) {
              return { 
                ...n, 
                status: 'running' as ServerStatus,
                last_checked: restartResponse.lastChecked || getCurrentTimeString()
              };
            }
            return n;
          }));
          
          messageApi.success(`${node.server_name || '노드'} 재시작이 완료되었습니다.`);
          break;
          
        case 'stop':
          // 노드 중지
          messageApi.loading(`${node.server_name || '노드'} 중지 중...`);
          const stopResponse = await kubernetesApi.stopServer({
            id: Number(node.id),
            hops: hopsData
          });
          
          // 상태 업데이트
          setNodes(prev => prev.map(n => {
            if (n.id === node.id) {
              return { 
                ...n, 
                status: 'stopped' as ServerStatus,
                last_checked: stopResponse.lastChecked || getCurrentTimeString()
              };
            }
            return n;
          }));
          
          messageApi.success(`${node.server_name || '노드'} 중지가 완료되었습니다.`);
          break;
          
        case 'delete':
          // 노드 삭제 처리
          handleRemoveNode(node.id);
          break;
          
        default:
          messageApi.warning('지원하지 않는 액션입니다.');
      }
    } catch (error) {
      console.error(`${action} 액션 처리 중 오류 발생:`, error);
      messageApi.error(`${action} 액션 처리에 실패했습니다.`);
      
      // 에러 발생 시 원래 상태로 복원
      setNodes(prev => prev.map(n => {
        if (n.id === node.id && n.status === 'maintenance') {
          return { ...n, status: node.status };
        }
        return n;
      }));
    }
  };

  // 노드 액션 버튼 렌더링
  const renderNodeActions = (node: Node) => {
    // 노드 상태와 타입에 따라 다른 액션 버튼 표시
    const actions = [];
    
    // 모든 상태에서 상태 확인 버튼 표시
    actions.push(
      <Button
        key="check"
        size="small"
        icon={<SearchOutlined />}
        onClick={() => handleCheckNodeStatus(node.id)}
      >
        상태 확인
      </Button>
    );
    
    // 리소스 확인 버튼 추가 (노드가 active인 경우만)
    if (node.status === 'running') {
      actions.push(
        <Button
          key="resource"
          size="small"
          icon={<DashboardOutlined />}
          onClick={() => {
            setResourceNode(node);
            setResourceAuthModalVisible(true);
          }}
        >
          리소스 확인
        </Button>
      );
    }
    
    // 노드 유형 확인
    const isHA = node.nodeType === 'ha' || (typeof node.nodeType === 'string' && node.nodeType.includes('ha'));
    const isMaster = node.nodeType === 'master' || (typeof node.nodeType === 'string' && node.nodeType.includes('master'));
    const isWorker = node.nodeType === 'worker' || (typeof node.nodeType === 'string' && node.nodeType.includes('worker'));
    const isMainMaster = isMaster && node.join_command && node.certificate_key;
    
    // 첫 번째 마스터인지 확인 (join_command와 certificate_key가 있는 노드)
    const isFirstMaster = isMaster && !nodes.some(n => 
      n.id !== node.id && 
      (n.nodeType === 'master' || (typeof n.nodeType === 'string' && n.nodeType.includes('master'))) && 
      n.status === 'running'
    );
    
    // 마스터 노드 삭제 버튼 - 상태와 관계없이 추가
    if (isMaster && activeTab === 'master') {
      // 마스터 노드 삭제 버튼 추가
      // 메인 마스터 노드는 다른 노드가 없을 때만 삭제 가능
      const otherMasterCount = nodes.filter(n => 
        n.id !== node.id && (n.nodeType === 'master' || (typeof n.nodeType === 'string' && n.nodeType.includes('master')))
      ).length;
      
      const otherWorkerCount = nodes.filter(n => 
        n.nodeType === 'worker' || (typeof n.nodeType === 'string' && n.nodeType.includes('worker'))
      ).length;
      
      // 메인 마스터 노드는 다른 노드가 없을 때만 삭제 가능
      const canDeleteMainMaster = isMainMaster && otherMasterCount === 0 && otherWorkerCount === 0;
      
      // 일반 마스터 노드는 언제든 삭제 가능
      const canDeleteMaster = !isMainMaster || canDeleteMainMaster;
      
      if (canDeleteMaster) {
        actions.push(
          <Button
            key="delete"
            size="small"
            icon={<DeleteOutlined />}
            danger
            onClick={() => handleRemoveNode(node.id)}
          >
            삭제
          </Button>
        );
      }
    }
    
    // 워커 노드 삭제 버튼 - 상태와 관계없이 추가
    if (isWorker && activeTab === 'worker') {
      actions.push(
        <Button
          key="delete"
          size="small"
          icon={<DeleteOutlined />}
          danger
          onClick={() => handleRemoveNode(node.id)}
        >
          삭제
        </Button>
      );
    }
    
    // 상태에 따른 액션 버튼 추가
    switch (node.status) {
      case '등록':
        // 등록 상태: 탭에 따라 다른 구축 버튼 표시
        if (activeTab === node.nodeType || (typeof node.nodeType === 'string' && node.nodeType.includes(activeTab))) {
          // 현재 활성화된 탭과 노드 유형이 일치하는 경우 상세 액션 버튼 표시
          if (!isMaster && !isWorker) {  // 마스터와 워커는 위에서 이미 삭제 버튼을 추가했음
            actions.push(
              <Button
                key="delete"
                size="small"
                icon={<DeleteOutlined />}
                danger
                onClick={() => handleRemoveNode(node.id)}
              >
                삭제
              </Button>
            );
          }
        }
        break;
        
      case 'preparing':
        // 구축 전 상태: 구축 버튼과 삭제 버튼
        // 탭에 따른 구축 버튼 표시
        if (activeTab === node.nodeType || (typeof node.nodeType === 'string' && node.nodeType.includes(activeTab))) {
          let buildButtonText = '구축';
          let icon = <ToolOutlined />;
          
          if (isHA && activeTab === 'ha') {
            buildButtonText = 'HA 구축';
          } else if (isMaster && activeTab === 'master') {
            buildButtonText = isFirstMaster ? '첫 마스터 구축' : '마스터 조인';
            icon = <ClusterOutlined />;
          } else if (isWorker && activeTab === 'worker') {
            buildButtonText = '워커 구축';
            icon = <CloudServerOutlined />;
          }
          
          actions.push(
            <Button
              key="build"
              size="small"
              icon={icon}
              onClick={() => handleStartBuild(node)}
            >
              {buildButtonText}
            </Button>
          );
        }
        
        // 삭제 버튼은 HA 노드에만 표시 (마스터와 워커는 위에서 이미 삭제 버튼을 추가했음)
        if (!isMaster && !isWorker) {
          actions.push(
            <Button
              key="delete"
              size="small"
              icon={<DeleteOutlined />}
              danger
              onClick={() => handleRemoveNode(node.id)}
            >
              삭제
            </Button>
          );
        }
        break;
        
      // case 'running':
      //   // 활성 상태: 중지, 재시작 버튼
      //   // 현재 활성화된 탭과 노드 유형이 일치하는 경우 상세 액션 버튼 표시
      //   if (activeTab === node.nodeType || (typeof node.nodeType === 'string' && node.nodeType.includes(activeTab))) {
      //     actions.push(
      //       <Button
      //         key="stop"
      //         size="small"
      //         icon={<PoweroffOutlined />}
      //         onClick={() => handleNodeAction(node, 'stop')}
      //       >
      //         중지
      //       </Button>
      //     );
          
      //     actions.push(
      //       <Button
      //         key="restart"
      //         size="small"
      //         icon={<SyncOutlined />}
      //         onClick={() => handleNodeAction(node, 'restart')}
      //       >
      //         재시작
      //       </Button>
      //     );
      //   }
      //   break;
        
      case 'stopped':
        // 비활성 상태: 시작 버튼
        // 현재 활성화된 탭과 노드 유형이 일치하는 경우 상세 액션 버튼 표시
        if (activeTab === node.nodeType || (typeof node.nodeType === 'string' && node.nodeType.includes(activeTab))) {
          actions.push(
            <Button
              key="start"
              size="small"
              icon={<PlayCircleOutlined />}
              onClick={() => handleNodeAction(node, 'start')}
            >
              시작
            </Button>
          );
        }
        
        // HA 노드에 대한 삭제 버튼 (마스터와 워커는 위에서 이미 삭제 버튼을 추가했음)
        if (!isMaster && !isWorker && (activeTab === node.nodeType || (typeof node.nodeType === 'string' && node.nodeType.includes(activeTab)))) {
          actions.push(
            <Button
              key="delete"
              size="small"
              icon={<DeleteOutlined />}
              danger
              onClick={() => handleRemoveNode(node.id)}
            >
              삭제
            </Button>
          );
        }
        break;
      
      default:
        // 다른 상태(maintenance, checking 등)의 경우 추가 버튼 없음
        break;
    }
    
    return <Space>{actions}</Space>;
  };

  // 서버 구축 모달 닫기 처리
  const handleBuildServerModalClose = () => {
    // 모달 닫기
    setIsBuildServerModalVisible(false);
    
    // 구축 중인 노드가 있는 경우 상태 복원
    if (buildingNode) {
      // 노드 상태를 원래대로 복원
      setNodes(prev => prev.map(node => {
        if (node.id === buildingNode.id && node.status === 'maintenance') {
          return { 
            ...node, 
            status: buildingNode.status === 'maintenance' ? '등록' : buildingNode.status 
          };
        }
        return node;
      }));
    }
    
    // 구축 중인 노드 초기화
    setBuildingNode(null);
    setPendingMasterBuild(null);
  };

  // HACredentialsModal 닫기 처리
  const handleHACredentialsModalClose = () => {
    // 모달 닫기
    setIsHACredentialsModalVisible(false);
    
    // 구축 중인 노드가 있는 경우 상태 복원
    if (buildingNode) {
      // 노드 상태를 원래대로 복원
      setNodes(prev => prev.map(node => {
        if (node.id === buildingNode.id && node.status === 'maintenance') {
          return { 
            ...node, 
            status: buildingNode.status === 'maintenance' ? '등록' : buildingNode.status 
          };
        }
        return node;
      }));
    }
    
    // 구축 중인 노드 초기화
    setBuildingNode(null);
    setPendingMasterBuild(null);
  };

  // handleDeleteWorker 함수 수정
  const handleDeleteWorker = async (workerUsername: string, workerPassword: string, mainUsername: string, mainPassword: string) => {
    try {
      if (!selectedNode) {
        messageApi.error('삭제할 워커 노드가 선택되지 않았습니다.');
        return;
      }
      
      // 모달 먼저 닫기
      setDeleteWorkerModalVisible(false);
      
      const currentNode = selectedNode; // null 체크 후 로컬 변수에 할당
      
      // 노드 상태를 처리중으로 업데이트
      setNodes(prev => prev.map(n => {
        if (n.id === currentNode.id) {
          return { ...n, status: 'maintenance' };
        }
        return n;
      }));
      
      messageApi.loading(`워커 노드 ${currentNode.server_name || currentNode.ip} 삭제 중...`);
      
      // 메인 마스터 노드 찾기
      const masterNode = nodes.find(n => 
        (n.nodeType === 'master' || (typeof n.nodeType === 'string' && n.nodeType.includes('master'))) && 
        n.join_command && 
        n.certificate_key
      );
      
      if (!masterNode) {
        messageApi.error('메인 마스터 노드를 찾을 수 없습니다.');
        
        // 노드 상태 원래대로 복원
        setNodes(prev => prev.map(n => {
          if (n.id === currentNode.id) {
            return { ...n, status: 'running' };
          }
          return n;
        }));
        return;
      }
      
      const workerHops = [{
        host: currentNode.ip,
        port: currentNode.port || "22",
        username: workerUsername,
        password: workerPassword
      }];
      
      const masterHops = [{
        host: masterNode.ip,
        port: masterNode.port || "22",
        username: mainUsername,
        password: mainPassword
      }];
      
      const response = await kubernetesApi.deleteWorker({
        id: Number(currentNode.id),
        main_id: Number(masterNode.id),
        password: workerPassword,
        main_password: mainPassword,
        hops: workerHops,
        main_hops: masterHops
      });
      
      if (response.success) {
        messageApi.success(`워커 노드 ${currentNode.server_name || currentNode.ip} 삭제 완료`);
        
        // 노드 목록에서 삭제된 노드 제거
        setNodes(prev => prev.filter(n => n.id !== currentNode.id));
      } else {
        messageApi.error(`워커 노드 삭제 실패: ${response.error || '알 수 없는 오류'}`);
        
        // 실패 시 노드 상태 원래대로 복원
        setNodes(prev => prev.map(n => {
          if (n.id === currentNode.id) {
            return { ...n, status: 'running' };
          }
          return n;
        }));
      }
    } catch (error) {
      console.error('워커 노드 삭제 중 오류:', error);
      messageApi.error('워커 노드 삭제 중 오류가 발생했습니다.');
      
      // 오류 발생 시 노드 상태 원래대로 복원 (selectedNode가 null이 아닌 경우만)
      if (selectedNode) {
        setNodes(prev => prev.map(n => {
          if (n.id === selectedNode.id) {
            return { ...n, status: 'running' };
          }
          return n;
        }));
      }
    }
  };

  // handleDeleteMaster 함수 수정
  const handleDeleteMaster = async (
    masterUsername: string, 
    masterPassword: string, 
    mainUsername: string,
    mainPassword: string,
    lbUsername?: string, 
    lbPassword?: string
  ) => {
    try {
      if (!selectedNode) {
        messageApi.error('삭제할 마스터 노드가 선택되지 않았습니다.');
        return;
      }
      
      // 모달을 먼저 닫음
      setDeleteMasterModalVisible(false);
      
      const currentNode = selectedNode; // null 체크 후 로컬 변수에 할당
      
      // 노드 상태를 처리중으로 업데이트
      setNodes(prev => prev.map(n => {
        if (n.id === currentNode.id) {
          return { ...n, status: 'maintenance' };
        }
        return n;
      }));
      
      messageApi.loading(`마스터 노드 ${currentNode.server_name || currentNode.ip} 삭제 중...`);
      
      // 삭제하려는 마스터 노드 호스트 정보
      const hopsData = [{
        host: currentNode.ip,
        port: currentNode.port || "22",
        username: masterUsername,
        password: masterPassword
      }];
      
      // 삭제 요청 데이터
      const requestData: any = {
        id: Number(currentNode.id),
        password: masterPassword,
        hops: hopsData
      };
      
      // HA 노드 정보 추가
      if (lbUsername && lbPassword) {
        const haNodes = nodes.filter(n => n.nodeType === 'ha' || (typeof n.nodeType === 'string' && n.nodeType.includes('ha')));
        if (haNodes.length > 0) {
          const haNode = haNodes[0];
          const lbHops = [{
            host: haNode.ip,
            port: haNode.port || "22",
            username: lbUsername,
            password: lbPassword
          }];
          requestData.lb_hops = lbHops;
          requestData.lb_password = lbPassword;
        }
      }
      
      // 메인 마스터 노드 여부 확인
      const isMainMaster = !!(currentNode.join_command && currentNode.certificate_key);
      
      if (isMainMaster) {
        // 삭제하려는 노드가 메인 마스터인 경우, 동일한 노드를 메인 마스터로 설정
        requestData.main_hops = [{
          host: currentNode.ip,
          port: currentNode.port || "22",
          username: mainUsername,
          password: mainPassword
        }];
        requestData.main_password = mainPassword;
      } else {
        // 삭제하려는 노드가 메인 마스터가 아닌 경우, 메인 마스터 노드 찾기
        const mainMasterNodes = nodes.filter(n => 
          (n.nodeType === 'master' || (typeof n.nodeType === 'string' && n.nodeType.includes('master'))) && 
          n.join_command && 
          n.certificate_key
        );
        
        if (mainMasterNodes.length > 0) {
          const mainMasterNode = mainMasterNodes[0];
          // 메인 마스터 노드의 호스트 정보 설정
          const mainHops = [{
            host: mainMasterNode.ip,
            port: mainMasterNode.port || "22",
            username: mainUsername,
            password: mainPassword
          }];
          requestData.main_hops = mainHops;
          requestData.main_password = mainPassword;
        } else {
          // 메인 마스터 노드가 없는 경우 (비정상적인 상황)
          messageApi.error('메인 마스터 노드를 찾을 수 없습니다.');
          return;
        }
      }
      
      // API 호출
      const response = await kubernetesApi.deleteMaster(requestData);
      
      if (response.success) {
        messageApi.success(`마스터 노드 ${currentNode.server_name || currentNode.ip} 삭제 완료`);
        
        // 노드 목록에서 삭제된 노드 제거
        setNodes(prev => prev.filter(n => n.id !== currentNode.id));
        
        // 메인 마스터인 경우 클러스터 전체가 제거될 수 있음
        if (response.details?.isMainMaster && response.details?.otherMasterCount === 0) {
          // 모든 워커 노드도 목록에서 제거
          setNodes(prev => prev.filter(n => 
            !(n.nodeType === 'worker' || (typeof n.nodeType === 'string' && n.nodeType.includes('worker')))
          ));
        }
      } else {
        messageApi.error(`마스터 노드 삭제 실패: ${response.error || '알 수 없는 오류'}`);
        
        // 실패 시 노드 상태 원래대로 복원
        setNodes(prev => prev.map(n => {
          if (n.id === currentNode.id) {
            return { ...n, status: 'running' };
          }
          return n;
        }));
      }
    } catch (error) {
      console.error('마스터 노드 삭제 중 오류:', error);
      messageApi.error('마스터 노드 삭제 중 오류가 발생했습니다.');
      
      // 오류 발생 시 노드 상태 원래대로 복원 (selectedNode가 null이 아닌 경우만)
      if (selectedNode) {
        setNodes(prev => prev.map(n => {
          if (n.id === selectedNode.id) {
            return { ...n, status: 'running' };
          }
          return n;
        }));
      }
    }
  };

  // 외부 쿠버네티스 인증 처리 함수
  const handleExternalAuthConfirm = async (username: string, password: string) => {
    try {
      if (!externalServer) return;
      
      // 로딩 상태 설정
      setCheckingLoading(true);
      
      // 호스트 정보 구성
      const hopsData = [{
        host: externalServer.ip,
        port: parseInt(externalServer.port),
        username,
        password
      }];
      
      // 노드 계산 API 호출
      const response = await kubernetesApi.calculateNodes({
        id: infra.id, // infra.id 추가
        hops: hopsData
      });
      
      if (response && response.success) {
        setExternalNodesInfo(response.nodes);
        
        // 서버 리소스 정보도 함께 가져오기
        try {
          // 리소스 계산 API 호출
          const resourceResponse = await kubernetesApi.calculateResources({
            id: infra.id,
            hops: hopsData
          });
          
          if (resourceResponse && resourceResponse.success) {
            // 리소스 데이터 저장
            const resourceData: ServerResource = {
              host_info: resourceResponse.host_info,
              cpu: resourceResponse.cpu,
              memory: resourceResponse.memory,
              disk: resourceResponse.disk
            };
            setServerResource(resourceData);
          }
        } catch (resourceError) {
          console.error('서버 리소스 정보 가져오기 실패:', resourceError);
        }
        
        messageApi.success('외부 쿠버네티스 클러스터 연결 성공');
      } else {
        messageApi.error('외부 쿠버네티스 클러스터 연결 실패');
      }
    } catch (error) {
      console.error('외부 쿠버네티스 클러스터 연결 오류:', error);
      messageApi.error('외부 쿠버네티스 클러스터 연결에 실패했습니다.');
    } finally {
      setCheckingLoading(false);
      setExternalAuthModalVisible(false);
    }
  };
  
  // 외부 쿠버네티스 초기화 효과
  useEffect(() => {
    // 외부 쿠버네티스인 경우 처리
    if (isExternal && infra.nodes && infra.nodes.length > 0) {
      setExternalServer({
        ip: infra.nodes[0].ip,
        port: infra.nodes[0].port
      });
    }
  }, [isExternal, infra]);

  // 서버 리소스 조회 함수 추가
  const getServerResource = async (node: Node, username: string, password: string) => {
    if (!node) return;
    
    try {
      setResourceLoading(true);
      
      // 호스트 정보 구성
      const hopsData = [{
        host: node.ip,
        port: parseInt(node.port),
        username,
        password
      }];
      
      // 리소스 계산 API 호출
      const response = await kubernetesApi.calculateResources({
        id: infra.id,
        hops: hopsData
      });
      
      if (response && response.success) {
        // response 자체가 아닌 필요한 리소스 데이터만 추출하여 설정
        const resourceData: ServerResource = {
          host_info: response.host_info,
          cpu: response.cpu,
          memory: response.memory,
          disk: response.disk
        };
        setServerResource(resourceData);
        setResourceModalVisible(true);
      } else {
        messageApi.error('서버 리소스 조회에 실패했습니다.');
      }
    } catch (error) {
      console.error('서버 리소스 조회 오류:', error);
      messageApi.error('서버 리소스 조회에 실패했습니다.');
    } finally {
      setResourceLoading(false);
      setResourceAuthModalVisible(false);
    }
  };

  // 외부 쿠버네티스 리소스 조회 핸들러
  const handleExternalResourceCheck = () => {
    if (!externalServer) return;
    
    // 외부 쿠버네티스 서버 노드 객체 생성
    const node: Node = {
      id: '0',
      nodeType: 'external',
      ip: externalServer.ip,
      port: externalServer.port,
      status: 'running',
      server_name: '외부 쿠버네티스'
    };
    
    setResourceNode(node);
    setResourceAuthModalVisible(true);
  };

  // 리소스 조회 인증 확인 핸들러
  const handleResourceAuthConfirm = (username: string, password: string) => {
    if (resourceNode) {
      getServerResource(resourceNode, username, password);
    }
  };

  // 리소스 모달 닫기 핸들러
  const handleResourceModalClose = () => {
    setResourceModalVisible(false);
    setServerResource(null);
  };

  return (
    <>
      {contextHolder}
      {isExternal ? (
        // 외부 쿠버네티스 UI
        <div className="infra-content-wrapper">
          <div className="infra-stats-container">
            <div className="node-stat-group">
              <div className="node-stat-item">
                <CloudServerOutlined className="node-stat-icon" />
                <div>
                  <Text className="node-stat-label">총 노드 수</Text>
                  <Text className="node-stat-number">{externalNodesInfo?.total || 0}개</Text>
                </div>
              </div>
              <div className="node-stat-item master-stat">
                <ClusterOutlined className="node-stat-icon" style={{ color: '#52c41a' }} />
                <div>
                  <Text className="node-stat-label">마스터 노드</Text>
                  <Text className="node-stat-number">{externalNodesInfo?.master || 0}개</Text>
                </div>
              </div>
              <div className="node-stat-item worker-stat">
                <CloudServerOutlined className="node-stat-icon" style={{ color: '#fa541c' }} />
                <div>
                  <Text className="node-stat-label">워커 노드</Text>
                  <Text className="node-stat-number">{externalNodesInfo?.worker || 0}개</Text>
                </div>
              </div>
            </div>
          </div>

          <Divider orientation="left">외부 쿠버네티스 클러스터</Divider>
          
          {externalNodesInfo ? (
            <>
              <Table 
                columns={[
                  { title: '노드명', dataIndex: 'name', key: 'name' },
                  { title: '역할', dataIndex: 'role', key: 'role', render: (role) => (
                    <Tag color={role === 'master' ? 'green' : 'orange'}>{role}</Tag>
                  )},
                  { title: '상태', dataIndex: 'status', key: 'status', render: (status) => (
                    <Tag color={status === 'Ready' ? 'success' : 'error'}>{status}</Tag>
                  )}
                ]} 
                dataSource={externalNodesInfo.list} 
                rowKey="name" 
                pagination={false}
                size="small"
                className="infra-node-table"
              />
              
              {/* 서버 리소스 정보 표시 */}
              {serverResource && (
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
              )}
              
              {/* 리소스 조회 버튼 제거 - 자동으로 가져오기 때문에 불필요 */}
              {/* <div style={{ marginTop: 16, textAlign: 'right' }}>
                <Button 
                  icon={<DashboardOutlined />} 
                  onClick={handleExternalResourceCheck}
                  disabled={!externalServer}
                >
                  서버 리소스 확인
                </Button>
              </div> */}
            </>
          ) : (
            <Empty description="클러스터 정보를 불러오려면 '연결' 버튼을 클릭하세요" />
          )}
          
          <div style={{ marginTop: 24, textAlign: 'right' }}>
            <Space>
              <Button 
                type="primary" 
                icon={<ApiOutlined />} 
                onClick={() => setExternalAuthModalVisible(true)}
                size="middle"
                shape="round"
              >
                연결
              </Button>
            </Space>
          </div>

          <ExternalKubeAuthModal
            visible={externalAuthModalVisible}
            onClose={() => setExternalAuthModalVisible(false)}
            onConfirm={handleExternalAuthConfirm}
            loading={checkingLoading}
            server={externalServer || { ip: '', port: '' }}
          />
        </div>
      ) : (
        // 원래 내부 쿠버네티스 UI (기존 코드)
        <div className="infra-content-wrapper">
          <div className="infra-stats-container">
            <div className="node-stat-group">
              <div className="node-stat-item">
                <CloudServerOutlined className="node-stat-icon" />
                <div>
                  <Text className="node-stat-label">총 노드 수</Text>
                  <Text className="node-stat-number">{nodes.length}개</Text>
                </div>
              </div>
              <div className="node-stat-item ha-stat">
                <ApiOutlined className="node-stat-icon" style={{ color: '#1677ff' }} />
                <div>
                  <Text className="node-stat-label">HA 노드</Text>
                  <Text className="node-stat-number">{haNodes.length}개</Text>
                </div>
              </div>
              <div className="node-stat-item master-stat">
                <ClusterOutlined className="node-stat-icon" style={{ color: '#52c41a' }} />
                <div>
                  <Text className="node-stat-label">마스터 노드</Text>
                  <Text className="node-stat-number">{masterNodes.length}개</Text>
                </div>
              </div>
              <div className="node-stat-item worker-stat">
                <CloudServerOutlined className="node-stat-icon" style={{ color: '#fa541c' }} />
                <div>
                  <Text className="node-stat-label">워커 노드</Text>
                  <Text className="node-stat-number">{workerNodes.length}개</Text>
                </div>
              </div>
            </div>
          </div>

          <Divider orientation="left">노드 목록</Divider>
          
          <Tabs 
            defaultActiveKey="ha" 
            style={{ marginBottom: 16 }} 
            onChange={(key) => setActiveTab(key)}
            activeKey={activeTab}
          >
            <TabPane tab="HA 노드" key="ha">
              <Table 
                columns={nodeColumns.filter(col => col.key !== 'server_name')}
                dataSource={haNodes} 
                rowKey="id" 
                pagination={false}
                size="small"
                className="infra-node-table"
                locale={{ emptyText: 'HA 노드가 없습니다. 설정 버튼을 클릭하여 노드를 추가해주세요.' }}
              />
            </TabPane>
            <TabPane tab="마스터 노드" key="master">
              <Table 
                columns={nodeColumns} 
                dataSource={masterNodes} 
                rowKey="id" 
                pagination={false}
                size="small"
                className="infra-node-table"
                locale={{ emptyText: '마스터 노드가 없습니다. 설정 버튼을 클릭하여 노드를 추가해주세요.' }}
              />
            </TabPane>
            <TabPane tab="워커 노드" key="worker">
              <Table 
                columns={nodeColumns} 
                dataSource={workerNodes} 
                rowKey="id" 
                pagination={false}
                size="small"
                className="infra-node-table"
                locale={{ emptyText: '워커 노드가 없습니다. 설정 버튼을 클릭하여 노드를 추가해주세요.' }}
              />
            </TabPane>
          </Tabs>

          <div style={{ marginTop: 24, textAlign: 'right' }}>
            <Space>
              <Button 
                type="primary" 
                icon={<SettingOutlined />} 
                onClick={() => {
                  if (activeTab === 'ha' || activeTab === 'master' || activeTab === 'worker') {
                    setIsAddNodeModalVisible(true);
                  } else {
                    showSettingsModal(infra);
                  }
                }}
                size="middle"
                shape="round"
              >
                설정
              </Button>
            </Space>
          </div>
        </div>
      )}

      {/* 공통 모달 컴포넌트들 (외부 쿠버네티스 모드에서는 나타나지 않음) */}
      {!isExternal && (
        <>
          <AddNodeModal
            visible={isAddNodeModalVisible}
            infraId={infra.id}
            onClose={() => setIsAddNodeModalVisible(false)}
            onAdd={handleAddNode}
            initialNodeType={activeTab === 'worker' ? 'worker' : activeTab === 'master' ? 'master' : 'ha'}
          />
          
          <BuildServerModal
            visible={isBuildServerModalVisible}
            onClose={handleBuildServerModalClose}
            onConfirm={handleBuildConfirm}
            loading={buildingLoading}
            node={buildingNode}
          />
          
          <CheckStatusModal
            visible={isCheckStatusModalVisible}
            onClose={() => setIsCheckStatusModalVisible(false)}
            onConfirm={handleAuthenticationSubmit}
            loading={checkingLoading}
            node={checkingNode}
          />
          
          <HACredentialsModal
            visible={isHACredentialsModalVisible}
            onClose={handleHACredentialsModalClose}
            onConfirm={handleHACredentialsConfirm}
            loading={buildingLoading}
            nodes={nodes.filter(node => 
              node.nodeType === 'ha' || 
              (typeof node.nodeType === 'string' && node.nodeType.includes('ha'))
            )}
          />
          
          <DeleteWorkerModal
            visible={deleteWorkerModalVisible}
            onClose={() => setDeleteWorkerModalVisible(false)}
            onConfirm={handleDeleteWorker}
            loading={false}
            node={selectedNode}
          />
          
          <DeleteMasterModal
            visible={deleteMasterModalVisible}
            onClose={() => setDeleteMasterModalVisible(false)}
            onConfirm={handleDeleteMaster}
            loading={false}
            node={selectedNode}
            nodes={nodes}
          />
        </>
      )}

      {/* 리소스 조회 관련 모달 추가 */}
      <ResourceAuthModal
        visible={resourceAuthModalVisible}
        onClose={() => setResourceAuthModalVisible(false)}
        onConfirm={handleResourceAuthConfirm}
        loading={resourceLoading}
        node={resourceNode}
      />
      
      <ServerResourceModal
        visible={resourceModalVisible}
        onClose={handleResourceModalClose}
        resource={serverResource}
        loading={resourceLoading}
        server={{
          name: resourceNode?.server_name,
          ip: resourceNode?.ip || ''
        }}
      />
    </>
  );
};

export default InfraKubernetesSetting; 