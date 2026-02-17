# VideoSubtitle

A desktop application for automatic video subtitle generation and translation, built with [Wails](https://wails.io/) (Go + React + TypeScript).

## Features

- **Automatic Subtitle Generation**: Uses OpenAI Whisper to transcribe video audio into subtitles
- **AI Translation**: Translates subtitles to Chinese using local LLM (Qwen2.5-3B via llama.cpp)
- **Real-time Preview**: Watch videos with synchronized bilingual subtitles
- **Progress Tracking**: Visual progress bars for generation and translation tasks
- **Auto-installation**: One-click setup for Whisper environment
- **Multiple Models**: Supports various Whisper models (tiny, base, small, medium, large)
- **Multi-language Support**: Auto-detects language or specify manually

## Tech Stack

- **Backend**: Go with Wails v2
- **Frontend**: React + TypeScript + Vite
- **AI/ML**: 
  - OpenAI Whisper for speech-to-text
  - llama.cpp with Qwen2.5-3B for translation
- **Environment**: Conda for Python dependency management

## Prerequisites

- [Go](https://golang.org/dl/) 1.18+
- [Node.js](https://nodejs.org/) 16+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)
- [Anaconda](https://www.anaconda.com/) or [Miniconda](https://docs.conda.io/en/latest/miniconda.html)

## Installation

### 1. Clone the repository

```bash
git clone <repository-url>
cd VideoSubtitle
```

### 2. Install dependencies

```bash
# Install Go dependencies
go mod tidy

# Install frontend dependencies
cd frontend
npm install
cd ..
```

### 3. Run in development mode

```bash
wails dev
```

### 4. Build for production

```bash
# Build for current platform
wails build

# Build for specific platforms
wails build -platform darwin/universal  # macOS Universal
wails build -platform windows/amd64     # Windows
wails build -platform linux/amd64       # Linux
```

## Usage

1. **Launch the application** - The app will check if Whisper is installed
2. **Install Whisper** (first time only) - Click "Auto Install Whisper" button
3. **Select video** - Choose a video file to process
4. **Generate subtitles** - Select model and language, then click generate
5. **Translate** (optional) - Click translate button to get Chinese subtitles
6. **Preview** - Watch video with real-time bilingual subtitle display

## Project Structure

```
VideoSubtitle/
├── app.go           # Main application logic & Whisper management
├── subtitle.go      # Subtitle generation & parsing
├── translate.go     # AI translation with llama.cpp
├── main.go          # Application entry point
├── frontend/        # React frontend
│   ├── src/
│   │   ├── App.tsx              # Main app component
│   │   └── components/          # UI components
│   │       ├── VideoPlayer.tsx  # Video player with subtitle sync
│   │       ├── SubtitlePanel.tsx # Subtitle list display
│   │       ├── ControlPanel.tsx # Control buttons & options
│   │       ├── InstallGuide.tsx # Whisper setup guide
│   │       └── ProgressBar.tsx  # Progress indicator
│   └── wailsjs/     # Wails generated bindings
├── build/           # Build configurations
└── wails.json       # Wails configuration
```

## Configuration

### Whisper Models

| Model  | Size  | Speed | Accuracy |
|--------|-------|-------|----------|
| tiny   | 39 MB | Fastest | Basic |
| base   | 74 MB | Fast | Good |
| small  | 244 MB | Moderate | Better |
| medium | 769 MB | Slow | Very Good |
| large  | 1550 MB | Slowest | Best |

### Translation Model

- **Model**: Qwen2.5-3B-Instruct-GGUF (q4_k_m quantized)
- **Size**: ~2GB
- **Download**: Automatic on first translation
- **Source**: [Hugging Face](https://huggingface.co/Qwen/Qwen2.5-3B-Instruct-GGUF)

## Development

### Frontend Development

```bash
cd frontend
npm run dev  # Starts Vite dev server
```

### Backend Development

The Go backend uses Wails bindings to expose functions to the frontend. Key modules:

- `CheckWhisperStatus()` - Check if Whisper is installed
- `InstallWhisper()` - Auto-install Whisper via conda
- `GenerateSubtitle(videoPath, model, language)` - Generate subtitles
- `TranslateSubtitles(subtitles)` - Translate to Chinese

### Events

The app uses Wails event system for progress updates:

- `subtitle:progress` - Subtitle generation progress
- `translate:progress` - Translation progress

## Troubleshooting

### Whisper not found
- Ensure conda is installed and accessible in PATH
- Try manual installation: `conda create -n whisper python=3.10 && conda install -n whisper openai-whisper`

### Translation fails
- Check internet connection for model download
- Model is cached at `~/.cache/video-subtitle-translator/models/`

### Build errors
- Ensure Wails CLI is up to date: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- Clear frontend cache: `rm -rf frontend/node_modules frontend/dist`

## License

MIT License

## Acknowledgments

- [OpenAI Whisper](https://github.com/openai/whisper) - Speech recognition model
- [llama.cpp](https://github.com/ggerganov/llama.cpp) - LLM inference engine
- [Qwen](https://github.com/QwenLM/Qwen) - Alibaba's large language model
- [Wails](https://wails.io/) - Go desktop application framework
