// DiskDataKit 前端逻辑 - GitHub 风格
const fileList = document.getElementById('fileList');
const breadcrumb = document.getElementById('breadcrumb');
const statusEl = document.getElementById('status');
const emptyEl = document.getElementById('empty');
const skeletonEl = document.getElementById('skeleton');
const footerInfo = document.getElementById('footerInfo');
const driveSelect = document.getElementById('driveSelect');
const countBadge = document.getElementById('countBadge');

let currentPath = '';
let loadToken = 0;

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

// 加载指定路径的文件列表
async function loadPath(path) {
    const token = ++loadToken;
    showSkeleton(true);
    clearStatus();
    countBadge.classList.add('hidden');
    fileList.innerHTML = '';

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
        return;
    }
    emptyEl.classList.add('hidden');

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

        fileList.appendChild(tr);
    });
}

// 并发获取文件夹大小，逐步更新对应单元格
function fetchDirSize(path, cell) {
    const myToken = loadToken;
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
            }
        })
        .catch(() => {
            if (myToken !== loadToken) return;
            cell.textContent = '—';
            cell.style.opacity = '';
        });
}

// 加载磁盘列表
async function loadDrives() {
    try {
        const res = await fetch('/api/drives');
        const drives = await res.json();
        driveSelect.innerHTML = '';
        if (!drives || drives.length === 0) return;

        const placeholder = document.createElement('option');
        placeholder.value = '';
        placeholder.textContent = '选择磁盘...';
        driveSelect.appendChild(placeholder);

        drives.forEach(d => {
            const opt = document.createElement('option');
            opt.value = d;
            opt.textContent = d;
            driveSelect.appendChild(opt);
        });

        driveSelect.onchange = () => {
            if (driveSelect.value) loadPath(driveSelect.value);
        };
    } catch (err) {
        // 静默忽略
    }
}

// 绑定按钮事件
document.getElementById('refreshBtn').onclick = () => loadPath(currentPath);
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

// 初始化
loadDrives();
loadPath('');
