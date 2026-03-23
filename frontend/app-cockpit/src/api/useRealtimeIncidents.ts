/**
 * WebSocket 实时事件订阅 Hook
 * 连接到后端事件流，实时接收活跃事件更新
 * 支持断线自动重连（指数退避，最大 30s）
 */
import { useState, useEffect, useRef, useCallback } from 'react';
import type { Incident } from './incident';

/** WebSocket 服务端推送的消息结构 */
interface WsMessage {
  /** 消息类型：snapshot 全量快照 / update 增量更新 / delete 事件移除 */
  type: 'snapshot' | 'update' | 'delete';
  /** 事件数据（snapshot 时为数组，update/delete 时为单条） */
  data: Incident | Incident[];
}

/** Hook 返回值类型 */
export interface RealtimeIncidentsResult {
  /** 当前活跃事件列表 */
  incidents: Incident[];
  /** WebSocket 连接状态 */
  connected: boolean;
  /** 错误信息（连接失败时） */
  error: string | null;
}

/** 最大重连间隔（毫秒） */
const MAX_RECONNECT_INTERVAL = 30000;
/** 初始重连间隔（毫秒） */
const INITIAL_RECONNECT_INTERVAL = 1000;

/**
 * 构建 WebSocket 连接地址
 * 根据当前页面协议自动选择 ws/wss
 * @returns WebSocket URL
 */
function getWsUrl(): string {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const host = window.location.host;
  return `${protocol}//${host}/api/v1/incidents/stream`;
}

/**
 * 实时事件订阅 Hook
 * 通过 WebSocket 连接后端事件流，维护活跃事件列表
 * 断线时自动重连（指数退避策略）
 * @returns { incidents, connected, error }
 */
export function useRealtimeIncidents(): RealtimeIncidentsResult {
  /** 活跃事件列表 */
  const [incidents, setIncidents] = useState<Incident[]>([]);
  /** 连接状态 */
  const [connected, setConnected] = useState(false);
  /** 错误信息 */
  const [error, setError] = useState<string | null>(null);

  /** WebSocket 实例引用 */
  const wsRef = useRef<WebSocket | null>(null);
  /** 重连定时器引用 */
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  /** 当前重连间隔（指数退避） */
  const reconnectIntervalRef = useRef(INITIAL_RECONNECT_INTERVAL);
  /** 是否已主动关闭（组件卸载时不再重连） */
  const unmountedRef = useRef(false);

  /**
   * 建立 WebSocket 连接
   * 处理消息接收、连接状态变化、错误和断线重连
   */
  const connect = useCallback(() => {
    // 组件已卸载，不再重连
    if (unmountedRef.current) return;

    const ws = new WebSocket(getWsUrl());
    wsRef.current = ws;

    /** 连接成功回调 */
    ws.onopen = () => {
      setConnected(true);
      setError(null);
      // 连接成功后重置重连间隔
      reconnectIntervalRef.current = INITIAL_RECONNECT_INTERVAL;
    };

    /** 接收消息回调 */
    ws.onmessage = (event) => {
      try {
        const msg: WsMessage = JSON.parse(event.data);

        switch (msg.type) {
          case 'snapshot':
            // 全量快照：替换整个事件列表
            setIncidents(Array.isArray(msg.data) ? msg.data : [msg.data]);
            break;

          case 'update':
            // 增量更新：更新或新增单条事件
            setIncidents((prev) => {
              const updated = msg.data as Incident;
              const index = prev.findIndex((i) => i.id === updated.id);
              if (index >= 0) {
                // 已存在则替换
                const next = [...prev];
                next[index] = updated;
                return next;
              }
              // 不存在则追加到列表头部
              return [updated, ...prev];
            });
            break;

          case 'delete':
            // 事件移除：从列表中删除
            setIncidents((prev) => {
              const deleted = msg.data as Incident;
              return prev.filter((i) => i.id !== deleted.id);
            });
            break;
        }
      } catch {
        // 忽略无法解析的消息
      }
    };

    /** 连接关闭回调，触发自动重连 */
    ws.onclose = () => {
      setConnected(false);
      scheduleReconnect();
    };

    /** 连接错误回调 */
    ws.onerror = () => {
      setError('WebSocket 连接异常');
      ws.close();
    };
  }, []);

  /**
   * 计划重连
   * 使用指数退避策略，间隔从 1s 递增至最大 30s
   */
  const scheduleReconnect = useCallback(() => {
    if (unmountedRef.current) return;

    // 清除已有的重连定时器
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current);
    }

    const interval = reconnectIntervalRef.current;
    reconnectTimerRef.current = setTimeout(() => {
      // 指数退避：每次重连间隔翻倍，不超过最大值
      reconnectIntervalRef.current = Math.min(
        interval * 2,
        MAX_RECONNECT_INTERVAL,
      );
      connect();
    }, interval);
  }, [connect]);

  /** 组件挂载时建立连接，卸载时清理资源 */
  useEffect(() => {
    unmountedRef.current = false;
    connect();

    return () => {
      unmountedRef.current = true;
      // 清除重连定时器
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
      }
      // 关闭 WebSocket 连接
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, [connect]);

  return { incidents, connected, error };
}
