// 启动项管理模块（登录启动项 / 资源管理器插件 / IE 加载项）
const startupModal = document.getElementById('startupModal');
const startupClose = document.getElementById('startupClose');
const startupStatus = document.getElementById('startupStatus');
const startupList = document.getElementById('startupList');

const startupCatNames = {
    logon: '登录启动项',
    explorer: '资源管理器插件',
    ie: 'IE 加载项',
};

let startupItemsCache = [];

// 打开弹窗
document.getElementById('homeStartupBtn').onclick = () => {
    startupModal.classList.remove('hidden');
    loadStartupItems();
};

// 关闭弹窗
startupClose.addEventListener('click', () => startupModal.classList.add('hidden'));
startupModal.addEventListener('click', e => {
    if (e.target === startupModal) startupModal.classList.add('hidden');
});

// 读取启动项列表
async function loadStartupItems() {
    startupStatus.innerHTML = `
        <div class="cleanup-scanning">
            <div class="cleanup-scan-spinner"></div>
            <span>正在读取启动项...</span>
        </div>`;
    startupStatus.classList.remove('hidden');
    startupList.classList.add('hidden');

    try {
        const resp = await fetch('/api/startup/list');
        const data = await resp.json();
        renderStartupList(data.items || []);
    } catch (e) {
        startupStatus.innerHTML = `<p class="cleanup-error-msg">读取失败: ${e.message}</p>`;
    }
}

// 渲染启动项列表（按类别分组）
function renderStartupList(items) {
    if (items.length === 0) {
        startupStatus.innerHTML = '<p class="cleanup-empty-msg">未找到启动项</p>';
        return;
    }
    startupItemsCache = items;
    startupStatus.classList.add('hidden');
    startupList.classList.remove('hidden');

    // 按类别分组并保留原始索引
    const cats = {};
    items.forEach((it, idx) => {
        if (!cats[it.category]) cats[it.category] = [];
        cats[it.category].push({ ...it, _idx: idx });
    });
    const order = ['logon', 'explorer', 'ie'];

    let html = '';
    order.forEach(cat => {
        if (!cats[cat]) return;
        const list = cats[cat];
        html += `<div class="startup-group">`;
        html += `<div class="startup-group-title">${startupCatNames[cat] || cat}<span class="startup-group-count">${list.length}</span></div>`;
        list.forEach(it => {
            const disabled = it.state === 'disabled';
            html += `
                <div class="startup-item ${disabled ? 'startup-item-disabled' : ''}">
                    <div class="startup-item-info">
                        <div class="startup-item-name">${it.zhName || it.name}</div>
                        ${it.zhName && it.zhName !== it.name ? `<div class="startup-item-orig">${it.name}</div>` : ''}
                        <div class="startup-item-path">${it.path || '-'}</div>
                        <div class="startup-item-loc">${it.location}</div>
                    </div>
                    <button class="startup-toggle ${disabled ? 'startup-toggle-enable' : 'startup-toggle-disable'}"
                        data-idx="${it._idx}">
                        ${disabled ? '启用' : '禁用'}
                    </button>
                </div>`;
        });
        html += `</div>`;
    });
    startupList.innerHTML = html;
}

// 启用/禁用切换
startupList.addEventListener('click', async e => {
    const btn = e.target.closest('.startup-toggle');
    if (!btn) return;
    const idx = parseInt(btn.dataset.idx);
    const it = startupItemsCache[idx];
    if (!it) return;

    const enable = it.state === 'disabled';
    const origText = btn.textContent;
    btn.disabled = true;
    btn.textContent = '处理中...';

    try {
        const resp = await fetch('/api/startup/toggle', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                category: it.category,
                name: it.name,
                location: it.location,
                enable: enable,
            }),
        });
        const data = await resp.json();
        if (data.error) {
            btn.textContent = origText;
            btn.disabled = false;
            alert('操作失败: ' + data.error);
        } else {
            // 刷新列表
            loadStartupItems();
        }
    } catch (err) {
        btn.textContent = origText;
        btn.disabled = false;
        alert('请求失败: ' + err.message);
    }
});
