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
