package cpa

const webUIDashboardHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>AIStudio2API 插件管理 · CLIProxyAPI</title>
<style>
  :root {
    --bg: #0f172a;
    --surface: #1e293b;
    --surface-hover: #334155;
    --border: #334155;
    --text: #f8fafc;
    --muted: #94a3b8;
    --accent: #3b82f6;
    --accent-hover: #2563eb;
    --success: #10b981;
    --warning: #f59e0b;
    --danger: #ef4444;
  }
  * { box-sizing: border-box; }
  body {
    margin: 0;
    padding: 24px;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    background: var(--bg);
    color: var(--text);
    line-height: 1.5;
  }
  .container { max-width: 1180px; margin: 0 auto; }
  header {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    align-items: center;
    gap: 16px;
    padding-bottom: 20px;
    border-bottom: 1px solid var(--border);
    margin-bottom: 24px;
  }
  h1 { margin: 0; font-size: 22px; font-weight: 600; display: flex; align-items: center; gap: 10px; }
  .badge {
    display: inline-block;
    padding: 2px 10px;
    border-radius: 999px;
    font-size: 12px;
    font-weight: 600;
    background: var(--surface-hover);
    color: var(--text);
  }
  .badge.success { background: rgba(16, 185, 129, 0.2); color: #34d399; }
  .badge.warning { background: rgba(245, 158, 11, 0.2); color: #fbbf24; }
  .badge.danger { background: rgba(239, 68, 68, 0.2); color: #f87171; }
  .auth-bar { display: flex; gap: 8px; align-items: center; }
  input, textarea, select {
    background: #0b1120;
    border: 1px solid var(--border);
    color: var(--text);
    padding: 8px 12px;
    border-radius: 6px;
    font-size: 13px;
  }
  button {
    background: var(--accent);
    color: #fff;
    border: none;
    padding: 8px 14px;
    border-radius: 6px;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    transition: background 0.15s;
  }
  button:hover { background: var(--accent-hover); }
  button.secondary { background: var(--surface-hover); }
  button.secondary:hover { background: #475569; }
  button.danger { background: var(--danger); }
  button.success { background: var(--success); }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 16px;
    margin-bottom: 24px;
  }
  .card {
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 18px;
    margin-bottom: 20px;
  }
  .stat-label { font-size: 12px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.04em; }
  .stat-value { font-size: 24px; font-weight: 700; margin-top: 4px; }
  .row { display: flex; gap: 12px; flex-wrap: wrap; align-items: center; margin-bottom: 12px; }
  table { width: 100%; border-collapse: collapse; font-size: 13px; }
  th, td { text-align: left; padding: 10px 12px; border-bottom: 1px solid var(--border); }
  th { color: var(--muted); font-weight: 500; }
  .logs {
    background: #090d16;
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 12px;
    max-height: 280px;
    overflow-y: auto;
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 12px;
  }
  .log-line { padding: 3px 0; border-bottom: 1px dashed rgba(255,255,255,0.05); }
  .toast {
    padding: 10px 14px;
    border-radius: 6px;
    margin-bottom: 16px;
    display: none;
    font-size: 13px;
  }
</style>
</head>
<body>
<div class="container">
  <header>
    <div>
      <h1>
        AIStudio2API 插件控制台
        <span id="stateBadge" class="badge">加载中</span>
        <span id="waaBadge" class="badge">WAA: spidermonkey</span>
      </h1>
      <div style="font-size: 12px; color: var(--muted); margin-top: 4px;">
        Google AI Studio (MakerSuite + Build) · 进程内 SpiderMonkey BotGuard VM · 支持 OpenAI / Responses / Claude / Gemini
      </div>
    </div>
    <div class="auth-bar">
      <input type="password" id="mgmtKey" placeholder="输入 CPA 管理密钥 (Management Key)" style="width: 240px;" />
      <button onclick="saveKeyAndReload()">连接并刷新</button>
    </div>
  </header>

  <div id="toast" class="toast"></div>

  <div class="grid">
    <div class="card" style="margin-bottom:0">
      <div class="stat-label">服务状态</div>
      <div class="stat-value" id="statState">-</div>
      <div style="margin-top: 10px; display: flex; gap: 8px;">
        <button id="serviceToggleBtn" class="success" onclick="toggleService()">启动服务</button>
      </div>
    </div>
    <div class="card" style="margin-bottom:0">
      <div class="stat-label">可用 / 总账户数</div>
      <div class="stat-value" id="statAccounts">0 / 0</div>
      <div style="margin-top: 10px;">
        <button class="secondary" onclick="syncAuthFiles()">同步至 CPA 凭证目录</button>
      </div>
    </div>
    <div class="card" style="margin-bottom:0">
      <div class="stat-label">已发现 AI Studio 模型</div>
      <div class="stat-value" id="statModels">0</div>
      <div style="font-size:12px; color:var(--muted); margin-top:8px;">支持原始模型名与 aistudio/* 前缀</div>
    </div>
    <div class="card" style="margin-bottom:0">
      <div class="stat-label">冷却中 / 需重登账户</div>
      <div class="stat-value" id="statCooldown">0 / 0</div>
      <div style="margin-top: 10px;">
        <button class="secondary" onclick="clearCooldowns('')">清除全部冷却</button>
      </div>
    </div>
  </div>

  <div class="card">
    <h3 style="margin-top:0; font-size:16px;">添加 / 导入 Google AI Studio 账户</h3>
    <div class="row">
      <input type="text" id="accLabel" placeholder="账户邮箱 (留空将自动从 Cookie 识别)" style="flex:1; min-width:220px;" />
      <input type="text" id="accProxy" placeholder="单账户代理 (可选，如 http://127.0.0.1:7890)" style="flex:1; min-width:220px;" />
      <input type="number" id="accAuthUser" placeholder="authuser 序号 (默认 0)" value="0" style="width:140px;" />
    </div>
    <div class="row">
      <textarea id="accCookies" rows="4" style="width:100%; font-family:monospace;" placeholder="粘贴浏览器请求头 Cookie (需包含 SAPISID 与 __Secure-1PSID)、Cookie-Editor JSON 数组、或 Playwright storage-state.json 内容..."></textarea>
    </div>
    <div class="row">
      <div style="display:flex; gap:8px;">
        <button onclick="importCookieAccount()">导入 Cookie / StorageState 账户</button>
        <button class="secondary" onclick="discoverCookieAccounts()">预览 Cookie 内登录的谷歌账号</button>
      </div>
    </div>
  </div>

  <div class="card">
    <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:12px;">
      <h3 style="margin:0; font-size:16px;">账户池与 WAA 状态</h3>
      <button class="secondary" onclick="loadStatus()">刷新列表</button>
    </div>
    <table>
      <thead>
        <tr>
          <th>账户邮箱 (ID)</th>
          <th>状态</th>
          <th>权益等级 (Tier)</th>
          <th>模型数</th>
          <th>代理</th>
          <th>冷却 / 错误信息</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody id="accountsBody">
        <tr><td colspan="7" style="color:var(--muted);">暂无账户数据</td></tr>
      </tbody>
    </table>
  </div>

  <div class="card">
    <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:12px;">
      <h3 style="margin:0; font-size:16px;">运行日志</h3>
      <button class="secondary" onclick="loadLogs()">刷新日志</button>
    </div>
    <div class="logs" id="logsBox">等待连接...</div>
  </div>
</div>

<script>
function getKey() {
  const input = document.getElementById('mgmtKey');
  if (input.value.trim()) return input.value.trim();
  const candidates = ['cpa_aistudio2api_key', 'management_key', 'cpa-management-key', 'apiKey', 'token'];
  for (const k of candidates) {
    const v = localStorage.getItem(k) || sessionStorage.getItem(k);
    if (v && v.trim()) {
      input.value = v.trim();
      return v.trim();
    }
  }
  return '';
}

function saveKeyAndReload() {
  const k = document.getElementById('mgmtKey').value.trim();
  if (k) localStorage.setItem('cpa_aistudio2api_key', k);
  loadStatus();
  loadLogs();
}

function showToast(msg, isErr) {
  const el = document.getElementById('toast');
  el.style.display = 'block';
  el.style.background = isErr ? 'rgba(239, 68, 68, 0.2)' : 'rgba(16, 185, 129, 0.2)';
  el.style.color = isErr ? '#fca5a5' : '#6ee7b7';
  el.textContent = msg;
  setTimeout(() => { el.style.display = 'none'; }, 5000);
}

async function apiCall(path, method = 'GET', body = null) {
  const headers = { 'Content-Type': 'application/json' };
  const key = getKey();
  if (key) {
    headers['Authorization'] = 'Bearer ' + key;
    headers['X-Management-Key'] = key;
  }
  const opts = { method, headers };
  if (body !== null) opts.body = JSON.stringify(body);
  const res = await fetch(path, opts);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.error || data.message || ('HTTP ' + res.status));
  }
  return data;
}

async function loadStatus() {
  try {
    const data = await apiCall('/v0/management/aistudio2api/status');
    const st = data.status || {};
    const counts = st.accounts || {};
    const state = (st.state || 'STOPPED').toUpperCase();
    const isRunning = Boolean(st.running || state === 'RUNNING');
    document.getElementById('statState').textContent = state;
    const badge = document.getElementById('stateBadge');
    badge.textContent = state;
    badge.className = 'badge ' + (isRunning ? 'success' : 'warning');
    const toggleBtn = document.getElementById('serviceToggleBtn');
    if (toggleBtn) {
      toggleBtn.dataset.running = isRunning ? 'true' : 'false';
      if (state === 'LAUNCHING' || state === 'STOPPING') {
        toggleBtn.textContent = state === 'LAUNCHING' ? '启动中...' : '停止中...';
        toggleBtn.className = 'secondary';
        toggleBtn.disabled = true;
      } else if (isRunning) {
        toggleBtn.textContent = '停止服务';
        toggleBtn.className = 'danger';
        toggleBtn.disabled = false;
      } else {
        toggleBtn.textContent = '启动服务';
        toggleBtn.className = 'success';
        toggleBtn.disabled = false;
      }
    }
    document.getElementById('waaBadge').textContent = 'WAA: ' + (data.waa_backend || 'spidermonkey');
    document.getElementById('statAccounts').textContent = ((counts.ready || 0) + (counts.busy || 0)) + ' / ' + (counts.total || 0);
    document.getElementById('statModels').textContent = data.models_count || 0;
    document.getElementById('statCooldown').textContent = (counts.cooldown || 0) + ' / ' + (counts.auth_required || 0);
    renderAccounts(data.accounts || []);
  } catch (e) {
    showToast('读取状态失败 (请确认右上角已填写 CPA 管理密钥): ' + e.message, true);
  }
}

function renderAccounts(accounts) {
  const tbody = document.getElementById('accountsBody');
  if (!accounts.length) {
    tbody.innerHTML = '<tr><td colspan="7" style="color:var(--muted);">暂无已导入账户，请在上方粘贴 Cookie 或使用本机 Chrome 一键导入</td></tr>';
    return;
  }
  tbody.innerHTML = accounts.map(acc => {
    let badgeClass = '';
    if (acc.state === 'ready' || acc.state === 'busy') badgeClass = 'success';
    else if (acc.state === 'cooldown') badgeClass = 'warning';
    else badgeClass = 'danger';
    const info = acc.cooldown_reason || acc.auth_error || acc.catalog_error || '-';
    return '<tr>' +
      '<td><strong>' + escapeHtml(acc.id) + '</strong></td>' +
      '<td><span class="badge ' + badgeClass + '">' + escapeHtml(acc.state) + '</span></td>' +
      '<td><span class="badge">' + escapeHtml(acc.benefit_tier || 'FREE') + '</span></td>' +
      '<td>' + (acc.models ? acc.models.length : 0) + '</td>' +
      '<td>' + escapeHtml(acc.proxy || '全局/直连') + '</td>' +
      '<td style="max-width:260px; word-break:break-all; color:var(--muted);">' + escapeHtml(info) + '</td>' +
      '<td style="display:flex; gap:6px; flex-wrap:wrap;">' +
        '<button class="secondary" onclick="refreshAccount(\'' + escapeJs(acc.id) + '\')">刷新目录</button>' +
        '<button class="secondary" onclick="toggleAccount(\'' + escapeJs(acc.id) + '\', ' + (!acc.enabled) + ', \'' + escapeJs(acc.proxy || '') + '\')">' + (acc.enabled ? '禁用' : '启用') + '</button>' +
        '<button class="danger" onclick="deleteAccount(\'' + escapeJs(acc.id) + '\')">删除</button>' +
      '</td>' +
    '</tr>';
  }).join('');
}

async function toggleService() {
  const toggleBtn = document.getElementById('serviceToggleBtn');
  if (toggleBtn && toggleBtn.dataset.running === 'true') {
    await stopService();
  } else {
    await startService();
  }
}

async function startService() {
  const toggleBtn = document.getElementById('serviceToggleBtn');
  if (toggleBtn) {
    toggleBtn.disabled = true;
    toggleBtn.textContent = '启动中...';
  }
  try {
    showToast('正在启动生成服务并预热 SpiderMonkey WAA Worker...', false);
    await apiCall('/v0/management/aistudio2api/service/start', 'POST', {});
    showToast('生成服务已启动', false);
    await loadStatus();
    loadLogs();
  } catch (e) {
    showToast('启动失败: ' + e.message, true);
    await loadStatus();
  }
}

async function stopService() {
  const toggleBtn = document.getElementById('serviceToggleBtn');
  if (toggleBtn) {
    toggleBtn.disabled = true;
    toggleBtn.textContent = '停止中...';
  }
  try {
    await apiCall('/v0/management/aistudio2api/service/stop', 'POST', {});
    showToast('生成服务已停止', false);
    await loadStatus();
  } catch (e) {
    showToast('停止失败: ' + e.message, true);
    await loadStatus();
  }
}

async function importCookieAccount() {
  const cookies = document.getElementById('accCookies').value.trim();
  if (!cookies) {
    showToast('请先粘贴 Cookie 或 storage-state.json 内容', true);
    return;
  }
  const label = document.getElementById('accLabel').value.trim();
  const proxy = document.getElementById('accProxy').value.trim();
  const authUser = parseInt(document.getElementById('accAuthUser').value || '0', 10);
  try {
    showToast('正在验证 Cookie 并导入账户...', false);
    const res = await apiCall('/v0/management/aistudio2api/accounts', 'POST', {
      label, proxy, auth_user: authUser, cookies
    });
    document.getElementById('accCookies').value = '';
    showToast('成功导入账户: ' + (res.account ? res.account.id : label), false);
    loadStatus();
    loadLogs();
  } catch (e) {
    showToast('导入失败: ' + e.message, true);
  }
}

async function discoverCookieAccounts() {
  const cookies = document.getElementById('accCookies').value.trim();
  if (!cookies) {
    showToast('请先粘贴 Cookie 内容', true);
    return;
  }
  const proxy = document.getElementById('accProxy').value.trim();
  try {
    const res = await apiCall('/v0/management/aistudio2api/accounts/discover-cookies', 'POST', { cookies, proxy });
    const list = (res.accounts || []).map(a => a.email + ' (authuser=' + a.auth_user + (a.imported ? ', 已导入' : '') + ')').join(', ');
    showToast('识别到账号: ' + (list || '无'), false);
  } catch (e) {
    showToast('识别失败: ' + e.message, true);
  }
}

async function refreshAccount(id) {
  try {
    showToast('正在刷新账户 ' + id + ' 的模型目录与权益...', false);
    const res = await apiCall('/v0/management/aistudio2api/accounts/refresh', 'POST', { account_id: id });
    showToast('刷新成功，共 ' + (res.models_count || 0) + ' 个模型', false);
    loadStatus();
  } catch (e) {
    showToast('刷新失败: ' + e.message, true);
  }
}

async function toggleAccount(id, enabled, proxy) {
  try {
    await apiCall('/v0/management/aistudio2api/accounts/update', 'POST', {
      account_id: id, enabled, proxy, locale: 'en-US', timezone: 'America/Los_Angeles'
    });
    loadStatus();
  } catch (e) {
    showToast('更新失败: ' + e.message, true);
  }
}

async function deleteAccount(id) {
  if (!confirm('确认删除账户 ' + id + ' 吗？')) return;
  try {
    await apiCall('/v0/management/aistudio2api/accounts/delete', 'POST', { account_id: id });
    showToast('已删除账户 ' + id, false);
    loadStatus();
  } catch (e) {
    showToast('删除失败: ' + e.message, true);
  }
}

async function syncAuthFiles() {
  try {
    const res = await apiCall('/v0/management/aistudio2api/accounts/sync-auth', 'POST', {});
    showToast('已同步 ' + (res.synced || 0) + ' 个账户至 CLIProxyAPI 凭证目录', false);
  } catch (e) {
    showToast('同步失败: ' + e.message, true);
  }
}

async function clearCooldowns(accountId) {
  try {
    await apiCall('/v0/management/aistudio2api/cooldowns/clear', 'POST', { account_id: accountId });
    showToast('冷却已清除', false);
    loadStatus();
  } catch (e) {
    showToast('清除冷却失败: ' + e.message, true);
  }
}

async function loadLogs() {
  try {
    const res = await apiCall('/v0/management/aistudio2api/logs');
    const logs = res.logs || [];
    const box = document.getElementById('logsBox');
    if (!logs.length) {
      box.innerHTML = '<div style="color:var(--muted);">暂无日志</div>';
      return;
    }
    box.innerHTML = logs.slice(-80).reverse().map(l =>
      '<div class="log-line"><span style="color:var(--muted);">[' + escapeHtml(l.time || '') + ']</span> ' +
      '<strong>[' + escapeHtml(l.level || 'INFO') + ']</strong> ' +
      '<span style="color:#60a5fa;">[' + escapeHtml(l.source || '') + ']</span> ' +
      escapeHtml(l.message || '') + '</div>'
    ).join('');
  } catch (_) {}
}

function escapeHtml(s) {
  return String(s || '').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}
function escapeJs(s) {
  return String(s || '').replace(/\\/g,'\\\\').replace(/'/g,"\\'");
}

getKey();
loadStatus();
loadLogs();
setInterval(() => { loadStatus(); loadLogs(); }, 6000);
</script>
</body>
</html>
`
