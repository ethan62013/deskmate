# 桌面宠物

常驻在 Windows 托盘里的三维桌面宠物（猫 / 狗 / 鹦鹉）。Go + Wails v3 做外壳，Vue 3 + Three.js 画宠物。

## 环境

- Go 1.26 或更新
- Node 20 或更新（用 nvm 时：`nvm use 22`）
- Wails 命令行：`go install github.com/wailsapp/wails/v3/cmd/wails3@latest`

`goproxy.cn` 连不上时，可以临时改用官方代理：`$env:GOPROXY = "https://proxy.golang.org,direct"`。

## 运行

```powershell
wails3 dev      # 开发模式，前后端改动自动重载
wails3 build    # 打包到 bin\pcserve.exe
```

修改了 `petservice.go` 里的导出方法后，要重新生成前端绑定：`wails3 generate bindings -ts`。

## 宠物模型

不放模型时，程序用代码拼出来的卡通猫、狗、鹦鹉。想换成正式模型，把 glTF 文件放到：

```text
frontend/public/models/cat.glb
frontend/public/models/dog.glb
frontend/public/models/parrot.glb
```

动画片段按名字匹配：`idle`、`walk`、`sit`、`sleep`、`jump`、`call`。[Vuena 的免费卡通动物](https://vuena.itch.io/blocky-pets-free)可以直接用。

自定义叫声放在 `frontend/public/sounds/<宠物>/<happy|talk|remind>.mp3`，没有时用程序合成的声音。

## 配置

保存在 `%APPDATA%\pcserve\config.json`，包括宠物种类、名字、窗口位置、模型接口、语音和 Key。

## 语音与情侣模式

- 语音使用火山引擎豆包语音合成（`/api/v3/tts/unidirectional`）。在控制台开通语音合成，新版控制台只需 API Key；旧版控制台填 App ID + Access Token。音色 ID 在控制台音色库查看，模型版本要和音色所属版本（2.0 / 1.0）一致。
- 情侣模式在设置页打开，可以自定义称呼和人设；人设会作为系统提示词发给对话模型。
