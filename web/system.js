// 系统管理模块：启动系统设置界面
const systemModal = document.getElementById('systemModal');
const systemClose = document.getElementById('systemClose');

document.getElementById('homeSystemBtn').onclick = () => {
    systemModal.classList.remove('hidden');
};

systemClose.addEventListener('click', () => systemModal.classList.add('hidden'));
systemModal.addEventListener('click', e => {
    if (e.target === systemModal) systemModal.classList.add('hidden');
});

// 点击启动项调用后端打开系统设置
document.querySelectorAll('.system-launch-item').forEach(btn => {
    btn.addEventListener('click', async () => {
        const target = btn.dataset.target;
        btn.classList.add('loading');
        try {
            const resp = await fetch('/api/system/open?target=' + encodeURIComponent(target));
            const data = await resp.json();
            if (data.error) {
                alert('启动失败: ' + data.error);
            }
        } catch (e) {
            alert('请求失败: ' + e.message);
        } finally {
            btn.classList.remove('loading');
        }
    });
});
