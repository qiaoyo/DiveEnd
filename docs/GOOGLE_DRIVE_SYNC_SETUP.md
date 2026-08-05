# Google Drive 主同步配置

DiveEnd 现在把 Google Drive 作为云同步主 provider，把百度云作为可选 fallback。两者共用同一套本地快照、manifest、冲突检测、恢复和进度逻辑，因此切换 provider 不会改变论文库的数据结构。

## 先说凭据类型

这里**不需要单独申请 Google Drive API key**。DiveEnd 同步的是你的私有 Drive 文件，使用的是：

- Google Cloud 项目中的 **OAuth 2.0 Client ID**；
- 类型选择 **Desktop app**；
- 下载的 OAuth client JSON；
- 第一次授权后，DiveEnd 本机保存的 refresh token。

代码只申请 `https://www.googleapis.com/auth/drive.file` scope。这个权限足够 DiveEnd 创建和管理自己创建的备份文件，也比完整 `drive` 权限更窄。不要把 OAuth client JSON、refresh token 或其他云端 token 提交到 Git。

## Google Cloud 配置步骤

### 1. 创建或选择项目

打开 [Google Cloud Console](https://console.cloud.google.com/)，在项目选择器中创建一个专门给 DiveEnd 使用的项目，或选择已有项目。

### 2. 启用 Drive API

进入 **APIs & Services -> Library**，搜索 **Google Drive API**，点击 **Enable**。

也可以直接打开 [Google Drive API 页面](https://console.cloud.google.com/apis/library/drive.googleapis.com)。

### 3. 配置 OAuth consent screen

进入 **Google Auth Platform**：

1. 在 **Branding / App information** 填写应用名，例如 `DiveEnd`，填写 support email 和 developer contact email。
2. 个人 Gmail 账号选择 **External**。`Internal` 只适用于 Google Workspace 组织账号。
3. 在 **Data Access / Scopes** 添加：
   `https://www.googleapis.com/auth/drive.file`
4. 在 **Audience / Test users** 添加你准备授权的 Gmail 账号。

个人使用可以先保持 Testing 状态完成开发验证。当前 Google 规则是：External + Testing 且请求了基础账号信息之外的 scope 时，refresh token 通常 7 天过期；长期使用可以把应用发布到生产状态，或在 DiveEnd 中重新点击授权。发布时若 Google 要求验证，按当前 scope 的要求处理。不要为了绕开测试状态去申请完整 Drive 权限。

官方参考：[Google Drive Go quickstart](https://developers.google.com/workspace/drive/api/quickstart/go)、[Drive API scopes](https://developers.google.com/workspace/drive/api/guides/api-specific-auth)、[OAuth 2.0 for desktop/native apps](https://developers.google.com/identity/protocols/oauth2/native-app)。

### 4. 创建 Desktop OAuth Client

进入 **Google Auth Platform -> Clients -> Create client**：

1. Application type 选择 **Desktop app**。
2. 名称填写 `DiveEnd Desktop`。
3. 创建后点击下载 JSON。

请确认下载的是 Desktop client JSON，而不是 Web application JSON。DiveEnd 使用本机 `127.0.0.1` 的随机端口完成 loopback OAuth 回调，不需要你手工填写固定 redirect URI。

### 5. 放置客户端 JSON

将下载的 JSON 放到项目本地：

```text
/Users/bytedance/DiveEnd/config/google_drive_client.json
```

这个路径已加入 `.gitignore`。也可以在 DiveEnd 的 **设置 -> 云同步** 中填写其他绝对路径。不要把 JSON 内容粘贴到 `config.json`、聊天记录或 Git 提交中。

## 在 DiveEnd 中完成授权

1. 启动桌面应用，进入 **设置 -> 云同步**。
2. 确认 Google Drive 客户端 JSON 路径已被识别。
3. 点击 **连接 Google Drive**，浏览器会打开 Google 授权页。
4. 使用第 3 步加入 Test users 的同一个 Google 账号登录并同意权限。
5. 浏览器显示授权完成后返回 DiveEnd。refresh token 默认保存到：

   ```text
   ~/Library/Application Support/DiveEnd/google_drive_token.json
   ```

6. 勾选 **启用 Google Drive 主同步**，确认 **Google Drive 失败时使用百度云 fallback** 是否开启，然后保存设置。
7. 进入 **同步** 页面执行预检或手动同步。第一次同步会在你的 My Drive 根目录创建 `DiveEnd Backup` 文件夹。

应用上传的逻辑路径仍以 `apps/pcstest_oauth/diveend-v1/` 开头，manifest 和冲突记录在 Google Drive、百度云之间保持一致；`DiveEnd Backup` 只是 Drive 中便于人工识别的容器目录。

## 运行策略

- Google Drive 已授权且预检成功：本次同步只使用 Google Drive。
- Google Drive 在本次上传开始前不可用，且开启 fallback、百度云可用：本次同步整体切换到百度云。
- 上传已经开始后不会中途把剩余文件拆到另一个 provider，避免产生半个快照在 Google、半个快照在百度的问题。
- 本次运行结束后，下一次运行仍优先尝试 Google Drive。
- Google Drive 访问令牌过期时，OAuth client 会自动用 refresh token 更新并以 `0600` 权限原子写回本机 token 文件。

## 常见问题

### `客户端文件未找到`

检查设置中的路径是否指向实际下载的 JSON；路径可填写绝对路径。不要把 `google_drive_token.json` 当成客户端 JSON。

### `redirect_uri_mismatch`

通常是创建成了 Web application client。重新创建 **Desktop app** client 并下载新的 JSON；不要手工把回调地址改成固定端口。

### `access_denied` 或应用未验证

确认授权账号已经加入 **Test users**，并且 OAuth consent screen 的 scope 包含 `drive.file`。个人开发阶段不需要申请完整 Drive 权限。

### 授权成功但几天后又要求授权

确认 OAuth 应用是否仍处于 Testing。External + Testing 的 `drive.file` refresh token 当前通常 7 天过期；个人长期使用时可以发布应用，或者在设置中重新授权。

### 想彻底换 Google 账号

在 DiveEnd 设置中点击 **移除本机授权**，然后使用新账号重新连接。Drive 中旧账号创建的备份不会自动删除。

## 发布前检查

```bash
bash scripts/secret_scan.sh
git status --short --ignored
```

下面这些文件必须保持本地 ignored：

- `config/google_drive_client.json`
- `google_drive_token.json`
- `~/Library/Application Support/DiveEnd/google_drive_token.json`
