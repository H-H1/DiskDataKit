// AI 聊天模块（VSCode 风格侧边栏）
const chatSidebar = document.getElementById('chatSidebar');
const chatMessages = document.getElementById('chatMessages');
const chatInput = document.getElementById('chatInput');
const chatSendBtn = document.getElementById('chatSendBtn');
const chatStatus = document.getElementById('chatStatus');
const chatToggleHeaderBtn = document.getElementById('chatToggleHeaderBtn');
const chatCloseBtn = document.getElementById('chatCloseBtn');
const chatNewBtn = document.getElementById('chatNewBtn');
const chatProviderSelect = document.getElementById('chatProviderSelect');

let chatSessionID = 'session-' + Date.now();
let chatSending = false;

// 侧边栏开关
function toggleChat(open) {
    if (open === undefined) {
        open = !chatSidebar.classList.contains('open');
    }
    chatSidebar.classList.toggle('open', open);
    document.body.classList.toggle('chat-open', open);
    if (open) {
        setTimeout(() => chatInput.focus(), 300);
    }
}

chatToggleHeaderBtn.addEventListener('click', () => toggleChat());
chatCloseBtn.addEventListener('click', () => toggleChat(false));

// 初始化：加载厂商列表 + 检查配置状态
async function initChat() {
    try {
        const resp = await fetch('/api/chat/config');
        const data = await resp.json();

        if (!data.configured) {
            chatStatus.textContent = '未配置';
            chatStatus.classList.add('not-configured');
            return;
        }

        // 填充厂商下拉框
        chatProviderSelect.innerHTML = '';
        if (data.providers) {
            for (const p of data.providers) {
                const opt = document.createElement('option');
                opt.value = p.name;
                opt.textContent = p.limited ? p.name + ' (限时)' : p.name;
                if (p.name === data.currentProvider) opt.selected = true;
                chatProviderSelect.appendChild(opt);
            }
        }

        // 更新状态
        const cur = data.providers && data.providers.find(p => p.name === data.currentProvider);
        chatStatus.textContent = data.currentProvider || '已配置';
        chatStatus.classList.toggle('limited', !!(cur && cur.limited));
    } catch (e) {
        // 忽略
    }
}

// 切换模型时更新状态显示
chatProviderSelect.addEventListener('change', () => {
    const opt = chatProviderSelect.options[chatProviderSelect.selectedIndex];
    const isLimited = opt && opt.textContent.includes('限时');
    chatStatus.textContent = chatProviderSelect.value;
    chatStatus.classList.toggle('limited', isLimited);
});

// 发送消息
async function sendChatMessage() {
    const text = chatInput.value.trim();
    if (!text || chatSending) return;

    chatSending = true;
    chatSendBtn.disabled = true;
    chatInput.value = '';
    autoResizeInput();

    // 移除欢迎消息
    const welcome = chatMessages.querySelector('.chat-welcome');
    if (welcome) welcome.remove();

    // 添加用户消息
    addMessage('user', text);

    // 添加 AI 回复占位
    const aiEl = addMessage('assistant', '');
    const contentEl = aiEl.querySelector('.chat-msg-content');

    // 显示打字指示器
    const typing = document.createElement('div');
    typing.className = 'chat-typing';
    typing.innerHTML = '<span></span><span></span><span></span>';
    contentEl.appendChild(typing);

    try {
        const resp = await fetch('/api/chat', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                message: text,
                sessionID: chatSessionID,
                model: chatProviderSelect.value || ''
            })
        });

        typing.remove();

        if (!resp.ok) {
            const err = await resp.json();
            addMessage('error', err.error || '请求失败');
            return;
        }

        // 读取 SSE 流
        const reader = resp.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        let aiText = '';

        while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            buffer += decoder.decode(value, { stream: true });

            const lines = buffer.split('\n');
            buffer = lines.pop() || '';

            for (const line of lines) {
                if (!line.startsWith('data: ')) continue;
                const data = line.slice(6).trim();
                if (data === '[DONE]') continue;

                try {
                    const chunk = JSON.parse(data);
                    if (chunk.error) {
                        contentEl.innerHTML = '';
                        addMessage('error', chunk.error);
                        return;
                    }
                    if (chunk.choices && chunk.choices[0] && chunk.choices[0].delta) {
                        const delta = chunk.choices[0].delta.content || '';
                        if (delta) {
                            aiText += delta;
                            contentEl.innerHTML = renderMarkdown(aiText);
                            scrollToBottom();
                        }
                    }
                } catch (e) {
                    // 跳过无法解析的行
                }
            }
        }

        if (!aiText) {
            contentEl.innerHTML = '<span style="color:var(--fg-muted)">（无回复内容）</span>';
        }
    } catch (err) {
        typing.remove();
        contentEl.innerHTML = '';
        addMessage('error', '网络错误: ' + err.message);
    } finally {
        chatSending = false;
        chatSendBtn.disabled = false;
        chatInput.focus();
    }
}

// 添加消息（VSCode 风格：头像 + 角色标签 + 内容）
function addMessage(role, content) {
    const el = document.createElement('div');
    el.className = 'chat-msg ' + role;

    // 头像
    const avatar = document.createElement('div');
    avatar.className = 'chat-msg-avatar';
    avatar.textContent = role === 'user' ? '🧑' : role === 'error' ? '⚠' : '🤖';

    // 消息体
    const body = document.createElement('div');
    body.className = 'chat-msg-body';

    // 角色标签
    const roleLabel = document.createElement('div');
    roleLabel.className = 'chat-msg-role';
    roleLabel.textContent = role === 'user' ? 'You' : role === 'error' ? 'Error' : 'Assistant';

    // 内容
    const contentEl = document.createElement('div');
    contentEl.className = 'chat-msg-content';
    if (content) {
        contentEl.innerHTML = role === 'user' ? escapeHtml(content) : renderMarkdown(content);
    }

    body.appendChild(roleLabel);
    body.appendChild(contentEl);
    el.appendChild(avatar);
    el.appendChild(body);
    chatMessages.appendChild(el);
    scrollToBottom();

    return el;
}

// 滚动到底部
function scrollToBottom() {
    chatMessages.scrollTop = chatMessages.scrollHeight;
}

// 简单 Markdown 渲染
function renderMarkdown(text) {
    let html = escapeHtml(text);

    // 代码块
    html = html.replace(/```(\w*)\n?([\s\S]*?)```/g, (_, lang, code) => {
        return '<pre><code>' + code.trim() + '</code></pre>';
    });

    // 行内代码
    html = html.replace(/`([^`]+)`/g, '<code>$1</code>');

    // 加粗
    html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');

    // 无序列表
    html = html.replace(/^[-*] (.+)$/gm, '<li>$1</li>');
    html = html.replace(/(<li>[\s\S]*?<\/li>)/g, '<ul>$1</ul>');

    // 换行
    html = html.replace(/\n/g, '<br>');

    return html;
}

// HTML 转义
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// 自动调整输入框高度
function autoResizeInput() {
    chatInput.style.height = 'auto';
    chatInput.style.height = Math.min(chatInput.scrollHeight, 120) + 'px';
}

// 事件绑定
chatSendBtn.addEventListener('click', sendChatMessage);

chatInput.addEventListener('keydown', e => {
    if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        sendChatMessage();
    }
});

chatInput.addEventListener('input', autoResizeInput);

// 新对话
chatNewBtn.addEventListener('click', async () => {
    try {
        await fetch('/api/chat/clear?sessionID=' + encodeURIComponent(chatSessionID), { method: 'POST' });
    } catch (e) {
        // 忽略
    }
    chatMessages.innerHTML = `
        <div class="chat-welcome">
            <div class="chat-welcome-icon">🤖</div>
            <p>新对话已开始</p>
            <p class="chat-welcome-hint">上下文记忆已清除</p>
        </div>`;
    chatSessionID = 'session-' + Date.now();
    chatInput.focus();
});

// 初始化
initChat();
toggleChat(true);

// ==================== 引用给AI ====================

// 引用文件/目录给 AI：目录则获取文件列表，文件则直接引用路径
async function referenceToAI(path, isDir, name) {
    // 确保 AI 侧边栏打开
    toggleChat(true);

    // 移除欢迎消息
    const welcome = chatMessages.querySelector('.chat-welcome');
    if (welcome) welcome.remove();

    if (isDir) {
        // 目录：获取文件列表，构建 tree
        chatInput.value = `正在获取目录信息...`;
        autoResizeInput();

        try {
            const resp = await fetch('/api/files?path=' + encodeURIComponent(path));
            const data = await resp.json();

            if (data.error) {
                chatInput.value = `引用目录失败: ${data.error}`;
                autoResizeInput();
                return;
            }

            const items = data.items || [];
            const dirs = items.filter(i => i.isDir);
            const files = items.filter(i => !i.isDir);

            let tree = `请分析以下目录:\n\n路径: ${path}\n`;
            tree += `共 ${items.length} 项（${dirs.length} 个文件夹 · ${files.length} 个文件）\n\n文件列表:\n`;

            // 先列文件夹，再列文件，各按名称排序
            dirs.sort((a, b) => a.name.localeCompare(b.name));
            files.sort((a, b) => a.name.localeCompare(b.name));

            for (const d of dirs) {
                tree += `  📁 ${d.name}/\n`;
            }
            for (const f of files) {
                tree += `  📄 ${f.name} (${formatSize(f.size)})\n`;
            }

            chatInput.value = tree;
        } catch (e) {
            chatInput.value = `引用目录失败: ${e.message}`;
        }
    } else {
        // 单文件：直接引用路径
        chatInput.value = `请分析以下文件:\n\n路径: ${path}\n名称: ${name || path.split(/[\\/]/).pop()}`;
    }

    autoResizeInput();
    chatInput.focus();
    // 将光标移到末尾
    chatInput.setSelectionRange(chatInput.value.length, chatInput.value.length);
}

// ==================== 自定义模型弹窗 ====================

const customModelModal = document.getElementById('customModelModal');
const chatCustomModelBtn = document.getElementById('chatCustomModelBtn');
const customModelClose = document.getElementById('customModelClose');
const cmCancelBtn = document.getElementById('cmCancelBtn');
const cmProviderSelect = document.getElementById('cmProviderSelect');
const cmProviderDesc = document.getElementById('cmProviderDesc');
const cmApiKey = document.getElementById('cmApiKey');
const cmFetchModelsBtn = document.getElementById('cmFetchModelsBtn');
const cmModelList = document.getElementById('cmModelList');
const cmModelCount = document.getElementById('cmModelCount');
const cmStatus = document.getElementById('cmStatus');
const cmSaveBtn = document.getElementById('cmSaveBtn');

let cmProviders = [];      // 厂商列表缓存
let cmFetchedModels = [];  // 已获取的模型列表

// 打开弹窗
chatCustomModelBtn.addEventListener('click', async () => {
    customModelModal.classList.remove('hidden');
    // 加载厂商列表
    if (cmProviders.length === 0) {
        try {
            const resp = await fetch('/api/chat/providers');
            const data = await resp.json();
            cmProviders = data.providers || [];
            cmProviderSelect.innerHTML = '<option value="">请选择厂商...</option>';
            for (const p of cmProviders) {
                const opt = document.createElement('option');
                opt.value = p.id;
                opt.textContent = `${p.name} · ${p.region}`;
                cmProviderSelect.appendChild(opt);
            }
        } catch (e) {
            showCmStatus('加载厂商列表失败: ' + e.message, 'error');
        }
    }
    // 重置状态
    cmApiKey.value = '';
    cmModelList.innerHTML = '<div class="custom-model-empty">请先获取模型列表</div>';
    cmModelCount.textContent = '';
    cmFetchedModels = [];
    cmSaveBtn.disabled = true;
    hideCmStatus();
});

// 关闭弹窗
customModelClose.addEventListener('click', () => customModelModal.classList.add('hidden'));
cmCancelBtn.addEventListener('click', () => customModelModal.classList.add('hidden'));

// 点击遮罩关闭
customModelModal.addEventListener('click', e => {
    if (e.target === customModelModal) customModelModal.classList.add('hidden');
});

// 厂商选择变化时显示描述
cmProviderSelect.addEventListener('change', () => {
    const provider = cmProviders.find(p => p.id === cmProviderSelect.value);
    if (provider) {
        cmProviderDesc.innerHTML = `${provider.desc} · <a href="https://${provider.website}" target="_blank">${provider.website}</a>`;
    } else {
        cmProviderDesc.textContent = '';
    }
});

// 获取模型列表
cmFetchModelsBtn.addEventListener('click', async () => {
    const provider = cmProviderSelect.value;
    const apiKey = cmApiKey.value.trim();

    if (!provider) {
        showCmStatus('请先选择厂商', 'error');
        return;
    }
    if (!apiKey) {
        showCmStatus('请输入 API Key', 'error');
        return;
    }

    cmFetchModelsBtn.disabled = true;
    cmFetchModelsBtn.textContent = '获取中...';
    showCmStatus('正在请求模型列表...', 'loading');

    try {
        const resp = await fetch('/api/chat/models', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ provider, api_key: apiKey })
        });
        const data = await resp.json();

        if (data.error) {
            showCmStatus(data.error, 'error');
            cmModelList.innerHTML = '<div class="custom-model-empty">获取失败，请检查 API Key</div>';
            cmFetchedModels = [];
            cmModelCount.textContent = '';
            cmSaveBtn.disabled = true;
            return;
        }

        cmFetchedModels = data.models || [];
        renderCmModelList();
        hideCmStatus();

        if (cmFetchedModels.length === 0) {
            showCmStatus('该厂商未返回任何模型', 'error');
        }
    } catch (e) {
        showCmStatus('请求失败: ' + e.message, 'error');
        cmFetchedModels = [];
        cmModelCount.textContent = '';
        cmSaveBtn.disabled = true;
    } finally {
        cmFetchModelsBtn.disabled = false;
        cmFetchModelsBtn.textContent = '获取模型';
    }
});

// 渲染模型列表（多选 checkbox）
function renderCmModelList() {
    if (cmFetchedModels.length === 0) {
        cmModelList.innerHTML = '<div class="custom-model-empty">无可用模型</div>';
        cmModelCount.textContent = '';
        cmSaveBtn.disabled = true;
        return;
    }

    cmModelCount.textContent = `(${cmFetchedModels.length} 个)`;
    cmModelList.innerHTML = '';

    for (const model of cmFetchedModels) {
        const label = document.createElement('label');
        label.className = 'custom-model-item';

        const checkbox = document.createElement('input');
        checkbox.type = 'checkbox';
        checkbox.value = model;
        checkbox.addEventListener('change', updateCmSaveBtn);

        const span = document.createElement('span');
        span.className = 'custom-model-item-label';
        span.textContent = model;

        label.appendChild(checkbox);
        label.appendChild(span);
        cmModelList.appendChild(label);
    }

    // 默认选中第一个
    const firstCheckbox = cmModelList.querySelector('input[type="checkbox"]');
    if (firstCheckbox) firstCheckbox.checked = true;
    updateCmSaveBtn();
}

// 更新保存按钮状态
function updateCmSaveBtn() {
    const checked = cmModelList.querySelectorAll('input[type="checkbox"]:checked');
    cmSaveBtn.disabled = checked.length === 0;
}

// 保存配置
cmSaveBtn.addEventListener('click', async () => {
    const provider = cmProviderSelect.value;
    const apiKey = cmApiKey.value.trim();
    const selectedModels = Array.from(cmModelList.querySelectorAll('input[type="checkbox"]:checked'))
        .map(cb => cb.value);

    if (!provider || !apiKey || selectedModels.length === 0) {
        showCmStatus('请填写完整信息', 'error');
        return;
    }

    cmSaveBtn.disabled = true;
    cmSaveBtn.textContent = '保存中...';
    showCmStatus('正在保存配置...', 'loading');

    try {
        const resp = await fetch('/api/chat/config/save', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                provider,
                api_key: apiKey,
                models: selectedModels
            })
        });
        const data = await resp.json();

        if (data.error) {
            showCmStatus(data.error, 'error');
            cmSaveBtn.disabled = false;
            cmSaveBtn.textContent = '保存';
            return;
        }

        showCmStatus('配置已保存，正在重新加载...', 'success');

        // 重新初始化聊天界面
        setTimeout(async () => {
            await initChat();
            customModelModal.classList.add('hidden');
            cmSaveBtn.disabled = false;
            cmSaveBtn.textContent = '保存';
        }, 800);

    } catch (e) {
        showCmStatus('保存失败: ' + e.message, 'error');
        cmSaveBtn.disabled = false;
        cmSaveBtn.textContent = '保存';
    }
});

// 状态提示辅助函数
function showCmStatus(msg, type) {
    cmStatus.textContent = msg;
    cmStatus.className = 'custom-model-status show ' + type;
}

function hideCmStatus() {
    cmStatus.className = 'custom-model-status';
}
