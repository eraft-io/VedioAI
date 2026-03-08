import { useState } from 'react';
import './ExportFormatSelector.css';

interface ExportFormatSelectorProps {
  onClose: () => void;
  onSelectHTML: (imageDir: string) => void;
  onSelectMarkdown: (imageDir: string) => void;
}

const ExportFormatSelector = ({ onClose, onSelectHTML, onSelectMarkdown }: ExportFormatSelectorProps) => {
  const [imageDir, setImageDir] = useState<string>('intelligent_ppt');

  return (
    <div className="export-format-overlay">
      <div className="export-format-dialog">
        <h2>选择导出格式</h2>
        <p className="description">请选择您想要导出的文件格式：</p>
        
        <div className="image-dir-input">
          <label htmlFor="imageDir">图片目录名：</label>
          <input
            type="text"
            id="imageDir"
            value={imageDir}
            onChange={(e) => setImageDir(e.target.value)}
            placeholder="intelligent_ppt"
          />
          <span className="hint">（存放 PPT 截图的文件夹名称）</span>
        </div>
        
        <div className="format-options">
          <div className="format-card" onClick={() => onSelectHTML(imageDir)}>
            <div className="format-icon">🌐</div>
            <h3>HTML 网页</h3>
            <p>生成美观的网页格式，可在浏览器中打开</p>
            <ul>
              <li>✓ 样式美观，排版清晰</li>
              <li>✓ 支持图片嵌入</li>
              <li>✓ 适合在线查看和分享</li>
            </ul>
          </div>
          
          <div className="format-card markdown" onClick={() => onSelectMarkdown(imageDir)}>
            <div className="format-icon">📝</div>
            <h3>Markdown 文档</h3>
            <p>生成 Markdown 格式文档，纯文本易编辑</p>
            <ul>
              <li>✓ 纯文本格式，易于编辑</li>
              <li>✓ 兼容各种 Markdown 编辑器</li>
              <li>✓ 适合导入笔记软件（Obsidian、Notion 等）</li>
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

export default ExportFormatSelector;
