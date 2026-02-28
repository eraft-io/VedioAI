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
  TranslateSubtitles,
  ExportSubtitlesToJSON
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
  const [progressOutput, setProgressOutput] = useState<string>('');
  const [isTranslating, setIsTranslating] = useState<boolean>(false);
  const [showTranslateButton, setShowTranslateButton] = useState<boolean>(false);

  const videoRef = useRef<HTMLVideoElement | null>(null);

  // 监听进度事件
  useEffect(() => {
    const unsubscribe = EventsOn("subtitle:progress", (data: any) => {
      setProgress(data.progress || 0);
      setProgressMessage(data.message || '');
      
      // 累积输出日志
      if (data.output) {
        setProgressOutput(prev => prev + data.output + '\n');
      }
      
      if (data.status === 'processing') {
        setShowProgress(true);
      } else if (data.status === 'completed') {
        setProgress(100);
        setProgressMessage(data.message || '完成！');
        // 3秒后关闭进度条并清空日志
        setTimeout(() => {
          setShowProgress(false);
          setProgressOutput('');
        }, 3000);
      } else if (data.status === 'error') {
        setShowProgress(false);
        setMessage(data.message);
        // 错误时不清空日志，让用户可以看到错误信息
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
      } else {
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
        const newSubtitles = result.subtitles || [];
        setSubtitles(newSubtitles);
        // 调试：检查翻译结果
        const firstSub = newSubtitles[0];
        setMessage(`翻译完成: ${newSubtitles.length}条, 第一条翻译=${firstSub?.translatedText?.substring(0, 20) || '空'}`);
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

  // 导入字幕
  const handleImportSubtitle = async () => {
    try {
      // 使用原生文件选择器选择 JSON 文件
      const input = document.createElement('input');
      input.type = 'file';
      input.accept = '.json';
      input.onchange = async (e: any) => {
        const file = e.target.files[0];
        if (!file) return;
        
        const reader = new FileReader();
        reader.onload = (event) => {
          try {
            const jsonData = JSON.parse(event.target?.result as string);
            // 解析 Whisper JSON 格式
            if (jsonData.segments && Array.isArray(jsonData.segments)) {
              const importedSubtitles = jsonData.segments.map((seg: any, index: number) => ({
                id: seg.id || index,
                startTime: seg.start,
                endTime: seg.end,
                text: seg.text?.trim() || '',
                translatedText: seg.translatedText || ''
              }));
              setSubtitles(importedSubtitles);
              setSubtitlePath(file.name.replace('.json', '.srt'));
              setMessage(`成功导入 ${importedSubtitles.length} 条字幕`);
              setShowTranslateButton(true);
            } else {
              setMessage('无效的 JSON 格式');
            }
          } catch (err) {
            setMessage('解析 JSON 文件失败');
          }
        };
        reader.readAsText(file);
      };
      input.click();
    } catch (err) {
      setMessage('导入字幕失败');
    }
  };

  // 导出字幕
  const handleExportSubtitle = async () => {
    console.log('handleExportSubtitle 被调用, subtitles.length=', subtitles.length);
    
    if (subtitles.length === 0) {
      console.log('字幕为空，返回');
      setMessage('没有可导出的字幕');
      return;
    }
    
    try {
      console.log('调用后端导出方法...');
      // 转换为后端需要的格式
      const exportSubtitles = subtitles.map((sub, index) => ({
        id: sub.id || index,
        startTime: sub.startTime,
        endTime: sub.endTime,
        text: sub.text,
        translatedText: sub.translatedText || ''
      }));
      
      console.log('导出数据:', exportSubtitles.length, '条');
      console.log('第一条:', exportSubtitles[0]);
      console.log('videoPath:', videoPath);
      
      const result = await ExportSubtitlesToJSON(exportSubtitles as any, videoPath);
      console.log('导出结果:', result);
      
      if (result.success) {
        setMessage(`成功导出字幕到: ${result.path}`);
      } else {
        setMessage(`导出失败: ${result.message}`);
      }
    } catch (err) {
      console.error('导出失败:', err);
      setMessage('导出字幕失败: ' + String(err));
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
        output={progressOutput}
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
        onImportSubtitle={handleImportSubtitle}
        onExportSubtitle={handleExportSubtitle}
        hasSubtitles={subtitles.length > 0}
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
