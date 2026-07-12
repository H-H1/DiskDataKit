// 常规清理模块
const cleanupModal = document.getElementById('cleanupModal');
const cleanupBtn = document.getElementById('cleanupBtn');
const cleanupClose = document.getElementById('cleanupClose');
const cleanupStatus = document.getElementById('cleanupStatus');
const cleanupList = document.getElementById('cleanupList');
const cleanupActions = document.getElementById('cleanupActions');
const cleanupTotal = document.getElementById('cleanupTotal');
const cleanupSelectAll = document.getElementById('cleanupSelectAll');
const cleanupDeleteBtn = document.getElementById('cleanupDeleteBtn');
const cleanupCancel = document.getElementById('cleanupCancel');
const cleanupResult = document.getElementById('cleanupResult');

let cleanupTargets = [];

// 打开弹窗
cleanupBtn.addEventListener('click', () => {
    cleanupModal.classList.remove('hidden');
});

// 关闭弹窗
cleanupClose.addEventListener('click', () => cleanupModal.classList.add('hidden'));
cleanupCancel.addEventListener('click', () => cleanupModal.classList.add('hidden'));
cleanupModal.addEventListener('click', e => {
    if (e.target === cleanupModal) cleanupModal.classList.add('hidden');
});

// 初始扫描按钮
const initialScanHTML = `
    <div class="cleanup-scan-icon">🧹</div>
    <p class="cleanup-scan-hint">点击下方按钮，扫描系统中的临时文件和缓存</p>
    <button class="btn btn-cleanup-scan">开始扫描</button>`;

function showInitialScan() {
    cleanupStatus.innerHTML = initialScanHTML;
    cleanupStatus.classList.remove('hidden');
    cleanupList.classList.add('hidden');
    cleanupActions.classList.add('hidden');
    cleanupResult.classList.add('hidden');
}

// 事件委托：处理扫描/重新扫描按钮点击
document.addEventListener('click', e => {
    if (e.target.classList.contains('btn-cleanup-scan') || e.target.classList.contains('btn-rescan')) {
        startScan();
    }
});

showInitialScan();

// 开始扫描
async function startScan() {
    cleanupStatus.innerHTML = `
        <div class="cleanup-scanning">
            <div class="cleanup-scan-spinner"></div>
            <span>正在扫描系统临时文件...</span>
        </div>`;
    cleanupStatus.classList.remove('hidden');
    cleanupList.classList.add('hidden');
    cleanupActions.classList.add('hidden');
    cleanupResult.classList.add('hidden');

    try {
        const resp = await fetch('/api/cleanup/scan');
        const data = await resp.json();

        if (data.targets && data.targets.length > 0) {
            cleanupTargets = data.targets;
            renderCleanupList(data);
        } else {
            cleanupStatus.innerHTML = `<p class="cleanup-empty-msg">${data.message || '未找到可清理的项目'}</p>`;
        }
    } catch (e) {
        cleanupStatus.innerHTML = `<p class="cleanup-error-msg">扫描失败: ${e.message}</p>`;
    }
}

// 格式化文件大小
function formatCleanupSize(bytes) {
    if (bytes === 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i];
}

// 渲染清理列表
function renderCleanupList(data) {
    cleanupStatus.classList.add('hidden');
    cleanupList.classList.remove('hidden');
    cleanupActions.classList.remove('hidden');
    cleanupResult.classList.add('hidden');

    cleanupList.innerHTML = data.targets.map(t => `
        <div class="cleanup-item ${t.size === 0 ? 'cleanup-item-empty' : ''}" data-path="${t.path}">
            <input type="checkbox" class="cleanup-check" data-id="${t.id}" checked ${t.size === 0 ? 'disabled' : ''}>
            <div class="cleanup-item-info">
                <div class="cleanup-item-name">${t.name}</div>
                <div class="cleanup-item-path">${t.path}</div>
                <div class="cleanup-item-desc">${t.desc}</div>
            </div>
            <div class="cleanup-item-stats">
                <div class="cleanup-item-size">${t.size > 0 ? formatCleanupSize(t.size) : '空'}</div>
                <div class="cleanup-item-count">${t.count > 0 ? t.count + ' 个文件' : '-'}</div>
            </div>
        </div>
    `).join('');

    cleanupSelectAll.checked = true;
    updateCleanupTotal();
}

// 更新总计
function updateCleanupTotal() {
    const checks = cleanupList.querySelectorAll('.cleanup-check:checked');
    let totalSize = 0;
    let totalFiles = 0;
    checks.forEach(c => {
        const target = cleanupTargets.find(t => t.id === c.dataset.id);
        if (target) {
            totalSize += target.size;
            totalFiles += target.count;
        }
    });
    cleanupTotal.textContent = `已选 ${formatCleanupSize(totalSize)} · ${totalFiles} 个文件`;
}

// 全选/取消全选
cleanupSelectAll.addEventListener('change', e => {
    cleanupList.querySelectorAll('.cleanup-check:not(:disabled)').forEach(c => {
        c.checked = e.target.checked;
    });
    updateCleanupTotal();
});

// 单项切换
cleanupList.addEventListener('change', updateCleanupTotal);

// 右键打开文件所在位置
cleanupList.addEventListener('contextmenu', e => {
    const item = e.target.closest('.cleanup-item');
    if (item && item.dataset.path) {
        e.preventDefault();
        showCtxMenu(e.clientX, e.clientY, item.dataset.path);
    }
});

// 执行清理
cleanupDeleteBtn.addEventListener('click', async () => {
    const ids = Array.from(cleanupList.querySelectorAll('.cleanup-check:checked'))
        .map(c => c.dataset.id);

    if (ids.length === 0) return;

    cleanupDeleteBtn.disabled = true;
    cleanupDeleteBtn.innerHTML = '<span class="cleanup-btn-spinner"></span> 清理中...';

    try {
        const resp = await fetch('/api/cleanup/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ ids })
        });
        const data = await resp.json();
        renderCleanupResult(data);
    } catch (e) {
        cleanupResult.classList.remove('hidden');
        cleanupResult.innerHTML = `<p class="cleanup-error-msg">清理失败: ${e.message}</p>`;
    } finally {
        cleanupDeleteBtn.disabled = false;
        cleanupDeleteBtn.textContent = '清理选中';
    }
});

// 渲染清理结果
function renderCleanupResult(data) {
    cleanupActions.classList.add('hidden');
    cleanupList.classList.add('hidden');
    cleanupResult.classList.remove('hidden');

    let html = `<div class="cleanup-result-title">✅ 清理完成 · ${data.timestamp}</div>`;

    if (data.results) {
        data.results.forEach(r => {
            const detail = r.error
                ? `<span class="cleanup-result-error">⚠ ${r.error}</span>`
                : `<span class="cleanup-result-detail">已删除 ${r.count} 个文件 · ${formatCleanupSize(r.size)}</span>`;
            html += `
                <div class="cleanup-result-item">
                    <span class="cleanup-result-name">${r.name}</span>
                    ${detail}
                </div>`;
        });
    }

    html += `<div class="cleanup-result-total">总计释放 ${formatCleanupSize(data.totalDeleted || 0)}</div>`;
    html += `<button class="btn btn-cleanup-scan btn-rescan" style="margin-top:12px;width:100%">重新扫描</button>`;
    cleanupResult.innerHTML = html;
}
