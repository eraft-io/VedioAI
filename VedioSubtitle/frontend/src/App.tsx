import { useState, useRef, useCallback, useEffect } from 'react';
import './App.css';
import VideoPlayer from './components/VideoPlayer';
import SubtitlePanel from './components/SubtitlePanel';
import ControlPanel from './components/ControlPanel';
import InstallGuide from './components/InstallGuide';
import ProgressBar from './components/ProgressBar';
import { 
  SelectVideoFile, 
  GenerateSubtitle,
  CheckWhisperStatus,
  InstallWhisper,
  TranslateSubtitles
} from "../wailsjs/go/main/App";
import { main } from "../wailsjs/go/models";
import { EventsOn } from "../wailsjs/runtime";

export interface Subtitle {
  id: number;
  startTime: number;
  endTime: number;
  text: string;
  translatedText?: string;
}

function App() {
  const [videoPath, setVideoPath] = useState<string>('');
  const [subtitles, setSubtitles] = useState<Subtitle[]>([]);
  const [subtitlePath, setSubtitlePath] = useState<string>('');
  const [currentTime, setCurrentTime] = useState<number>(0);
  const [isGenerating, setIsGenerating] = useState<boolean>(false);
  const [selectedModel, setSelectedModel] = useState<string>('base');
  const [selectedLanguage, setSelectedLanguage] = useState<string>('auto');
  const [message, setMessage] = useState<string>('');
  const [currentSubtitle, setCurrentSubtitle] = useState<Subtitle | null>(null);
  const [isPlaying, setIsPlaying] = useState<boolean>(false);
  const [whisperInstalled, setWhisperInstalled] = useState<boolean>(false);
  const [showGuide, setShowGuide] = useState<boolean>(false);
  const [progress, setProgress] = useState<number>(0);
  const [progressMessage, setProgressMessage] = useState<string>('');
  const [showProgress, setShowProgress] = useState<boolean>(false);
  const [isTranslating, setIsTranslating] = useState<boolean>(false);
  const [showTranslateButton, setShowTranslateButton] = useState<boolean>(false);

  const videoRef = useRef<HTMLVideoElement | null>(null);

  // 监听进度事件
  useEffect(() => {
    const unsubscribe = EventsOn("subtitle:progress", (data: any) => {
      setProgress(data.progress || 0);
      setProgressMessage(data.message || '');
      
      if (data.status === 'processing') {
        setShowProgress(true);
      } else if (data.status === 'completed') {
        setProgress(100);
        setProgressMessage(data.message || '完成！');
        // 3秒后关闭进度条
        setTimeout(() => setShowProgress(false), 3000);
      } else if (data.status === 'error') {
        setShowProgress(false);
        setMessage(data.message);
      }
    });
    
    return () => {
      unsubscribe();
    };
  }, []);

  // 监听翻译进度事件
  useEffect(() => {
    const unsubscribe = EventsOn("translate:progress", (data: any) => {
      setProgress(data.progress || 0);
      setProgressMessage(data.message || '');
      
      if (data.status === 'processing') {
        setShowProgress(true);
      } else if (data.status === 'completed') {
        setProgress(100);
        setProgressMessage(data.message || '完成！');
        setIsTranslating(false);
        // 3秒后关闭进度条
        setTimeout(() => setShowProgress(false), 3000);
      } else if (data.status === 'error') {
        setShowProgress(false);
        setIsTranslating(false);
        setMessage(data.message);
      }
    });
    
    return () => {
      unsubscribe();
    };
  }, []);

  // 检查 Whisper 安装状态
  useEffect(() => {
    checkWhisper();
  }, []);

  const checkWhisper = async () => {
    try {
      const status = await CheckWhisperStatus();
      
      setWhisperInstalled(status.installed as boolean);
      
      if (status.installed) {
        setMessage('Whisper 已就绪，可以开始生成字幕');
        setShowGuide(false);
      } else {
        setShowGuide(true);
        setMessage(status.message as string);
      }
    } catch (err) {
      setWhisperInstalled(false);
    }
  };

  // 自动安装 Whisper
  const handleInstallWhisper = async () => {
    try {
      setMessage('正在安装 Whisper，请耐心等待...');
      const result = await InstallWhisper();
      if (result.success) {
        setWhisperInstalled(true);
        setMessage('Whisper 安装成功！');
        setShowGuide(false);
      } else {
        setMessage('安装失败: ' + result.message);
      }
    } catch (err) {
      setMessage('安装过程出错');
    }
  };

  // 选择视频文件
  const handleSelectVideo = async () => {
    try {
      const path = await SelectVideoFile();
      if (path) {
        setVideoPath(path);
        setSubtitles([]);
        setMessage('');
      }
    } catch (err) {
      setMessage('选择文件失败');
    }
  };

  // 生成字幕
  const handleGenerateSubtitle = async () => {
    if (!videoPath) {
      setMessage('请先选择视频文件');
      return;
    }

    setIsGenerating(true);
    setMessage('正在生成字幕，请稍候...');
    setShowProgress(true);
    setProgress(0);
    setProgressMessage('正在初始化...');

    try {
      const result: main.SubtitleResult = await GenerateSubtitle(videoPath, selectedModel, selectedLanguage);
      if (result.success) {
        setSubtitles(result.subtitles || []);
        // 设置字幕文件路径（SRT 文件与视频同目录）
        const baseName = videoPath.substring(0, videoPath.lastIndexOf('.'));
        setSubtitlePath(baseName + '.srt');
        setMessage(result.message || `字幕生成成功！共 ${result.subtitles?.length || 0} 条字幕`);
        // 显示翻译按钮
        setShowTranslateButton(true);
        // 3秒后关闭进度条
        setTimeout(() => setShowProgress(false), 3000);
      } else {
        setMessage(result.message || '字幕生成失败');
        setShowProgress(false);
      }
    } catch (err) {
      setMessage('生成字幕时发生错误');
      setShowProgress(false);
    } finally {
      setIsGenerating(false);
    }
  };

  // 翻译字幕
  const handleTranslateSubtitle = async () => {
    if (subtitles.length === 0) {
      setMessage('请先生成字幕');
      return;
    }

    setIsTranslating(true);
    setMessage('正在翻译字幕，请稍候...');
    setShowProgress(true);
    setProgress(0);
    setProgressMessage('正在初始化翻译环境...');

    try {
      const result = await TranslateSubtitles(subtitles as any);
      if (result.success) {
        setSubtitles(result.subtitles || []);
        setMessage(result.message || `翻译成功！共 ${result.subtitles?.length || 0} 条字幕`);
      } else {
        setMessage(result.message || '翻译失败');
        setShowProgress(false);
      }
    } catch (err) {
      setMessage('翻译时发生错误');
      setShowProgress(false);
      setIsTranslating(false);
    }
  };

  // 更新当前时间
  const handleTimeUpdate = useCallback((time: number) => {
    setCurrentTime(time);
    
    // 查找当前应该显示的字幕
    const subtitle = subtitles.find(
      s => time >= s.startTime && time <= s.endTime
    );
    setCurrentSubtitle(subtitle || null);
  }, [subtitles]);

  // 点击字幕跳转到对应时间
  const handleSubtitleClick = useCallback((subtitle: Subtitle) => {
    if (videoRef.current) {
      videoRef.current.currentTime = subtitle.startTime;
      videoRef.current.play();
    }
  }, []);

  return (
    <div className="app-container">
      <header className="app-header">
        <h1>视频字幕生成器</h1>
        <p className="subtitle">使用 Whisper AI 生成视频字幕，支持实时播放展示</p>
      </header>

      {showGuide && (
        <InstallGuide 
          onClose={() => setShowGuide(false)}
          onRefresh={checkWhisper}
          onInstall={handleInstallWhisper}
        />
      )}

      <ProgressBar 
        progress={progress}
        message={progressMessage}
        visible={showProgress}
      />

      <ControlPanel
        videoPath={videoPath}
        isGenerating={isGenerating}
        isTranslating={isTranslating}
        showTranslateButton={showTranslateButton}
        selectedModel={selectedModel}
        selectedLanguage={selectedLanguage}
        message={message}
        whisperInstalled={whisperInstalled}
        onSelectVideo={handleSelectVideo}
        onGenerateSubtitle={handleGenerateSubtitle}
        onTranslateSubtitle={handleTranslateSubtitle}
        onModelChange={setSelectedModel}
        onLanguageChange={setSelectedLanguage}
        onShowGuide={() => setShowGuide(true)}
      />

      <div className="main-content">
        <div className="video-section">
          <VideoPlayer
            videoPath={videoPath}
            subtitlePath={subtitlePath}
            onTimeUpdate={handleTimeUpdate}
            onPlayStateChange={setIsPlaying}
            videoRef={videoRef}
          />
          
          {/* Coursera 风格的实时字幕展示 - 双语 */}
          <div className="live-subtitle-container">
            <div className={`live-subtitle ${currentSubtitle ? 'visible' : ''}`}>
              <div className="subtitle-original">{currentSubtitle?.text || ''}</div>
              {currentSubtitle?.translatedText && (
                <div className="subtitle-translated">{currentSubtitle.translatedText}</div>
              )}
            </div>
          </div>
        </div>

        <div className="subtitle-section">
          <SubtitlePanel
            subtitles={subtitles}
            currentTime={currentTime}
            currentSubtitle={currentSubtitle}
            onSubtitleClick={handleSubtitleClick}
          />
        </div>
      </div>
    </div>
  );
}

export default App;
