import { useState, useEffect, useRef } from 'react';
import './InstallGuide.css';
import { EventsOn } from "../../wailsjs/runtime";

interface InstallGuideProps {
  onClose: () => void;
  onRefresh: () => void;
  onInstall: () => Promise<void>;
}

const InstallGuide = ({ onClose, onRefresh, onInstall }: InstallGuideProps) => {
  const [isInstalling, setIsInstalling] = useState(false);
  const [installLog, setInstallLog] = useState<string[]>([]);
  const [progress, setProgress] = useState(0);
  const [status, setStatus] = useState<'idle' | 'running' | 'completed' | 'error'>('idle');
  const logEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    // 监听安装进度事件
    const unsubscribe = EventsOn("install:progress", (data: any) => {
      setStatus(data.status);
      setProgress(data.progress || 0);
      
      const timestamp = new Date().toLocaleTimeString();
      const message = `[${timestamp}] ${data.message}`;
      setInstallLog(prev => [...prev, message]);
      
      if (data.output) {
        const outputLines = String(data.output).split('\n').filter((line: string) => line.trim());
        outputLines.forEach((line: string) => {
          setInstallLog(prev => [...prev, `  > ${line}`]);
        });
      }
    });

    return () => {
      unsubscribe();
    };
  }, []);

  useEffect(() => {
    // 自动滚动到底部
    logEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [installLog]);

  const handleInstall = async () => {
    setIsInstalling(true);
    setInstallLog([]);
    setProgress(0);
    setStatus('running');
    try {
      await onInstall();
    } finally {
      setIsInstalling(false);
    }
  };

  const getStatusColor = () => {
    switch (status) {
      case 'running': return '#1890ff';
      case 'completed': return '#52c41a';
      case 'error': return '#ff4d4f';
      default: return '#666';
    }
  };

  return (
    <div className="install-guide-overlay">
      <div className="install-guide-modal" style={{ maxWidth: '600px', width: '90%' }}>
        <div className="install-guide-header">
          <h2>安装 Whisper</h2>
          <button className="close-btn" onClick={onClose} disabled={isInstalling}>×</button>
        </div>
        
        {!isInstalling && status === 'idle' ? (
          <div className="install-guide-content">
            <p>Whisper 是生成字幕必需的依赖组件</p>
            <p className="install-note">安装过程可能需要 5-10 分钟，请耐心等待</p>
            <p className="install-note">您可以选择稍后安装，点击右上角 × 关闭此窗口</p>
          </div>
        ) : (
          <div className="install-guide-content">
            {/* 进度条 */}
            <div className="install-progress">
              <div className="progress-bar">
                <div 
                  className="progress-fill" 
                  style={{ 
                    width: `${progress}%`, 
                    backgroundColor: getStatusColor(),
                    transition: 'width 0.3s ease'
                  }}
                />
              </div>
              <div className="progress-text" style={{ color: getStatusColor() }}>
                {status === 'running' && '正在安装...'}
                {status === 'completed' && '安装完成！'}
                {status === 'error' && '安装失败'}
                {status === 'idle' && '准备安装...'}
                {progress > 0 && ` (${progress}%)`}
              </div>
            </div>
            
            {/* 日志窗口 */}
            <div className="install-log-container">
              <div className="install-log">
                {installLog.length === 0 ? (
                  <div className="log-empty">等待开始...</div>
                ) : (
                  installLog.map((line, index) => (
                    <div key={index} className="log-line">{line}</div>
                  ))
                )}
                <div ref={logEndRef} />
              </div>
            </div>
          </div>
        )}
        
        <div className="install-guide-footer">
          {!isInstalling && status === 'completed' && (
            <button className="btn btn-secondary" onClick={onRefresh}>
              刷新状态
            </button>
          )}
          <button 
            className="btn btn-primary" 
            onClick={handleInstall}
            disabled={isInstalling}
          >
            {isInstalling ? (
              <>
                <span className="spinner"></span>
                正在安装...
              </>
            ) : status === 'completed' ? (
              '重新安装'
            ) : (
              '自动安装 Whisper'
            )}
          </button>
        </div>
      </div>
    </div>
  );
};

export default InstallGuide;
