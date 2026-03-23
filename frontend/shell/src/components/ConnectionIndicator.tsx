/**
 * ConnectionIndicator - WebSocket 连接状态指示器组件
 *
 * 用途：在页面顶部显示 WebSocket 连接状态提示条
 * 行为逻辑：
 * - 正常连接：不显示任何提示
 * - 断线超过 3 秒：显示橙色警告条 "数据连接已中断，正在重连..."
 * - 重连成功：橙色警告条变为绿色 "已恢复"，闪烁 2 秒后自动消失
 *
 * 当前使用模拟状态（通过 props 控制），后期可接入真实 WebSocket 连接管理
 *
 * Props：
 * @param connected - WebSocket 是否已连接
 *
 * 使用方式：
 * ```tsx
 * <ConnectionIndicator connected={wsConnected} />
 * ```
 */
import React, { useState, useEffect, useRef } from 'react';
import { WifiOutlined, DisconnectOutlined } from '@ant-design/icons';

export interface ConnectionIndicatorProps {
  /** WebSocket 是否已连接 */
  connected: boolean;
}

/** 指示器当前显示状态 */
type IndicatorStatus = 'hidden' | 'disconnected' | 'reconnected';

export const ConnectionIndicator: React.FC<ConnectionIndicatorProps> = ({ connected }) => {
  /** 当前显示状态 */
  const [status, setStatus] = useState<IndicatorStatus>('hidden');
  /** 断线计时器引用 */
  const disconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  /** 恢复后自动隐藏计时器引用 */
  const hideTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  /** 记录上一次连接状态，用于检测状态变化 */
  const prevConnectedRef = useRef(connected);

  useEffect(() => {
    const wasConnected = prevConnectedRef.current;
    prevConnectedRef.current = connected;

    if (!connected && wasConnected) {
      // 连接断开：3 秒后显示断线提示（避免短暂抖动）
      disconnectTimerRef.current = setTimeout(() => {
        setStatus('disconnected');
      }, 3000);

      // 清除恢复隐藏计时器
      if (hideTimerRef.current) {
        clearTimeout(hideTimerRef.current);
        hideTimerRef.current = null;
      }
    } else if (connected && !wasConnected) {
      // 连接恢复
      // 清除断线计时器
      if (disconnectTimerRef.current) {
        clearTimeout(disconnectTimerRef.current);
        disconnectTimerRef.current = null;
      }

      // 只有之前已经显示了断线提示时，才显示恢复提示
      if (status === 'disconnected') {
        setStatus('reconnected');
        // 2 秒后自动隐藏
        hideTimerRef.current = setTimeout(() => {
          setStatus('hidden');
        }, 2000);
      } else {
        setStatus('hidden');
      }
    }

    return () => {
      if (disconnectTimerRef.current) clearTimeout(disconnectTimerRef.current);
      if (hideTimerRef.current) clearTimeout(hideTimerRef.current);
    };
  }, [connected, status]);

  // 不显示时返回 null
  if (status === 'hidden') return null;

  /** 断线状态样式：橙色警告 */
  const disconnectedStyle: React.CSSProperties = {
    position: 'fixed',
    top: 0,
    left: 0,
    right: 0,
    zIndex: 9999,
    padding: '8px 16px',
    textAlign: 'center',
    fontSize: 13,
    fontWeight: 500,
    color: '#FFFFFF',
    backgroundColor: '#FF7D00',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 8,
    transition: 'background-color 0.3s ease',
  };

  /** 恢复状态样式：绿色提示 + 闪烁动画 */
  const reconnectedStyle: React.CSSProperties = {
    ...disconnectedStyle,
    backgroundColor: '#00B42A',
    animation: 'connectionPulse 0.5s ease-in-out 3',
  };

  return (
    <>
      <div style={status === 'disconnected' ? disconnectedStyle : reconnectedStyle}>
        {status === 'disconnected' ? (
          <>
            <DisconnectOutlined />
            <span>数据连接已中断，正在重连...</span>
          </>
        ) : (
          <>
            <WifiOutlined />
            <span>连接已恢复</span>
          </>
        )}
      </div>

      {/* 闪烁动画关键帧 */}
      <style>{`
        @keyframes connectionPulse {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.6; }
        }
      `}</style>
    </>
  );
};

export default ConnectionIndicator;
