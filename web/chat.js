// AI 聊天模块（VSCode 风格侧边栏）
const chatSidebar = document.getElementById('chatSidebar');
const chatMessages = document.getElementById('chatMessages');
const chatInput = document.getElementById('chatInput');
const chatSendBtn = document.getElementById('chatSendBtn');
const chatStatus = document.getElementById('chatStatus');
const chatToggleHeaderBtn = document.getElementById('chatToggleHeaderBtn');
const chatCloseBtn = document.getElementById('chatCloseBtn');
const chatNewBtn = document.getElementById('chatNewBtn');
const chatModelTrigger = document.getElementById('chatModelTrigger');
const chatModelLabel = document.getElementById('chatModelLabel');
const chatModelIcon = document.getElementById('chatModelIcon');
const chatModelOptions = document.getElementById('chatModelOptions');
const chatModelDropdown = document.getElementById('chatModelDropdown');

// 顶栏模型切换
const headerModelTrigger = document.getElementById('headerModelTrigger');
const headerModelLabel = document.getElementById('headerModelLabel');
const headerModelIcon = document.getElementById('headerModelIcon');
const headerModelOptions = document.getElementById('headerModelOptions');
const headerModelDropdown = document.getElementById('headerModelDropdown');
const headerCustomModelBtn = document.getElementById('headerCustomModelBtn');

let chatSessionID = 'session-' + Date.now();
let chatSending = false;
let chatCurrentModel = '';

// 厂商图标映射 — 统一暖金色调
const providerIcons = {
    deepseek:   { letter: 'D', color: '#c8a45c', bg: 'rgba(200, 164, 92, 0.15)' },
    alibaba:    { letter: 'Q', color: '#d4b06a', bg: 'rgba(212, 176, 106, 0.15)' },
    bytedance:  { letter: 'B', color: '#b8954a', bg: 'rgba(184, 149, 74, 0.15)' },
    zhipu:      { letter: 'Z', color: '#d4b06a', bg: 'rgba(212, 176, 106, 0.15)' },
    minimax:    { letter: 'M', color: '#b8954a', bg: 'rgba(184, 149, 74, 0.15)' },
    xiaomi:     { letter: 'X', color: '#d4b06a', bg: 'rgba(212, 176, 106, 0.15)' },
    unknown:    { letter: 'AI', color: '#c8a45c', bg: 'rgba(200, 164, 92, 0.15)' },
};

function getProviderByBaseURL(baseURL) {
    if (!baseURL) return 'unknown';
    const u = baseURL.toLowerCase();
    if (u.includes('deepseek')) return 'deepseek';
    if (u.includes('dashscope') || u.includes('aliyun')) return 'alibaba';
    if (u.includes('volces') || u.includes('ark.cn')) return 'bytedance';
    if (u.includes('bigmodel')) return 'zhipu';
    if (u.includes('minimaxi')) return 'minimax';
    if (u.includes('xiaomimimo')) return 'xiaomi';
    return 'unknown';
}

function getProviderByModelName(name) {
    const n = name.toLowerCase();
    if (n.startsWith('deepseek')) return 'deepseek';
    if (n.startsWith('qwen') || n.startsWith('chatglm')) return 'alibaba';
    if (n.startsWith('doubao')) return 'bytedance';
    if (n.startsWith('glm')) return 'zhipu';
    if (n.startsWith('minimax') || n.startsWith('abab')) return 'minimax';
    if (n.startsWith('mimo')) return 'xiaomi';
    return 'unknown';
}

function getProviderIcon(name, baseURL) {
    let key = getProviderByBaseURL(baseURL);
    if (key === 'unknown') key = getProviderByModelName(name);
    return providerIcons[key] || providerIcons.unknown;
}

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

// 初始化：加载模型列表 + 检查配置状态
async function initChat() {
    try {
        const resp = await fetch('/api/chat/config');
        const data = await resp.json();

        if (!data.configured) {
            chatStatus.textContent = '未配置';
            chatStatus.classList.add('not-configured');
            chatModelLabel.textContent = '未配置';
            return;
        }

        // 构建自定义下拉列表
        chatModelOptions.innerHTML = '';
        headerModelOptions.innerHTML = '';
        let currentProvider = null;
        if (data.providers) {
            for (const p of data.providers) {
                const icon = getProviderIcon(p.name, p.baseURL);
                const item = document.createElement('div');
                item.className = 'chat-model-option' + (p.name === data.currentProvider ? ' active' : '');
                item.dataset.value = p.name;
                item.dataset.limited = p.limited ? '1' : '';
                item.innerHTML = `
                    <span class="chat-model-option-icon" style="background:${icon.bg};color:${icon.color}">${icon.letter}</span>
                    <span class="chat-model-option-label">${p.name}${p.limited ? ' <span class="chat-model-limited">限时</span>' : ''}</span>
                `;
                item.addEventListener('click', () => {
                    selectModel(p.name, p.limited);
                });
                chatModelOptions.appendChild(item);
                // 顶栏下拉同步
                const hItem = item.cloneNode(true);
                hItem.addEventListener('click', () => {
                    selectModel(p.name, p.limited);
                });
                headerModelOptions.appendChild(hItem);
                if (p.name === data.currentProvider) currentProvider = p;
            }
        }

        // 设置当前选中
        if (currentProvider) {
            chatCurrentModel = currentProvider.name;
            const labelText = currentProvider.name + (currentProvider.limited ? ' (限时)' : '');
            chatModelLabel.textContent = labelText;
            const icon = getProviderIcon(currentProvider.name, currentProvider.baseURL);
            chatModelIcon.textContent = icon.letter;
            chatModelIcon.style.background = icon.bg;
            chatModelIcon.style.color = icon.color;
            chatStatus.textContent = currentProvider.name;
            chatStatus.classList.toggle('limited', !!currentProvider.limited);
            // 顶栏同步
            headerModelLabel.textContent = labelText;
            headerModelIcon.textContent = icon.letter;
            headerModelIcon.style.background = icon.bg;
            headerModelIcon.style.color = icon.color;
        } else if (!data.configured) {
            headerModelLabel.textContent = '未配置';
        }
    } catch (e) {
        // 忽略
    }
}

// 选择模型
function selectModel(name, limited) {
    chatCurrentModel = name;
    const labelText = name + (limited ? ' (限时)' : '');
    const selectedItem = chatModelOptions.querySelector(`.chat-model-option[data-value="${name}"]`);
    if (selectedItem) {
        const iconEl = selectedItem.querySelector('.chat-model-option-icon');
        chatModelLabel.textContent = labelText;
        chatModelIcon.textContent = iconEl.textContent;
        chatModelIcon.style.background = iconEl.style.background;
        chatModelIcon.style.color = iconEl.style.color;
        chatModelOptions.querySelectorAll('.chat-model-option').forEach(el => el.classList.remove('active'));
        selectedItem.classList.add('active');
    }
    // 顶栏同步
    const hItem = headerModelOptions.querySelector(`.chat-model-option[data-value="${name}"]`);
    if (hItem) {
        const iconEl = hItem.querySelector('.chat-model-option-icon');
        headerModelLabel.textContent = labelText;
        headerModelIcon.textContent = iconEl.textContent;
        headerModelIcon.style.background = iconEl.style.background;
        headerModelIcon.style.color = iconEl.style.color;
        headerModelOptions.querySelectorAll('.chat-model-option').forEach(el => el.classList.remove('active'));
        hItem.classList.add('active');
    }
    chatStatus.textContent = name;
    chatStatus.classList.toggle('limited', !!limited);
    chatModelOptions.classList.add('hidden');
    chatModelTrigger.classList.remove('open');
    headerModelOptions.classList.add('hidden');
    headerModelTrigger.classList.remove('open');
    // 同步切换后端默认模型，让流氓软件检测/启动项分析/缓存分析也使用此模型
    fetch('/api/chat/switch', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ model: name })
    }).catch(() => {});
}

// 下拉开关
chatModelTrigger.addEventListener('click', (e) => {
    e.stopPropagation();
    chatModelOptions.classList.toggle('hidden');
    chatModelTrigger.classList.toggle('open');
});

// 顶栏下拉开关
headerModelTrigger.addEventListener('click', (e) => {
    e.stopPropagation();
    headerModelOptions.classList.toggle('hidden');
    headerModelTrigger.classList.toggle('open');
});

// 顶栏自定义模型按钮 -> 打开弹窗
headerCustomModelBtn.addEventListener('click', async () => {
    customModelModal.classList.remove('hidden');
    if (cmProviders.length === 0) {
        await loadCustomModelProviders();
    }
    resetCustomModelForm();
});

// 点击外部关闭下拉
document.addEventListener('click', (e) => {
    if (!chatModelDropdown.contains(e.target)) {
        chatModelOptions.classList.add('hidden');
        chatModelTrigger.classList.remove('open');
    }
    if (!headerModelDropdown.contains(e.target)) {
        headerModelOptions.classList.add('hidden');
        headerModelTrigger.classList.remove('open');
    }
});

// 发送消息
async function sendChatMessage() {
    const text = chatInput.value.trim();
    if (!text || chatSending) return;

    chatSending = true;
    chatSendBtn.disabled = true;
    chatInput.value = '';
    autoResizeInput();

    const welcome = chatMessages.querySelector('.chat-welcome');
    if (welcome) welcome.remove();

    addMessage('user', text);

    const aiEl = addMessage('assistant', '');
    const contentEl = aiEl.querySelector('.chat-msg-content');

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
                model: chatCurrentModel || ''
            })
        });

        typing.remove();

        if (!resp.ok) {
            const err = await resp.json();
            addMessage('error', err.error || '请求失败');
            return;
        }

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

// 添加消息
function addMessage(role, content) {
    const el = document.createElement('div');
    el.className = 'chat-msg ' + role;

    const avatar = document.createElement('div');
    avatar.className = 'chat-msg-avatar';
    avatar.textContent = role === 'user' ? '🧑' : role === 'error' ? '⚠' : '🤖';

    const body = document.createElement('div');
    body.className = 'chat-msg-body';

    const roleLabel = document.createElement('div');
    roleLabel.className = 'chat-msg-role';
    roleLabel.textContent = role === 'user' ? 'You' : role === 'error' ? 'Error' : 'Assistant';

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

function scrollToBottom() {
    chatMessages.scrollTop = chatMessages.scrollHeight;
}

function renderMarkdown(text) {
    let html = escapeHtml(text);
    html = html.replace(/```(\w*)\n?([\s\S]*?)```/g, (_, lang, code) => {
        return '<pre><code>' + code.trim() + '</code></pre>';
    });
    html = html.replace(/`([^`]+)`/g, '<code>$1</code>');
    html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');
    html = html.replace(/^[-*] (.+)$/gm, '<li>$1</li>');
    html = html.replace(/(<li>[\s\S]*?<\/li>)/g, '<ul>$1</ul>');
    html = html.replace(/\n/g, '<br>');
    return html;
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function autoResizeInput() {
    chatInput.style.height = 'auto';
    chatInput.style.height = Math.min(chatInput.scrollHeight, 120) + 'px';
}

chatSendBtn.addEventListener('click', sendChatMessage);

chatInput.addEventListener('keydown', e => {
    if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        sendChatMessage();
    }
});

chatInput.addEventListener('input', autoResizeInput);

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

initChat();
toggleChat(true);

// ==================== 引用给AI ====================

async function referenceToAI(path, isDir, name) {
    toggleChat(true);
    const welcome = chatMessages.querySelector('.chat-welcome');
    if (welcome) welcome.remove();

    if (isDir) {
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
        chatInput.value = `请分析以下文件:\n\n路径: ${path}\n名称: ${name || path.split(/[\\/]/).pop()}`;
    }
    autoResizeInput();
    chatInput.focus();
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
const cmMyModelsList = document.getElementById('cmMyModelsList');

let cmProviders = [];
let cmFetchedModels = [];

// 加载厂商列表
async function loadCustomModelProviders() {
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

// 重置表单
function resetCustomModelForm() {
    cmApiKey.value = '';
    cmModelList.innerHTML = '<div class="custom-model-empty">请先获取模型列表</div>';
    cmModelCount.textContent = '';
    cmFetchedModels = [];
    cmSaveBtn.classList.add('not-ready');
    hideCmStatus();
    loadMyModels();
}

// 打开弹窗
chatCustomModelBtn.addEventListener('click', async () => {
    customModelModal.classList.remove('hidden');
    if (cmProviders.length === 0) {
        await loadCustomModelProviders();
    }
    resetCustomModelForm();
});

customModelClose.addEventListener('click', () => customModelModal.classList.add('hidden'));
cmCancelBtn.addEventListener('click', () => customModelModal.classList.add('hidden'));
customModelModal.addEventListener('click', e => {
    if (e.target === customModelModal) customModelModal.classList.add('hidden');
});

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
    if (!provider) { showCmStatus('请先选择厂商', 'error'); return; }
    if (!apiKey) { showCmStatus('请输入 API Key', 'error'); return; }

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
            cmSaveBtn.classList.add('not-ready');
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
        cmSaveBtn.classList.add('not-ready');
    } finally {
        cmFetchModelsBtn.disabled = false;
        cmFetchModelsBtn.textContent = '获取模型';
    }
});

function renderCmModelList() {
    if (cmFetchedModels.length === 0) {
        cmModelList.innerHTML = '<div class="custom-model-empty">无可用模型</div>';
        cmModelCount.textContent = '';
        cmSaveBtn.classList.add('not-ready');
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
    const firstCheckbox = cmModelList.querySelector('input[type="checkbox"]');
    if (firstCheckbox) firstCheckbox.checked = true;
    updateCmSaveBtn();
}

function updateCmSaveBtn() {
    const checked = cmModelList.querySelectorAll('input[type="checkbox"]:checked');
    cmSaveBtn.classList.toggle('not-ready', checked.length === 0);
    console.log('[cmSave] updateCmSaveBtn checked=', checked.length, 'not-ready=', cmSaveBtn.classList.contains('not-ready'));
}

// 保存配置
cmSaveBtn.addEventListener('click', async () => {
    console.log('[cmSave] click 触发, disabled=', cmSaveBtn.disabled);
    const provider = cmProviderSelect.value;
    const apiKey = cmApiKey.value.trim();
    const selectedModels = Array.from(cmModelList.querySelectorAll('input[type="checkbox"]:checked'))
        .map(cb => cb.value);
    console.log('[cmSave] provider=', provider, 'apiKey长度=', apiKey.length, '选中模型=', selectedModels);
    // 具体提示缺失项，避免"无响应"
    if (!provider) {
        showCmStatus('请先选择厂商', 'error');
        return;
    }
    if (!apiKey) {
        showCmStatus('请输入 API Key', 'error');
        return;
    }
    if (selectedModels.length === 0) {
        showCmStatus('请先获取模型并至少勾选一个模型', 'error');
        return;
    }
    // 保存中禁用防重复点击
    cmSaveBtn.disabled = true;
    cmSaveBtn.textContent = '保存中...';
    showCmStatus('正在保存配置...', 'loading');
    console.log('[cmSave] 发起 fetch /api/chat/config/save');
    let ok = false;
    try {
        const resp = await fetch('/api/chat/config/save', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ provider, api_key: apiKey, models: selectedModels })
        });
        console.log('[cmSave] fetch 响应 status=', resp.status);
        const data = await resp.json();
        console.log('[cmSave] 响应体=', data);
        if (data.error) {
            showCmStatus(data.error, 'error');
        } else {
            ok = true;
            showCmStatus('配置已保存，正在重新加载...', 'success');
        }
    } catch (e) {
        console.log('[cmSave] fetch 异常:', e);
        showCmStatus('保存失败: ' + e.message, 'error');
    }
    // 按钮恢复不依赖 initChat/loadMyModels，避免异步阻塞导致按钮卡死
    setTimeout(() => {
        cmSaveBtn.disabled = false;
        cmSaveBtn.textContent = '保存';
        updateCmSaveBtn();
        console.log('[cmSave] 按钮已恢复');
    }, ok ? 1000 : 200);
    // 异步重新加载模型列表与已配置模型，失败不影响按钮
    if (ok) {
        setTimeout(() => {
            console.log('[cmSave] 开始重新加载 initChat/loadMyModels');
            Promise.all([initChat(), loadMyModels()]).catch((e) => {
                console.log('[cmSave] 重载异常:', e);
            });
        }, 800);
    }
});
console.log('[cmSave] click 监听器已绑定到 cmSaveBtn=', cmSaveBtn);

// 加载已配置的模型列表（用于删除）
async function loadMyModels() {
    if (!cmMyModelsList) return;
    try {
        const resp = await fetch('/api/chat/config');
        const data = await resp.json();
        if (!data.configured || !data.providers) {
            cmMyModelsList.innerHTML = '<div class="custom-model-empty">暂无已配置的模型</div>';
            return;
        }
        cmMyModelsList.innerHTML = '';
        for (const p of data.providers) {
            const icon = getProviderIcon(p.name, p.baseURL);
            const item = document.createElement('div');
            item.className = 'cm-my-model-item';
            const isLimited = p.limited;
            item.innerHTML = `
                <span class="chat-model-option-icon" style="background:${icon.bg};color:${icon.color}">${icon.letter}</span>
                <span class="cm-my-model-name">${p.name}${isLimited ? ' <span class="chat-model-limited">限时</span>' : ''}</span>
                ${isLimited ? '' : `<button class="cm-my-model-del" data-model="${p.name}" title="删除">✕</button>`}
            `;
            cmMyModelsList.appendChild(item);
        }
        if (cmMyModelsList.children.length === 0) {
            cmMyModelsList.innerHTML = '<div class="custom-model-empty">暂无已配置的模型</div>';
        }
    } catch (e) {
        cmMyModelsList.innerHTML = '<div class="custom-model-empty">加载失败</div>';
    }
}

// 删除模型
if (cmMyModelsList) {
    cmMyModelsList.addEventListener('click', async (e) => {
    const delBtn = e.target.closest('.cm-my-model-del');
    if (!delBtn) return;
    const model = delBtn.dataset.model;
    if (!confirm(`确定删除模型「${model}」？`)) return;
    delBtn.disabled = true;
    delBtn.textContent = '...';
    try {
        const resp = await fetch('/api/chat/config/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ model })
        });
        const data = await resp.json();
        if (data.error) {
            alert(data.error);
            delBtn.disabled = false;
            delBtn.textContent = '✕';
            return;
        }
        await initChat();
        await loadMyModels();
    } catch (e) {
        alert('删除失败: ' + e.message);
        delBtn.disabled = false;
        delBtn.textContent = '✕';
    }
});
}

function showCmStatus(msg, type) {
    cmStatus.textContent = msg;
    cmStatus.className = 'custom-model-status show ' + type;
}

function hideCmStatus() {
    cmStatus.className = 'custom-model-status';
}
