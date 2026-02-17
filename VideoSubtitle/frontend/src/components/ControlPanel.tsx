import './ControlPanel.css';

interface ControlPanelProps {
  videoPath: string;
  isGenerating: boolean;
  isTranslating: boolean;
  showTranslateButton: boolean;
  selectedModel: string;
  selectedLanguage: string;
  message: string;
  whisperInstalled: boolean;
  onSelectVideo: () => void;
  onGenerateSubtitle: () => void;
  onTranslateSubtitle: () => void;
  onModelChange: (model: string) => void;
  onLanguageChange: (language: string) => void;
  onShowGuide: () => void;
}

const ControlPanel = ({
  videoPath,
  isGenerating,
  isTranslating,
  showTranslateButton,
  selectedModel,
  selectedLanguage,
  message,
  whisperInstalled,
  onSelectVideo,
  onGenerateSubtitle,
  onTranslateSubtitle,
  onModelChange,
  onLanguageChange,
  onShowGuide
}: ControlPanelProps) => {
  const models = [
    { value: 'tiny', label: 'Tiny (最快, 精度一般)', description: '39M 参数' },
    { value: 'base', label: 'Base (推荐)', description: '74M 参数' },
    { value: 'small', label: 'Small (较慢, 精度较好)', description: '244M 参数' },
    { value: 'medium', label: 'Medium (慢, 精度好)', description: '769M 参数' },
    { value: 'large', label: 'Large (最慢, 精度最好)', description: '1550M 参数' },
  ];

  const languages = [
    { value: 'auto', label: '自动检测' },
    { value: 'zh', label: '中文' },
    { value: 'en', label: 'English' },
    { value: 'ja', label: '日本語' },
    { value: 'ko', label: '한국어' },
    { value: 'fr', label: 'Français' },
    { value: 'de', label: 'Deutsch' },
    { value: 'es', label: 'Español' },
  ];

  const getFileName = (path: string): string => {
    if (!path) return '';
    const parts = path.split('/');
    return parts[parts.length - 1];
  };

  return (
    <div className="control-panel">
      {!whisperInstalled && (
        <div className="whisper-warning">
          <span>⚠️ Whisper 未安装</span>
          <button className="btn btn-warning" onClick={onShowGuide}>
            查看安装指南
          </button>
        </div>
      )}

      <div className="control-row">
        <div className="control-group file-selection">
          <label>视频文件</label>
          <div className="file-input-wrapper">
            <button className="btn btn-primary" onClick={onSelectVideo} disabled={isGenerating}>
              <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
                <polyline points="17 8 12 3 7 8"></polyline>
                <line x1="12" y1="3" x2="12" y2="15"></line>
              </svg>
              选择视频
            </button>
            <span className="file-name" title={videoPath}>
              {videoPath ? getFileName(videoPath) : '未选择文件'}
            </span>
          </div>
        </div>

        <div className="control-group">
          <label>Whisper 模型</label>
          <select value={selectedModel} onChange={(e) => onModelChange(e.target.value)} disabled={isGenerating} className="select-input">
            {models.map(model => (
              <option key={model.value} value={model.value}>
                {model.label} - {model.description}
              </option>
            ))}
          </select>
        </div>

        <div className="control-group">
          <label>语言</label>
          <select value={selectedLanguage} onChange={(e) => onLanguageChange(e.target.value)} disabled={isGenerating} className="select-input">
            {languages.map(lang => (
              <option key={lang.value} value={lang.value}>
                {lang.label}
              </option>
            ))}
          </select>
        </div>

        <div className="control-group">
          <label>&nbsp;</label>
          <button className="btn btn-generate" onClick={onGenerateSubtitle} disabled={!videoPath || isGenerating || !whisperInstalled}>
            {isGenerating ? (
              <>
                <span className="spinner"></span>
                生成中...
              </>
            ) : (
              <>
                <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
                  <polygon points="5 3 19 12 5 21 5 3"></polygon>
                </svg>
                生成字幕
              </>
            )}
          </button>
        </div>

        {showTranslateButton && (
          <div className="control-group">
            <label>&nbsp;</label>
            <button className="btn btn-translate" onClick={onTranslateSubtitle} disabled={isTranslating}>
              {isTranslating ? (
                <>
                  <span className="spinner"></span>
                  翻译中...
                </>
              ) : (
                <>
                  <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2">
                    <path d="M12 2L2 7l10 5 10-5-10-5z"></path>
                    <path d="M2 17l10 5 10-5"></path>
                    <path d="M2 12l10 5 10-5"></path>
                  </svg>
                  翻译字幕
                </>
              )}
            </button>
          </div>
        )}
      </div>

      {message && (
        <div className={`message ${message.includes('成功') ? 'success' : message.includes('失败') || message.includes('错误') ? 'error' : 'info'}`}>
          {message}
        </div>
      )}
    </div>
  );
};

export default ControlPanel;
