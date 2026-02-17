import { useRef, useEffect } from 'react';
import './SubtitlePanel.css';
import type { Subtitle } from '../App';

interface SubtitlePanelProps {
  subtitles: Subtitle[];
  currentTime: number;
  currentSubtitle: Subtitle | null;
  onSubtitleClick: (subtitle: Subtitle) => void;
}

const SubtitlePanel = ({ subtitles, currentTime, currentSubtitle, onSubtitleClick }: SubtitlePanelProps) => {
  const scrollRef = useRef<HTMLDivElement>(null);
  const activeRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (activeRef.current && scrollRef.current) {
      activeRef.current.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
  }, [currentSubtitle]);

  const formatTime = (seconds: number): string => {
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    const ms = Math.floor((seconds % 1) * 100);
    return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}.${ms.toString().padStart(2, '0')}`;
  };

  if (subtitles.length === 0) {
    return (
      <div className="subtitle-panel">
        <div className="subtitle-header">
          <h3>字幕列表</h3>
          <span className="subtitle-count">0 条字幕</span>
        </div>
        <div className="subtitle-empty">
          <p>暂无字幕</p>
          <p className="empty-hint">生成字幕后将在此显示</p>
        </div>
      </div>
    );
  }

  return (
    <div className="subtitle-panel">
      <div className="subtitle-header">
        <h3>字幕列表</h3>
        <span className="subtitle-count">{subtitles.length} 条字幕</span>
      </div>
      <div className="subtitle-list" ref={scrollRef}>
        {subtitles.map((subtitle) => {
          const isActive = currentSubtitle?.id === subtitle.id;
          return (
            <div
              key={subtitle.id}
              ref={isActive ? activeRef : null}
              className={`subtitle-item ${isActive ? 'active' : ''}`}
              onClick={() => onSubtitleClick(subtitle)}
            >
              <div className="subtitle-time">
                <span className="time-start">{formatTime(subtitle.startTime)}</span>
                <span className="time-separator">→</span>
                <span className="time-end">{formatTime(subtitle.endTime)}</span>
              </div>
              <div className="subtitle-text">{subtitle.text}</div>
              {subtitle.translatedText && (
                <div className="subtitle-translated-text">{subtitle.translatedText}</div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
};

export default SubtitlePanel;
