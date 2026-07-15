// 软件缓存清理模块
const cacheModal = document.getElementById('cacheModal');
const cacheClose = document.getElementById('cacheClose');
const cacheStatus = document.getElementById('cacheStatus');
const cacheListEl = document.getElementById('cacheList');
const cacheRescanBtn = document.getElementById('cacheRescanBtn');
let cacheDetailItems = [];
let cacheDetailId = '';

document.getElementById('homeCacheBtn').onclick = () => {
    cacheModal.classList.remove('hidden');
    loadCacheCacheList();
};

cacheClose.addEventListener('click', () => cacheModal.classList.add('hidden'));
cacheModal.addEventListener('click', e => {
    if (e.target === cacheModal) cacheModal.classList.add('hidden');
});

// 格式化文件大小
function formatCacheSize(bytes) {
    if (bytes < 1024) return bytes + ' B';
    const units = ['KB', 'MB', 'GB', 'TB'];
    let v = bytes / 1024, i = 0;
    while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
    return v.toFixed(1) + ' ' + units[i];
}

// 加载缓存记录列表
async function loadCacheCacheList() {
    cacheStatus.innerHTML = `
        <div class="cleanup-scanning">
            <div class="cleanup-scan-spinner"></div>
            <span>正在加载缓存记录...</span>
        </div>`;
    cacheStatus.classList.remove('hidden');
    cacheListEl.classList.add('hidden');

    try {
        const resp = await fetch('/api/cache/cache');
        const data = await resp.json();
        renderCacheCacheList(data.records || []);
    } catch (e) {
        cacheStatus.innerHTML = `<p class="cleanup-error-msg">加载失败: ${e.message}</p>`;
    }
}

function renderCacheCacheList(records) {
    if (records.length === 0) {
        cacheStatus.innerHTML = '<p class="cleanup-empty-msg">暂无扫描记录，点击「重新扫描」开始</p>';
        return;
    }
    cacheStatus.classList.add('hidden');
    cacheListEl.classList.remove('hidden');

    let html = '<div class="startup-cache-list-header">扫描记录（共 ' + records.length + ' 条）</div>';
    html += '<div class="startup-cache-list">';
    records.forEach(r => {
        html += `
            <div class="startup-cache-record">
                <div class="startup-cache-record-info">
                    <div class="startup-cache-record-time">${r.savedAt}</div>
                    <div class="startup-cache-record-stats">
                        <span class="startup-cache-stat"><span class="startup-cache-dot sys"></span>可清理 ${r.safeCount}</span>
                        <span class="startup-cache-stat"><span class="startup-cache-dot app"></span>谨慎 ${r.warnCount}</span>
                        <span class="startup-cache-stat">共 ${r.itemsCount} 项</span>
                        <span class="startup-cache-stat">${formatCacheSize(r.totalSize)}</span>
                    </div>
                </div>
                <div class="startup-cache-record-actions">
                    <button class="startup-cache-btn startup-view-btn" data-id="${r.id}">查看</button>
                    <button class="startup-cache-btn startup-cache-btn-danger cache-del-btn" data-id="${r.id}">删除</button>
                </div>
            </div>`;
    });
    html += '</div>';
    cacheListEl.innerHTML = html;
}

// 查看缓存记录详情
async function viewCacheCache(id) {
    cacheStatus.innerHTML = `
        <div class="cleanup-scanning">
            <div class="cleanup-scan-spinner"></div>
            <span>正在加载缓存详情...</span>
        </div>`;
    cacheStatus.classList.remove('hidden');
    cacheListEl.classList.add('hidden');

    try {
        const resp = await fetch('/api/cache/cache?id=' + encodeURIComponent(id));
        const data = await resp.json();
        if (data.error) {
            cacheStatus.innerHTML = `<p class="cleanup-error-msg">${data.error}</p>`;
            return;
        }
        renderCacheDetail(data, id);
    } catch (e) {
        cacheStatus.innerHTML = `<p class="cleanup-error-msg">加载失败: ${e.message}</p>`;
    }
}

function renderCacheDetail(data, id) {
    cacheStatus.classList.add('hidden');
    cacheListEl.classList.remove('hidden');
    cacheDetailId = id;

    const items = (data.items || []).slice();
    cacheDetailItems = items;

    let html = '<div class="scan-sticky-header">';
    html += '<div class="startup-back-bar">';
    html += '<button class="startup-back-btn">← 返回记录列表</button>';
    html += '</div>';

    // 缓存信息栏
    const safeCount = items.filter(i => i.safe).length;
    const warnCount = items.length - safeCount;
    const totalSize = items.reduce((s, i) => s + i.size, 0);
    html += '<div class="startup-cache-bar">';
    html += '<span class="startup-cache-time">扫描时间: ' + data.savedAt + ' · 共 ' + items.length + ' 项 · 可清理 ' + safeCount + ' · 谨慎 ' + warnCount + ' · 总计 ' + formatCacheSize(totalSize) + '</span>';
    html += '<div class="startup-cache-actions">';
    html += '<button class="startup-cache-btn startup-cache-btn-danger cache-del-btn" data-id="' + id + '">删除此记录</button>';
    html += '</div>';
    html += '</div>';

    // 操作栏：全选 + 清理选中
    html += '<div class="startup-cache-bar">';
    html += '<label class="cleanup-select-all"><input type="checkbox" id="cacheSelectAll"><span>全选</span></label>';
    html += '<span id="cacheTotal" class="cleanup-total"></span>';
    html += '<button id="cacheDeleteBtn" class="btn btn-danger">清理选中</button>';
    html += '</div>';
    html += '</div>';

    // 缓存条目列表
    items.forEach((c, idx) => {
        const safeTag = c.safe
            ? '<span class="cache-tag cache-tag-safe">可清理</span>'
            : '<span class="cache-tag cache-tag-warn">谨慎</span>';
        html += `
            <div class="cleanup-item">
                <input type="checkbox" class="cache-check" data-idx="${idx}">
                <div class="cleanup-item-info">
                    <div class="cleanup-item-name">${c.zhName || c.path} ${safeTag}</div>
                    ${c.desc ? `<div class="cleanup-item-desc">${c.desc}</div>` : ''}
                    <div class="cleanup-item-path">${c.path}</div>
                </div>
                <div class="cleanup-item-stats">
                    <div class="cleanup-item-size">${c.size > 0 ? formatCacheSize(c.size) : '空'}</div>
                    <div class="cleanup-item-count">${c.count > 0 ? c.count + ' 个文件' : '-'}</div>
                </div>
            </div>`;
    });

    cacheListEl.innerHTML = html;
    updateCacheTotal();
}

function updateCacheTotal() {
    const checks = cacheListEl.querySelectorAll('.cache-check:checked');
    let totalSize = 0, totalFiles = 0;
    checks.forEach(c => {
        const item = cacheDetailItems[parseInt(c.dataset.idx)];
        if (item) { totalSize += item.size; totalFiles += item.count; }
    });
    const total = document.getElementById('cacheTotal');
    if (total) total.textContent = `已选 ${formatCacheSize(totalSize)} · ${totalFiles} 个文件`;
}

// 删除缓存记录
async function deleteCacheCache(id) {
    if (!confirm('确定删除此缓存记录？')) return;
    try {
        await fetch('/api/cache/cache?id=' + encodeURIComponent(id), { method: 'DELETE' });
        loadCacheCacheList();
    } catch (e) {
        alert('删除失败: ' + e.message);
    }
}

// 清理选中的缓存
async function deleteCacheSelected() {
    const paths = Array.from(cacheListEl.querySelectorAll('.cache-check:checked'))
        .map(c => cacheDetailItems[parseInt(c.dataset.idx)]?.path)
        .filter(Boolean);
    if (paths.length === 0) return;

    const deleteBtn = document.getElementById('cacheDeleteBtn');
    if (deleteBtn) {
        deleteBtn.disabled = true;
        deleteBtn.innerHTML = '<span class="cleanup-btn-spinner"></span> 清理中...';
    }

    try {
        const resp = await fetch('/api/cache/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ paths, recordId: cacheDetailId }),
        });
        const data = await resp.json();
        renderCacheResult(data);
    } catch (e) {
        cacheStatus.classList.remove('hidden');
        cacheListEl.classList.add('hidden');
        cacheStatus.innerHTML = `<p class="cleanup-error-msg">清理失败: ${e.message}</p>`;
    } finally {
        if (deleteBtn) {
            deleteBtn.disabled = false;
            deleteBtn.textContent = '清理选中';
        }
    }
}

function renderCacheResult(data) {
    cacheStatus.classList.add('hidden');
    cacheListEl.classList.remove('hidden');

    const results = data.results || [];
    const totalCleaned = data.totalCleaned || 0;
    const failedCount = data.failedCount || 0;

    let html = '<div class="scan-sticky-header">';
    html += '<div class="startup-back-bar">';
    html += '<button class="startup-back-btn">← 返回记录列表</button>';
    html += '</div>';
    html += '</div>';

    html += `<div class="cleanup-result-title">✅ 缓存清理完成</div>`;

    if (failedCount > 0) {
        html += `<div class="cleanup-result-item" style="margin-bottom:8px"><span class="cleanup-result-name" style="color:var(--danger)">⚠ ${failedCount} 项无法清理（文件被占用）或右键引用打开文件夹需要手动清理</span></div>`;
    }

    results.forEach(r => {
        const detail = r.error
            ? `<span class="cleanup-result-error">⚠ ${r.error}</span>`
            : `<span class="cleanup-result-detail">已释放 ${formatCacheSize(r.size)}</span>`;
        html += `<div class="cleanup-result-item"><span class="cleanup-result-name">${r.path}</span>${detail}</div>`;
    });
    html += `<div class="cleanup-result-total">总计释放 ${formatCacheSize(totalCleaned)}</div>`;
    html += `<div style="margin-top:12px"><button class="btn" id="cacheRefreshRecordBtn">刷新记录</button></div>`;

    cacheListEl.innerHTML = html;
}

// 事件委托
cacheListEl.addEventListener('click', e => {
    // 返回按钮
    if (e.target.classList.contains('startup-back-btn')) {
        loadCacheCacheList();
        return;
    }
    // 查看按钮
    const viewBtn = e.target.closest('.startup-view-btn');
    if (viewBtn) {
        viewCacheCache(viewBtn.dataset.id);
        return;
    }
    // 删除记录按钮
    const delBtn = e.target.closest('.cache-del-btn');
    if (delBtn) {
        deleteCacheCache(delBtn.dataset.id);
        return;
    }
    // 清理选中按钮
    const cacheDeleteBtn = e.target.closest('#cacheDeleteBtn');
    if (cacheDeleteBtn) {
        deleteCacheSelected();
        return;
    }
    // 刷新记录按钮
    if (e.target.id === 'cacheRefreshRecordBtn') {
        if (cacheDetailId) viewCacheCache(cacheDetailId);
        return;
    }
});

// 全选 / 复选框变化
cacheListEl.addEventListener('change', e => {
    if (e.target.id === 'cacheSelectAll') {
        cacheListEl.querySelectorAll('.cache-check').forEach(c => c.checked = e.target.checked);
    }
    updateCacheTotal();
});

// 右键打开文件所在位置
cacheListEl.addEventListener('contextmenu', e => {
    const item = e.target.closest('.cleanup-item');
    if (!item) return;
    const cb = item.querySelector('.cache-check');
    if (!cb) return;
    const it = cacheDetailItems[parseInt(cb.dataset.idx)];
    if (it && it.path) {
        e.preventDefault();
        showCtxMenu(e.clientX, e.clientY, it.path);
    }
});

// 重新扫描
cacheRescanBtn.addEventListener('click', async () => {
    cacheRescanBtn.disabled = true;
    cacheRescanBtn.textContent = '正在扫描...';
    const depth = document.getElementById('cacheDepthInput')?.value || 3;
    cacheStatus.innerHTML = `
        <div class="cleanup-scanning">
            <div class="cleanup-scan-spinner"></div>
            <span>正在扫描缓存目录并调用 AI 识别，请稍候...</span>
        </div>`;
    cacheStatus.classList.remove('hidden');
    cacheListEl.classList.add('hidden');

    try {
        await fetch('/api/cache/list?depth=' + encodeURIComponent(depth));
        await loadCacheCacheList();
    } catch (e) {
        cacheStatus.innerHTML = `<p class="cleanup-error-msg">扫描失败: ${e.message}</p>`;
    } finally {
        cacheRescanBtn.disabled = false;
        cacheRescanBtn.textContent = '重新扫描';
    }
});
