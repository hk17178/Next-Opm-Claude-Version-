import React, { useState, useEffect, useCallback } from 'react';

export interface CountdownProps {
  /** Target date/time for the countdown */
  targetDate: Date | string;
  /** Label displayed above the countdown */
  label?: string;
  /** Remaining hours below which color turns warning/orange (default 72) */
  warningThreshold?: number;
  /** Remaining hours below which color turns danger/red (default 24) */
  dangerThreshold?: number;
}

interface TimeLeft {
  days: number;
  hours: number;
  minutes: number;
  seconds: number;
  totalMs: number;
}

function calcTimeLeft(target: Date): TimeLeft {
  const now = Date.now();
  const diff = target.getTime() - now;
  if (diff <= 0) {
    return { days: 0, hours: 0, minutes: 0, seconds: 0, totalMs: 0 };
  }
  return {
    days: Math.floor(diff / (1000 * 60 * 60 * 24)),
    hours: Math.floor((diff / (1000 * 60 * 60)) % 24),
    minutes: Math.floor((diff / (1000 * 60)) % 60),
    seconds: Math.floor((diff / 1000) % 60),
    totalMs: diff,
  };
}

function pad(n: number): string {
  return String(n).padStart(2, '0');
}

export const Countdown: React.FC<CountdownProps> = ({
  targetDate,
  label,
  warningThreshold = 72,
  dangerThreshold = 24,
}) => {
  const target = typeof targetDate === 'string' ? new Date(targetDate) : targetDate;

  const [timeLeft, setTimeLeft] = useState<TimeLeft>(() => calcTimeLeft(target));

  const tick = useCallback(() => {
    setTimeLeft(calcTimeLeft(target));
  }, [target]);

  useEffect(() => {
    tick();
    const id = setInterval(tick, 1000);
    return () => clearInterval(id);
  }, [tick]);

  const totalHoursLeft = timeLeft.totalMs / (1000 * 60 * 60);

  let color = 'var(--color-success, #00e5a0)';
  if (totalHoursLeft <= dangerThreshold) {
    color = 'var(--color-danger, #ff4d4f)';
  } else if (totalHoursLeft <= warningThreshold) {
    color = 'var(--color-warning, #faad14)';
  }

  const display = `${pad(timeLeft.days)}:${pad(timeLeft.hours)}:${pad(timeLeft.minutes)}:${pad(timeLeft.seconds)}`;

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: 6,
      }}
    >
      {label && (
        <span
          style={{
            fontSize: 12,
            fontWeight: 500,
            color: 'var(--text-secondary, #8899a6)',
            letterSpacing: '0.04em',
          }}
        >
          {label}
        </span>
      )}
      <span
        style={{
          fontFamily: '"JetBrains Mono", "Fira Code", "SF Mono", "Cascadia Code", monospace',
          fontVariantNumeric: 'tabular-nums',
          fontSize: 28,
          fontWeight: 700,
          color,
          letterSpacing: '0.05em',
          transition: 'color 0.6s ease',
        }}
      >
        {display}
      </span>
      {timeLeft.totalMs <= 0 && (
        <span
          style={{
            fontSize: 11,
            color: 'var(--color-danger, #ff4d4f)',
            fontWeight: 600,
          }}
        >
          EXPIRED
        </span>
      )}
    </div>
  );
};
