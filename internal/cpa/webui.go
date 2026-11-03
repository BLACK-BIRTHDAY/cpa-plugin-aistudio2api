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
  button.warning { background: var(--warning); color: #0f172a; font-weight: 600; }
  button.warning:hover { background: #d97706; color: #fff; }
  button.sm { padding: 5px 10px; font-size: 12px; white-space: nowrap; }
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
  .method-panel {
    background: #0b1120;
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 14px 16px;
    margin-bottom: 14px;
  }
  .method-panel:last-child { margin-bottom: 0; }
  .method-title {
    font-weight: 600;
    font-size: 14px;
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .method-desc {
    font-size: 12px;
    color: var(--muted);
    margin-top: 4px;
    margin-bottom: 12px;
  }
  .stat-label { font-size: 12px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.04em; }
  .stat-value { font-size: 24px; font-weight: 700; margin-top: 4px; }
  .row { display: flex; gap: 10px; flex-wrap: wrap; align-items: center; margin-bottom: 10px; }
  .row:last-child { margin-bottom: 0; }
  table { width: 100%; border-collapse: collapse; font-size: 13px; }
  th, td { text-align: left; padding: 10px 12px; border-bottom: 1px solid var(--border); vertical-align: middle; }
  th { color: var(--muted); font-weight: 500; }
  .cooldown-item {
    background: rgba(245, 158, 11, 0.12);
    border: 1px solid rgba(245, 158, 11, 0.35);
    border-radius: 6px;
    padding: 6px 8px;
    margin-bottom: 6px;
    font-size: 12px;
  }
  .cooldown-item:last-child { margin-bottom: 0; }
  .proxy-editor {
    display: flex;
    gap: 6px;
    align-items: center;
    flex-wrap: wrap;
    margin-top: 6px;
  }
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
      <span id="keyStatusBadge" class="badge">只读模式 (查看无需密钥)</span>
      <input type="password" id="mgmtKey" placeholder="输入 CPA 管理密钥 (仅修改操作需要)" style="width: 250px;" oninput="onKeyInputChanged()" />
      <button onclick="saveKeyAndReload()">保存密钥</button>
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
      <div style="font-size:12px; color:var(--muted); margin-top:8px;">含 Playground 与 Build 通道原生模型</div>
    </div>
    <div class="card" style="margin-bottom:0">
      <div class="stat-label">冷却中 / 需重登账户</div>
      <div class="stat-value" id="statCooldown">0 / 0</div>
      <div style="font-size:12px; color:var(--muted); margin-top:8px;">支持在下方按单账号查看结束时间并单独解除</div>
    </div>
  </div>

  <div class="card">
    <h3 style="margin-top:0; margin-bottom:14px; font-size:16px;">添加 / 登录 Google AI Studio 账户</h3>

    <div class="method-panel">
      <div class="method-title">
        <span>方式一：本机 Chrome 无痕浏览器登录</span>
        <span class="badge success">推荐</span>
      </div>
      <div id="chromeLoginHint" class="method-desc">
        弹出隔离的本机 Chrome 窗口（独立临时目录，不读取本机已有浏览器数据），完成新号条款确认与 Google Drive 授权后自动拉取凭证并关闭窗口。
      </div>
      <div class="row">
        <input type="text" id="chromeLoginProxy" placeholder="单账户代理 (可选，如 http://127.0.0.1:7890 或 socks5://127.0.0.1:1080，留空则使用全局/直连)" style="flex:1; min-width:280px;" />
        <button id="chromeLoginBtn" class="success" onclick="loginWithChromeBrowser()" style="white-space:nowrap; padding:9px 18px; font-weight:600;">
          🌐 启动本机 Chrome 无痕登录
        </button>
      </div>
    </div>

    <div class="method-panel">
      <div class="method-title">
        <span>方式二：手动粘贴 Cookie / StorageState 导入</span>
        <span class="badge">适用于无桌面或远程服务器</span>
      </div>
      <div class="method-desc">
        支持直接粘贴请求头 Cookie（含 SAPISID 与 __Secure-1PSID）、Cookie-Editor JSON 数组或 Playwright storage-state.json。
      </div>
      <div class="row">
        <input type="text" id="accLabel" placeholder="账户邮箱 (留空将自动从 Cookie 识别)" style="flex:1; min-width:200px;" />
        <input type="text" id="cookieProxy" placeholder="单账户代理 (可选，如 http://127.0.0.1:7890，留空使用全局/直连)" style="flex:1; min-width:260px;" />
        <input type="number" id="accAuthUser" placeholder="authuser 序号 (默认 0)" value="0" style="width:130px;" title="Google 多账号会话序号 (authuser)" />
      </div>
      <div class="row">
        <textarea id="accCookies" rows="3" style="width:100%; font-family:monospace;" placeholder="粘贴浏览器请求头 Cookie (需包含 SAPISID 与 __Secure-1PSID)、Cookie-Editor JSON 数组、或 Playwright storage-state.json 内容..."></textarea>
      </div>
      <div class="row">
        <button onclick="importCookieAccount()">导入 Cookie / StorageState 账户</button>
        <button class="secondary" onclick="discoverCookieAccounts()">预览 Cookie 内登录的谷歌账号</button>
      </div>
    </div>
  </div>

  <div class="card">
    <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:12px;">
      <div style="display:flex; align-items:center; gap:10px;">
        <h3 style="margin:0; font-size:16px;">冷却中账户一览</h3>
        <span id="cooldownCountBadge" class="badge">0 条冷却</span>
      </div>
      <span style="font-size:12px; color:var(--muted);">展示各账户冷却原因、精确恢复时间与剩余倒计时</span>
    </div>
    <div id="cooldownOverviewBox">
      <div style="color:var(--muted); font-size:13px;">当前没有处于冷却中的账户或模型</div>
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
          <th>单账户代理</th>
          <th>冷却结束时间 / 状态详情</th>
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
let lastRejectedKey = null;
let latestAccounts = [];
let latestCooldowns = [];
let editingProxyAccountId = null;
let editingProxyDraft = '';

function resourceUrl(view) {
  const base = window.location.pathname && window.location.pathname !== '/'
    ? window.location.pathname
    : '/v0/resource/plugins/aistudio2api/accounts';
  return base + '?view=' + encodeURIComponent(view);
}

function updateKeyBadge() {
  const badge = document.getElementById('keyStatusBadge');
  const input = document.getElementById('mgmtKey');
  if (!badge || !input) return;
  const k = input.value.trim();
  if (!k) {
    badge.textContent = '只读模式 (查看无需密钥)';
    badge.className = 'badge';
    input.style.borderColor = '';
  } else if (lastRejectedKey !== null && k === lastRejectedKey) {
    badge.textContent = '密钥无效 (已防封拦截)';
    badge.className = 'badge danger';
    input.style.borderColor = 'var(--danger)';
  } else {
    badge.textContent = '已配置管理密钥';
    badge.className = 'badge success';
    input.style.borderColor = '';
  }
}

function onKeyInputChanged() {
  updateKeyBadge();
}

function getKey() {
  const input = document.getElementById('mgmtKey');
  if (input && input.value.trim()) {
    updateKeyBadge();
    return input.value.trim();
  }
  const params = new URLSearchParams(window.location.search);
  const urlKey = params.get('key') || params.get('management_key') || params.get('token');
  if (urlKey && urlKey.trim()) {
    input.value = urlKey.trim();
    localStorage.setItem('cpa_aistudio2api_key', urlKey.trim());
    updateKeyBadge();
    return urlKey.trim();
  }
  const candidates = ['cpa_aistudio2api_key', 'management_key', 'cpa-management-key', 'secret-key', 'secretKey', 'managementKey', 'apiKey', 'token'];
  const stores = [localStorage, sessionStorage];
  try {
    if (window.parent && window.parent !== window && window.parent.localStorage) {
      stores.push(window.parent.localStorage);
    }
  } catch (_) {}
  try {
    if (window.opener && window.opener.localStorage) {
      stores.push(window.opener.localStorage);
    }
  } catch (_) {}
  for (const store of stores) {
    for (const k of candidates) {
      try {
        const v = store.getItem(k);
        if (v && v.trim()) {
          input.value = v.trim();
          updateKeyBadge();
          return v.trim();
        }
      } catch (_) {}
    }
  }
  updateKeyBadge();
  return '';
}

function saveKeyAndReload() {
  const k = document.getElementById('mgmtKey').value.trim();
  if (k) {
    localStorage.setItem('cpa_aistudio2api_key', k);
    if (lastRejectedKey === k) lastRejectedKey = null;
    showToast('管理密钥已保存，现在可以执行管理操作', false);
  } else {
    localStorage.removeItem('cpa_aistudio2api_key');
    lastRejectedKey = null;
    showToast('已切换为免密只读浏览模式', false);
  }
  updateKeyBadge();
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

async function apiCall(path, method = 'GET', body = null, extraHeaders = null) {
  const headers = { 'Content-Type': 'application/json' };
  if (extraHeaders) {
    Object.assign(headers, extraHeaders);
  }
  const isMgmtRoute = path.startsWith('/v0/management/');
  const key = getKey();
  if (isMgmtRoute) {
    if (!key) {
      const input = document.getElementById('mgmtKey');
      if (input) {
        input.focus();
        input.style.borderColor = 'var(--warning)';
      }
      throw new Error('请先在右上角填入 CPA 管理密钥后再执行修改操作（查看页面与日志无需密钥）');
    }
    if (lastRejectedKey !== null && key === lastRejectedKey) {
      const input = document.getElementById('mgmtKey');
      if (input) input.focus();
      throw new Error('当前管理密钥验证失败 (401/403)，为防止触发 CPA IP 封禁已拦截请求，请修改正确的密钥后点击「保存密钥」');
    }
    headers['Authorization'] = 'Bearer ' + key;
    headers['X-Management-Key'] = key;
  }
  const opts = { method, headers };
  if (body !== null) opts.body = JSON.stringify(body);
  const res = await fetch(path, opts);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    if (isMgmtRoute && (res.status === 401 || res.status === 403)) {
      lastRejectedKey = key;
      updateKeyBadge();
      throw new Error('CPA 管理密钥错误 (HTTP ' + res.status + ')，已停止重复请求以防止 IP 被封禁，请核对右上角管理密钥');
    }
    throw new Error(data.error || data.message || ('HTTP ' + res.status));
  }
  return data;
}

async function loadStatus() {
  try {
    const data = await apiCall(resourceUrl('status'), 'GET', null, { 'X-AIStudio-view': 'status' });
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
    latestAccounts = data.accounts || [];
    latestCooldowns = deduplicateCooldowns(data.cooldowns || []);
    const cooldownAccountsSet = new Set(latestCooldowns.map(c => String(c.account_id || '').toLowerCase()));
    for (const acc of latestAccounts) {
      if (acc.state === 'cooldown') cooldownAccountsSet.add(String(acc.id || '').toLowerCase());
    }
    const effectiveCooldownCount = Math.max(counts.cooldown || 0, cooldownAccountsSet.size);
    document.getElementById('statCooldown').textContent = effectiveCooldownCount + ' / ' + (counts.auth_required || 0);
    renderCooldownOverview(latestCooldowns);
    renderAccounts(latestAccounts, latestCooldowns);
  } catch (e) {
    showToast('读取状态失败: ' + e.message, true);
  }
}

function deduplicateCooldowns(rawList) {
  const map = new Map();
  for (const item of (rawList || [])) {
    if (!item || !item.account_id) continue;
    const scope = item.scope || item.model_id || '*';
    const key = String(item.account_id).toLowerCase() + '|' + scope;
    const existing = map.get(key);
    if (!existing || new Date(item.until) > new Date(existing.until)) {
      map.set(key, {
        account_id: item.account_id,
        account_label: item.account_label || item.account_id,
        channel: scope === '*' ? 'all' : (item.channel || 'playground'),
        model_id: scope === '*' ? '全部模型 (全局冷却)' : (item.model_id || scope),
        scope: scope,
        until: item.until,
        reason: item.reason || ''
      });
    }
  }
  return Array.from(map.values()).sort((a, b) => new Date(a.until) - new Date(b.until));
}

function formatCooldownEndTime(untilStr) {
  if (!untilStr) return '-';
  const d = new Date(untilStr);
  if (isNaN(d.getTime())) return String(untilStr);
  const pad = n => String(n).padStart(2, '0');
  return d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate()) + ' ' +
    pad(d.getHours()) + ':' + pad(d.getMinutes()) + ':' + pad(d.getSeconds());
}

function formatCooldownRemaining(untilStr) {
  if (!untilStr) return '';
  const d = new Date(untilStr);
  if (isNaN(d.getTime())) return '';
  const diffSec = Math.max(0, Math.ceil((d.getTime() - Date.now()) / 1000));
  if (diffSec <= 0) return '即将恢复';
  const h = Math.floor(diffSec / 3600);
  const m = Math.floor((diffSec % 3600) / 60);
  const s = diffSec % 60;
  if (h > 0) return '剩余 ' + h + '小时' + m + '分' + s + '秒';
  if (m > 0) return '剩余 ' + m + '分' + String(s).padStart(2, '0') + '秒';
  return '剩余 ' + s + '秒';
}

function renderCooldownOverview(cooldowns) {
  const box = document.getElementById('cooldownOverviewBox');
  const badge = document.getElementById('cooldownCountBadge');
  if (!box) return;
  const active = (cooldowns || []).filter(c => {
    const t = new Date(c.until).getTime();
    return isNaN(t) || t > Date.now() - 1000;
  });
  if (badge) {
    badge.textContent = active.length ? (active.length + ' 项冷却中') : '0 项冷却';
    badge.className = 'badge ' + (active.length ? 'warning' : 'success');
  }
  if (!active.length) {
    box.innerHTML = '<div style="color:var(--muted); font-size:13px;">✓ 当前没有处于冷却中的账户或模型</div>';
    return;
  }
  box.innerHTML = '<table>' +
    '<thead><tr>' +
      '<th>账户邮箱</th>' +
      '<th>冷却范围 / 模型</th>' +
      '<th>冷却结束时间</th>' +
      '<th>剩余时间</th>' +
      '<th>冷却原因</th>' +
      '<th>操作</th>' +
    '</tr></thead>' +
    '<tbody>' +
    active.map(c => {
      const endText = formatCooldownEndTime(c.until);
      const remText = formatCooldownRemaining(c.until);
      const scopeBadge = c.scope === '*'
        ? '<span class="badge warning">全局冷却 (全部模型)</span>'
        : '<span class="badge">' + escapeHtml(c.model_id) + '</span> <span style="font-size:11px; color:var(--muted);">[' + escapeHtml(c.channel) + ']</span>';
      return '<tr>' +
        '<td><strong>' + escapeHtml(c.account_id) + '</strong></td>' +
        '<td>' + scopeBadge + '</td>' +
        '<td style="font-family:monospace; color:#fbbf24; font-weight:600;">' + escapeHtml(endText) + '</td>' +
        '<td><span class="badge warning" data-until="' + escapeHtml(c.until) + '">' + escapeHtml(remText) + '</span></td>' +
        '<td style="max-width:280px; word-break:break-all; color:var(--muted);">' + escapeHtml(c.reason || '-') + '</td>' +
        '<td style="display:flex; gap:6px; flex-wrap:wrap;">' +
          '<button class="warning sm" onclick="clearCooldowns(\'' + escapeJs(c.account_id) + '\', \'' + escapeJs(c.scope) + '\')">清除此项冷却</button>' +
        '</td>' +
      '</tr>';
    }).join('') +
    '</tbody></table>';
}

function renderAccounts(accounts, cooldowns) {
  const tbody = document.getElementById('accountsBody');
  if (!accounts.length) {
    tbody.innerHTML = '<tr><td colspan="7" style="color:var(--muted);">暂无已导入账户，请点击上方「启动本机 Chrome 无痕登录」或粘贴 Cookie 导入</td></tr>';
    return;
  }
  const cdByAccount = new Map();
  for (const c of (cooldowns || [])) {
    const key = String(c.account_id || '').toLowerCase();
    if (!cdByAccount.has(key)) cdByAccount.set(key, []);
    cdByAccount.get(key).push(c);
  }
  tbody.innerHTML = accounts.map(acc => {
    const accKey = String(acc.id || '').toLowerCase();
    const accCooldowns = cdByAccount.get(accKey) || [];
    const hasCooldown = acc.state === 'cooldown' || accCooldowns.length > 0;
    let badgeClass = '';
    if (acc.state === 'ready' || acc.state === 'busy') badgeClass = 'success';
    else if (acc.state === 'cooldown') badgeClass = 'warning';
    else badgeClass = 'danger';

    // 单账户代理列（支持直接点击设置/修改/清空）
    let proxyCell = '';
    if (editingProxyAccountId === acc.id) {
      proxyCell =
        '<div class="proxy-editor">' +
          '<input type="text" id="proxyInput_' + escapeHtml(acc.id) + '" value="' + escapeHtml(editingProxyDraft) + '" placeholder="如 http://127.0.0.1:7890 (留空直连)" style="width:210px; padding:5px 8px; font-size:12px;" />' +
          '<button class="success sm" onclick="saveAccountProxy(\'' + escapeJs(acc.id) + '\')">保存</button>' +
          '<button class="secondary sm" onclick="saveAccountProxy(\'' + escapeJs(acc.id) + '\', \'\')">改为直连</button>' +
          '<button class="secondary sm" onclick="cancelEditProxy()">取消</button>' +
        '</div>';
    } else {
      const proxyLabel = acc.proxy ? escapeHtml(acc.proxy) : '<span style="color:var(--muted);">全局/直连</span>';
      const egressHint = acc.egress && acc.egress !== acc.proxy ? '<div style="font-size:11px; color:var(--muted);">出口: ' + escapeHtml(acc.egress) + '</div>' : '';
      proxyCell =
        '<div style="display:flex; align-items:center; gap:8px; flex-wrap:wrap;">' +
          '<div>' + proxyLabel + egressHint + '</div>' +
          '<button class="secondary sm" onclick="openEditProxy(\'' + escapeJs(acc.id) + '\', \'' + escapeJs(acc.proxy || '') + '\')">设置代理</button>' +
        '</div>';
    }

    // 冷却结束时间与错误详情列
    let detailParts = [];
    for (const c of accCooldowns) {
      const endText = formatCooldownEndTime(c.until);
      const remText = formatCooldownRemaining(c.until);
      const title = c.scope === '*' ? '全局冷却' : c.model_id;
      detailParts.push(
        '<div class="cooldown-item">' +
          '<div style="display:flex; justify-content:space-between; gap:6px; align-items:center;">' +
            '<strong style="color:#fbbf24;">⏳ ' + escapeHtml(title) + '</strong>' +
            '<span class="badge warning" data-until="' + escapeHtml(c.until) + '" style="font-size:11px; padding:1px 6px;">' + escapeHtml(remText) + '</span>' +
          '</div>' +
          '<div style="color:#fde68a; font-family:monospace; margin-top:2px;">结束时间: ' + escapeHtml(endText) + '</div>' +
          (c.reason ? '<div style="color:var(--muted); margin-top:2px;">原因: ' + escapeHtml(c.reason) + '</div>' : '') +
        '</div>'
      );
    }
    const rawMsg = acc.message || acc.cooldown_reason || acc.auth_error || acc.catalog_error || '';
    if (rawMsg && !accCooldowns.some(c => c.reason === rawMsg)) {
      detailParts.push('<div style="color:var(--muted);">' + escapeHtml(rawMsg) + '</div>');
    }
    const detailHtml = detailParts.length ? detailParts.join('') : '<span style="color:var(--muted);">-</span>';

    const clearCooldownBtn = hasCooldown
      ? '<button class="warning sm" onclick="clearCooldowns(\'' + escapeJs(acc.id) + '\', \'\')">清除冷却</button>'
      : '';

    return '<tr>' +
      '<td><strong>' + escapeHtml(acc.id) + '</strong></td>' +
      '<td><span class="badge ' + badgeClass + '">' + escapeHtml(acc.state) + '</span></td>' +
      '<td><span class="badge">' + escapeHtml(acc.benefit_tier || 'FREE') + '</span></td>' +
      '<td>' + (acc.models ? acc.models.length : 0) + '</td>' +
      '<td>' + proxyCell + '</td>' +
      '<td style="max-width:310px; word-break:break-all;">' + detailHtml + '</td>' +
      '<td><div style="display:flex; gap:6px; flex-wrap:wrap;">' +
        clearCooldownBtn +
        '<button class="sm" onclick="reloginAccount(\'' + escapeJs(acc.id) + '\', \'' + escapeJs(acc.proxy || '') + '\')">无痕重登</button>' +
        '<button class="secondary sm" onclick="refreshAccount(\'' + escapeJs(acc.id) + '\')">刷新目录</button>' +
        '<button class="secondary sm" onclick="toggleAccount(\'' + escapeJs(acc.id) + '\', ' + (!acc.enabled) + ', \'' + escapeJs(acc.proxy || '') + '\')">' + (acc.enabled ? '禁用' : '启用') + '</button>' +
        '<button class="danger sm" onclick="deleteAccount(\'' + escapeJs(acc.id) + '\')">删除</button>' +
      '</div></td>' +
    '</tr>';
  }).join('');
}

function openEditProxy(accountId, currentProxy) {
  editingProxyAccountId = accountId;
  editingProxyDraft = currentProxy || '';
  renderAccounts(latestAccounts, latestCooldowns);
  setTimeout(() => {
    const el = document.getElementById('proxyInput_' + accountId);
    if (el) el.focus();
  }, 30);
}

function cancelEditProxy() {
  editingProxyAccountId = null;
  editingProxyDraft = '';
  renderAccounts(latestAccounts, latestCooldowns);
}

async function saveAccountProxy(accountId, overrideProxy) {
  const acc = (latestAccounts || []).find(a => a.id === accountId);
  const enabled = acc ? Boolean(acc.enabled) : true;
  const locale = (acc && acc.locale) ? acc.locale : 'en-US';
  const timezone = (acc && acc.timezone) ? acc.timezone : 'America/Los_Angeles';
  let proxy = '';
  if (typeof overrideProxy === 'string') {
    proxy = overrideProxy.trim();
  } else {
    const input = document.getElementById('proxyInput_' + accountId);
    proxy = input ? input.value.trim() : '';
  }
  try {
    showToast('正在更新账户 ' + accountId + ' 的代理配置...', false);
    await apiCall('/v0/management/aistudio2api/accounts/update', 'POST', {
      account_id: accountId,
      enabled: enabled,
      proxy: proxy,
      locale: locale,
      timezone: timezone
    });
    editingProxyAccountId = null;
    editingProxyDraft = '';
    showToast('账户 ' + accountId + ' 代理已更新为: ' + (proxy || '全局/直连'), false);
    await loadStatus();
    loadLogs();
  } catch (e) {
    showToast('更新代理失败: ' + e.message, true);
  }
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


async function loginWithChromeBrowser() {
  const btn = document.getElementById('chromeLoginBtn');
  const proxyInput = document.getElementById('chromeLoginProxy');
  const proxy = proxyInput ? proxyInput.value.trim() : '';
  if (btn) {
    btn.disabled = true;
    btn.textContent = '⏳ 正在等待 Chrome 无痕窗口登录...';
  }
  try {
    showToast('已启动本机 Chrome 无痕窗口，请在弹出的浏览器中登录 Google 账号并完成 AI Studio 授权，完成后窗口将自动关闭...', false);
    const res = await apiCall('/v0/management/aistudio2api/accounts/login-browser', 'POST', { proxy });
    const accId = res.account ? res.account.id : '';
    showToast('Chrome 无痕登录成功，已自动导入账户: ' + accId, false);
    await loadStatus();
    loadLogs();
  } catch (e) {
    showToast('Chrome 无痕登录未完成: ' + e.message, true);
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.textContent = '🌐 启动本机 Chrome 无痕登录';
    }
  }
}

async function reloginAccount(id, proxy) {
  try {
    showToast('已为账户 ' + id + ' 启动本机 Chrome 无痕窗口，请使用该谷歌账号完成登录...', false);
    const res = await apiCall('/v0/management/aistudio2api/accounts/relogin', 'POST', { account_id: id, proxy: proxy || '' });
    showToast('账户 ' + (res.account ? res.account.id : id) + ' 重新登录并更新成功！', false);
    await loadStatus();
    loadLogs();
  } catch (e) {
    showToast('重新登录失败: ' + e.message, true);
  }
}
async function importCookieAccount() {
  const cookies = document.getElementById('accCookies').value.trim();
  if (!cookies) {
    showToast('请先粘贴 Cookie 或 storage-state.json 内容', true);
    return;
  }
  const label = document.getElementById('accLabel').value.trim();
  const proxyEl = document.getElementById('cookieProxy');
  const proxy = proxyEl ? proxyEl.value.trim() : '';
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
  const proxyEl = document.getElementById('cookieProxy');
  const proxy = proxyEl ? proxyEl.value.trim() : '';
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

async function clearCooldowns(accountId, scope = '') {
  if (!accountId) {
    showToast('请指定要清除冷却的账户', true);
    return;
  }
  try {
    await apiCall('/v0/management/aistudio2api/cooldowns/clear', 'POST', {
      account_id: accountId,
      scope: scope || ''
    });
    showToast('已清除账户 ' + accountId + (scope && scope !== '*' ? (' 模型 ' + scope) : '') + ' 的冷却状态', false);
    await loadStatus();
    loadLogs();
  } catch (e) {
    showToast('清除冷却失败: ' + e.message, true);
  }
}

async function loadLogs() {
  try {
    const res = await apiCall(resourceUrl('logs'), 'GET', null, { 'X-AIStudio-view': 'logs' });
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
if (new URLSearchParams(window.location.search).get('autologin') === '1') {
  window.history.replaceState({}, '', window.location.pathname);
  setTimeout(() => loginWithChromeBrowser(), 300);
}
setInterval(() => {
  document.querySelectorAll('[data-until]').forEach(el => {
    const u = el.getAttribute('data-until');
    if (u) el.textContent = formatCooldownRemaining(u);
  });
}, 1000);
setInterval(() => {
  if (!editingProxyAccountId) {
    loadStatus();
  }
  loadLogs();
}, 6000);
</script>
</body>
</html>
`
