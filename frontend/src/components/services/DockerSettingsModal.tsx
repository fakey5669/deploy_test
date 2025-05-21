import React, { useState, useEffect } from 'react';
import { Modal, Button, Divider, Typography, Form, Input, message, Space, Tabs, Select, Radio, Spin } from 'antd';
import { DockerOutlined, SaveOutlined, FileOutlined, BuildOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { Service } from '../../types/service';
import * as serviceApi from '../../lib/api/service'; // API 추가

const { Text, Title } = Typography;
const { TextArea } = Input;
const { Option } = Select;
const { TabPane } = Tabs;

interface DockerSettingsModalProps {
  visible: boolean;
  service: Service | null;
  onCancel: () => void;
}

// 도커파일 템플릿 유형
type DockerfileTemplateType = 'node' | 'python' | 'java' | 'go' | 'custom';

// 도커 컴포즈 템플릿 유형
type DockerComposeTemplateType = 'basic' | 'withDb' | 'withCache' | 'custom';

const DockerSettingsModal: React.FC<DockerSettingsModalProps> = ({
  visible,
  service,
  onCancel
}) => {
  const [messageApi, contextHolder] = message.useMessage();
  const [activeTab, setActiveTab] = useState<string>('dockerfile');
  const [form] = Form.useForm();
  
  // 로딩 상태
  const [loading, setLoading] = useState<boolean>(false);
  
  // 파일 존재 여부 상태
  const [hasDockerfile, setHasDockerfile] = useState<boolean | null>(null);
  const [hasDockerCompose, setHasDockerCompose] = useState<boolean | null>(null);
  
  // 템플릿 선택 모드 상태
  const [isDockerfileTemplateMode, setIsDockerfileTemplateMode] = useState<boolean>(false);
  const [isDockerComposeTemplateMode, setIsDockerComposeTemplateMode] = useState<boolean>(false);
  
  // 도커 파일 관련 상태
  const [dockerfileContent, setDockerfileContent] = useState<string>('');
  const [dockerComposeContent, setDockerComposeContent] = useState<string>('');
  
  // 템플릿 정보 상태
  const [dockerfileTemplateType, setDockerfileTemplateType] = useState<DockerfileTemplateType>('node');
  const [dockerComposeTemplateType, setDockerComposeTemplateType] = useState<DockerComposeTemplateType>('basic');
  const [appPort, setAppPort] = useState<string>('3000');
  const [dbType, setDbType] = useState<string>('mysql');
  const [useRedis, setUseRedis] = useState<boolean>(false);

  // 도커 파일 로딩
  const loadDockerFiles = async () => {
    if (!service) return;
    
    try {
      setLoading(true);
      
      // 서비스 API를 사용하여 도커 파일 정보 가져오기
      const response = await serviceApi.getDockerFiles(service.id);
      
      if (response.success) {
        // API 응답에서 가져온 도커 파일 상태 및 내용 설정
        setHasDockerfile(response.hasDockerfile);
        setHasDockerCompose(response.hasDockerCompose);
        
        // 도커 파일이 있을 경우 내용 설정
        if (response.hasDockerfile && response.dockerfileContent) {
          setDockerfileContent(response.dockerfileContent);
        } else {
          // 템플릿 모드 활성화
          setIsDockerfileTemplateMode(true);
        }
        
        // 도커 컴포즈 파일이 있을 경우 내용 설정
        if (response.hasDockerCompose && response.dockerComposeContent) {
          setDockerComposeContent(response.dockerComposeContent);
        } else {
          // 템플릿 모드 활성화
          setIsDockerComposeTemplateMode(true);
        }
      } else {
        // API 응답이 실패한 경우 오류 메시지 표시
        throw new Error(response.error || '도커 파일 정보를 가져오는데 실패했습니다.');
      }
    } catch (error) {
      console.error('도커 파일 로딩 중 오류 발생:', error);
      messageApi.error('도커 파일을 불러오는데 실패했습니다.');
      
      // 오류 발생 시 템플릿 모드 활성화
      setHasDockerfile(false);
      setHasDockerCompose(false);
      setIsDockerfileTemplateMode(true);
      setIsDockerComposeTemplateMode(true);
    } finally {
      setLoading(false);
    }
  };

  // 서비스가 변경되면 도커 파일 로딩
  useEffect(() => {
    if (visible && service) {
      loadDockerFiles();
    }
    
    // 모달이 닫힐 때 상태 초기화
    if (!visible) {
      setHasDockerfile(null);
      setHasDockerCompose(null);
      setIsDockerfileTemplateMode(false);
      setIsDockerComposeTemplateMode(false);
      setDockerfileContent('');
      setDockerComposeContent('');
    }
  }, [visible, service]);

  // Dockerfile 생성
  const generateDockerfile = () => {
    let template = '';
    
    switch (dockerfileTemplateType) {
      case 'node':
        template = `FROM node:16-alpine

WORKDIR /app

COPY package*.json ./
RUN npm install

COPY . .

RUN npm run build

EXPOSE ${appPort}

CMD ["npm", "start"]`;
        break;
        
      case 'python':
        template = `FROM python:3.9-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

EXPOSE ${appPort}

CMD ["python", "app.py"]`;
        break;
        
      case 'java':
        template = `FROM maven:3.8.4-openjdk-17-slim AS build

WORKDIR /app
COPY pom.xml .
COPY src ./src

RUN mvn clean package -DskipTests

FROM openjdk:17-slim

WORKDIR /app
COPY --from=build /app/target/*.jar app.jar

EXPOSE ${appPort}

CMD ["java", "-jar", "app.jar"]`;
        break;
        
      case 'go':
        template = `FROM golang:1.18-alpine AS build

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o main .

FROM alpine:latest

WORKDIR /app
COPY --from=build /app/main .

EXPOSE ${appPort}

CMD ["./main"]`;
        break;
        
      default:
        template = `# 여기에 Dockerfile 내용을 작성하세요
FROM alpine:latest

WORKDIR /app
COPY . .

EXPOSE ${appPort}

CMD ["echo", "컨테이너가 시작되었습니다"]`;
    }
    
    setDockerfileContent(template);
    setIsDockerfileTemplateMode(false);
    messageApi.success('Dockerfile이 생성되었습니다.');
  };

  // Docker Compose 생성
  const generateDockerCompose = () => {
    let template = '';
    const serviceName = service?.name || 'app';
    
    switch (dockerComposeTemplateType) {
      case 'basic':
        template = `version: '3'

services:
  ${serviceName}:
    build: .
    ports:
      - "${appPort}:${appPort}"
    environment:
      - NODE_ENV=production
    restart: always`;
        break;
        
      case 'withDb':
        if (dbType === 'mysql') {
          template = `version: '3'

services:
  ${serviceName}:
    build: .
    ports:
      - "${appPort}:${appPort}"
    environment:
      - NODE_ENV=production
      - DB_HOST=db
      - DB_PORT=3306
      - DB_USER=user
      - DB_PASSWORD=password
      - DB_NAME=${serviceName.toLowerCase()}_db
    depends_on:
      - db
    restart: always

  db:
    image: mysql:8.0
    ports:
      - "3306:3306"
    environment:
      - MYSQL_ROOT_PASSWORD=rootpassword
      - MYSQL_DATABASE=${serviceName.toLowerCase()}_db
      - MYSQL_USER=user
      - MYSQL_PASSWORD=password
    volumes:
      - db_data:/var/lib/mysql
    restart: always

volumes:
  db_data:`;
        } else if (dbType === 'postgres') {
          template = `version: '3'

services:
  ${serviceName}:
    build: .
    ports:
      - "${appPort}:${appPort}"
    environment:
      - NODE_ENV=production
      - DB_HOST=db
      - DB_PORT=5432
      - DB_USER=user
      - DB_PASSWORD=password
      - DB_NAME=${serviceName.toLowerCase()}_db
    depends_on:
      - db
    restart: always

  db:
    image: postgres:14
    ports:
      - "5432:5432"
    environment:
      - POSTGRES_PASSWORD=password
      - POSTGRES_USER=user
      - POSTGRES_DB=${serviceName.toLowerCase()}_db
    volumes:
      - db_data:/var/lib/postgresql/data
    restart: always

volumes:
  db_data:`;
        } else {
          template = `version: '3'

services:
  ${serviceName}:
    build: .
    ports:
      - "${appPort}:${appPort}"
    environment:
      - NODE_ENV=production
    restart: always

  db:
    image: mongo:5.0
    ports:
      - "27017:27017"
    environment:
      - MONGO_INITDB_ROOT_USERNAME=root
      - MONGO_INITDB_ROOT_PASSWORD=rootpassword
      - MONGO_INITDB_DATABASE=${serviceName.toLowerCase()}_db
    volumes:
      - db_data:/data/db
    restart: always

volumes:
  db_data:`;
        }
        break;
        
      case 'withCache':
        template = `version: '3'

services:
  ${serviceName}:
    build: .
    ports:
      - "${appPort}:${appPort}"
    environment:
      - NODE_ENV=production
      - REDIS_HOST=redis
      - REDIS_PORT=6379
    depends_on:
      - redis
    restart: always

  redis:
    image: redis:alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    restart: always

volumes:
  redis_data:`;
        break;
        
      default:
        template = `version: '3'

services:
  ${serviceName}:
    build: .
    ports:
      - "${appPort}:${appPort}"
    environment:
      - NODE_ENV=production
    restart: always`;
    }
    
    setDockerComposeContent(template);
    setIsDockerComposeTemplateMode(false);
    messageApi.success('Docker Compose 파일이 생성되었습니다.');
  };

  // Dockerfile 저장하기
  const saveDockerfile = async () => {
    if (!service) return;
    
    try {
      setLoading(true);
      
      // 자동 커밋 메시지 생성
      const commitMessage = `Update Dockerfile for ${service.name} - ${new Date().toLocaleString()}`;
      
      // 서비스 API를 사용하여 Dockerfile 저장
      const response = await serviceApi.saveDockerfile(
        service.id, 
        dockerfileContent, 
        commitMessage
      );
      
      if (response.success) {
        messageApi.success('Dockerfile이 성공적으로 저장되었습니다.');
        setHasDockerfile(true);
      } else {
        throw new Error(response.error || 'Dockerfile 저장에 실패했습니다.');
      }
    } catch (error) {
      console.error('Dockerfile 저장 중 오류 발생:', error);
      messageApi.error('Dockerfile 저장에 실패했습니다.');
    } finally {
      setLoading(false);
    }
  };

  // Docker Compose 파일 저장하기
  const saveDockerCompose = async () => {
    if (!service) return;
    
    try {
      setLoading(true);
      
      // 자동 커밋 메시지 생성
      const commitMessage = `Update docker-compose.yml for ${service.name} - ${new Date().toLocaleString()}`;
      
      // 서비스 API를 사용하여 Docker Compose 파일 저장
      const response = await serviceApi.saveDockerCompose(
        service.id, 
        dockerComposeContent, 
        commitMessage
      );
      
      if (response.success) {
        messageApi.success('Docker Compose 파일이 성공적으로 저장되었습니다.');
        setHasDockerCompose(true);
      } else {
        throw new Error(response.error || 'Docker Compose 파일 저장에 실패했습니다.');
      }
    } catch (error) {
      console.error('Docker Compose 저장 중 오류 발생:', error);
      messageApi.error('Docker Compose 저장에 실패했습니다.');
    } finally {
      setLoading(false);
    }
  };

  // 파일 새로고침
  const refreshDockerFiles = () => {
    loadDockerFiles();
    messageApi.success('도커 파일 정보가 새로고침되었습니다.');
  };

  if (!service) return null;

  const renderDockerfileTemplateForm = () => (
    <div className="dockerfile-template-form">
      <Title level={4}>Dockerfile 생성</Title>
      <Text type="secondary">
        서비스에 적합한 Dockerfile을 생성하기 위한 정보를 입력해주세요.
      </Text>
      
      <Form 
        layout="vertical" 
        style={{ marginTop: '20px' }}
        initialValues={{
          templateType: dockerfileTemplateType,
          appPort: appPort
        }}
      >
        <Form.Item
          label="애플리케이션 유형"
          name="templateType"
          extra="애플리케이션 유형에 따라 적합한 Dockerfile 템플릿이 생성됩니다."
        >
          <Radio.Group 
            onChange={(e) => setDockerfileTemplateType(e.target.value)}
            value={dockerfileTemplateType}
          >
            <Space direction="vertical">
              <Radio value="node">Node.js</Radio>
              <Radio value="python">Python</Radio>
              <Radio value="java">Java</Radio>
              <Radio value="go">Go</Radio>
              <Radio value="custom">기타/직접 작성</Radio>
            </Space>
          </Radio.Group>
        </Form.Item>
        
        <Form.Item
          label="포트 번호"
          name="appPort"
          extra="애플리케이션이 사용할 포트 번호를 입력해주세요."
        >
          <Input 
            placeholder="3000" 
            value={appPort}
            onChange={(e) => setAppPort(e.target.value)}
          />
        </Form.Item>
        
        <Form.Item>
          <Button 
            type="primary" 
            icon={<FileOutlined />} 
            onClick={generateDockerfile}
          >
            Dockerfile 생성
          </Button>
        </Form.Item>
      </Form>
    </div>
  );

  const renderDockerComposeTemplateForm = () => (
    <div className="docker-compose-template-form">
      <Title level={4}>Docker Compose 파일 생성</Title>
      <Text type="secondary">
        서비스 구성에 적합한 Docker Compose 파일을 생성하기 위한 정보를 입력해주세요.
      </Text>
      
      <Form 
        layout="vertical" 
        style={{ marginTop: '20px' }}
        initialValues={{
          templateType: dockerComposeTemplateType,
          appPort: appPort,
          dbType: dbType
        }}
      >
        <Form.Item
          label="서비스 구성"
          name="templateType"
          extra="서비스 구성에 따라 적합한 Docker Compose 템플릿이 생성됩니다."
        >
          <Radio.Group 
            onChange={(e) => setDockerComposeTemplateType(e.target.value)}
            value={dockerComposeTemplateType}
          >
            <Space direction="vertical">
              <Radio value="basic">기본 구성 (애플리케이션만)</Radio>
              <Radio value="withDb">데이터베이스 포함</Radio>
              <Radio value="withCache">캐시(Redis) 포함</Radio>
              <Radio value="custom">기타/직접 작성</Radio>
            </Space>
          </Radio.Group>
        </Form.Item>
        
        <Form.Item
          label="포트 번호"
          name="appPort"
          extra="애플리케이션이 사용할 포트 번호를 입력해주세요."
        >
          <Input 
            placeholder="3000" 
            value={appPort}
            onChange={(e) => setAppPort(e.target.value)}
          />
        </Form.Item>
        
        {dockerComposeTemplateType === 'withDb' && (
          <Form.Item
            label="데이터베이스 유형"
            name="dbType"
            extra="사용할 데이터베이스 유형을 선택해주세요."
          >
            <Select 
              value={dbType}
              onChange={(value) => setDbType(value)}
            >
              <Option value="mysql">MySQL</Option>
              <Option value="postgres">PostgreSQL</Option>
              <Option value="mongodb">MongoDB</Option>
            </Select>
          </Form.Item>
        )}
        
        <Form.Item>
          <Button 
            type="primary" 
            icon={<BuildOutlined />} 
            onClick={generateDockerCompose}
          >
            Docker Compose 파일 생성
          </Button>
        </Form.Item>
      </Form>
    </div>
  );

  return (
    <Modal
      title={
        <div style={{ 
          display: 'flex', 
          alignItems: 'center', 
          padding: '8px 0',
        }}>
          <DockerOutlined style={{ 
            color: '#1890ff', 
            fontSize: '20px', 
            marginRight: '12px' 
          }} />
          <span style={{
            fontSize: '16px',
            fontWeight: 600
          }}>{service.name} 도커 설정</span>
          
          <Button
            type="text"
            icon={<ReloadOutlined />}
            onClick={refreshDockerFiles}
            style={{ marginLeft: 'auto' }}
            loading={loading}
          />
        </div>
      }
      open={visible}
      onCancel={onCancel}
      footer={[
        <Button key="back" onClick={onCancel}>
          닫기
        </Button>
      ]}
      width={800}
      style={{ 
        borderRadius: '12px',
        overflow: 'hidden'
      }}
    >
      {contextHolder}
      
      {loading ? (
        <div style={{ textAlign: 'center', padding: '50px 0' }}>
          <Spin size="large" />
          <div style={{ marginTop: '20px' }}>도커 파일 정보를 불러오는 중...</div>
        </div>
      ) : (
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          type="card"
          size="large"
          style={{ marginTop: '16px' }}
          items={[
            {
              key: 'dockerfile',
              label: (
                <span>
                  <FileOutlined /> Dockerfile
                  {hasDockerfile === false && <PlusOutlined style={{ fontSize: '12px', marginLeft: '5px' }} />}
                </span>
              ),
              children: (
                <div className="docker-settings-tab-content">
                  {isDockerfileTemplateMode ? (
                    renderDockerfileTemplateForm()
                  ) : (
                    <>
                      <div style={{ 
                        display: 'flex', 
                        alignItems: 'center', 
                        justifyContent: 'space-between',
                        padding: '8px 0'
                      }}>
                        <Text strong style={{ fontSize: '16px' }}>Dockerfile</Text>
                        
                        <Space>
                          {hasDockerfile === false && (
                            <Button 
                              type="primary" 
                              icon={<PlusOutlined />} 
                              onClick={() => setIsDockerfileTemplateMode(true)}
                            >
                              템플릿 사용
                            </Button>
                          )}
                          <Button 
                            type="primary" 
                            icon={<SaveOutlined />} 
                            onClick={saveDockerfile}
                          >
                            저장
                          </Button>
                        </Space>
                      </div>
                      <Divider style={{ margin: '0 0 16px 0' }} />
                      
                      <div>
                        <TextArea
                          style={{ 
                            fontFamily: 'monospace', 
                            fontSize: '14px',
                            minHeight: '400px' 
                          }}
                          value={dockerfileContent}
                          onChange={e => setDockerfileContent(e.target.value)}
                          placeholder="# Dockerfile 내용을 입력하세요"
                        />
                      </div>
                    </>
                  )}
                </div>
              )
            },
            {
              key: 'docker-compose',
              label: (
                <span>
                  <BuildOutlined /> Docker Compose
                  {hasDockerCompose === false && <PlusOutlined style={{ fontSize: '12px', marginLeft: '5px' }} />}
                </span>
              ),
              children: (
                <div className="docker-settings-tab-content">
                  {isDockerComposeTemplateMode ? (
                    renderDockerComposeTemplateForm()
                  ) : (
                    <>
                      <div style={{ 
                        display: 'flex', 
                        alignItems: 'center', 
                        justifyContent: 'space-between',
                        padding: '8px 0'
                      }}>
                        <Text strong style={{ fontSize: '16px' }}>docker-compose.yml</Text>
                        
                        <Space>
                          {hasDockerCompose === false && (
                            <Button 
                              type="primary" 
                              icon={<PlusOutlined />} 
                              onClick={() => setIsDockerComposeTemplateMode(true)}
                            >
                              템플릿 사용
                            </Button>
                          )}
                          <Button 
                            type="primary" 
                            icon={<SaveOutlined />} 
                            onClick={saveDockerCompose}
                          >
                            저장
                          </Button>
                        </Space>
                      </div>
                      <Divider style={{ margin: '0 0 16px 0' }} />
                      
                      <div>
                        <TextArea
                          style={{ 
                            fontFamily: 'monospace', 
                            fontSize: '14px',
                            minHeight: '400px' 
                          }}
                          value={dockerComposeContent}
                          onChange={e => setDockerComposeContent(e.target.value)}
                          placeholder="# docker-compose.yml 내용을 입력하세요"
                        />
                      </div>
                    </>
                  )}
                </div>
              )
            }
          ]}
        />
      )}
    </Modal>
  );
};

export default DockerSettingsModal; 