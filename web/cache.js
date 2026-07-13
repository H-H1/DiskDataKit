// 软件缓存清理模块
const cacheModal = document.getElementById('cacheModal');
const cacheClose = document.getElementById('cacheClose');
const cacheStatus = document.getElementById('cacheStatus');
const cacheListEl = document.getElementById('cacheList');
const cacheActions = document.getElementById('cacheActions');
const cacheTotal = document.getElementById('cacheTotal');
const cacheSelectAll = document.getElementById('cacheSelectAll');
const cacheDeleteBtn = document.getElementById('cacheDeleteBtn');
const cacheCancel = document.getElementById('cacheCancel');
const cacheResult = document.getElementById('cacheResult');

let cacheItemsCache = [];

document.getElementById('homeCacheBtn').onclick = () => {
    cacheModal.classList.remove('hidden');
    loadCacheList();
};
cacheClose.addEventListener('click', () => cacheModal.classList.add('hidden'));
cacheCancel.addEventListener('click', () => cacheModal.classList.add('hidden'));
cacheModal.addEventListener('click', e => {
    if (e.target === cacheModal) cacheModal.classList.add('hidden');
});

async function loadCacheList() {
    cacheStatus.innerHTML = `
        <div class="cleanup-scanning">
            <div class="cleanup-scan-spinner"></div>
            <span>正在扫描缓存目录并调用 AI 识别，请稍候...</span>
        </div>`;
    cacheStatus.classList.remove('hidden');
    cacheListEl.classList.add('hidden');
    cacheActions.classList.add('hidden');
    cacheResult.classList.add('hidden');

    try {
        const resp = await fetch('/api/cache/list');
        const data = await resp.json();
        renderCacheList(data.items || []);
    } catch (e) {
        cacheStatus.innerHTML = `<p class="cleanup-error-msg">扫描失败: ${e.message}</p>`;
    }
}

function formatCacheSize(bytes) {
    if (bytes < 1024) return bytes + ' B';
    const units = ['KB', 'MB', 'GB', 'TB'];
    let v = bytes / 1024, i = 0;
    while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
    return v.toFixed(1) + ' ' + units[i];
}

function renderCacheList(items) {
    if (items.length === 0) {
        cacheStatus.innerHTML = '<p class="cleanup-empty-msg">未发现可清理的缓存</p>';
        return;
    }
    cacheItemsCache = items;
    cacheStatus.classList.add('hidden');
    cacheListEl.classList.remove('hidden');
    cacheActions.classList.remove('hidden');
    cacheResult.classList.add('hidden');

    // 按大小降序
    items.sort((a, b) => b.size - a.size);

    let html = '';
    items.forEach((c, idx) => {
        const safeTag = c.safe
            ? '<span class="cache-tag cache-tag-safe">可清理</span>'
            : '<span class="cache-tag cache-tag-warn">谨慎</span>';
        html += `
            <div class="cleanup-item">
                <input type="checkbox" class="cache-check" data-idx="${idx}" checked>
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
    cacheSelectAll.checked = true;
    updateCacheTotal();
}

function updateCacheTotal() {
    const checks = cacheListEl.querySelectorAll('.cache-check:checked');
    let totalSize = 0, totalFiles = 0;
    checks.forEach(c => {
        const item = cacheItemsCache[parseInt(c.dataset.idx)];
        if (item) { totalSize += item.size; totalFiles += item.count; }
    });
    cacheTotal.textContent = `已选 ${formatCacheSize(totalSize)} · ${totalFiles} 个文件`;
}

cacheSelectAll.addEventListener('change', e => {
    cacheListEl.querySelectorAll('.cache-check').forEach(c => c.checked = e.target.checked);
    updateCacheTotal();
});
cacheListEl.addEventListener('change', updateCacheTotal);

// 右键打开文件所在位置
cacheListEl.addEventListener('contextmenu', e => {
    const item = e.target.closest('.cleanup-item');
    if (!item) return;
    const cb = item.querySelector('.cache-check');
    if (!cb) return;
    const it = cacheItemsCache[parseInt(cb.dataset.idx)];
    if (it && it.path) {
        e.preventDefault();
        showCtxMenu(e.clientX, e.clientY, it.path);
    }
});

cacheDeleteBtn.addEventListener('click', async () => {
    const paths = Array.from(cacheListEl.querySelectorAll('.cache-check:checked'))
        .map(c => cacheItemsCache[parseInt(c.dataset.idx)]?.path)
        .filter(Boolean);
    if (paths.length === 0) return;

    cacheDeleteBtn.disabled = true;
    cacheDeleteBtn.innerHTML = '<span class="cleanup-btn-spinner"></span> 清理中...';
    try {
        const resp = await fetch('/api/cache/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ paths }),
        });
        const data = await resp.json();
        renderCacheResult(data);
    } catch (e) {
        cacheResult.classList.remove('hidden');
        cacheResult.innerHTML = `<p class="cleanup-error-msg">清理失败: ${e.message}</p>`;
    } finally {
        cacheDeleteBtn.disabled = false;
        cacheDeleteBtn.textContent = '清理选中';
    }
});

function renderCacheResult(data) {
    cacheActions.classList.add('hidden');
    cacheListEl.classList.add('hidden');
    cacheResult.classList.remove('hidden');
    let total = 0;
    let html = `<div class="cleanup-result-title">✅ 缓存清理完成</div>`;
    (data.results || []).forEach(r => {
        total += r.size;
        const detail = r.error
            ? `<span class="cleanup-result-error">⚠ ${r.error}</span>`
            : `<span class="cleanup-result-detail">已释放 ${formatCacheSize(r.size)}</span>`;
        html += `<div class="cleanup-result-item"><span class="cleanup-result-name">${r.path}</span>${detail}</div>`;
    });
    html += `<div class="cleanup-result-total">总计释放 ${formatCacheSize(total)}</div>`;
    html += `<button class="btn btn-accent btn-cache-rescan" style="margin-top:12px;width:100%">重新扫描</button>`;
    cacheResult.innerHTML = html;
}

document.addEventListener('click', e => {
    if (e.target.classList.contains('btn-cache-rescan')) {
        loadCacheList();
    }
});
