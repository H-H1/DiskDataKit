// 文件夹追踪模块

const trackModal = document.getElementById('trackModal');
const trackClose = document.getElementById('trackClose');
const trackPickBtn = document.getElementById('trackPickBtn');
const trackRefreshAllBtn = document.getElementById('trackRefreshAllBtn');
const trackStatus = document.getElementById('trackStatus');
const trackList = document.getElementById('trackList');
const homeTrackBtn = document.getElementById('homeTrackBtn');

// 打开弹窗
homeTrackBtn.addEventListener('click', () => {
    trackModal.classList.remove('hidden');
    loadTracks();
});

// 关闭弹窗
trackClose.addEventListener('click', () => trackModal.classList.add('hidden'));
trackModal.addEventListener('click', e => {
    if (e.target === trackModal) trackModal.classList.add('hidden');
});

// 选择文件夹添加追踪
trackPickBtn.addEventListener('click', async () => {
    try {
        const resp = await fetch('/api/pickFolder');
        const data = await resp.json();
        if (data.error || !data.path) return;

        showTrackStatus('正在添加追踪并扫描文件夹...');
        const addResp = await fetch('/api/track/add', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: data.path })
        });
        const addData = await addResp.json();

        if (addData.error) {
            showTrackStatus('添加失败: ' + addData.error, true);
            return;
        }

        hideTrackStatus();
        loadTracks();
    } catch (e) {
        showTrackStatus('操作失败: ' + e.message, true);
    }
});

// 全部刷新
trackRefreshAllBtn.addEventListener('click', async () => {
    const items = trackList.querySelectorAll('.track-item');
    if (items.length === 0) return;

    showTrackStatus('正在刷新所有追踪...');
    for (const item of items) {
        const path = item.dataset.path;
        if (path) {
            await refreshTrack(path, item);
        }
    }
    hideTrackStatus();
});

// 加载追踪列表
async function loadTracks() {
    showTrackStatus('加载中...');
    try {
        const resp = await fetch('/api/track/list');
        const data = await resp.json();

        if (data.error) {
            showTrackStatus('加载失败: ' + data.error, true);
            return;
        }

        const tracks = data.tracks || [];
        if (tracks.length === 0) {
            trackList.classList.add('hidden');
            trackList.innerHTML = '';
            showTrackStatus('暂无追踪文件夹，点击"选择文件夹"添加');
            return;
        }

        hideTrackStatus();
        trackList.classList.remove('hidden');
        trackList.innerHTML = '';

        for (const t of tracks) {
            trackList.appendChild(renderTrackItem(t));
        }
    } catch (e) {
        showTrackStatus('加载失败: ' + e.message, true);
    }
}

// 渲染单个追踪项
function renderTrackItem(t) {
    const div = document.createElement('div');
    div.className = 'track-item';
    div.dataset.path = t.path;

    // 计算总大小
    let total = 0;
    if (t.children) {
        for (const c of t.children) total += c.size;
    }

    // 头部
    const header = document.createElement('div');
    header.className = 'track-item-header';

    const name = document.createElement('span');
    name.className = 'track-item-name';
    name.textContent = t.name || t.path;
    name.title = t.path;

    const path = document.createElement('span');
    path.className = 'track-item-path';
    path.textContent = t.path;

    const actions = document.createElement('div');
    actions.className = 'track-item-actions';

    const refreshBtn = document.createElement('button');
    refreshBtn.className = 'track-btn';
    refreshBtn.textContent = '刷新对比';
    refreshBtn.addEventListener('click', () => refreshTrack(t.path, div));

    const removeBtn = document.createElement('button');
    removeBtn.className = 'track-btn danger';
    removeBtn.textContent = '取消追踪';
    removeBtn.addEventListener('click', () => removeTrack(t.path));

    actions.appendChild(refreshBtn);
    actions.appendChild(removeBtn);

    header.appendChild(name);
    header.appendChild(path);
    header.appendChild(actions);
    div.appendChild(header);

    // 内容区
    const body = document.createElement('div');
    body.className = 'track-item-body';

    // 摘要
    const summary = document.createElement('div');
    summary.className = 'track-summary';
    const updated = t.updated ? new Date(t.updated).toLocaleString('zh-CN') : '未知';
    summary.innerHTML = `
        <span class="track-summary-item">📊 总大小: ${formatSize(total)}</span>
        <span class="track-summary-item">📁 ${t.childCount || 0} 个子项</span>
        <span class="track-summary-item">🕐 ${updated}</span>
    `;
    body.appendChild(summary);

    // 子文件列表
    if (t.children && t.children.length > 0) {
        const childList = document.createElement('div');
        childList.className = 'track-child-list';

        // 按大小降序
        const sorted = [...t.children].sort((a, b) => b.size - a.size);
        for (const c of sorted) {
            const row = document.createElement('div');
            row.className = 'track-child-row';
            row.innerHTML = `
                <span class="track-diff-icon">${c.isDir ? '📁' : '📄'}</span>
                <span class="track-child-name" title="${c.path}">${c.name}</span>
                <span class="track-child-size">${formatSize(c.size)}</span>
            `;
            childList.appendChild(row);
        }
        body.appendChild(childList);
    }

    div.appendChild(body);
    return div;
}

// 刷新单个追踪，显示对比结果
async function refreshTrack(path, itemEl) {
    const body = itemEl.querySelector('.track-item-body');
    if (!body) return;

    // 显示加载状态
    const oldDiff = body.querySelector('.track-diff-section');
    if (oldDiff) oldDiff.remove();

    const loadingDiv = document.createElement('div');
    loadingDiv.className = 'track-diff-section';
    loadingDiv.innerHTML = '<div class="track-diff-title">正在扫描对比...</div>';
    body.appendChild(loadingDiv);

    try {
        const resp = await fetch('/api/track/refresh', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path })
        });
        const data = await resp.json();

        loadingDiv.remove();

        if (data.error) {
            const errDiv = document.createElement('div');
            errDiv.className = 'track-diff-section';
            errDiv.innerHTML = `<div class="track-diff-title" style="color:var(--danger)">❌ ${data.error}</div>`;
            body.appendChild(errDiv);
            return;
        }

        const diff = data.diff;
        renderDiff(body, diff);

        // 刷新摘要数据
        const summary = body.querySelector('.track-summary');
        if (summary) {
            const deltaClass = diff.delta > 0 ? 'track-delta-up' : 'track-delta-down';
            const deltaSign = diff.delta > 0 ? '+' : '';
            summary.innerHTML = `
                <span class="track-summary-item">📊 ${formatSize(diff.oldTotal)} → ${formatSize(diff.newTotal)}</span>
                <span class="track-summary-item">变化: <span class="${deltaClass}">${deltaSign}${formatSize(diff.delta)}</span></span>
                <span class="track-summary-item" style="color:var(--danger)">+${diff.added.length} 新增</span>
                <span class="track-summary-item" style="color:var(--fg-subtle)">-${diff.removed.length} 删除</span>
                <span class="track-summary-item" style="color:#828590">~${diff.changed.length} 变化</span>
            `;
        }
    } catch (e) {
        loadingDiv.remove();
        const errDiv = document.createElement('div');
        errDiv.className = 'track-diff-section';
        errDiv.innerHTML = `<div class="track-diff-title" style="color:var(--danger)">❌ ${e.message}</div>`;
        body.appendChild(errDiv);
    }
}

// 渲染对比差异
function renderDiff(body, diff) {
    // 移除旧的 diff
    const oldDiff = body.querySelector('.track-diff-section');
    if (oldDiff) oldDiff.remove();

    const hasChanges = diff.added.length > 0 || diff.removed.length > 0 || diff.changed.length > 0;

    if (!hasChanges) {
        const noChange = document.createElement('div');
        noChange.className = 'track-diff-section';
        noChange.innerHTML = '<div class="track-diff-title" style="color:var(--accent)">✅ 无变化</div>';
        body.appendChild(noChange);
        return;
    }

    const section = document.createElement('div');
    section.className = 'track-diff-section';

    // 新增
    if (diff.added.length > 0) {
        const title = document.createElement('div');
        title.className = 'track-diff-title';
        title.style.color = 'var(--danger)';
        title.textContent = `新增 (${diff.added.length})`;
        section.appendChild(title);

        const sorted = [...diff.added].sort((a, b) => b.size - a.size);
        for (const c of sorted) {
            const row = document.createElement('div');
            row.className = 'track-diff-row';
            row.innerHTML = `
                <span class="track-diff-icon">🟢</span>
                <span class="track-diff-name" title="${c.path}">${c.isDir ? '📁' : '📄'} ${c.name}</span>
                <span class="track-diff-size">${formatSize(c.size)}</span>
            `;
            section.appendChild(row);
        }
    }

    // 删除
    if (diff.removed.length > 0) {
        const title = document.createElement('div');
        title.className = 'track-diff-title';
        title.style.color = 'var(--fg-subtle)';
        title.style.marginTop = '8px';
        title.textContent = `删除 (${diff.removed.length})`;
        section.appendChild(title);

        const sorted = [...diff.removed].sort((a, b) => b.size - a.size);
        for (const c of sorted) {
            const row = document.createElement('div');
            row.className = 'track-diff-row';
            row.innerHTML = `
                <span class="track-diff-icon">🔴</span>
                <span class="track-diff-name" title="${c.path}">${c.isDir ? '📁' : '📄'} ${c.name}</span>
                <span class="track-diff-size">${formatSize(c.size)}</span>
            `;
            section.appendChild(row);
        }
    }

    // 大小变化
    if (diff.changed.length > 0) {
        const title = document.createElement('div');
        title.className = 'track-diff-title';
        title.style.color = '#828590';
        title.style.marginTop = '8px';
        title.textContent = `大小变化 (${diff.changed.length})`;
        section.appendChild(title);

        const sorted = [...diff.changed].sort((a, b) => Math.abs(b.delta) - Math.abs(a.delta));
        for (const c of sorted) {
            const deltaClass = c.delta > 0 ? 'track-delta-up' : 'track-delta-down';
            const deltaSign = c.delta > 0 ? '+' : '';
            const row = document.createElement('div');
            row.className = 'track-diff-row';
            row.innerHTML = `
                <span class="track-diff-icon">🟡</span>
                <span class="track-diff-name" title="${c.path}">${c.isDir ? '📁' : '📄'} ${c.name}</span>
                <span class="track-diff-size">${formatSize(c.oldSize)} → ${formatSize(c.newSize)} (<span class="${deltaClass}">${deltaSign}${formatSize(c.delta)}</span>)</span>
            `;
            section.appendChild(row);
        }
    }

    body.appendChild(section);
}

// 取消追踪
async function removeTrack(path) {
    try {
        await fetch('/api/track/remove', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path })
        });
        loadTracks();
    } catch (e) {
        showTrackStatus('取消追踪失败: ' + e.message, true);
    }
}

// 状态提示
function showTrackStatus(msg, isError) {
    trackStatus.textContent = msg;
    trackStatus.style.color = isError ? 'var(--danger)' : 'var(--fg-muted)';
}

function hideTrackStatus() {
    trackStatus.textContent = '';
}
