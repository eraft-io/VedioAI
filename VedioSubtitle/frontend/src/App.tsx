import { useState, useRef, useCallback, useEffect } from 'react';
import './App.css';
import VideoPlayer from './components/VideoPlayer';
import SubtitlePanel from './components/SubtitlePanel';
import ControlPanel from './components/ControlPanel';
import InstallGuide from './components/InstallGuide';
import ProgressBar from './components/ProgressBar';
import APIKeyDialog from './components/APIKeyDialog';
import ExportFormatSelector from './components/ExportFormatSelector';
import { 
  SelectVideoFile, 
  GenerateSubtitle,
  CheckWhisperStatus,
  InstallWhisper,
  ExportSubtitlesToJSON,
  SummarizeSubtitles,
  AnalyzeSubtitlesByContent,
  CheckQwenAPIKey,
  SaveQwenAPIKey,
  TranslateSubtitlesWithQwen
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
  const [isSummarizing, setIsSummarizing] = useState<boolean>(false);
  const [showAPIKeyDialog, setShowAPIKeyDialog] = useState<boolean>(false);
  const [showExportFormatSelector, setShowExportFormatSelector] = useState<boolean>(false);

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

  //监听总结进度事件
  useEffect(() => {
    const unsubscribe = EventsOn("summarize:progress", (data: any) => {
      setProgress(data.progress || 0);
      setProgressMessage(data.message || '');
        
      if (data.status === 'processing') {
        setShowProgress(true);
      } else if (data.status === 'completed') {
        setProgress(100);
        setProgressMessage(data.message || '完成！');
        setIsSummarizing(false);
        setTimeout(() => setShowProgress(false), 3000);
      } else if (data.status === 'error') {
        setShowProgress(false);
        setIsSummarizing(false);
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
  
    // 直接调用阿里云千问翻译
    handleCloudTranslate();
  };
  
  // 使用阿里云千问翻译
  const handleCloudTranslate = async () => {
  
    // 检查 API Key
    try {
      const keyCheck = await CheckQwenAPIKey();
      if (!keyCheck.has_key) {
        // 需要输入 API Key
        setShowAPIKeyDialog(true);
        return;
      }
  
      // 开始翻译
      setIsTranslating(true);
      setMessage('正在调用阿里云百炼千问翻译...');
      setShowProgress(true);
      setProgress(0);
      setProgressMessage('正在连接阿里云...');
  
      const result = await TranslateSubtitlesWithQwen(subtitles as any);
        
      if (result.success) {
        const newSubtitles = result.subtitles || [];
        setSubtitles(newSubtitles);
        setMessage(`翻译完成：${newSubtitles.length}条`);
      } else {
        setMessage(result.message || '翻译失败');
        setShowProgress(false);
        setIsTranslating(false);
      }
    } catch (err) {
      setMessage('翻译时发生错误：' + String(err));
      setShowProgress(false);
      setIsTranslating(false);
    }
  };
  
  // 保存 API Key
  const handleSaveAPIKey = async (apiKey: string) => {
    try {
      const result = await SaveQwenAPIKey(apiKey);
      if (result.success) {
        setMessage('API Key 保存成功！');
        setShowAPIKeyDialog(false);
        // 继续翻译
        handleCloudTranslate();
      } else {
        setMessage('保存失败：' + result.message);
      }
    } catch (err) {
      setMessage('保存 API Key 失败：' + String(err));
    }
  };
  
  // 导入字幕
  const handleImportSubtitle = async () => {
    console.log('[导入字幕] ========== 开始导入流程 ==========');
    console.log('[导入字幕] 当前时间:', new Date().toISOString());
    
    // 检查浏览器是否支持 File API
    if (!window.File || !window.FileReader || !window.FileList || !window.Blob) {
      const errorMsg = '您的浏览器不支持文件操作，请使用现代浏览器（Chrome/Firefox/Safari）';
      console.error('[导入字幕]', errorMsg);
      setMessage('❌ ' + errorMsg);
      return;
    }
    console.log('[导入字幕] File API 支持检查通过');
    
    try {
      // 使用原生文件选择器选择 JSON 文件
      const input = document.createElement('input');
      input.type = 'file';
      input.accept = '.json,application/json';
      input.style.display = 'none';
      document.body.appendChild(input);
      
      console.log('[导入字幕] input 元素已创建并添加到 DOM');
      
      // 处理文件选择 - 使用 addEventListener 替代 onchange
      const handleFileSelect = async (e: Event) => {
        console.log('[导入字幕] 文件选择事件触发 (addEventListener)');
        console.log('[导入字幕] 事件对象:', e);
        
        const target = e.target as HTMLInputElement;
        const files = target.files;
        
        console.log('[导入字幕] files 对象:', files);
        console.log('[导入字幕] files.length:', files?.length);
        
        if (!files || files.length === 0) {
          console.log('[导入字幕] 未选择文件或文件列表为空');
          setMessage('⚠️ 未选择文件');
          document.body.removeChild(input);
          return;
        }
        
        const file = files[0];
        console.log(`[导入字幕] 选择的文件: ${file.name}`);
        console.log(`[导入字幕] 文件大小: ${file.size} bytes`);
        console.log(`[导入字幕] 文件类型: ${file.type}`);
        console.log(`[导入字幕] 文件最后修改: ${new Date(file.lastModified).toISOString()}`);
        
        // 检查文件大小（最大 50MB）
        const maxSize = 50 * 1024 * 1024;
        if (file.size > maxSize) {
          const errorMsg = `文件过大 (${(file.size / 1024 / 1024).toFixed(2)}MB)，请上传小于 50MB 的文件`;
          console.error('[导入字幕]', errorMsg);
          setMessage('❌ ' + errorMsg);
          document.body.removeChild(input);
          return;
        }
        
        // 检查文件扩展名
        const fileName = file.name.toLowerCase();
        if (!fileName.endsWith('.json')) {
          const errorMsg = `文件格式错误：${file.name} 不是 JSON 文件`;
          console.error('[导入字幕]', errorMsg);
          setMessage('❌ ' + errorMsg);
          document.body.removeChild(input);
          return;
        }
        
        // 检查文件是否为空
        if (file.size === 0) {
          const errorMsg = '文件为空，请检查文件内容';
          console.error('[导入字幕]', errorMsg);
          setMessage('❌ ' + errorMsg);
          document.body.removeChild(input);
          return;
        }
        
        setMessage('📂 正在读取文件...');
        
        const reader = new FileReader();
        
        reader.onloadstart = () => {
          console.log('[导入字幕] FileReader: 开始读取文件');
        };
        
        reader.onload = (event) => {
          console.log('[导入字幕] 文件读取完成');
          
          try {
            const content = event.target?.result as string;
            
            if (!content || content.trim() === '') {
              const errorMsg = '文件内容为空';
              console.error('[导入字幕]', errorMsg);
              setMessage('❌ ' + errorMsg);
              return;
            }
            
            console.log(`[导入字幕] 文件内容长度: ${content.length} 字符`);
            
            // 尝试解析 JSON
            let jsonData: any;
            try {
              jsonData = JSON.parse(content);
            } catch (jsonErr: any) {
              // JSON 解析失败，尝试修复常见问题
              console.error('[导入字幕] JSON 解析失败:', jsonErr);
              
              // 检查是否是 BOM 问题
              let cleanedContent = content;
              if (content.charCodeAt(0) === 0xFEFF) {
                cleanedContent = content.substring(1);
                console.log('[导入字幕] 检测到 BOM，已移除');
              }
              
              // 尝试再次解析
              try {
                jsonData = JSON.parse(cleanedContent);
                console.log('[导入字幕] 清理后 JSON 解析成功');
              } catch (retryErr) {
                const errorMsg = `JSON 解析失败: ${jsonErr.message || '格式错误'}。请确保文件是有效的 JSON 格式`;
                console.error('[导入字幕]', errorMsg);
                setMessage('❌ ' + errorMsg);
                return;
              }
            }
            
            console.log('[导入字幕] JSON 解析成功', jsonData);
            
            // 检查数据结构
            if (!jsonData) {
              const errorMsg = 'JSON 数据为空';
              console.error('[导入字幕]', errorMsg);
              setMessage('❌ ' + errorMsg);
              return;
            }
            
            // 解析 Whisper JSON 格式
            if (jsonData.segments && Array.isArray(jsonData.segments)) {
              const segmentCount = jsonData.segments.length;
              console.log(`[导入字幕] 找到 ${segmentCount} 条字幕段`);
              
              if (segmentCount === 0) {
                const errorMsg = '字幕段数组为空';
                console.error('[导入字幕]', errorMsg);
                setMessage('❌ ' + errorMsg);
                return;
              }
              
              // 验证每条字幕段的数据
              const invalidSegments = jsonData.segments.filter((seg: any, idx: number) => {
                if (!seg || typeof seg !== 'object') {
                  console.error(`[导入字幕] 第 ${idx + 1} 条字幕段格式无效:`, seg);
                  return true;
                }
                if (typeof seg.start !== 'number' || typeof seg.end !== 'number') {
                  console.error(`[导入字幕] 第 ${idx + 1} 条字幕段时间戳无效:`, seg);
                  return true;
                }
                return false;
              });
              
              if (invalidSegments.length > 0) {
                const errorMsg = `发现 ${invalidSegments.length} 条格式无效的字幕段`;
                console.error('[导入字幕]', errorMsg);
                setMessage('⚠️ ' + errorMsg + '，已跳过无效数据');
              }
              
              const importedSubtitles = jsonData.segments
                .filter((seg: any) => seg && typeof seg === 'object')
                .map((seg: any, index: number) => ({
                  id: seg.id ?? index,
                  startTime: seg.start ?? 0,
                  endTime: seg.end ?? 0,
                  text: String(seg.text || '').trim(),
                  translatedText: String(seg.translatedText || '').trim()
                }));
              
              console.log(`[导入字幕] 成功解析 ${importedSubtitles.length} 条字幕`);
              
              setSubtitles(importedSubtitles);
              setSubtitlePath(file.name.replace('.json', '.srt'));
              setMessage(`✅ 成功导入 ${importedSubtitles.length} 条字幕`);
              setShowTranslateButton(true);
              
              console.log('[导入字幕] 字幕状态已更新');
            } else if (Array.isArray(jsonData)) {
              // 尝试解析为纯数组格式
              console.log('[导入字幕] 检测到数组格式，尝试解析');
              
              const importedSubtitles = jsonData
                .filter((seg: any) => seg && typeof seg === 'object')
                .map((seg: any, index: number) => ({
                  id: seg.id ?? index,
                  startTime: seg.start ?? seg.startTime ?? 0,
                  endTime: seg.end ?? seg.endTime ?? 0,
                  text: String(seg.text || '').trim(),
                  translatedText: String(seg.translatedText || seg.translated || '').trim()
                }));
              
              if (importedSubtitles.length > 0) {
                console.log(`[导入字幕] 成功解析 ${importedSubtitles.length} 条字幕（数组格式）`);
                setSubtitles(importedSubtitles);
                setSubtitlePath(file.name.replace('.json', '.srt'));
                setMessage(`✅ 成功导入 ${importedSubtitles.length} 条字幕`);
                setShowTranslateButton(true);
              } else {
                const errorMsg = '无法从数组中解析出有效字幕数据';
                console.error('[导入字幕]', errorMsg);
                setMessage('❌ ' + errorMsg);
              }
            } else {
              const errorMsg = `无效的 JSON 格式：未找到 segments 数组。支持的格式: { "segments": [...] } 或直接数组 [...]`;
              console.error('[导入字幕]', errorMsg, jsonData);
              setMessage('❌ ' + errorMsg);
            }
          } catch (err: any) {
            console.error('[导入字幕] 处理文件时发生错误:', err);
            setMessage('❌ 处理文件时发生错误: ' + (err.message || String(err)));
          }
        };
        
        reader.onerror = (error) => {
          console.error('[导入字幕] 文件读取错误:', error);
          let errorMsg = '文件读取失败';
          
          // 尝试获取更详细的错误信息
          if (reader.error) {
            const errorCode = (reader.error as any).code;
            switch (errorCode) {
              case 1: // NOT_FOUND_ERR
                errorMsg = '文件未找到';
                break;
              case 2: // NOT_READABLE_ERR
                errorMsg = '文件无法读取（可能没有权限）';
                break;
              case 3: // ABORT_ERR
                errorMsg = '读取操作被中断';
                break;
              default:
                errorMsg = '文件读取错误: ' + reader.error.message;
            }
          }
          
          setMessage('❌ ' + errorMsg);
        };
        
        reader.onabort = () => {
          console.log('[导入字幕] 读取操作被用户取消');
          setMessage('⚠️ 读取操作已取消');
        };
        
        // 设置超时处理
        const timeoutId = setTimeout(() => {
          console.error('[导入字幕] 文件读取超时');
          reader.abort();
          setMessage('❌ 文件读取超时，请重试');
        }, 30000); // 30秒超时
        
        reader.onloadend = () => {
          clearTimeout(timeoutId);
        };
        
        reader.readAsText(file);
      };
      
      // 绑定事件监听器
      input.addEventListener('change', handleFileSelect);
      
      // 处理取消选择的情况
      input.addEventListener('cancel', () => {
        console.log('[导入字幕] 用户取消文件选择 (cancel 事件)');
        setMessage('⚠️ 已取消文件选择');
        document.body.removeChild(input);
      });
      
      // 点击事件
      input.addEventListener('click', () => {
        console.log('[导入字幕] 点击文件选择器 (click 事件)');
      });
      
      // 使用 setTimeout 确保 DOM 更新后再触发点击
      setTimeout(() => {
        console.log('[导入字幕] 准备触发文件选择器 click()');
        try {
          input.click();
          console.log('[导入字幕] 文件选择器 click() 已触发');
        } catch (clickErr) {
          console.error('[导入字幕] 触发文件选择器失败:', clickErr);
          setMessage('❌ 无法打开文件选择器');
          document.body.removeChild(input);
        }
      }, 100);
      
    } catch (err: any) {
      console.error('[导入字幕] 导入字幕失败:', err);
      setMessage('❌ 导入字幕失败: ' + (err.message || String(err)));
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

  //总结字幕内容（包含智能PPT提取）
  const handleSummarizeSubtitle = async () => {
    if (subtitles.length === 0) {
      setMessage('没有字幕可以总结');
      return;
    }
    if (!videoPath) {
      setMessage('请选择视频文件');
      return;
    }
  
    // 显示格式选择对话框
    setShowExportFormatSelector(true);
  };

  // 导出为 HTML 格式
  const handleExportHTML = async (imageDir: string) => {
    setShowExportFormatSelector(false);
    await performSummarize('html', imageDir);
  };

  // 导出为 Markdown 格式
  const handleExportMarkdown = async (imageDir: string) => {
    setShowExportFormatSelector(false);
    await performSummarize('markdown', imageDir);
  };

  // 执行总结导出
  const performSummarize = async (format: string, imageDir: string) => {
    setIsSummarizing(true);
    setMessage(`正在生成${format === 'markdown' ? 'Markdown' : 'HTML'}文档...`);
    setShowProgress(true);
    setProgress(0);
    setProgressMessage('正在分析字幕内容并提取关键帧...');
  
    try {
      //先提取智能PPT
      const subtitleItems = subtitles.map((sub, index) => ({
        id: sub.id || index,
        startTime: sub.startTime,
        endTime: sub.endTime,
        text: sub.text,
        translatedText: sub.translatedText || ''
      }));

      //提取PPT关键帧
      const pptResult: main.IntelligentPPTResult = await AnalyzeSubtitlesByContent(subtitleItems as any, videoPath, imageDir);
      
      if (pptResult.success) {
        setProgressMessage(`已提取 ${pptResult.frames?.length || 0} 个关键帧，正在生成${format === 'markdown' ? 'Markdown' : 'HTML'}...`);
      }

      //然后生成双语文档
      const summarySubtitles = subtitles.map((sub, index) => ({
        id: sub.id || index,
        startTime: sub.startTime,
        endTime: sub.endTime,
        text: sub.text,
        translatedText: sub.translatedText || ''
      }));
  
      const result = await SummarizeSubtitles(summarySubtitles as any, videoPath, format, imageDir);
        
      if (result.success) {
        setMessage(`${format === 'markdown' ? 'Markdown' : 'HTML'}文档已保存到: ${result.outputPath}`);
      } else {
        setMessage(`生成文档失败: ${result.message}`);
      }
    } catch (err) {
      console.error('总结失败:', err);
      setMessage('生成文档失败: ' + String(err));
    } finally {
      setIsSummarizing(false);
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



      {showAPIKeyDialog && (
        <APIKeyDialog
          onClose={() => setShowAPIKeyDialog(false)}
          onSave={handleSaveAPIKey}
        />
      )}

      {showExportFormatSelector && (
        <ExportFormatSelector
          onClose={() => setShowExportFormatSelector(false)}
          onSelectHTML={handleExportHTML}
          onSelectMarkdown={handleExportMarkdown}
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
        isSummarizing={isSummarizing}
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
        onSummarizeSubtitle={handleSummarizeSubtitle}
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
