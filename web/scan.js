// 流氓软件检测模块
const scanModal = document.getElementById('scanModal');
const scanClose = document.getElementById('scanClose');
const scanStatus = document.getElementById('scanStatus');
const scanListEl = document.getElementById('scanList');
const scanRescanBtn = document.getElementById('scanRescanBtn');
let scanDetailItems = [];
let scanDetailId = '';

const scanCatNames = {
    installed: '已安装程序',
    process: '运行中进程',
    file: '可疑文件',
    startup: '启动项',
};
const scanCatOrder = ['installed', 'process', 'file', 'startup'];
const verdictNames = { safe: '安全', suspicious: '可疑', unknown: '未知' };

document.getElementById('homeScanBtn').onclick = () => {
    scanModal.classList.remove('hidden');
    loadScanCacheList();
};

scanClose.addEventListener('click', () => scanModal.classList.add('hidden'));
scanModal.addEventListener('click', e => {
    if (e.target === scanModal) scanModal.classList.add('hidden');
});

// 加载缓存记录列表
async function loadScanCacheList() {
    scanStatus.innerHTML = `
        <div class="cleanup-scanning">
            <div class="cleanup-scan-spinner"></div>
            <span>正在加载扫描记录...</span>
        </div>`;
    scanStatus.classList.remove('hidden');
    scanListEl.classList.add('hidden');

    try {
        const resp = await fetch('/api/scan/cache');
        const data = await resp.json();
        renderScanCacheList(data.records || []);
    } catch (e) {
        scanStatus.innerHTML = `<p class="cleanup-error-msg">加载失败: ${e.message}</p>`;
    }
}

function renderScanCacheList(records) {
    if (records.length === 0) {
        scanStatus.innerHTML = '<p class="cleanup-empty-msg">暂无扫描记录，点击「重新扫描」开始</p>';
        return;
    }
    scanStatus.classList.add('hidden');
    scanListEl.classList.remove('hidden');

    let html = '<div class="startup-cache-list-header">扫描记录（共 ' + records.length + ' 条）</div>';
    html += '<div class="startup-cache-list">';
    records.forEach(r => {
        html += `
            <div class="startup-cache-record">
                <div class="startup-cache-record-info">
                    <div class="startup-cache-record-time">${r.savedAt}</div>
                    <div class="startup-cache-record-stats">
                        <span class="startup-cache-stat"><span class="startup-cache-dot sys"></span>安全 ${r.safeCount}</span>
                        <span class="startup-cache-stat"><span class="startup-cache-dot app"></span>可疑 ${r.susCount}</span>
                        <span class="startup-cache-stat">未知 ${r.unknownCount}</span>
                        <span class="startup-cache-stat">共 ${r.itemsCount} 项</span>
                    </div>
                </div>
                <div class="startup-cache-record-actions">
                    <button class="startup-cache-btn startup-view-btn" data-id="${r.id}">查看</button>
                    <button class="startup-cache-btn startup-cache-btn-danger scan-del-btn" data-id="${r.id}">删除</button>
                </div>
            </div>`;
    });
    html += '</div>';
    scanListEl.innerHTML = html;
}

// 查看缓存记录详情
async function viewScanCache(id) {
    scanStatus.innerHTML = `
        <div class="cleanup-scanning">
            <div class="cleanup-scan-spinner"></div>
            <span>正在加载扫描详情...</span>
        </div>`;
    scanStatus.classList.remove('hidden');
    scanListEl.classList.add('hidden');

    try {
        const resp = await fetch('/api/scan/cache?id=' + encodeURIComponent(id));
        const data = await resp.json();
        if (data.error) {
            scanStatus.innerHTML = `<p class="cleanup-error-msg">${data.error}</p>`;
            return;
        }
        renderScanDetail(data, id);
    } catch (e) {
        scanStatus.innerHTML = `<p class="cleanup-error-msg">加载失败: ${e.message}</p>`;
    }
}

function renderScanDetail(data, id) {
    scanStatus.classList.add('hidden');
    scanListEl.classList.remove('hidden');
    scanDetailId = id;

    const items = data.items || [];
    scanDetailItems = items;

    let html = '<div class="scan-sticky-header">';
    html += '<div class="startup-back-bar">';
    html += '<button class="startup-back-btn">← 返回记录列表</button>';
    html += '</div>';

    // 缓存信息栏
    const susCount = items.filter(i => i.verdict === 'suspicious').length;
    const safeCount = items.filter(i => i.verdict === 'safe').length;
    html += '<div class="startup-cache-bar">';
    html += '<span class="startup-cache-time">扫描时间: ' + data.savedAt + ' · 共 ' + items.length + ' 项 · 安全 ' + safeCount + ' · 可疑 ' + susCount + '</span>';
    html += '<div class="startup-cache-actions">';
    html += '<button class="startup-cache-btn startup-cache-btn-danger scan-del-btn" data-id="' + id + '">删除此记录</button>';
    html += '</div>';
    html += '</div>';
    html += '</div>';

    // 按类别分组
    const cats = {};
    items.forEach((it, idx) => {
        if (!cats[it.category]) cats[it.category] = [];
        cats[it.category].push({ ...it, _idx: idx });
    });

    scanCatOrder.forEach(cat => {
        if (!cats[cat]) return;
        const list = cats[cat];
        const catSus = list.filter(i => i.verdict === 'suspicious').length;

        html += '<div class="startup-group">';
        html += '<div class="startup-group-header">';
        html += '<svg class="startup-group-chevron" viewBox="0 0 16 16" fill="currentColor"><path d="M13.78 5.22a.75.75 0 0 1 0 1.06l-5.25 5.25a.75.75 0 0 1-1.06 0L2.22 6.28a.751.751 0 0 1 .018-1.042.751.751 0 0 1 1.042-.018L8 9.94l4.72-4.72a.75.75 0 0 1 1.06 0Z"/></svg>';
        html += '<span class="startup-group-title">' + (scanCatNames[cat] || cat) + '</span>';
        html += '<span class="startup-group-count">' + list.length + '</span>';
        if (catSus > 0) {
            html += '<span class="startup-group-stat"><span class="startup-group-stat-dot app"></span>可疑 ' + catSus + '</span>';
        }
        html += '</div>';
        html += '<div class="startup-group-body">';

        list.forEach(it => {
            const v = it.verdict || 'unknown';
            const vName = verdictNames[v] || v;
            const orig = it.zhName && it.zhName !== it.name ? ` <span class="scan-item-orig">${it.name}</span>` : '';
            const resourceInfo = (it.cpuUsage || it.memUsage) ? `<div class="scan-item-resource">CPU: ${it.cpuUsage || '-'} · 内存: ${it.memUsage || '-'}</div>` : '';
            html += `
                <div class="scan-item scan-verdict-${v}" data-idx="${it._idx}">
                    <div class="scan-item-info">
                        <div class="scan-item-name">${it.zhName || it.name}${orig}</div>
                        <div class="startup-item-path">${it.path || '-'}</div>
                        ${resourceInfo}
                        ${it.publisher ? `<div class="scan-item-pub">发布者: ${it.publisher}</div>` : ''}
                        ${it.reason ? `<div class="scan-item-reason">${it.reason}</div>` : ''}
                    </div>
                    <span class="scan-verdict-badge scan-verdict-${v}">${vName}</span>
                </div>`;
        });

        html += '</div></div>';
    });

    scanListEl.innerHTML = html;

    // 动态测量 sticky header 高度，设置分类标题的 top 偏移
    requestAnimationFrame(() => {
        const stickyHeader = scanListEl.querySelector('.scan-sticky-header');
        if (stickyHeader) {
            const h = stickyHeader.offsetHeight;
            scanListEl.querySelectorAll('.startup-group-header').forEach(el => {
                el.style.top = h + 'px';
            });
        }
    });
}

// 删除缓存记录
async function deleteScanCache(id) {
    if (!confirm('确定删除此扫描记录？')) return;
    try {
        await fetch('/api/scan/cache?id=' + encodeURIComponent(id), { method: 'DELETE' });
        loadScanCacheList();
    } catch (e) {
        alert('删除失败: ' + e.message);
    }
}

// 事件委托
scanListEl.addEventListener('click', e => {
    // 返回按钮
    if (e.target.classList.contains('startup-back-btn')) {
        loadScanCacheList();
        return;
    }
    // 查看按钮
    const viewBtn = e.target.closest('.startup-view-btn');
    if (viewBtn) {
        viewScanCache(viewBtn.dataset.id);
        return;
    }
    // 删除按钮
    const delBtn = e.target.closest('.scan-del-btn');
    if (delBtn) {
        deleteScanCache(delBtn.dataset.id);
        return;
    }
    // 折叠分组标题点击
    const header = e.target.closest('.startup-group-header');
    if (header) {
        header.parentElement.classList.toggle('collapsed');
    }
});

// 右键打开文件所在位置（复用全局 showCtxMenu）
scanListEl.addEventListener('contextmenu', e => {
    const item = e.target.closest('.scan-item');
    if (!item) return;
    const idx = parseInt(item.dataset.idx);
    const it = scanDetailItems[idx];
    if (it && it.path) {
        e.preventDefault();
        showCtxMenu(e.clientX, e.clientY, it.path);
    }
});

// 重新扫描
scanRescanBtn.addEventListener('click', async () => {
    scanRescanBtn.disabled = true;
    scanRescanBtn.textContent = '正在扫描...';
    scanStatus.innerHTML = `
        <div class="cleanup-scanning">
            <div class="cleanup-scan-spinner"></div>
            <span>正在扫描系统并调用 AI 判断，请稍候...</span>
        </div>`;
    scanStatus.classList.remove('hidden');
    scanListEl.classList.add('hidden');

    try {
        await fetch('/api/scan/list');
        await loadScanCacheList();
    } catch (e) {
        scanStatus.innerHTML = `<p class="cleanup-error-msg">扫描失败: ${e.message}</p>`;
    } finally {
        scanRescanBtn.disabled = false;
        scanRescanBtn.textContent = '重新扫描';
    }
});
