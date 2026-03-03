import { useState } from 'react';
import './APIKeyDialog.css';

interface APIKeyDialogProps {
  onClose: () => void;
  onSave: (apiKey: string) => void;
}

const APIKeyDialog = ({ onClose, onSave }: APIKeyDialogProps) => {
  const [apiKey, setApiKey] = useState('');
  const [isSaving, setIsSaving] = useState(false);

  const handleSave = async () => {
    if (!apiKey.trim()) {
      alert('请输入 API Key');
      return;
    }

    setIsSaving(true);
    try {
      await onSave(apiKey.trim());
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div className="apikey-dialog-overlay">
      <div className="apikey-dialog">
        <h2>配置阿里云 API Key</h2>
        
        <div className="info-box">
          <p><strong>如何获取 API Key：</strong></p>
          <ol>
            <li>访问 <a href="https://dashscope.console.aliyun.com/" target="_blank" rel="noopener noreferrer">阿里云百炼控制台</a></li>
            <li>登录/注册阿里云账号</li>
            <li>进入 API-KEY 管理页面</li>
            <li>创建新的 API Key 并复制</li>
            <li>粘贴到下方输入框</li>
          </ol>
        </div>

        <div className="input-group">
          <label>API Key:</label>
          <input
            type="password"
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            placeholder="请输入您的 API Key"
            autoFocus
          />
        </div>

        <div className="actions">
          <button className="btn-cancel" onClick={onClose}>取消</button>
          <button 
            className="btn-save" 
            onClick={handleSave}
            disabled={isSaving || !apiKey.trim()}
          >
            {isSaving ? '保存中...' : '保存'}
          </button>
        </div>
      </div>
    </div>
  );
};

export default APIKeyDialog;
