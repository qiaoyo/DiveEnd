import json
import os
from hashlib import md5
from urllib.parse import quote

import requests

# ================================================================
#  配置
# ================================================================
TOKEN_FILE = "baiduyun_token.json"
APP_NAME = "pcstest_oauth"
XPAN = "https://pan.baidu.com/rest/2.0/xpan"
PCS = "https://d.pcs.baidu.com/rest/2.0/pcs/superfile2"
BLOCK = 4 * 1024 * 1024  # 4MB


# ================================================================
#  Token 管理（自动读取、自动刷新、自动保存）
# ================================================================
def load_token():
    with open(TOKEN_FILE, "r") as f:
        return json.load(f)


def save_token(data):
    with open(TOKEN_FILE, "w") as f:
        json.dump(data, f, indent=2, ensure_ascii=False)


def get_access_token():
    """获取有效的 access_token，过期自动用 refresh_token 刷新"""
    data = load_token()

    # 检查当前 token 是否有效
    resp = requests.get(
        "https://pan.baidu.com/api/quota",
        params={
            "access_token": data["access_token"],
            "checkexpire": 1,
        },
    ).json()

    if resp.get("errno") == 0 and not resp.get("expire"):
        print("[Token] 有效")
        return data["access_token"]

    # 过期了，用 refresh_token 换新的
    print("[Token] 已过期，正在刷新...")
    resp = requests.get(
        "https://openapi.baidu.com/oauth/2.0/token",
        params={
            "grant_type": "refresh_token",
            "refresh_token": data["refresh_token"],
            "client_id": data["client_id"],
            "client_secret": data["client_secret"],
        },
    ).json()

    if "access_token" not in resp:
        raise Exception(f"刷新失败: {resp}")

    # 新的 access_token 和 refresh_token 都要存
    data["access_token"] = resp["access_token"]
    data["refresh_token"] = resp["refresh_token"]
    save_token(data)
    print(f"[Token] 刷新成功，已保存到 {TOKEN_FILE}")
    return data["access_token"]


# 启动时自动获取
ACCESS_TOKEN = get_access_token()


# ================================================================
#  上传
# ================================================================
def upload(local_path, remote_name=None):
    """
    上传本地文件到百度网盘
    :param local_path:  本地文件路径
    :param remote_name: 网盘文件名（默认和本地同名）
    """
    if remote_name is None:
        remote_name = os.path.basename(local_path)
    remote_path = f"/apps/{APP_NAME}/{remote_name}"
    file_size = os.path.getsize(local_path)

    # 1. 读取所有分片并计算 MD5
    chunks = []
    block_list = []
    with open(local_path, "rb") as f:
        while True:
            chunk = f.read(BLOCK)
            if not chunk:
                break
            chunks.append(chunk)
            block_list.append(md5(chunk).hexdigest())
    print(f"[上传] {local_path} ({file_size}B) -> {remote_path}, 分片数: {len(chunks)}")

    # 2. 预上传
    resp = requests.post(
        f"{XPAN}/file?method=precreate&access_token={ACCESS_TOKEN}",
        data={
            "path": remote_path,
            "size": file_size,
            "isdir": 0,
            "autoinit": 1,
            "block_list": json.dumps(block_list),
            "rtype": 3,
        },
    ).json()
    if resp.get("errno") != 0:
        print(f"[预上传失败] {resp}")
        return
    uploadid = resp["uploadid"]
    print(f"[预上传] OK, uploadid={uploadid}")

    # 3. 分片上传（POST + multipart，官方写法）
    for i, chunk in enumerate(chunks):
        url = (
            f"{PCS}?method=upload"
            f"&access_token={ACCESS_TOKEN}"
            f"&type=tmpfile"
            f"&path={quote(remote_path, safe='')}"
            f"&uploadid={uploadid}"
            f"&partseq={i}"
        )
        r = requests.post(url, files={"file": ("chunk", chunk)}).json()
        match = r.get("md5") == block_list[i]
        print(f"[分片{i}] md5匹配={match}")
        if not match:
            print(f"  本地: {block_list[i]}, 服务端: {r.get('md5')}")

    # 4. 合并创建文件
    result = requests.post(
        f"{XPAN}/file?method=create&access_token={ACCESS_TOKEN}",
        data={
            "path": remote_path,
            "size": file_size,
            "isdir": 0,
            "uploadid": uploadid,
            "block_list": json.dumps(block_list),
            "rtype": 3,
        },
    ).json()

    if result.get("fs_id"):
        print(f"[上传成功] path={result['path']}, fs_id={result['fs_id']}")
    else:
        print(
            f"[上传失败] errno={result.get('errno')}, msg={result.get('show_msg', '')}"
        )
    return result


# ================================================================
#  下载
# ================================================================
def download(remote_path, local_path=None):
    """
    从百度网盘下载文件
    :param remote_path: 网盘文件路径
    :param local_path:  本地保存路径（默认当前目录同名文件）
    """
    if local_path is None:
        local_path = os.path.basename(remote_path)

    # 1. 列目录拿 fs_id
    dir_name = os.path.dirname(remote_path)
    file_list = (
        requests.get(
            f"{XPAN}/file?method=list&access_token={ACCESS_TOKEN}",
            params={"dir": dir_name, "limit": 1000},
        )
        .json()
        .get("list", [])
    )

    fs_id = next((f["fs_id"] for f in file_list if f["path"] == remote_path), None)
    if not fs_id:
        print(f"[错误] 找不到: {remote_path}")
        return

    # 2. 获取下载链接
    meta = requests.get(
        f"{XPAN}/multimedia?method=filemetas&access_token={ACCESS_TOKEN}",
        params={"fsids": json.dumps([fs_id]), "dlink": 1},
    ).json()["list"][0]
    dlink = meta["dlink"]

    # 3. 下载（必须带 User-Agent）
    resp = requests.get(
        dlink,
        params={"access_token": ACCESS_TOKEN},
        headers={"User-Agent": "pan.baidu.com"},
        stream=True,
    )
    with open(local_path, "wb") as f:
        for chunk in resp.iter_content(1024 * 1024):
            f.write(chunk)
    print(f"[下载完成] -> {local_path} ({os.path.getsize(local_path)} bytes)")


# ================================================================
#  手动刷新 Token（也可以单独调用）
# ================================================================
def refresh_token_now():
    """手动触发 token 刷新"""
    data = load_token()
    resp = requests.get(
        "https://openapi.baidu.com/oauth/2.0/token",
        params={
            "grant_type": "refresh_token",
            "refresh_token": data["refresh_token"],
            "client_id": data["client_id"],
            "client_secret": data["client_secret"],
        },
    ).json()
    if "access_token" not in resp:
        print(f"[刷新失败] {resp}")
        return
    data["access_token"] = resp["access_token"]
    data["refresh_token"] = resp["refresh_token"]
    save_token(data)
    print(f"[刷新成功] 新 token 已保存")
    print(f"  access_token:  {resp['access_token'][:30]}...")
    print(f"  refresh_token: {resp['refresh_token'][:30]}...")
    print(f"  有效期: {resp['expires_in']} 秒 ({resp['expires_in'] // 86400} 天)")


# ================================================================
#  测试
# ================================================================
if __name__ == "__main__":
    import sys

    # python baidu_pan.py refresh  -> 手动刷新 token
    if len(sys.argv) > 1 and sys.argv[1] == "refresh":
        refresh_token_now()
        sys.exit(0)

    # 默认：上传 + 下载测试
    with open("test.txt", "w") as f:
        f.write("hello baidu pan!")

    upload("test.txt")
    print("=" * 50)
    download(f"/apps/{APP_NAME}/test.txt", "downloaded.txt")
