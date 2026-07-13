// 启动项管理模块
const startupModal = document.getElementById('startupModal');
const startupClose = document.getElementById('startupClose');
const startupStatus = document.getElementById('startupStatus');
const startupListEl = document.getElementById('startupList');
const startupRescanBtn = document.getElementById('startupRescanBtn');
let startupDetailItems = [];
let startupDetailId = '';

const startupCatNames = {
    logon: '登录启动项',
    explorer: '资源管理器插件',
    ie: 'IE 加载项',
};
const startupCatOrder = ['logon', 'explorer', 'ie'];

document.getElementById('homeStartupBtn').onclick = () => {
    startupModal.classList.remove('hidden');
    loadStartupCacheList();
};

startupClose.addEventListener('click', () => startupModal.classList.add('hidden'));
startupModal.addEventListener('click', e => {
    if (e.target === startupModal) startupModal.classList.add('hidden');
});

// 加载缓存记录列表
async function loadStartupCacheList() {
    startupStatus.innerHTML = `
        <div class="cleanup-scanning">
            <div class="cleanup-scan-spinner"></div>
            <span>正在加载启动项缓存记录...</span>
        </div>`;
    startupStatus.classList.remove('hidden');
    startupListEl.classList.add('hidden');

    try {
        const resp = await fetch('/api/startup/cache');
        const data = await resp.json();
        renderStartupCacheList(data.records || []);
    } catch (e) {
        startupStatus.innerHTML = `<p class="cleanup-error-msg">加载失败: ${e.message}</p>`;
    }
}

function renderStartupCacheList(records) {
    if (records.length === 0) {
        startupStatus.innerHTML = '<p class="cleanup-empty-msg">空</p>';
        return;
    }
    startupStatus.classList.add('hidden');
    startupListEl.classList.remove('hidden');

    let html = '<div class="startup-cache-list-header">扫描记录（共 ' + records.length + ' 条）</div>';
    html += '<div class="startup-cache-list">';
    records.forEach(r => {
        html += `
            <div class="startup-cache-record">
                <div class="startup-cache-record-info">
                    <div class="startup-cache-record-time">${r.savedAt}</div>
                    <div class="startup-cache-record-stats">
                        <span class="startup-cache-stat"><span class="startup-cache-dot sys"></span>系统 ${r.sysCount}</span>
                        <span class="startup-cache-stat"><span class="startup-cache-dot app"></span>应用 ${r.appCount}</span>
                        <span class="startup-cache-stat">共 ${r.itemsCount} 项</span>
                    </div>
                </div>
                <div class="startup-cache-record-actions">
                    <button class="startup-cache-btn startup-view-btn" data-id="${r.id}">查看</button>
                    <button class="startup-cache-btn startup-cache-btn-danger startup-del-btn" data-id="${r.id}">删除</button>
                </div>
            </div>`;
    });
    html += '</div>';
    startupListEl.innerHTML = html;
}

// 查看缓存记录详情
async function viewStartupCache(id) {
    startupStatus.innerHTML = `
        <div class="cleanup-scanning">
            <div class="cleanup-scan-spinner"></div>
            <span>正在加载缓存详情...</span>
        </div>`;
    startupStatus.classList.remove('hidden');
    startupListEl.classList.add('hidden');

    try {
        const resp = await fetch('/api/startup/cache?id=' + encodeURIComponent(id));
        const data = await resp.json();
        if (data.error) {
            startupStatus.innerHTML = `<p class="cleanup-error-msg">${data.error}</p>`;
            return;
        }
        renderStartupDetail(data, id);
    } catch (e) {
        startupStatus.innerHTML = `<p class="cleanup-error-msg">加载失败: ${e.message}</p>`;
    }
}

function renderStartupDetail(data, id) {
    startupStatus.classList.add('hidden');
    startupListEl.classList.remove('hidden');
    startupDetailId = id;

    const items = data.items || [];
    startupDetailItems = items;
    // 按类别分组
    const groups = {};
    items.forEach(it => {
        const cat = it.category || 'other';
        if (!groups[cat]) groups[cat] = [];
        groups[cat].push(it);
    });

    let html = '<div class="startup-back-bar">';
    html += '<button class="startup-back-btn">← 返回记录列表</button>';
    html += '</div>';

    // 缓存信息栏
    html += '<div class="startup-cache-bar">';
    html += '<span class="startup-cache-time">缓存时间: ' + data.savedAt + ' · 共 ' + items.length + ' 项</span>';
    html += '<div class="startup-cache-actions">';
    html += '<button class="startup-cache-btn startup-cache-btn-danger startup-del-btn" data-id="' + id + '">删除此记录</button>';
    html += '</div>';
    html += '</div>';

    // 按类别渲染折叠分组
    startupCatOrder.forEach(cat => {
        if (!groups[cat]) return;
        const list = groups[cat];
        const sysCount = list.filter(i => i.isSystem).length;
        const appCount = list.length - sysCount;

        html += '<div class="startup-group">';
        html += '<div class="startup-group-header">';
        html += '<svg class="startup-group-chevron" viewBox="0 0 16 16" fill="currentColor"><path d="M13.78 5.22a.75.75 0 0 1 0 1.06l-5.25 5.25a.75.75 0 0 1-1.06 0L2.22 6.28a.751.751 0 0 1 .018-1.042.751.751 0 0 1 1.042-.018L8 9.94l4.72-4.72a.75.75 0 0 1 1.06 0Z"/></svg>';
        html += '<span class="startup-group-title">' + (startupCatNames[cat] || cat) + '</span>';
        html += '<span class="startup-group-count">' + list.length + '</span>';
        html += '<div class="startup-group-stats">';
        html += '<span class="startup-group-stat"><span class="startup-group-stat-dot sys"></span>系统 ' + sysCount + '</span>';
        html += '<span class="startup-group-stat"><span class="startup-group-stat-dot app"></span>应用 ' + appCount + '</span>';
        html += '</div>';
        html += '</div>';
        html += '<div class="startup-group-body">';

        list.forEach((it, idx) => {
            const itemIdx = items.indexOf(it);
            const disabled = it.state === 'disabled';
            const badge = it.isSystem
                ? '<span class="startup-badge startup-badge-sys">系统</span>'
                : '<span class="startup-badge startup-badge-app">应用</span>';
            const disBadge = disabled ? ' <span class="startup-badge startup-badge-disabled">已禁用</span>' : '';
            const orig = it.zhName && it.zhName !== it.name
                ? '<div class="startup-item-orig">' + it.name + '</div>' : '';
            // 右键打开文件所在位置的目标路径
            let openPath = '';
            if (cat === 'logon') {
                openPath = extractFilePath(it.path);
            } else if (cat === 'explorer') {
                openPath = it.path; // CLSID
            } else if (cat === 'ie') {
                openPath = it.name; // CLSID
            }
            const toggleBtn = disabled
                ? '<button class="startup-toggle startup-toggle-disable" data-idx="' + itemIdx + '">禁用</button>'
                : '<button class="startup-toggle startup-toggle-enable" data-idx="' + itemIdx + '">启用</button>';
            html += `
                <div class="startup-item${disabled ? ' startup-item-disabled' : ''}" data-openpath="${openPath}" data-idx="${itemIdx}" style="animation-delay:${Math.min(idx * 30, 300)}ms">
                    <div class="startup-item-info">
                        <div class="startup-item-header">
                            <span class="startup-item-name">${it.zhName || it.name}</span>
                            ${badge}${disBadge}
                        </div>
                        ${orig}
                        ${it.path ? '<div class="startup-item-path">' + it.path + '</div>' : ''}
                        ${it.location ? '<div class="startup-item-loc">' + it.location + '</div>' : ''}
                    </div>
                    ${toggleBtn}
                </div>`;
        });

        html += '</div></div>';
    });

    startupListEl.innerHTML = html;
}

// 删除缓存记录
async function deleteStartupCache(id) {
    if (!confirm('确定删除此缓存记录？')) return;
    try {
        await fetch('/api/startup/cache?id=' + encodeURIComponent(id), { method: 'DELETE' });
        loadStartupCacheList();
    } catch (e) {
        alert('删除失败: ' + e.message);
    }
}

// 事件委托：查看 / 删除 / 返回 / 折叠分组 / 启用禁用
startupListEl.addEventListener('click', e => {
    // 返回按钮
    if (e.target.classList.contains('startup-back-btn')) {
        loadStartupCacheList();
        return;
    }
    // 查看按钮
    const viewBtn = e.target.closest('.startup-view-btn');
    if (viewBtn) {
        viewStartupCache(viewBtn.dataset.id);
        return;
    }
    // 删除按钮
    const delBtn = e.target.closest('.startup-del-btn');
    if (delBtn) {
        deleteStartupCache(delBtn.dataset.id);
        return;
    }
    // 启用/禁用按钮
    const toggleBtn = e.target.closest('.startup-toggle');
    if (toggleBtn) {
        toggleStartupItem(toggleBtn);
        return;
    }
    // 折叠分组标题点击
    const header = e.target.closest('.startup-group-header');
    if (header) {
        header.parentElement.classList.toggle('collapsed');
    }
});

// 启用/禁用启动项
async function toggleStartupItem(btn) {
    const idx = parseInt(btn.dataset.idx);
    const it = startupDetailItems[idx];
    if (!it) return;

    const enable = it.state === 'disabled';
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
            alert(data.error);
            btn.disabled = false;
            btn.textContent = enable ? '禁用' : '启用';
            return;
        }
        // 更新本地状态和 UI
        it.state = enable ? 'enabled' : 'disabled';
        const itemEl = btn.closest('.startup-item');
        const disBadge = itemEl.querySelector('.startup-badge-disabled');
        if (it.state === 'disabled') {
            itemEl.classList.add('startup-item-disabled');
            if (!disBadge) {
                const header = itemEl.querySelector('.startup-item-header');
                header.insertAdjacentHTML('beforeend', ' <span class="startup-badge startup-badge-disabled">已禁用</span>');
            }
            btn.className = 'startup-toggle startup-toggle-disable';
            btn.textContent = '禁用';
        } else {
            itemEl.classList.remove('startup-item-disabled');
            if (disBadge) disBadge.remove();
            btn.className = 'startup-toggle startup-toggle-enable';
            btn.textContent = '启用';
        }
        btn.disabled = false;
    } catch (e) {
        alert('操作失败: ' + e.message);
        btn.disabled = false;
        btn.textContent = enable ? '禁用' : '启用';
    }
}

// 从命令行中提取纯文件路径
// "D:\steam\steam.exe" -silent -> D:\steam\steam.exe
// C:\path\app.exe /arg -> C:\path\app.exe
// {CLSID} / rundll32 等 -> '' (无法提取)
function extractFilePath(cmdLine) {
    if (!cmdLine) return '';
    const s = cmdLine.trim();
    // 带引号的路径
    if (s.startsWith('"')) {
        const end = s.indexOf('"', 1);
        if (end > 0) return s.substring(1, end);
    }
    // 不带引号但含盘符或 UNC 路径
    const match = s.match(/^(?:[A-Za-z]:\\|\\\\)[^\s]+/);
    if (match) return match[0];
    // CLSID、rundll32 等无法定位文件
    return '';
}

// 右键打开文件所在位置
startupListEl.addEventListener('contextmenu', e => {
    const item = e.target.closest('.startup-item');
    if (!item) return;
    const openPath = item.dataset.openpath;
    if (openPath) {
        e.preventDefault();
        showCtxMenu(e.clientX, e.clientY, openPath);
    }
});

// 重新扫描：调用 /api/startup/list 触发新扫描并缓存，完成后刷新列表
startupRescanBtn.addEventListener('click', async () => {
    startupRescanBtn.disabled = true;
    startupRescanBtn.textContent = '正在扫描...';
    startupStatus.innerHTML = `
        <div class="cleanup-scanning">
            <div class="cleanup-scan-spinner"></div>
            <span>正在扫描启动项并调用 AI 分析，请稍候...</span>
        </div>`;
    startupStatus.classList.remove('hidden');
    startupListEl.classList.add('hidden');

    try {
        await fetch('/api/startup/list');
        await loadStartupCacheList();
    } catch (e) {
        startupStatus.innerHTML = `<p class="cleanup-error-msg">扫描失败: ${e.message}</p>`;
    } finally {
        startupRescanBtn.disabled = false;
        startupRescanBtn.textContent = '重新扫描';
    }
});
