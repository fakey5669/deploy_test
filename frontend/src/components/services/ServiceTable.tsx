import React from 'react';
import { Table, Space, Button, Tooltip, Spin } from 'antd';
import { 
  EditOutlined, 
  DeleteOutlined, 
  CloudOutlined, 
  CloudSyncOutlined,
  CheckCircleOutlined, 
  GlobalOutlined,
  SyncOutlined,
  CloseCircleOutlined,
  EyeOutlined,
  ReloadOutlined,
  GithubOutlined,
  RocketOutlined,
  PauseCircleOutlined,
  CloudUploadOutlined,
  CodeOutlined,
  DockerOutlined,
  LoadingOutlined,
  InfoCircleOutlined
} from '@ant-design/icons';
import { Service } from '../../types/service';
import StatusBadge from './StatusBadge';

// Service 타입 확장 (프로퍼티가 있는지 확인을 위한 타입 가드)
interface ServiceWithLoadingStatus extends Service {
  loadingStatus?: boolean;
  namespaceStatus?: string;
  runningPods?: number;
  totalPods?: number;
}

interface ServiceTableProps {
  services: ServiceWithLoadingStatus[];
  loading: boolean;
  pagination: {
    current: number;
    pageSize: number;
    total: number;
    onChange: (page: number) => void;
    onShowSizeChange: (current: number, size: number) => void;
  };
  onView: (service: ServiceWithLoadingStatus, tabKey?: string) => void;
  onEdit: (service: ServiceWithLoadingStatus) => void;
  onDelete: (serviceId: string | number) => void;
  onReload?: (serviceId: string | number) => void;
  onGitlabClick?: (service: ServiceWithLoadingStatus) => void;
  onDeploy: (service: ServiceWithLoadingStatus) => void;
  onRestart: (service: ServiceWithLoadingStatus) => void;
  onStop: (service: ServiceWithLoadingStatus) => void;
  onCheckStatus: (service: ServiceWithLoadingStatus) => void;
  onInfraClick?: (service: ServiceWithLoadingStatus) => void;
  onDockerSettings: (service: ServiceWithLoadingStatus) => void;
}

const ServiceTable: React.FC<ServiceTableProps> = ({
  services,
  loading,
  pagination,
  onView,
  onEdit,
  onDelete,
  onReload,
  onGitlabClick,
  onDeploy,
  onRestart,
  onStop,
  onCheckStatus,
  onInfraClick,
  onDockerSettings
}) => {
  // 날짜 포맷팅
  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
  };

  // 테이블 컬럼 정의
  const columns = [
    {
      title: '상태',
      dataIndex: 'status',
      key: 'status',
      width: 120,
      render: (status: string, record: ServiceWithLoadingStatus) => (
        <div className="status-cell">
          {record.loadingStatus ? (
            <div className="status-tag loading">
              <Spin indicator={<LoadingOutlined style={{ fontSize: 14 }} spin />} />
              <span className="status-text" style={{ marginLeft: '5px' }}>조회중</span>
            </div>
          ) : status === 'active' ? (
            <div className="status-tag active">
              <div className="status-circle active"></div>
              <span className="status-text">활성</span>
            </div>
          ) : status === 'inactive' ? (
            <div className="status-tag inactive">
              <div className="status-circle inactive"></div>
              <span className="status-text">비활성</span>
            </div>
          ) : (
            <div className="status-tag registered">
              <div className="status-circle registered"></div>
              <span className="status-text">등록</span>
            </div>
          )}
        </div>
      ),
    },
    {
      title: '서비스 이름',
      dataIndex: 'name',
      key: 'name',
      render: (text: string, record: ServiceWithLoadingStatus) => (
        <a onClick={() => onView(record)} style={{ cursor: 'pointer', fontWeight: 'bold' }}>
          {text}
        </a>
      ),
    },
    {
      title: '도메인',
      dataIndex: 'domain',
      key: 'domain',
      render: (domain: string) => (
        domain ? (
          <div className="domain-cell">
            <GlobalOutlined className="domain-icon" />
            <a href={domain} target="_blank" rel="noopener noreferrer" className="domain-link">
              {domain}
            </a>
          </div>
        ) : (
          '-'
        )
      ),
    },
    {
      title: '네임스페이스',
      dataIndex: 'namespace',
      key: 'namespace',
      render: (namespace: string) => (
        <div className="namespace-cell">
          <span>{namespace || '-'}</span>
        </div>
      ),
    },
    {
      title: '인프라',
      dataIndex: 'infra_id',
      key: 'infra',
      render: (infraId: number | null, record: ServiceWithLoadingStatus) => (
        <div className="infra-cell">
          <CloudOutlined className="infra-icon" />
          {record.infraName ? (
            <a onClick={() => onInfraClick && infraId && onInfraClick(record)} style={{ cursor: 'pointer' }}>
              {record.infraName}
            </a>
          ) : (
            <span className="infra-name not-set">미설정</span>
          )}
        </div>
      ),
    },
    {
      title: 'GitLab',
      dataIndex: 'gitlab_url',
      key: 'gitlab_url',
      render: (text: string | null, record: ServiceWithLoadingStatus) => (
        <div className="gitlab-cell" onClick={() => onGitlabClick && onGitlabClick(record)}>
          {text ? (
            <div className="gitlab-button registered" style={{ cursor: 'pointer' }}>
              <CheckCircleOutlined className="gitlab-icon" />
              등록됨
            </div>
          ) : (
            <div className="gitlab-button not-registered">
              <CloseCircleOutlined className="gitlab-icon" />
              미등록
            </div>
          )}
        </div>
      ),
    },
    {
      title: '도커',
      key: 'docker',
      width: 80,
      render: (_: unknown, record: ServiceWithLoadingStatus) => (
        <div style={{ textAlign: 'center' }}>
          <Tooltip title="도커 설정">
            <Button
              type="text"
              icon={<DockerOutlined style={{ fontSize: '18px', color: '#1890ff' }} />}
              onClick={() => onDockerSettings(record)}
              style={{ padding: '0' }}
            />
          </Tooltip>
        </div>
      ),
    },
    {
      title: '생성일',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (text: string) => formatDate(text),
    },
    {
      title: '작업',
      key: 'action',
      width: 200,
      render: (_: unknown, record: ServiceWithLoadingStatus) => (
        <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center' }}>
          <Space size="small" split={<span style={{ width: '4px' }}></span>}>
            {/* 상태 확인 버튼은 항상 표시 */}
            <Tooltip title="서비스 상태 확인">
              <Button
                type="text"
                icon={<CloudSyncOutlined style={{ fontSize: '16px' }} />}
                onClick={() => onCheckStatus(record)}
                style={{ color: '#000000', padding: '8px', border: 'none' }}
                disabled={record.loadingStatus}
              />
            </Tooltip>

            {/* 서비스 상태에 따른 컨트롤 버튼 */}
            {/* {record.status === 'active' && (
              <>
                <Tooltip title="재실행">
                  <Button 
                    type="text"
                    icon={<ReloadOutlined style={{ fontSize: '16px' }} />} 
                    onClick={(e) => {
                      e.stopPropagation();
                      onRestart(record);
                    }}
                    style={{ border: 'none', padding: '8px' }}
                  />
                </Tooltip>
                
                <Tooltip title="중지">
                  <Button 
                    type="text"
                    icon={<PauseCircleOutlined style={{ fontSize: '16px', color: '#ff4d4f' }} />} 
                    onClick={(e) => {
                      e.stopPropagation();
                      onStop(record);
                    }}
                    style={{ border: 'none', padding: '8px' }}
                  />
                </Tooltip>
              </>
            )} */}
            
            {/* 실행(배포) 버튼 - 비활성 상태일 때만 */}
            {record.status === 'inactive' && record.infra_id && (
              <Tooltip title="배포">
                <Button 
                  type="text"
                  icon={<CloudUploadOutlined style={{ fontSize: '16px', color: '#1890ff' }} />} 
                  onClick={(e) => {
                    e.stopPropagation();
                    onDeploy(record);
                  }}
                  style={{ border: 'none', padding: '8px' }}
                />
              </Tooltip>
            )}

            {/* 서비스 편집 버튼 추가 */}
            <Tooltip title="서비스 편집">
              <Button
                type="text"
                icon={<EditOutlined style={{ fontSize: '16px' }} />}
                onClick={() => onEdit(record)}
                style={{ color: '#000000', padding: '8px', border: 'none' }}
              />
            </Tooltip>
            
            <Tooltip title="삭제">
              <Button
                type="text"
                danger
                icon={<DeleteOutlined style={{ fontSize: '16px' }} />}
                onClick={() => onDelete(record.id)}
                style={{ padding: '8px', border: 'none' }}
              />
            </Tooltip>
          </Space>
        </div>
      ),
    },
  ];

  return (
    <Table
      className="service-table"
      columns={columns}
      dataSource={services}
      rowKey="id"
      loading={loading}
      pagination={{
        ...pagination,
        showSizeChanger: true,
        pageSizeOptions: ['10', '20', '50'],
        showTotal: (total) => `총 ${total}개 항목`,
      }}
      locale={{ emptyText: '서비스가 없습니다.' }}
    />
  );
};

export default ServiceTable; 