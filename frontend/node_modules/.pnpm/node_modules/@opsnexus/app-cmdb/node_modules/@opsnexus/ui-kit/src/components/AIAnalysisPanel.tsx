import React from 'react';
import { Card, Tag, Button, Space, List, Progress } from 'antd';
import { LikeOutlined, DislikeOutlined, RobotOutlined } from '@ant-design/icons';
import type { AIAnalysisResult } from '../types';
import { RootCauseBadge } from './RootCauseBadge';

interface AIAnalysisPanelProps {
  analysis: AIAnalysisResult;
  onFeedback?: (helpful: boolean) => void;
  categoryLabel: string;
}

export const AIAnalysisPanel: React.FC<AIAnalysisPanelProps> = ({
  analysis,
  onFeedback,
  categoryLabel,
}) => {
  return (
    <Card
      title={
        <Space>
          <RobotOutlined />
          <span>AI</span>
        </Space>
      }
      extra={
        <Space>
          <Progress
            type="circle"
            percent={analysis.confidence}
            size={32}
            format={(p) => `${p}%`}
          />
          <Tag>{analysis.model}</Tag>
        </Space>
      }
      style={{
        borderLeft: '4px solid #3491FA',
        borderRadius: 8,
      }}
    >
      <div style={{ marginBottom: 12 }}>
        <strong>{analysis.rootCause}</strong>
      </div>
      <div style={{ marginBottom: 12 }}>
        <RootCauseBadge category={analysis.category} label={categoryLabel} />
      </div>

      <List
        size="small"
        header={<strong>Evidence</strong>}
        dataSource={analysis.evidences}
        renderItem={(item, i) => (
          <List.Item>{i + 1}. {item}</List.Item>
        )}
        style={{ marginBottom: 12 }}
      />

      <List
        size="small"
        header={<strong>Suggestions</strong>}
        dataSource={analysis.suggestions}
        renderItem={(item, i) => (
          <List.Item>{i + 1}. {item}</List.Item>
        )}
        style={{ marginBottom: 12 }}
      />

      <div style={{ marginBottom: 12, color: '#86909C', fontSize: 12 }}>
        {analysis.estimatedRecovery} | {analysis.duration}s
      </div>

      {onFeedback && (
        <Space>
          <Button
            icon={<LikeOutlined />}
            size="small"
            onClick={() => onFeedback(true)}
          />
          <Button
            icon={<DislikeOutlined />}
            size="small"
            onClick={() => onFeedback(false)}
          />
        </Space>
      )}
    </Card>
  );
};
