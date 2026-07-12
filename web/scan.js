// 流氓软件检测模块
const scanModal = document.getElementById('scanModal');
const scanClose = document.getElementById('scanClose');
const scanStatus = document.getElementById('scanStatus');
const scanList = document.getElementById('scanList');

const scanCatNames = {
    installed: '已安装程序',
    process: '运行中进程',
    file: '可疑文件',
    startup: '启动项',
};
const verdictNames = { safe: '安全', suspicious: '可疑', unknown: '未知' };
let scanItemsCache = [];

document.getElementById('homeScanBtn').onclick = () => {
    scanModal.classList.remove('hidden');
    loadScanItems();
};

scanClose.addEventListener('click', () => scanModal.classList.add('hidden'));
scanModal.addEventListener('click', e => {
    if (e.target === scanModal) scanModal.classList.add('hidden');
});

async function loadScanItems() {
    scanStatus.innerHTML = `
        <div class="cleanup-scanning">
            <div class="cleanup-scan-spinner"></div>
            <span>正在扫描系统并调用 AI 判断，请稍候...</span>
        </div>`;
    scanStatus.classList.remove('hidden');
    scanList.classList.add('hidden');

    try {
        const resp = await fetch('/api/scan/list');
        const data = await resp.json();
        renderScanList(data.items || []);
    } catch (e) {
        scanStatus.innerHTML = `<p class="cleanup-error-msg">扫描失败: ${e.message}</p>`;
    }
}

function renderScanList(items) {
    if (items.length === 0) {
        scanStatus.innerHTML = '<p class="cleanup-empty-msg">未扫描到任何项目</p>';
        return;
    }
    scanItemsCache = items;
    scanStatus.classList.add('hidden');
    scanList.classList.remove('hidden');

    const susCount = items.filter(i => i.verdict === 'suspicious').length;
    const cats = {};
    items.forEach((it, idx) => {
        if (!cats[it.category]) cats[it.category] = [];
        cats[it.category].push({ ...it, _idx: idx });
    });
    const order = ['installed', 'process', 'file', 'startup'];

    let html = `<div class="scan-summary">共 ${items.length} 项 · 可疑 <span class="scan-sus-count">${susCount}</span></div>`;
    order.forEach(cat => {
        if (!cats[cat]) return;
        const list = cats[cat];
        html += `<div class="startup-group">`;
        html += `<div class="startup-group-title">${scanCatNames[cat] || cat}<span class="startup-group-count">${list.length}</span></div>`;
        list.forEach(it => {
            const v = it.verdict || 'unknown';
            const vName = verdictNames[v] || v;
            const orig = it.zhName && it.zhName !== it.name ? ` <span class="scan-item-orig">${it.name}</span>` : '';
            html += `
                <div class="scan-item scan-verdict-${v}" data-idx="${it._idx}">
                    <div class="scan-item-info">
                        <div class="scan-item-name">${it.zhName || it.name}${orig}</div>
                        <div class="startup-item-path">${it.path || '-'}</div>
                        ${it.publisher ? `<div class="scan-item-pub">发布者: ${it.publisher}</div>` : ''}
                        ${it.reason ? `<div class="scan-item-reason">${it.reason}</div>` : ''}
                    </div>
                    <span class="scan-verdict-badge scan-verdict-${v}">${vName}</span>
                </div>`;
        });
        html += `</div>`;
    });
    scanList.innerHTML = html;
}

// 右键打开文件所在位置（复用全局 showCtxMenu）
scanList.addEventListener('contextmenu', e => {
    const item = e.target.closest('.scan-item');
    if (!item) return;
    const idx = parseInt(item.dataset.idx);
    const it = scanItemsCache[idx];
    if (it && it.path) {
        e.preventDefault();
        showCtxMenu(e.clientX, e.clientY, it.path);
    }
});
