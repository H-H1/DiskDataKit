// DiskDataKit 前端逻辑 - GitHub 风格
const fileList = document.getElementById('fileList');
const breadcrumb = document.getElementById('breadcrumb');
const statusEl = document.getElementById('status');
const emptyEl = document.getElementById('empty');
const skeletonEl = document.getElementById('skeleton');
const footerInfo = document.getElementById('footerInfo');
const driveSelect = document.getElementById('driveSelect');
const countBadge = document.getElementById('countBadge');
const totalSizeBadge = document.getElementById('totalSizeBadge');

// 帕累托图元素
const chartCard = document.getElementById('chartCard');
const paretoCanvas = document.getElementById('paretoCanvas');
const chartTooltip = document.getElementById('chartTooltip');
const chartHint = document.getElementById('chartHint');
const chartCtx = paretoCanvas.getContext('2d');
const expandSection = document.getElementById('expandSection');
const expandLabel = document.getElementById('expandLabel');
const expandCanvas = document.getElementById('expandCanvas');
const expandCtx = expandCanvas.getContext('2d');
const expandTooltip = document.getElementById('expandTooltip');
const expandModal = document.getElementById('expandModal');
const expandInput = document.getElementById('expandInput');
const expandMoreBtn = document.getElementById('expandMore');
const remainCountEl = document.getElementById('remainCount');
const ctxMenu = document.getElementById('ctxMenu');
const ctxOpenExplorer = document.getElementById('ctxOpenExplorer');
let ctxTargetPath = '';   // 当前右键菜单对应的文件路径

let currentPath = '';
let loadToken = 0;
let recentFolders = [];

// 帕累托图状态
let chartItems = [];          // [{name, size, isDir, path, computed}]
let chartData = [];           // 排序后的绘图数据
let chartAnimId = null;
let chartHoverIdx = -1;
let chartBarRects = [];       // 每个条的命中区域 [{x, y, w, h, idx}]
let chartResizeTimer = null;
const MAX_BARS = 40;

// 折线开关
let showSelfLine = true;
let showCumulativeLine = true;

// "其他"展开
let chartTailItems = [];      // 被合并到"其他"的尾部项
let otherBarRect = null;      // "其他"条形的命中区域
let expandData = [];          // 展开图的数据
let expandAnimId = null;
let expandBarRects = [];      // 展开图条形命中区域
let expandHoverIdx = -1;      // 展开图悬停索引
let modalDefaultCount = 50;   // 弹框默认值

// 总大小统计
let totalSize = 0;
let dirComputingCount = 0;

// 心跳：定期通知后端前端仍在线，关闭浏览器后后端自动退出
setInterval(() => {
    fetch('/api/heartbeat', { method: 'POST' }).catch(() => {});
}, 10000);
// 页面卸载时立即发一次，刷新时避免后端误退
window.addEventListener('beforeunload', () => {
    navigator.sendBeacon('/api/heartbeat');
});

// 根据扩展名映射文件图标
const ICONS = {
    folder: '📁',
    txt: '📄', md: '📝', pdf: '📕', doc: '📘', docx: '📘',
    xls: '📗', xlsx: '📗', ppt: '📙', pptx: '📙',
    zip: '🗜️', rar: '🗜️', gz: '🗜️', '7z': '🗜️',
    mp3: '🎵', wav: '🎵', flac: '🎵',
    mp4: '🎬', mkv: '🎬', avi: '🎬', mov: '🎬',
    jpg: '🖼️', jpeg: '🖼️', png: '🖼️', gif: '🖼️', bmp: '🖼️',
    exe: '⚙️', msi: '⚙️', bat: '⚙️',
    go: '🐹', js: '📜', ts: '📜', py: '🐍', java: '☕',
    json: '🔧', xml: '🔧', yml: '🔧', yaml: '🔧',
    html: '🌐', css: '🎨',
};

function iconFor(ext) {
    return ICONS[ext] || '📄';
}

// 人类可读的文件大小
function formatSize(bytes) {
    if (bytes < 1024) return bytes + ' B';
    const units = ['KB', 'MB', 'GB', 'TB'];
    let val = bytes / 1024, i = 0;
    while (val >= 1024 && i < units.length - 1) {
        val /= 1024;
        i++;
    }
    return val.toFixed(1) + ' ' + units[i];
}

// 相对时间（如 "2 分钟前"）
function relativeTime(dateStr) {
    const d = new Date(dateStr);
    if (isNaN(d.getTime())) return '-';
    const diff = Date.now() - d.getTime();
    const sec = Math.floor(diff / 1000);
    if (sec < 60) return '刚刚';
    const min = Math.floor(sec / 60);
    if (min < 60) return min + ' 分钟前';
    const hr = Math.floor(min / 60);
    if (hr < 24) return hr + ' 小时前';
    const day = Math.floor(hr / 24);
    if (day < 7) return day + ' 天前';
    // 超过一周显示完整日期
    return d.toLocaleDateString('zh-CN', {
        year: 'numeric', month: '2-digit', day: '2-digit',
    }) + ' ' + d.toLocaleTimeString('zh-CN', {
        hour: '2-digit', minute: '2-digit',
    });
}

function showSkeleton(show) {
    skeletonEl.classList.toggle('hidden', !show);
    if (show) {
        const table = document.querySelector('.file-table');
        if (table) table.style.visibility = 'hidden';
        emptyEl.classList.add('hidden');
    } else {
        const table = document.querySelector('.file-table');
        if (table) table.style.visibility = '';
    }
}

function showStatus(msg, isError) {
    statusEl.textContent = msg;
    statusEl.className = isError ? 'status error' : 'status';
}

function clearStatus() {
    statusEl.className = 'status hidden';
    statusEl.textContent = '';
}

// 检查当前是否有文件夹大小正在计算中（通过 DOM 状态判断，不依赖计数器）
function hasComputingTasks() {
    const cells = fileList.querySelectorAll('.col-size');
    let count = 0;
    cells.forEach(cell => {
        if (cell.textContent === '计算中...' || cell.dataset.computing === '1') {
            count++;
        }
    });
    return count;
}

// 加载指定路径的文件列表
async function loadPath(path) {
    // 有正在计算中的文件夹大小任务时，提示用户确认是否跳转
    const pending = hasComputingTasks();
    if (pending > 0) {
        const ok = confirm(
            `当前有 ${pending} 个文件夹大小正在后台计算中。\n` +
            `跳转到新目录会导致正在进行的计算任务中断，已计算的部分结果可能丢失。\n\n` +
            `确定要跳转吗？`
        );
        if (!ok) return;
    }

    const token = ++loadToken;
    showSkeleton(true);
    clearStatus();
    countBadge.classList.add('hidden');
    totalSizeBadge.classList.add('hidden');
    totalSize = 0;
    dirComputingCount = 0;
    fileList.innerHTML = '';
    chartCard.classList.add('hidden');
    chartCard.dataset.shown = '';
    chartItems = [];
    chartData = [];
    chartTailItems = [];
    otherBarRect = null;
    expandSection.classList.add('hidden');
    expandData = [];
    expandBarRects = [];
    expandHoverIdx = -1;
    expandTooltip.classList.add('hidden');
    expandMoreBtn.classList.add('hidden');

    try {
        const url = '/api/files' + (path ? '?path=' + encodeURIComponent(path) : '');
        const res = await fetch(url);
        const data = await res.json();

        // 防止旧请求覆盖新请求结果
        if (token !== loadToken) return;

        showSkeleton(false);

        if (data.error) {
            showStatus('错误: ' + data.error, true);
            footerInfo.textContent = '';
            return;
        }

        currentPath = data.path;
        renderBreadcrumb(data.path, data.parent, data.isRoot);
        renderList(data.items);

        // 更新统计徽标
        const dirs = data.items.filter(i => i.isDir).length;
        const files = data.items.length - dirs;
        countBadge.textContent = data.items.length;
        countBadge.classList.remove('hidden');

        footerInfo.textContent = `${data.items.length} 项（${dirs} 个文件夹 · ${files} 个文件）`;
    } catch (err) {
        if (token !== loadToken) return;
        showSkeleton(false);
        showStatus('请求失败: ' + err.message, true);
    }
}

// 渲染面包屑导航
function renderBreadcrumb(path, parent, isRoot) {
    breadcrumb.innerHTML = '';
    if (!path) {
        const span = document.createElement('span');
        span.className = 'crumb is-last';
        span.textContent = '加载中...';
        breadcrumb.appendChild(span);
        return;
    }

    const sep = path.includes('\\') ? '\\' : '/';
    const parts = path.split(sep).filter(Boolean);
    const isWindowsDrive = /^[A-Za-z]:$/.test(parts[0]);

    // 无可见部分的情况（如 Unix 根目录），直接显示完整路径
    if (parts.length === 0) {
        const span = document.createElement('span');
        span.className = 'crumb is-last';
        span.textContent = path;
        breadcrumb.appendChild(span);
        return;
    }

    let acc = '';
    parts.forEach((part, idx) => {
        if (idx === 0 && isWindowsDrive) {
            acc = part + sep;
        } else {
            acc = acc ? acc + sep + part : part;
        }

        // 固定当前累积路径，避免闭包共享被覆盖
        const target = acc;
        const crumb = document.createElement('span');
        crumb.className = 'crumb' + (idx === parts.length - 1 ? ' is-last' : '');
        crumb.textContent = part;
        if (idx < parts.length - 1) {
            crumb.onclick = () => loadPath(target);
        }
        breadcrumb.appendChild(crumb);

        if (idx < parts.length - 1) {
            const sepSpan = document.createElement('span');
            sepSpan.className = 'crumb-sep';
            sepSpan.textContent = sep;
            breadcrumb.appendChild(sepSpan);
        }
    });
}

// 渲染文件列表（带错位淡入动画）
function renderList(items) {
    fileList.innerHTML = '';
    clearStatus();

    if (!items || items.length === 0) {
        emptyEl.classList.remove('hidden');
        totalSizeBadge.classList.add('hidden');
        return;
    }
    emptyEl.classList.add('hidden');

    // 统计文件总大小 + 待计算目录数
    totalSize = 0;
    dirComputingCount = 0;
    items.forEach(item => {
        if (item.isDir) {
            dirComputingCount++;
        } else {
            totalSize += item.size;
        }
    });

    // 有目录需计算时显示"计算中"，否则直接显示总大小
    if (dirComputingCount > 0) {
        totalSizeBadge.textContent = '总大小计算中...';
        totalSizeBadge.className = 'size-badge computing';
    } else {
        totalSizeBadge.textContent = '总大小 ' + formatSize(totalSize);
        totalSizeBadge.className = 'size-badge done';
    }
    totalSizeBadge.classList.remove('hidden');

    items.forEach((item, idx) => {
        const tr = document.createElement('tr');
        tr.className = 'file-row' + (item.isDir ? ' is-dir' : '');
        // 错位淡入动画，限制最大延迟避免过多
        tr.style.animationDelay = Math.min(idx * 20, 400) + 'ms';

        const nameCell = document.createElement('td');
        nameCell.className = 'file-name';
        const icon = document.createElement('span');
        icon.className = 'file-icon';
        icon.textContent = item.isDir ? ICONS.folder : iconFor(item.ext);
        const name = document.createElement('span');
        name.className = 'name-text';
        name.textContent = item.name;
        nameCell.appendChild(icon);
        nameCell.appendChild(name);

        const sizeCell = document.createElement('td');
        sizeCell.className = 'col-size';
        if (item.isDir) {
            sizeCell.textContent = '计算中...';
            sizeCell.style.opacity = '0.5';
            fetchDirSize(item.path, sizeCell);
        } else {
            sizeCell.textContent = formatSize(item.size);
        }

        const dateCell = document.createElement('td');
        dateCell.className = 'col-date';
        dateCell.textContent = relativeTime(item.modTime);
        dateCell.title = new Date(item.modTime).toLocaleString('zh-CN');

        tr.appendChild(nameCell);
        tr.appendChild(sizeCell);
        tr.appendChild(dateCell);

        if (item.isDir) {
            tr.onclick = () => loadPath(item.path);
        }

        // 右键打开上下文菜单
        tr.addEventListener('contextmenu', e => {
            e.preventDefault();
            showCtxMenu(e.clientX, e.clientY, item.path);
        });

        fileList.appendChild(tr);
    });

    // 初始化帕累托图数据
    initChartData(items);
}

// 并发获取文件夹大小，逐步更新对应单元格
function fetchDirSize(path, cell) {
    const myToken = loadToken;
    cell.dataset.computing = '1';
    fetch('/api/size?path=' + encodeURIComponent(path))
        .then(r => r.json())
        .then(data => {
            if (myToken !== loadToken) return;
            if (data.error) {
                cell.textContent = '—';
                cell.style.opacity = '';
            } else {
                cell.textContent = formatSize(data.size);
                cell.style.transition = 'opacity 0.4s ease';
                cell.style.opacity = '1';
                updateChartItemSize(path, data.size);
                // 累加到总大小
                totalSize += data.size;
            }
        })
        .catch(() => {
            if (myToken !== loadToken) return;
            cell.textContent = '—';
            cell.style.opacity = '';
        })
        .finally(() => {
            cell.removeAttribute('data-computing');
            // 无论成功失败，计算任务结束
            if (myToken !== loadToken) return;
            dirComputingCount--;
            if (dirComputingCount <= 0) {
                dirComputingCount = 0;
                totalSizeBadge.textContent = '总大小 ' + formatSize(totalSize);
                totalSizeBadge.className = 'size-badge done';
            }
        });
}

// ==================== 帕累托图 ====================

// 初始化图表数据
function initChartData(items) {
    chartItems = items.map(item => ({
        name: item.name,
        size: item.isDir ? 0 : item.size,
        isDir: item.isDir,
        path: item.path,
        computed: !item.isDir
    }));
    updateChart();
}

// 更新某个条目的尺寸（文件夹异步计算完成后调用）
function updateChartItemSize(path, size) {
    const item = chartItems.find(i => i.path === path);
    if (item) {
        item.size = size;
        item.computed = true;
        updateChart();
    }
}

// 汇总数据并触发绘图
function updateChart() {
    const computed = chartItems.filter(i => i.computed && i.size > 0);
    if (computed.length < 2) {
        chartCard.classList.add('hidden');
        return;
    }
    computed.sort((a, b) => b.size - a.size);

    // 超过 MAX_BARS 则合并尾部为"其他"
    if (computed.length > MAX_BARS) {
        const head = computed.slice(0, MAX_BARS - 1);
        chartTailItems = computed.slice(MAX_BARS - 1);
        const tailSum = chartTailItems.reduce((s, d) => s + d.size, 0);
        head.push({ name: `其他 (${chartTailItems.length} 项)`, size: tailSum, isDir: false, path: '', computed: true });
        chartData = head;
    } else {
        chartTailItems = [];
        chartData = computed;
    }

    // 有新数据时隐藏展开区
    if (chartTailItems.length === 0) {
        expandSection.classList.add('hidden');
    }

    // 是否仍有未计算的条目
    const pending = chartItems.filter(i => !i.computed).length;
    chartHint.classList.toggle('hidden', pending === 0);
    if (pending > 0) chartHint.textContent = `${pending} 项计算中`;

    chartCard.classList.remove('hidden');
    // 首次显示播放动画，后续更新静默重绘避免反复重启
    if (chartCard.dataset.shown === '1') {
        if (chartAnimId) { cancelAnimationFrame(chartAnimId); chartAnimId = null; }
        drawPareto(1);
    } else {
        chartCard.dataset.shown = '1';
        startChartAnim();
    }
}

// 启动动画
function startChartAnim() {
    if (chartAnimId) cancelAnimationFrame(chartAnimId);
    const start = performance.now();
    const duration = 900;

    function frame(now) {
        const t = Math.min((now - start) / duration, 1);
        const eased = 1 - Math.pow(1 - t, 3); // easeOutCubic
        drawPareto(eased);
        if (t < 1) {
            chartAnimId = requestAnimationFrame(frame);
        } else {
            chartAnimId = null;
        }
    }
    chartAnimId = requestAnimationFrame(frame);
}

// 绘制折线（含圆点 + 发光），progress 控制从左到右逐步绘制
function drawLine(points, progress, color, glowColor) {
    const drawCount = Math.max(1, Math.ceil(points.length * progress));

    chartCtx.strokeStyle = color;
    chartCtx.lineWidth = 2;
    chartCtx.lineJoin = 'round';
    chartCtx.lineCap = 'round';
    chartCtx.shadowColor = glowColor;
    chartCtx.shadowBlur = 6;

    chartCtx.beginPath();
    for (let i = 0; i < drawCount; i++) {
        const p = points[i];
        if (i === 0) chartCtx.moveTo(p.x, p.y);
        else chartCtx.lineTo(p.x, p.y);
    }
    chartCtx.stroke();
    chartCtx.shadowBlur = 0;

    // 圆点
    for (let i = 0; i < drawCount; i++) {
        const p = points[i];
        chartCtx.fillStyle = '#0c0e13';
        chartCtx.beginPath();
        chartCtx.arc(p.x, p.y, 3.5, 0, Math.PI * 2);
        chartCtx.fill();
        chartCtx.fillStyle = color;
        chartCtx.beginPath();
        chartCtx.arc(p.x, p.y, 2.5, 0, Math.PI * 2);
        chartCtx.fill();
    }
}

// 绘制帕累托图
function drawPareto(progress) {
    const dpr = window.devicePixelRatio || 1;
    const rect = paretoCanvas.getBoundingClientRect();
    const W = rect.width;
    const H = rect.height;
    paretoCanvas.width = W * dpr;
    paretoCanvas.height = H * dpr;
    chartCtx.setTransform(dpr, 0, 0, dpr, 0, 0);
    chartCtx.clearRect(0, 0, W, H);

    const data = chartData;
    const n = data.length;
    if (n === 0) return;

    // 布局
    const ml = 56, mr = 52, mt = 14, mb = 42;
    const cw = W - ml - mr;
    const ch = H - mt - mb;
    if (cw < 10 || ch < 10) return;

    const total = data.reduce((s, d) => s + d.size, 0);
    const maxVal = data[0].size;

    // 条形参数
    const gap = Math.max(1, Math.min(3, cw / n * 0.12));
    const barW = Math.max(1, cw / n - gap);
    const slot = cw / n;

    // ---- 绘制水平网格线 + 左轴标签（大小） ----
    chartCtx.font = '11px ui-monospace, monospace';
    chartCtx.textAlign = 'right';
    chartCtx.textBaseline = 'middle';
    const gridLines = 4;
    for (let g = 0; g <= gridLines; g++) {
        const y = mt + ch - (ch * g / gridLines);
        const val = maxVal * g / gridLines;
        chartCtx.strokeStyle = g === 0 ? 'rgba(255,255,255,0.1)' : 'rgba(255,255,255,0.04)';
        chartCtx.lineWidth = 1;
        chartCtx.beginPath();
        chartCtx.moveTo(ml, y);
        chartCtx.lineTo(W - mr, y);
        chartCtx.stroke();
        chartCtx.fillStyle = '#5c626e';
        chartCtx.fillText(formatSize(val), ml - 8, y);
    }

    // ---- 右轴标签（百分比） ----
    chartCtx.textAlign = 'left';
    for (let g = 0; g <= gridLines; g++) {
        const y = mt + ch - (ch * g / gridLines);
        const pct = (g / gridLines * 100);
        chartCtx.fillStyle = '#5c626e';
        chartCtx.fillText(pct.toFixed(0) + '%', W - mr + 8, y);
    }

    // ---- 绘制条形 ----
    chartBarRects = [];
    otherBarRect = null;
    const hasOther = chartTailItems.length > 0;
    let cumSum = 0;
    const linePoints = [];      // 累计百分比
    const selfPoints = [];      // 自身占比

    for (let i = 0; i < n; i++) {
        const d = data[i];
        cumSum += d.size;
        const cumPct = (cumSum / total) * 100;
        const selfPct = (d.size / total) * 100;

        const barH = Math.max((d.size / maxVal) * ch, 2);
        const x = ml + i * slot + (slot - barW) / 2;

        // 逐条延迟动画
        const delay = i * 0.025;
        const localP = Math.max(0, Math.min(1, (progress - delay) / (1 - delay)));
        const animH = barH * (1 - Math.pow(1 - localP, 3));
        const y = mt + ch - animH;

        // 渐变透明：前几条清晰，后部逐渐变淡
        const fadeFactor = i < 5 ? 1 : Math.max(0.2, 1 - (i - 5) / (n - 5) * 0.65);
        const isHover = i === chartHoverIdx;

        // 条形渐变
        const grad = chartCtx.createLinearGradient(0, mt + ch - barH, 0, mt + ch);
        if (isHover) {
            grad.addColorStop(0, `rgba(78, 224, 208, ${fadeFactor})`);
            grad.addColorStop(0.7, `rgba(78, 224, 208, ${fadeFactor * 0.5})`);
            grad.addColorStop(1, `rgba(78, 224, 208, ${fadeFactor * 0.1})`);
        } else {
            grad.addColorStop(0, `rgba(78, 224, 208, ${fadeFactor * 0.85})`);
            grad.addColorStop(0.7, `rgba(78, 224, 208, ${fadeFactor * 0.35})`);
            grad.addColorStop(1, `rgba(78, 224, 208, ${fadeFactor * 0.08})`);
        }
        chartCtx.fillStyle = grad;

        if (isHover) {
            chartCtx.shadowColor = 'rgba(78, 224, 208, 0.5)';
            chartCtx.shadowBlur = 12;
        }
        chartCtx.fillRect(x, y, barW, animH);
        chartCtx.shadowBlur = 0;

        // 顶部高光
        if (animH > 3 && i < 8) {
            chartCtx.fillStyle = `rgba(255, 255, 255, ${fadeFactor * 0.3})`;
            chartCtx.fillRect(x, y, barW, 1.5);
        }

        // 自身占比标注（柱子上方）
        if (localP > 0.5 && selfPct >= 0.1 && barW >= 12) {
            const label = selfPct.toFixed(selfPct < 1 ? 1 : 0) + '%';
            chartCtx.save();
            const fontSize = barW >= 30 ? 10 : barW >= 18 ? 9 : 8;
            chartCtx.font = `600 ${fontSize}px "Segoe UI", system-ui, sans-serif`;
            chartCtx.textAlign = 'center';
            chartCtx.textBaseline = 'bottom';
            chartCtx.fillStyle = `rgba(78, 224, 208, ${fadeFactor})`;
            chartCtx.fillText(label, x + barW / 2, y - 2);
            chartCtx.restore();
        }

        chartBarRects.push({ x, y: mt + ch - barH, w: barW, h: barH, idx: i });

        // 记录"其他"条的命中区域
        if (hasOther && i === n - 1) {
            otherBarRect = { x, y: mt + ch - barH, w: barW, h: barH };
        }

        const cx = x + barW / 2;
        linePoints.push({ x: cx, y: mt + ch - (cumPct / 100) * ch, pct: cumPct, name: d.name, size: d.size });
        selfPoints.push({ x: cx, y: mt + ch - (selfPct / 100) * ch, pct: selfPct, name: d.name, size: d.size });
    }

    // ---- 绘制累计百分比折线 ----
    // ---- 绘制折线（自身占比 + 累计百分比） ----
    if (progress > 0.55) {
        const lineProgress = Math.min(1, (progress - 0.55) / 0.45);

        // 自身占比线（紫色）
        if (showSelfLine && selfPoints.length > 0) {
            drawLine(selfPoints, lineProgress, '#c084fc', 'rgba(192, 132, 252, 0.4)');
        }

        // 累计百分比线（蓝色）
        if (showCumulativeLine && linePoints.length > 0) {
            drawLine(linePoints, lineProgress, '#5b9eff', 'rgba(91, 158, 255, 0.4)');
        }
    }

    // ---- 80% 参考线 ----
    if (progress > 0.8) {
        const y80 = mt + ch - 0.8 * ch;
        chartCtx.strokeStyle = 'rgba(255, 107, 107, 0.25)';
        chartCtx.lineWidth = 1;
        chartCtx.setLineDash([4, 4]);
        chartCtx.beginPath();
        chartCtx.moveTo(ml, y80);
        chartCtx.lineTo(W - mr, y80);
        chartCtx.stroke();
        chartCtx.setLineDash([]);
        chartCtx.font = '10px ui-monospace, monospace';
        chartCtx.textAlign = 'left';
        chartCtx.fillStyle = 'rgba(255, 107, 107, 0.5)';
        chartCtx.fillText('80%', ml + 4, y80 - 6);
    }

    // ---- X 轴标签（仅前几个 + 末尾） ----
    chartCtx.font = '10px -apple-system, sans-serif';
    chartCtx.textAlign = 'center';
    chartCtx.textBaseline = 'top';
    const labelStep = n > 20 ? Math.ceil(n / 8) : 1;
    for (let i = 0; i < n; i++) {
        if (i % labelStep !== 0 && i !== n - 1) continue;
        const d = data[i];
        const x = ml + i * slot + slot / 2;
        let label = d.name;
        if (label.length > 8) label = label.slice(0, 7) + '…';
        chartCtx.fillStyle = i < 5 ? '#8b909a' : '#5c626e';
        chartCtx.fillText(label, x, mt + ch + 6);
    }
}

// 鼠标悬停tooltip
function handleChartHover(e) {
    const rect = paretoCanvas.getBoundingClientRect();
    const mx = e.clientX - rect.left;
    const my = e.clientY - rect.top;

    let found = -1;
    for (const r of chartBarRects) {
        // 扩大命中范围
        if (mx >= r.x - 2 && mx <= r.x + r.w + 2 && my >= r.y - 4 && my <= r.y + r.h + 4) {
            found = r.idx;
            break;
        }
    }

    if (found !== chartHoverIdx) {
        chartHoverIdx = found;
        if (!chartAnimId) drawPareto(1);
    }

    if (found >= 0) {
        const d = chartData[found];
        const total = chartData.reduce((s, x) => s + x.size, 0);
        const cum = chartData.slice(0, found + 1).reduce((s, x) => s + x.size, 0);
        const pct = (d.size / total * 100).toFixed(1);
        const cumPct = (cum / total * 100).toFixed(1);

        let html = `<div class="tt-name">${d.name}</div>` +
            `<div class="tt-size">大小: ${formatSize(d.size)}</div>`;
        if (showSelfLine) html += `<div class="tt-pct" style="color:#c084fc">自身: ${pct}%</div>`;
        if (showCumulativeLine) html += `<div class="tt-pct">累计: ${cumPct}%</div>`;
        // "其他"条提示可点击展开
        if (d.name.startsWith('其他')) {
            html += `<div class="tt-pct" style="color:var(--accent);margin-top:4px">点击展开详情 →</div>`;
        }
        chartTooltip.innerHTML = html;
        chartTooltip.style.left = mx + 'px';
        chartTooltip.style.top = my + 'px';
        chartTooltip.classList.remove('hidden');
    } else {
        chartTooltip.classList.add('hidden');
    }
}

function handleChartLeave() {
    if (chartHoverIdx !== -1) {
        chartHoverIdx = -1;
        if (!chartAnimId) drawPareto(1);
    }
    chartTooltip.classList.add('hidden');
}

// 窗口尺寸变化时重绘
window.addEventListener('resize', () => {
    clearTimeout(chartResizeTimer);
    chartResizeTimer = setTimeout(() => {
        if (chartData.length > 0 && !chartAnimId) drawPareto(1);
    }, 150);
});

paretoCanvas.addEventListener('mousemove', handleChartHover);
paretoCanvas.addEventListener('mouseleave', handleChartLeave);

document.getElementById('toggleSelf').addEventListener('change', e => {
    showSelfLine = e.target.checked;
    if (chartData.length > 0 && !chartAnimId) drawPareto(1);
});
document.getElementById('toggleCumulative').addEventListener('change', e => {
    showCumulativeLine = e.target.checked;
    if (chartData.length > 0 && !chartAnimId) drawPareto(1);
});

// ==================== 上下文菜单 ====================

// 在指定位置显示上下文菜单
function showCtxMenu(clientX, clientY, filePath) {
    ctxTargetPath = filePath;
    ctxMenu.classList.remove('hidden');
    // 防止超出视口
    const rect = ctxMenu.getBoundingClientRect();
    const maxX = window.innerWidth - rect.width - 8;
    const maxY = window.innerHeight - rect.height - 8;
    ctxMenu.style.left = Math.min(clientX, maxX) + 'px';
    ctxMenu.style.top = Math.min(clientY, maxY) + 'px';
}

function hideCtxMenu() {
    ctxMenu.classList.add('hidden');
    ctxTargetPath = '';
}

// 点击"打开文件夹所在位置"
ctxOpenExplorer.addEventListener('click', () => {
    if (!ctxTargetPath) return;
    fetch('/api/openInExplorer?path=' + encodeURIComponent(ctxTargetPath))
        .then(r => r.json())
        .then(data => {
            if (data.error) {
                statusEl.textContent = '打开失败: ' + data.error;
                statusEl.classList.remove('hidden');
            }
        })
        .catch(() => {});
    hideCtxMenu();
});

// 点击其他区域关闭菜单
document.addEventListener('click', e => {
    if (!ctxMenu.classList.contains('hidden') && !ctxMenu.contains(e.target)) {
        hideCtxMenu();
    }
});
document.addEventListener('keydown', e => {
    if (e.key === 'Escape') hideCtxMenu();
});
// 滚动时关闭
window.addEventListener('scroll', hideCtxMenu, true);

// 在主图中查找点击命中的条形
function findBarAt(canvas, barRects, clientX, clientY) {
    const rect = canvas.getBoundingClientRect();
    const mx = clientX - rect.left;
    const my = clientY - rect.top;
    for (const r of barRects) {
        if (mx >= r.x - 1 && mx <= r.x + r.w + 1 && my >= r.y - 2 && my <= r.y + r.h + 2) {
            return r.idx;
        }
    }
    return -1;
}

// ==================== "其他"展开逻辑 ====================

// 点击主图：左键检测"其他"条或常规条，右键显示菜单
paretoCanvas.addEventListener('click', e => {
    const idx = findBarAt(paretoCanvas, chartBarRects, e.clientX, e.clientY);
    if (idx < 0) return;

    // "其他"条 → 打开展开弹框
    if (idx === chartData.length - 1 && otherBarRect && chartTailItems.length > 0) {
        modalDefaultCount = Math.min(chartTailItems.length, 50);
        expandInput.value = modalDefaultCount;
        expandInput.max = chartTailItems.length;
        expandModal.classList.remove('hidden');
        expandInput.focus();
        expandInput.select();
        return;
    }

    // 常规条 → 显示上下文菜单
    const d = chartData[idx];
    if (d.path) {
        showCtxMenu(e.clientX, e.clientY, d.path);
    }
});

// 右键主图 → 显示菜单
paretoCanvas.addEventListener('contextmenu', e => {
    const idx = findBarAt(paretoCanvas, chartBarRects, e.clientX, e.clientY);
    if (idx < 0) return;
    const d = chartData[idx];
    if (d.path) {
        e.preventDefault();
        showCtxMenu(e.clientX, e.clientY, d.path);
    }
});

// 弹框确认展开
document.getElementById('expandConfirm').addEventListener('click', () => {
    const count = parseInt(expandInput.value) || 50;
    expandModal.classList.add('hidden');
    showExpanded(Math.max(1, Math.min(count, chartTailItems.length)));
});

// 弹框取消
document.getElementById('expandCancel').addEventListener('click', () => {
    expandModal.classList.add('hidden');
});

// 回车确认
expandInput.addEventListener('keydown', e => {
    if (e.key === 'Enter') document.getElementById('expandConfirm').click();
    if (e.key === 'Escape') expandModal.classList.add('hidden');
});

// 关闭展开区
document.getElementById('expandClose').addEventListener('click', () => {
    expandSection.classList.add('hidden');
    expandData = [];
    expandBarRects = [];
    expandHoverIdx = -1;
    expandTooltip.classList.add('hidden');
});

// "展开剩余"按钮 → 重新打开弹框，默认值加上当前已展开的数量
expandMoreBtn.addEventListener('click', () => {
    modalDefaultCount = Math.min(expandData.length + 50, chartTailItems.length);
    expandInput.value = modalDefaultCount;
    expandInput.max = chartTailItems.length;
    expandModal.classList.remove('hidden');
    expandInput.focus();
    expandInput.select();
});

// 展开图鼠标悬停 → 显示 Tooltip
expandCanvas.addEventListener('mousemove', e => {
    if (expandBarRects.length === 0) return;
    const rect = expandCanvas.getBoundingClientRect();
    const mx = e.clientX - rect.left;
    const my = e.clientY - rect.top;

    let found = -1;
    for (const r of expandBarRects) {
        if (mx >= r.x - 1 && mx <= r.x + r.w + 1 && my >= r.y - 2 && my <= r.y + r.h + 2) {
            found = r.idx;
            break;
        }
    }

    if (found !== expandHoverIdx) {
        expandHoverIdx = found;
        if (!expandAnimId) drawExpanded(1);
    }

    if (found >= 0) {
        const d = expandData[found];
        const total = expandData.reduce((s, x) => s + x.size, 0);
        const cum = expandData.slice(0, found + 1).reduce((s, x) => s + x.size, 0);
        const pct = (d.size / total * 100).toFixed(1);
        const cumPct = (cum / total * 100).toFixed(1);

        let html = `<div class="tt-name">${d.name}</div>` +
            `<div class="tt-size">大小: ${formatSize(d.size)}</div>`;
        if (showSelfLine) html += `<div class="tt-pct" style="color:#c084fc">自身: ${pct}%</div>`;
        if (showCumulativeLine) html += `<div class="tt-pct">累计: ${cumPct}%</div>`;
        expandTooltip.innerHTML = html;
        expandTooltip.style.left = mx + 'px';
        expandTooltip.style.top = (my + 14) + 'px';
        expandTooltip.classList.remove('hidden');
    } else {
        expandTooltip.classList.add('hidden');
    }
});

expandCanvas.addEventListener('mouseleave', () => {
    expandHoverIdx = -1;
    expandTooltip.classList.add('hidden');
    if (!expandAnimId) drawExpanded(1);
});

// 左键展开图 → 显示上下文菜单
expandCanvas.addEventListener('click', e => {
    const idx = findBarAt(expandCanvas, expandBarRects, e.clientX, e.clientY);
    if (idx < 0) return;
    const d = expandData[idx];
    if (d.path) {
        showCtxMenu(e.clientX, e.clientY, d.path);
    }
});

// 右键展开图 → 显示菜单
expandCanvas.addEventListener('contextmenu', e => {
    const idx = findBarAt(expandCanvas, expandBarRects, e.clientX, e.clientY);
    if (idx < 0) return;
    const d = expandData[idx];
    if (d.path) {
        e.preventDefault();
        showCtxMenu(e.clientX, e.clientY, d.path);
    }
});

// 展开图绘制（复用 drawPareto 的逻辑，但用 expandCtx 和 expandCanvas）
function showExpanded(count) {
    expandData = chartTailItems.slice(0, count);
    if (expandData.length === 0) return;

    expandLabel.textContent = `其他 (${count}/${chartTailItems.length} 项)`;
    expandSection.classList.remove('hidden');

    // 检查是否有剩余未展开项
    const remaining = chartTailItems.length - count;
    if (remaining > 0) {
        remainCountEl.textContent = remaining;
        expandMoreBtn.classList.remove('hidden');
    } else {
        expandMoreBtn.classList.add('hidden');
    }

    // 启动展开图动画
    if (expandAnimId) cancelAnimationFrame(expandAnimId);
    const start = performance.now();
    const duration = 800;

    function frame(now) {
        const t = Math.min((now - start) / duration, 1);
        const eased = 1 - Math.pow(1 - t, 3);
        drawExpanded(eased);
        if (t < 1) {
            expandAnimId = requestAnimationFrame(frame);
        } else {
            expandAnimId = null;
        }
    }
    expandAnimId = requestAnimationFrame(frame);
}

// 在展开 canvas 上绘制帕累托图
function drawExpanded(progress) {
    const dpr = window.devicePixelRatio || 1;
    const rect = expandCanvas.getBoundingClientRect();
    const W = rect.width;
    const H = rect.height;
    expandCanvas.width = W * dpr;
    expandCanvas.height = H * dpr;
    expandCtx.setTransform(dpr, 0, 0, dpr, 0, 0);
    expandCtx.clearRect(0, 0, W, H);

    const data = expandData;
    const n = data.length;
    if (n === 0) return;

    const ml = 56, mr = 52, mt = 14, mb = 42;
    const cw = W - ml - mr;
    const ch = H - mt - mb;
    if (cw < 10 || ch < 10) return;

    const total = data.reduce((s, d) => s + d.size, 0);
    const maxVal = data[0].size;

    const gap = Math.max(1, Math.min(3, cw / n * 0.12));
    const barW = Math.max(1, cw / n - gap);
    const slot = cw / n;

    // 网格 + 左轴
    expandCtx.font = '11px ui-monospace, monospace';
    expandCtx.textAlign = 'right';
    expandCtx.textBaseline = 'middle';
    const gridLines = 4;
    for (let g = 0; g <= gridLines; g++) {
        const y = mt + ch - (ch * g / gridLines);
        const val = maxVal * g / gridLines;
        expandCtx.strokeStyle = g === 0 ? 'rgba(255,255,255,0.1)' : 'rgba(255,255,255,0.04)';
        expandCtx.lineWidth = 1;
        expandCtx.beginPath();
        expandCtx.moveTo(ml, y);
        expandCtx.lineTo(W - mr, y);
        expandCtx.stroke();
        expandCtx.fillStyle = '#5c626e';
        expandCtx.fillText(formatSize(val), ml - 8, y);
    }

    // 右轴百分比
    expandCtx.textAlign = 'left';
    for (let g = 0; g <= gridLines; g++) {
        const y = mt + ch - (ch * g / gridLines);
        expandCtx.fillStyle = '#5c626e';
        expandCtx.fillText((g / gridLines * 100).toFixed(0) + '%', W - mr + 8, y);
    }

    // 条形
    expandBarRects = [];
    let cumSum = 0;
    const linePoints = [];
    const selfPoints = [];

    for (let i = 0; i < n; i++) {
        const d = data[i];
        cumSum += d.size;
        const cumPct = (cumSum / total) * 100;
        const selfPct = (d.size / total) * 100;

        const barH = Math.max((d.size / maxVal) * ch, 2);
        const x = ml + i * slot + (slot - barW) / 2;

        const delay = i * 0.02;
        const localP = Math.max(0, Math.min(1, (progress - delay) / (1 - delay)));
        const animH = barH * (1 - Math.pow(1 - localP, 3));
        const y = mt + ch - animH;

        const fadeFactor = i < 5 ? 1 : Math.max(0.2, 1 - (i - 5) / Math.max(n - 5, 1) * 0.65);
        const isHover = i === expandHoverIdx;

        const grad = expandCtx.createLinearGradient(0, mt + ch - barH, 0, mt + ch);
        if (isHover) {
            grad.addColorStop(0, `rgba(192, 132, 252, ${fadeFactor})`);
            grad.addColorStop(0.7, `rgba(192, 132, 252, ${fadeFactor * 0.5})`);
            grad.addColorStop(1, `rgba(192, 132, 252, ${fadeFactor * 0.1})`);
        } else {
            grad.addColorStop(0, `rgba(192, 132, 252, ${fadeFactor * 0.85})`);
            grad.addColorStop(0.7, `rgba(192, 132, 252, ${fadeFactor * 0.35})`);
            grad.addColorStop(1, `rgba(192, 132, 252, ${fadeFactor * 0.08})`);
        }
        expandCtx.fillStyle = grad;

        if (isHover) {
            expandCtx.shadowColor = 'rgba(192, 132, 252, 0.5)';
            expandCtx.shadowBlur = 12;
        }
        expandCtx.fillRect(x, y, barW, animH);
        expandCtx.shadowBlur = 0;

        if (animH > 3 && i < 8) {
            expandCtx.fillStyle = `rgba(255, 255, 255, ${fadeFactor * 0.3})`;
            expandCtx.fillRect(x, y, barW, 1.5);
        }

        // 自身占比标注（柱子上方）
        if (localP > 0.5 && selfPct >= 0.1 && barW >= 12) {
            const label = selfPct.toFixed(selfPct < 1 ? 1 : 0) + '%';
            expandCtx.save();
            const fontSize = barW >= 30 ? 10 : barW >= 18 ? 9 : 8;
            expandCtx.font = `600 ${fontSize}px "Segoe UI", system-ui, sans-serif`;
            expandCtx.textAlign = 'center';
            expandCtx.textBaseline = 'bottom';
            expandCtx.fillStyle = `rgba(192, 132, 252, ${fadeFactor})`;
            expandCtx.fillText(label, x + barW / 2, y - 2);
            expandCtx.restore();
        }

        expandBarRects.push({ x, y: mt + ch - barH, w: barW, h: barH, idx: i });

        const cx = x + barW / 2;
        linePoints.push({ x: cx, y: mt + ch - (cumPct / 100) * ch, pct: cumPct, name: d.name, size: d.size });
        selfPoints.push({ x: cx, y: mt + ch - (selfPct / 100) * ch, pct: selfPct });
    }

    // 折线
    if (progress > 0.55) {
        const lineProgress = Math.min(1, (progress - 0.55) / 0.45);
        if (showSelfLine && selfPoints.length > 0) {
            drawLineOn(expandCtx, selfPoints, lineProgress, '#c084fc', 'rgba(192, 132, 252, 0.4)');
        }
        if (showCumulativeLine && linePoints.length > 0) {
            drawLineOn(expandCtx, linePoints, lineProgress, '#5b9eff', 'rgba(91, 158, 255, 0.4)');
        }
    }

    // X 轴标签
    expandCtx.font = '10px -apple-system, sans-serif';
    expandCtx.textAlign = 'center';
    expandCtx.textBaseline = 'top';
    const labelStep = n > 20 ? Math.ceil(n / 8) : 1;
    for (let i = 0; i < n; i++) {
        if (i % labelStep !== 0 && i !== n - 1) continue;
        const d = data[i];
        const x = ml + i * slot + slot / 2;
        let label = d.name;
        if (label.length > 8) label = label.slice(0, 7) + '…';
        expandCtx.fillStyle = i < 5 ? '#8b909a' : '#5c626e';
        expandCtx.fillText(label, x, mt + ch + 6);
    }
}

// 在指定 context 上绘制折线
function drawLineOn(ctx, points, progress, color, glowColor) {
    const drawCount = Math.max(1, Math.ceil(points.length * progress));

    ctx.strokeStyle = color;
    ctx.lineWidth = 2;
    ctx.lineJoin = 'round';
    ctx.lineCap = 'round';
    ctx.shadowColor = glowColor;
    ctx.shadowBlur = 6;

    ctx.beginPath();
    for (let i = 0; i < drawCount; i++) {
        const p = points[i];
        if (i === 0) ctx.moveTo(p.x, p.y);
        else ctx.lineTo(p.x, p.y);
    }
    ctx.stroke();
    ctx.shadowBlur = 0;

    for (let i = 0; i < drawCount; i++) {
        const p = points[i];
        ctx.fillStyle = '#0c0e13';
        ctx.beginPath();
        ctx.arc(p.x, p.y, 3.5, 0, Math.PI * 2);
        ctx.fill();
        ctx.fillStyle = color;
        ctx.beginPath();
        ctx.arc(p.x, p.y, 2.5, 0, Math.PI * 2);
        ctx.fill();
    }
}

// 展开图也响应窗口缩放
window.addEventListener('resize', () => {
    if (expandData.length > 0 && !expandAnimId) drawExpanded(1);
});

// 加载磁盘列表与最近访问文件夹
async function loadDrives() {
    try {
        const [drivesRes, recentRes] = await Promise.all([
            fetch('/api/drives'),
            fetch('/api/recent')
        ]);
        const drives = await drivesRes.json();
        const recentData = await recentRes.json();
        recentFolders = recentData.folders || [];

        driveSelect.innerHTML = '';

        // 占位项
        const placeholder = document.createElement('option');
        placeholder.value = '';
        placeholder.textContent = '磁盘 / 最近访问...';
        driveSelect.appendChild(placeholder);

        // 最近访问分组
        if (recentFolders.length > 0) {
            const group = document.createElement('optgroup');
            group.label = '最近访问';
            recentFolders.forEach(f => {
                const opt = document.createElement('option');
                opt.value = f;
                opt.textContent = f;
                group.appendChild(opt);
            });
            driveSelect.appendChild(group);
        }

        // 磁盘分组
        if (drives && drives.length > 0) {
            const group = document.createElement('optgroup');
            group.label = '磁盘';
            drives.forEach(d => {
                const opt = document.createElement('option');
                opt.value = d;
                opt.textContent = d;
                group.appendChild(opt);
            });
            driveSelect.appendChild(group);
        }

        driveSelect.onchange = () => {
            if (!driveSelect.value) return;
            const path = driveSelect.value;
            driveSelect.value = '';
            addRecent(path);
            loadPath(path);
        };
    } catch (err) {
        // 静默忽略
    }
}

// 添加到最近访问记录
async function addRecent(path) {
    try {
        await fetch('/api/recent', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: path })
        });
    } catch (err) {
        // 静默忽略
    }
}

// 打开系统原生文件夹选择器
async function pickFolder() {
    showStatus('正在打开文件夹选择器...', false);
    try {
        const res = await fetch('/api/pickFolder');
        const data = await res.json();
        if (data.error) {
            showStatus('无法打开文件夹选择器: ' + data.error, true);
            return;
        }
        if (!data.path) {
            clearStatus();
            return; // 用户取消
        }
        clearStatus();
        await addRecent(data.path);
        await loadPath(data.path);
        await loadDrives(); // 刷新下拉列表
    } catch (err) {
        showStatus('请求失败: ' + err.message, true);
    }
}

// 绑定按钮事件
document.getElementById('refreshBtn').onclick = () => loadPath(currentPath);
document.getElementById('pickBtn').onclick = () => pickFolder();
document.getElementById('upBtn').onclick = () => {
    if (!currentPath) return;
    const sep = currentPath.includes('\\') ? '\\' : '/';
    const parts = currentPath.split(sep).filter(Boolean);
    if (parts.length <= 1) return;
    // 去掉最后一级
    const parentParts = parts.slice(0, -1);
    let parent;
    if (/^[A-Za-z]:$/.test(parentParts[0]) && parentParts.length === 1) {
        // 仅剩盘符，跳到盘符根
        parent = parentParts[0] + '\\';
    } else {
        parent = parentParts.join(sep);
    }
    loadPath(parent);
};

// 初始化：加载磁盘与最近访问，优先打开上次访问的文件夹
loadDrives().then(() => {
    if (recentFolders.length > 0) {
        loadPath(recentFolders[0]);
    } else {
        loadPath('');
    }
});
