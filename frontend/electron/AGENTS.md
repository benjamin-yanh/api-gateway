# AGENTS.md — `frontend/electron`

这里是 Electron 桌面壳，不是 Web 前端主体，也不是 Go 后端主体。

## 先看哪里

- `main.js`：主进程入口
- `preload.js`：桥接层
- `build.sh`：构建脚本
- `README.md`

## 下一跳

- 排查窗口、托盘、桌面生命周期：看 `main.js`
- 排查页面逻辑：去父目录 `frontend/`
- 排查 API 行为：去 `backend/`
