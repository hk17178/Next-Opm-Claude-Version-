import React, { useState, useEffect, useRef } from 'react';
import { injectGlobalStyles } from '../utils/injectGlobalStyles';

interface AITypewriterProps {
  text: string;
  speed?: number;
  onComplete?: () => void;
  showCursor?: boolean;
  className?: string;
  style?: React.CSSProperties;
}

injectGlobalStyles('uikit-typewriter-keyframes', `
@keyframes uikit-cursorBlink {
  0%, 100% { opacity: 1; }
  50%      { opacity: 0; }
}
@media (prefers-reduced-motion: reduce) {
  .uikit-typewriter-cursor { animation: none !important; }
}
`);

export const AITypewriter: React.FC<AITypewriterProps> = ({
  text,
  speed = 25,
  onComplete,
  showCursor = true,
  className = '',
  style,
}) => {
  const [displayed, setDisplayed] = useState('');
  const [done, setDone] = useState(false);
  const onCompleteRef = useRef(onComplete);
  onCompleteRef.current = onComplete;

  useEffect(() => {
    setDisplayed('');
    setDone(false);
    let idx = 0;
    const timer = setInterval(() => {
      idx++;
      if (idx <= text.length) {
        setDisplayed(text.slice(0, idx));
      } else {
        clearInterval(timer);
        setDone(true);
        onCompleteRef.current?.();
      }
    }, speed);
    return () => clearInterval(timer);
  }, [text, speed]);

  return (
    <span className={className} style={style}>
      {displayed}
      {showCursor && (
        <span
          className="uikit-typewriter-cursor"
          style={{
            display: 'inline-block',
            width: '2px',
            height: '1em',
            backgroundColor: 'currentColor',
            marginLeft: 1,
            verticalAlign: 'text-bottom',
            animation: done ? 'none' : 'uikit-cursorBlink 1s step-end infinite',
            opacity: 1,
          }}
        />
      )}
    </span>
  );
};
