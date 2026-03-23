import React, { useEffect, useState } from 'react';
import { Tooltip } from 'antd';

interface TimeAgoProps {
  timestamp: string;
}

function formatRelative(ts: string): string {
  const diff = Date.now() - new Date(ts).getTime();
  const seconds = Math.floor(diff / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}min`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h`;
  const days = Math.floor(hours / 24);
  return `${days}d`;
}

export const TimeAgo: React.FC<TimeAgoProps> = ({ timestamp }) => {
  const [display, setDisplay] = useState(() => formatRelative(timestamp));

  useEffect(() => {
    const interval = setInterval(() => setDisplay(formatRelative(timestamp)), 30000);
    return () => clearInterval(interval);
  }, [timestamp]);

  return (
    <Tooltip title={new Date(timestamp).toLocaleString()}>
      <span>{display}</span>
    </Tooltip>
  );
};
