import './ProgressBar.css';

interface ProgressBarProps {
  progress: number;
  message: string;
  visible: boolean;
}

const ProgressBar = ({ progress, message, visible }: ProgressBarProps) => {
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
      </div>
    </div>
  );
};

export default ProgressBar;
