# DiskDataKit Makefile
# 跨平台构建 / 图标嵌入 / 清理
#
# 用法:
#   make            # 当前平台构建（自动检测 OS）
#   make windows    # Windows 构建（含图标，无控制台窗口）
#   make linux      # Linux 构建
#   make darwin     # macOS 构建
#   make icon       # 重新生成 rsrc.syso（需 rsrc 工具）
#   make clean      # 清理构建产物
#   make run        # 构建并运行

BINARY   = DiskDataKit
VERSION  = 1.0.0
LDFLAGS  = -s -w
GOPATH   = $(shell go env GOPATH)

# 按平台设置输出名和额外标志
ifeq ($(OS),Windows_NT)
    EXE      = $(BINARY).exe
    GOOS     = windows
    GUI_FLAG = -H windowsgui
    LDFLAGS += $(GUI_FLAG)
else
    EXE      = $(BINARY)
    GOOS     = $(shell go env GOOS)
endif

# 平台目标
.PHONY: all windows linux darwin icon clean run fmt vet

all: $(EXE)

$(EXE):
	go build -ldflags "$(LDFLAGS)" -o $(EXE) .

# Windows 构建（含图标 + 无控制台窗口）
windows:
	go build -ldflags "$(LDFLAGS) -H windowsgui" -o $(BINARY).exe .

# Linux 构建
linux:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-amd64 .

# macOS 构建 
darwin:
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-darwin-amd64 .
	@echo "如需 ARM64: make darwin-arm64"

# macOS ARM64
darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)-darwin-arm64 .

# 生成 Windows 图标资源（rsrc.syso）
# 依赖: go install github.com/akavel/rsrc@latest
icon:
	$(GOPATH)/bin/rsrc -ico icon.ico -o rsrc_windows_amd64.syso
	@echo "rsrc.syso 已生成，go build 会自动链接"

# 构建并运行
run: $(EXE)
	./$(EXE)

# 格式化
fmt:
	go fmt ./...

# 静态检查
vet:
	go vet ./...

# 清理
clean:
	rm -f $(BINARY) $(BINARY).exe $(BINARY)-linux-amd64 $(BINARY)-darwin-amd64 $(BINARY)-darwin-arm64
