# 百度开放平台 OAuth 获取 access_token & refresh_token 操作文档
## 一、前置准备
1. 登录 [百度开放平台控制台](https://openapi.baidu.com/)，进入已创建应用
2. 保存应用凭证：
   - `client_id`：API Key
   - `client_secret`：Secret Key
3. 应用授权回调地址配置为：`oob`（本地调试专用）

## 二、获取临时授权码 Code
### 1. 授权链接
https://openapi.baidu.com/oauth/2.0/authorize?response_type=code&client_id=你的 APIKey&redirect_uri=oob&scope=basic


### 2. 操作步骤
1. 浏览器打开链接，登录百度账号并授权
2. 页面返回一次性授权码 `code`
> 注意：`code` 只能使用一次，兑换 Token 后立即失效

## 三、Code 换取 access_token、refresh_token
### 接口地址
https://openapi.baidu.com/oauth/2.0/token

### 请求参数
| 参数 | 参数值说明 |
| ---- | ---- |
| grant_type | `authorization_code` |
| client_id | 应用 API Key |
| client_secret | 应用 Secret Key |
| code | 上一步获取的临时授权码 |
| redirect_uri | `oob`（必须与授权链接一致） |

### CURL 请求示例
```bash
curl "https://openapi.baidu.com/oauth/2.0/token?grant_type=authorization_code&client_id=你的APIKey&client_secret=你的SecretKey&code=授权码&redirect_uri=oob"
```

{
    "access_token": "xxxx",
    "refresh_token": "xxxx",
    "expires_in": 2592000,
    "scope": "basic"
}


https://openapi.baidu.com/oauth/2.0/token
curl 刷新命令
curl "https://openapi.baidu.com/oauth/2.0/token?grant_type=refresh_token&client_id=你的APIKey&client_secret=你的SecretKey&refresh_token=你的refresh_token&redirect_uri=oob"


五、重要注意事项
临时授权码 code 单次有效，授权失败 / 已使用需重新获取
client_secret、refresh_token 属于敏感密钥，禁止明文上传至公开仓库
access_token 有效期 30 天，过期需通过 refresh_token 刷新获取新令牌
授权与刷新接口中回调地址必须保持一致，否则会授权异常