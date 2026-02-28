import { useRef, useEffect } from 'react';
import './ProgressBar.css';

interface ProgressBarProps {
  progress: number;
  message: string;
  visible: boolean;
  output?: string;
}

const ProgressBar = ({ progress, message, visible, output }: ProgressBarProps) => {
  const logEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    // 自动滚动到底部
    logEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [output]);

  if (!visible) return null;

  return (
    <div className="progress-overlay">
      <div className="progress-container">
        <div className="progress-message">{message}</div>
        <div className="progress-bar-wrapper">
          <div className="progress-bar">
            <div 
              className="progress-fill" 
              style={{ width: `${progress}%` }}
            ></div>
          </div>
          <span className="progress-text">{Math.round(progress)}%</span>
        </div>
        {/* 实时输出日志 */}
        {output && (
          <div className="progress-log-container">
            <div className="progress-log">
              {output.split('\n').map((line, index) => (
                line && <div key={index} className="log-line">{line}</div>
              ))}
              <div ref={logEndRef} />
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default ProgressBar;
