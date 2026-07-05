// AI 聊天模块（VSCode 风格侧边栏）
const chatSidebar = document.getElementById('chatSidebar');
const chatMessages = document.getElementById('chatMessages');
const chatInput = document.getElementById('chatInput');
const chatSendBtn = document.getElementById('chatSendBtn');
const chatStatus = document.getElementById('chatStatus');
const chatToggleHeaderBtn = document.getElementById('chatToggleHeaderBtn');
const chatCloseBtn = document.getElementById('chatCloseBtn');
const chatNewBtn = document.getElementById('chatNewBtn');

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

// 初始化：检查 AI 配置状态
async function initChat() {
    try {
        const resp = await fetch('/api/chat/config');
        const data = await resp.json();
        if (!data.configured) {
            chatStatus.textContent = '未配置';
            chatStatus.classList.add('not-configured');
        }
    } catch (e) {
        // 忽略
    }
}

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
                model: ''
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
