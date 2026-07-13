// 启动项管理模块（登录启动项 / 资源管理器插件 / IE 加载项）
const startupModal = document.getElementById('startupModal');
const startupClose = document.getElementById('startupClose');
const startupStatus = document.getElementById('startupStatus');
const startupList = document.getElementById('startupList');
const startupCacheBar = document.getElementById('startupCacheBar');
const startupCacheTime = document.getElementById('startupCacheTime');

const startupCatNames = {
    logon: '登录启动项',
    explorer: '资源管理器插件',
    ie: 'IE 加载项',
};

let startupItemsCache = [];

// 打开弹窗 -> 先展示历史缓存列表
document.getElementById('homeStartupBtn').onclick = () => {
    startupModal.classList.remove('hidden');
    loadStartupCacheList();
};

// 关闭弹窗
startupClose.addEventListener('click', () => startupModal.classList.add('hidden'));
startupModal.addEventListener('click', e => {
    if (e.target === startupModal) startupModal.classList.add('hidden');
});

// 刷新按钮 -> 强制请求 API 扫描
document.getElementById('startupRefreshBtn').onclick = () => loadStartupItems();

// 加载历史缓存记录列表
async function loadStartupCacheList() {
    startupStatus.innerHTML = `
        <div class="cleanup-scanning">
            <div class="cleanup-scan-spinner"></div>
            <span>正在读取缓存记录...</span>
        </div>`;
    startupStatus.classList.remove('hidden');
    startupList.classList.add('hidden');
    startupCacheTime.textContent = '正在读取...';

    try {
        const resp = await fetch('/api/startup/cache');
        const data = await resp.json();
        const records = data.records || [];
        if (records.length === 0) {
            startupCacheTime.textContent = '无缓存记录';
            // 无缓存，直接扫描
            loadStartupItems();
            return;
        }
        startupCacheTime.textContent = `共 ${records.length} 条缓存记录`;
        renderCacheList(records);
    } catch (e) {
        startupCacheTime.textContent = '读取失败';
        loadStartupItems();
    }
}

// 渲染历史缓存记录列表
function renderCacheList(records) {
    startupStatus.classList.add('hidden');
    startupList.classList.remove('hidden');

    let html = `<div class="startup-cache-list">`;
    html += `<div class="startup-cache-list-header">历史缓存记录</div>`;
    records.forEach((r, idx) => {
        const delay = Math.min(idx * 40, 300);
        html += `
            <div class="startup-cache-record" style="animation-delay:${delay}ms">
                <div class="startup-cache-record-info">
                    <div class="startup-cache-record-time">${r.savedAt}</div>
                    <div class="startup-cache-record-stats">
                        <span class="startup-cache-stat">${r.itemsCount} 项</span>
                        <span class="startup-cache-stat"><span class="startup-cache-dot sys"></span>系统 ${r.sysCount}</span>
                        <span class="startup-cache-stat"><span class="startup-cache-dot app"></span>应用 ${r.appCount}</span>
                    </div>
                </div>
                <div class="startup-cache-record-actions">
                    <button class="startup-cache-btn" onclick="viewStartupCache('${r.id}')">查看</button>
                    <button class="startup-cache-btn startup-cache-btn-danger" onclick="deleteStartupCache('${r.id}', '${r.savedAt}')">删除</button>
                </div>
            </div>`;
    });
    html += `</div>`;
    startupList.innerHTML = html;
}

// 查看指定缓存记录
async function viewStartupCache(id) {
    startupStatus.innerHTML = `
        <div class="cleanup-scanning">
            <div class="cleanup-scan-spinner"></div>
            <span>正在加载缓存数据...</span>
        </div>`;
    startupStatus.classList.remove('hidden');
    startupList.classList.add('hidden');

    try {
        const resp = await fetch('/api/startup/cache?id=' + encodeURIComponent(id));
        const data = await resp.json();
        if (data.error) {
            startupStatus.innerHTML = `<p class="cleanup-error-msg">${data.error}</p>`;
            return;
        }
        renderStartupList(data.items || []);
        showCacheBar(data.savedAt);
    } catch (e) {
        startupStatus.innerHTML = `<p class="cleanup-error-msg">加载失败: ${e.message}</p>`;
    }
}

// 删除指定缓存记录
async function deleteStartupCache(id, savedAt) {
    if (!confirm(`确定删除 ${savedAt} 的缓存记录？`)) return;
    try {
        await fetch('/api/startup/cache?id=' + encodeURIComponent(id), { method: 'DELETE' });
        loadStartupCacheList();
    } catch (e) {
        alert('删除失败: ' + e.message);
    }
}

// 显示缓存信息栏
function showCacheBar(savedAt) {
    startupCacheTime.textContent = `缓存时间: ${savedAt}`;
    startupCacheBar.classList.remove('hidden');
}

// 请求 API 获取最新启动项
async function loadStartupItems() {
    startupStatus.innerHTML = `
        <div class="cleanup-scanning">
            <div class="cleanup-scan-spinner"></div>
            <span>正在扫描启动项（AI 翻译中，可能需要一些时间）...</span>
        </div>`;
    startupStatus.classList.remove('hidden');
    startupList.classList.add('hidden');
    startupCacheTime.textContent = '扫描中...';

    try {
        const resp = await fetch('/api/startup/list');
        const data = await resp.json();
        renderStartupList(data.items || []);
        // 扫描完成后显示缓存时间
        try {
            const cacheResp = await fetch('/api/startup/cache');
            const cacheData = await cacheResp.json();
            const records = cacheData.records || [];
            if (records.length > 0) showCacheBar(records[0].savedAt);
        } catch (_) {}
    } catch (e) {
        startupStatus.innerHTML = `<p class="cleanup-error-msg">读取失败: ${e.message}</p>`;
    }
}

// 返回缓存列表
function backToCacheList() {
    loadStartupCacheList();
}

// 渲染启动项列表（按类别分组，可折叠）
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

    let html = `<div class="startup-back-bar"><button class="startup-back-btn" onclick="backToCacheList()">← 返回缓存列表</button></div>`;
    order.forEach(cat => {
        if (!cats[cat]) return;
        const list = cats[cat];
        const sysCount = list.filter(it => it.isSystem).length;
        const appCount = list.length - sysCount;

        html += `<div class="startup-group">`;
        html += `<div class="startup-group-header" onclick="this.parentElement.classList.toggle('collapsed')">`;
        html += `<svg class="startup-group-chevron" viewBox="0 0 16 16" fill="currentColor"><path d="M3.22 5.72a.75.75 0 0 1 1.06 0L8 9.44l3.72-3.72a.75.75 0 1 1 1.06 1.06l-4.25 4.25a.75.75 0 0 1-1.06 0L3.22 6.78a.75.75 0 0 1 0-1.06Z"/></svg>`;
        html += `<span class="startup-group-title">${startupCatNames[cat] || cat}</span>`;
        html += `<span class="startup-group-count">${list.length}</span>`;
        html += `<div class="startup-group-stats">`;
        if (sysCount > 0) html += `<span class="startup-group-stat"><span class="startup-group-stat-dot sys"></span>系统 ${sysCount}</span>`;
        if (appCount > 0) html += `<span class="startup-group-stat"><span class="startup-group-stat-dot app"></span>应用 ${appCount}</span>`;
        html += `</div>`;
        html += `</div>`;
        html += `<div class="startup-group-body">`;

        list.forEach((it, idx2) => {
            const disabled = it.state === 'disabled';
            const delay = Math.min(idx2 * 30, 300);
            html += `
                <div class="startup-item ${disabled ? 'startup-item-disabled' : ''}" style="animation-delay:${delay}ms">
                    <div class="startup-item-info">
                        <div class="startup-item-header">
                            <span class="startup-item-name">${it.zhName || it.name}</span>
                            <span class="startup-badge ${it.isSystem ? 'startup-badge-sys' : 'startup-badge-app'}">${it.isSystem ? '系统' : '应用'}</span>
                            ${disabled ? '<span class="startup-badge startup-badge-disabled">已禁用</span>' : ''}
                        </div>
                        ${it.zhName && it.zhName !== it.name ? `<div class="startup-item-orig">${it.name}</div>` : ''}
                        ${it.path ? `<div class="startup-item-path">${it.path}</div>` : ''}
                        <div class="startup-item-loc">${it.location}</div>
                    </div>
                    <button class="startup-toggle ${disabled ? 'startup-toggle-enable' : 'startup-toggle-disable'}"
                        data-idx="${it._idx}">
                        ${disabled ? '启用' : '禁用'}
                    </button>
                </div>`;
        });

        html += `</div></div>`;
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
            loadStartupItems();
        }
    } catch (err) {
        btn.textContent = origText;
        btn.disabled = false;
        alert('请求失败: ' + err.message);
    }
});
