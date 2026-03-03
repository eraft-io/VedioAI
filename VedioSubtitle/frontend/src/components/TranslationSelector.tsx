import { useState } from 'react';
import './TranslationSelector.css';

interface TranslationSelectorProps {
  onClose: () => void;
  onLocalSelect: () => void;
  onCloudSelect: () => void;
}

const TranslationSelector = ({ onClose, onLocalSelect, onCloudSelect }: TranslationSelectorProps) => {
  return (
    <div className="translation-selector-overlay">
      <div className="translation-selector">
        <h2>选择翻译方式</h2>
        <p className="description">请选择您想要使用的翻译服务：</p>
        
        <div className="options">
          <div className="option-card" onClick={onLocalSelect}>
            <div className="icon">💻</div>
            <h3>本地模型</h3>
            <p>使用 Qwen2.5-3B 本地运行，无需网络，完全离线</p>
            <ul>
              <li>✓ 免费使用</li>
              <li>✓ 隐私安全</li>
              <li>✓ 无需联网</li>
              <li>✗ 速度较慢</li>
            </ul>
          </div>
          
          <div className="option-card cloud" onClick={onCloudSelect}>
            <div className="icon">☁️</div>
            <h3>阿里云百炼千问</h3>
            <p>使用阿里云通义千问 API，需要网络连接</p>
            <ul>
              <li>✓ 翻译质量高</li>
              <li>✓ 速度快</li>
              <li>✓ 上下文理解好</li>
              <li>⚠ 需要 API Key</li>
            </ul>
          </div>
        </div>
        
        <div className="actions">
          <button className="btn-cancel" onClick={onClose}>取消</button>
        </div>
      </div>
    </div>
  );
};

export default TranslationSelector;
