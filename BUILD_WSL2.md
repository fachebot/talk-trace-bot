# WSL2 编译指南

本指南将帮助你在 WSL2 (Ubuntu) 环境中编译 Talk Trace Bot。

## 前置准备

### 1. 安装 Go

```bash
# 下载 Go 1.24+
wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz

# 删除旧版本（如果存在）
sudo rm -rf /usr/local/go

# 解压安装
sudo tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz

# 添加到 PATH（添加到 ~/.bashrc 或 ~/.zshrc）
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# 验证安装
go version
```

### 2. 安装 TDLib 依赖

```bash
# 更新包管理器
sudo apt-get update

# 安装编译依赖
sudo apt-get install -y \
    build-essential \
    cmake \
    gperf \
    libssl-dev \
    zlib1g-dev \
    libreadline-dev \
    libc++-dev \
    libc++abi-dev \
    pkg-config
```

### 3. 编译安装 TDLib

```bash
# 克隆 TDLib 仓库
cd ~
git clone https://github.com/tdlib/td.git
cd td

# 创建构建目录
mkdir build
cd build

# 配置编译选项
cmake -DCMAKE_BUILD_TYPE=Release \
      -DCMAKE_INSTALL_PREFIX=/usr/local \
      ..

# 编译（使用多核加速）
cmake --build . -j$(nproc)

# 安装
sudo cmake --install .

# 更新动态库链接
sudo ldconfig

# 验证安装
pkg-config --modversion tdlib
```

## 编译项目

### 方法 1: 使用编译脚本（推荐）

```bash
# 进入项目目录
cd /mnt/d/Work/Trading/talk-trace-bot

# 给脚本添加执行权限
chmod +x build.sh

# 运行编译脚本
./build.sh
```

### 方法 2: 手动编译

```bash
# 进入项目目录
cd /mnt/d/Work/Trading/talk-trace-bot

# 下载依赖
go mod download

# 编译
go build -o talk-trace-bot .
```

## 验证编译结果

```bash
# 检查可执行文件
ls -lh talk-trace-bot

# 查看文件信息
file talk-trace-bot

# 测试运行（需要先配置 config.yaml）
./talk-trace-bot -f etc/config.yaml
```

## 常见问题

### 1. TDLib 找不到

如果编译时提示找不到 TDLib：

```bash
# 检查 TDLib 是否安装
pkg-config --exists tdlib && echo "TDLib 已安装" || echo "TDLib 未安装"

# 如果未安装，检查库文件位置
find /usr/local -name "libtdjson.so*" 2>/dev/null

# 手动设置库路径
export LD_LIBRARY_PATH=/usr/local/lib:$LD_LIBRARY_PATH
```

### 2. Go 版本不匹配

确保使用 Go 1.24+：

```bash
# 检查当前版本
go version

# 如果版本过低，按照上面的步骤重新安装
```

### 3. 编译错误：找不到头文件

确保 TDLib 头文件已安装：

```bash
# 检查头文件
ls /usr/local/include/td/

# 如果没有，重新安装 TDLib
```

### 4. 运行时错误：找不到动态库

```bash
# 添加到环境变量
echo 'export LD_LIBRARY_PATH=/usr/local/lib:$LD_LIBRARY_PATH' >> ~/.bashrc
source ~/.bashrc

# 或创建链接
sudo ln -s /usr/local/lib/libtdjson.so /usr/lib/libtdjson.so
sudo ldconfig
```

## 快速安装脚本

如果你想一键安装所有依赖，可以使用以下脚本：

```bash
#!/bin/bash
# install_deps.sh

set -e

echo "📦 安装系统依赖..."
sudo apt-get update
sudo apt-get install -y \
    build-essential \
    cmake \
    gperf \
    libssl-dev \
    zlib1g-dev \
    libreadline-dev \
    libc++-dev \
    libc++abi-dev \
    pkg-config \
    wget \
    git

echo "🔧 安装 Go..."
if ! command -v go &> /dev/null; then
    wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    export PATH=$PATH:/usr/local/go/bin
    rm go1.24.0.linux-amd64.tar.gz
    echo "✅ Go 安装完成"
else
    echo "✅ Go 已安装: $(go version)"
fi

echo "📚 安装 TDLib..."
if ! pkg-config --exists tdlib; then
    cd ~
    if [ ! -d "td" ]; then
        git clone https://github.com/tdlib/td.git
    fi
    cd td
    mkdir -p build
    cd build
    cmake -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=/usr/local ..
    cmake --build . -j$(nproc)
    sudo cmake --install .
    sudo ldconfig
    echo "✅ TDLib 安装完成"
else
    echo "✅ TDLib 已安装"
fi

echo "🎉 所有依赖安装完成！"
```

保存为 `install_deps.sh`，然后运行：

```bash
chmod +x install_deps.sh
./install_deps.sh
```

## 下一步

安装完依赖后，按照上面的编译步骤编译项目，然后参考主 README.md 配置和运行 Bot。
