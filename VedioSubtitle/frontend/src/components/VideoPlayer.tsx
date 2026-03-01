import { useEffect, useRef } from 'react';
import './VideoPlayer.css';

interface VideoPlayerProps {
  videoPath: string;
  subtitlePath?: string;
  onTimeUpdate: (time: number) => void;
  onPlayStateChange: (isPlaying: boolean) => void;
  videoRef: React.RefObject<HTMLVideoElement | null>;
}

const VideoPlayer = ({ videoPath, subtitlePath, onTimeUpdate, onPlayStateChange, videoRef }: VideoPlayerProps) => {
  const localVideoRef = useRef<HTMLVideoElement>(null);

  useEffect(() => {
    if (localVideoRef.current && videoRef) {
      (videoRef as React.MutableRefObject<HTMLVideoElement | null>).current = localVideoRef.current;
    }
  }, [videoRef]);

  useEffect(() => {
    const video = localVideoRef.current;
    if (!video) return;

    const handleTimeUpdate = () => onTimeUpdate(video.currentTime);
    const handlePlay = () => onPlayStateChange(true);
    const handlePause = () => onPlayStateChange(false);
    const handleError = (e: Event) => {
      console.error('视频播放错误:', e);
      const videoElement = e.target as HTMLVideoElement;
      console.error('错误代码:', videoElement.error?.code);
      console.error('错误信息:', videoElement.error?.message);
    };
    const handleLoaded = () => {
      console.log('视频加载成功:', videoPath);
    };

    video.addEventListener('timeupdate', handleTimeUpdate);
    video.addEventListener('play', handlePlay);
    video.addEventListener('pause', handlePause);
    video.addEventListener('error', handleError);
    video.addEventListener('loadeddata', handleLoaded);

    return () => {
      video.removeEventListener('timeupdate', handleTimeUpdate);
      video.removeEventListener('play', handlePlay);
      video.removeEventListener('pause', handlePause);
      video.removeEventListener('error', handleError);
      video.removeEventListener('loadeddata', handleLoaded);
    };
  }, [onTimeUpdate, onPlayStateChange, videoPath]);

  const getVideoUrl = (path: string): string => {
    if (!path) return '';
    // 处理 Windows 路径：将反斜杠转换为正斜杠
    let normalizedPath = path.replace(/\\/g, '/');
    // 确保路径以 / 开头（用于 URL）
    if (!normalizedPath.startsWith('/')) {
      normalizedPath = '/' + normalizedPath;
    }
    // URL 编码路径（保留斜杠和冒号）
    const encodedPath = normalizedPath.split('/').map(part => encodeURIComponent(part)).join('/');
    return '/local-file' + encodedPath;
  };

  const getSubtitleUrl = (path?: string): string => {
    if (!path) return '';
    return path.startsWith('file://') ? path : 'file://' + path;
  };

  // 获取视频 MIME 类型
  const getVideoMimeType = (path: string): string => {
    const ext = path.split('.').pop()?.toLowerCase();
    const mimeTypes: Record<string, string> = {
      'mp4': 'video/mp4',
      'webm': 'video/webm',
      'ogg': 'video/ogg',
      'mkv': 'video/webm', // MKV 使用 webm 容器播放
      'mov': 'video/quicktime',
      'avi': 'video/x-msvideo',
    };
    return mimeTypes[ext || ''] || 'video/mp4';
  };

  // 当视频路径改变时，手动加载新视频
  useEffect(() => {
    const video = localVideoRef.current;
    if (video && videoPath) {
      video.load(); // 强制重新加载视频
    }
  }, [videoPath]);

  return (
    <div className="video-player-container">
      {videoPath ? (
        <video 
          ref={localVideoRef} 
          className="video-element" 
          controls 
          preload="metadata"
        >
          <source src={getVideoUrl(videoPath)} type={getVideoMimeType(videoPath)} />
          您的浏览器不支持视频播放。
        </video>
      ) : (
        <div className="video-placeholder">
          <div className="placeholder-content">
            <svg className="video-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <rect x="2" y="2" width="20" height="20" rx="2.18" ry="2.18"></rect>
              <line x1="7" y1="2" x2="7" y2="22"></line>
              <line x1="17" y1="2" x2="17" y2="22"></line>
              <line x1="2" y1="12" x2="22" y2="12"></line>
            </svg>
            <p>请选择视频文件开始</p>
          </div>
        </div>
      )}
    </div>
  );
};

export default VideoPlayer;
