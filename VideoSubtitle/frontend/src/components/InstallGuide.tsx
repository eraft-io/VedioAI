import { useState } from 'react';
import './InstallGuide.css';

interface InstallGuideProps {
  onClose: () => void;
  onRefresh: () => void;
  onInstall: () => Promise<void>;
}

const InstallGuide = ({ onClose, onRefresh, onInstall }: InstallGuideProps) => {
  const [isInstalling, setIsInstalling] = useState(false);

  const handleInstall = async () => {
    setIsInstalling(true);
    try {
      await onInstall();
    } finally {
      setIsInstalling(false);
    }
  };

  return (
    <div className="install-guide-overlay">
      <div className="install-guide-modal">
        <div className="install-guide-header">
          <h2>⚠️ Whisper 未安装</h2>
          <button className="close-btn" onClick={onClose}>×</button>
        </div>
        <div className="install-guide-content">
          <p>首次使用需要安装 Whisper，点击下方按钮自动安装</p>
          <p className="install-note">安装过程可能需要 5-10 分钟，请耐心等待</p>
        </div>
        <div className="install-guide-footer">
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
