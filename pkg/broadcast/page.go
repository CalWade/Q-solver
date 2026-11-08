package broadcast

// indexHTML 是推给 iPad/Mac 浏览器的答案页面。
// 设计重点：
//   - 支持深色 / 浅色切换（点右上角切换按钮，localStorage 持久化）。
//   - SSE 实时接收流式 chunk，断线自动重连。
//   - Markdown 渲染：优先 CDN 加载 marked.js + highlight.js，加载失败则降级为 <pre> 纯文本。
//   - iPad 友好：大字号、无缩放、全局无选中高亮（贴桌面使用场景）。
//   - 不依赖任何跨域资源时必须跑起来（CDN 挂了也能用）。
const indexHTML = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover,user-scalable=no">
<meta name="apple-mobile-web-app-capable" content="yes">
<meta name="apple-mobile-web-app-status-bar-style" content="black-translucent">
<title>Answer</title>
<style>
  :root {
    --bg: #0b0f14;
    --fg: #e8eef7;
    --muted: #8a96a8;
    --accent: #6ea8ff;
    --code-bg: #161d26;
    --border: #1e2633;
  }
  html.light {
    --bg: #fdfdfb;
    --fg: #1a1f29;
    --muted: #6b7684;
    --accent: #2b6cff;
    --code-bg: #f1f3f7;
    --border: #e4e7ec;
  }
  * { box-sizing: border-box; }
  html, body {
    margin: 0;
    padding: 0;
    background: var(--bg);
    color: var(--fg);
    font-family: -apple-system, "SF Pro Text", "PingFang SC", "Microsoft YaHei", sans-serif;
    font-size: 17px;
    line-height: 1.65;
    -webkit-tap-highlight-color: transparent;
    -webkit-user-select: text;
    overscroll-behavior: none;
  }
  body {
    min-height: 100vh;
    padding: 16px 20px 40px;
  }
  #topbar {
    position: sticky;
    top: 0;
    z-index: 10;
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: var(--bg);
    padding: 8px 0 10px;
    border-bottom: 1px solid var(--border);
    margin-bottom: 14px;
  }
  #status {
    font-size: 13px;
    color: var(--muted);
    display: flex;
    align-items: center;
    gap: 6px;
  }
  #status .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #888;
    transition: background 0.2s;
  }
  #status.live .dot { background: #2ecc71; }
  #status.err .dot { background: #e74c3c; }
  #theme-toggle {
    background: transparent;
    border: 1px solid var(--border);
    color: var(--fg);
    padding: 6px 12px;
    border-radius: 8px;
    font-size: 14px;
    cursor: pointer;
    -webkit-appearance: none;
  }
  #content {
    font-size: 17px;
    word-wrap: break-word;
  }
  #content h1, #content h2, #content h3, #content h4 {
    margin: 1.2em 0 0.5em;
    line-height: 1.3;
  }
  #content h1 { font-size: 1.5em; }
  #content h2 { font-size: 1.3em; }
  #content h3 { font-size: 1.15em; }
  #content p { margin: 0.6em 0; }
  #content ul, #content ol { padding-left: 1.5em; }
  #content li { margin: 0.2em 0; }
  #content code {
    background: var(--code-bg);
    padding: 2px 6px;
    border-radius: 4px;
    font-family: "SF Mono", Menlo, Consolas, monospace;
    font-size: 0.92em;
  }
  #content pre {
    background: var(--code-bg);
    padding: 12px 14px;
    border-radius: 8px;
    overflow-x: auto;
    font-size: 14px;
    line-height: 1.5;
    border: 1px solid var(--border);
  }
  #content pre code {
    background: transparent;
    padding: 0;
    font-size: 14px;
  }
  #content blockquote {
    border-left: 3px solid var(--accent);
    padding-left: 12px;
    color: var(--muted);
    margin: 0.8em 0;
  }
  #content table {
    border-collapse: collapse;
    width: 100%;
    margin: 0.8em 0;
  }
  #content th, #content td {
    border: 1px solid var(--border);
    padding: 6px 10px;
    text-align: left;
  }
  #content hr {
    border: none;
    border-top: 1px solid var(--border);
    margin: 1.2em 0;
  }
  #empty {
    color: var(--muted);
    font-style: italic;
    padding: 40px 0;
    text-align: center;
  }
</style>
</head>
<body>
  <div id="topbar">
    <div id="status"><span class="dot"></span><span id="status-text">连接中...</span></div>
    <button id="theme-toggle" type="button">🌙 深色</button>
  </div>
  <div id="content"><div id="empty">等待答案推送...</div></div>

<script>
(function() {
  // ---- 主题切换 ----
  var stored = null;
  try { stored = localStorage.getItem('qa-theme'); } catch(e) {}
  if (stored === 'light') document.documentElement.classList.add('light');
  updateThemeBtn();
  document.getElementById('theme-toggle').addEventListener('click', function() {
    document.documentElement.classList.toggle('light');
    var isLight = document.documentElement.classList.contains('light');
    try { localStorage.setItem('qa-theme', isLight ? 'light' : 'dark'); } catch(e) {}
    updateThemeBtn();
  });
  function updateThemeBtn() {
    var btn = document.getElementById('theme-toggle');
    var isLight = document.documentElement.classList.contains('light');
    btn.textContent = isLight ? '☀️ 浅色' : '🌙 深色';
  }

  // ---- Markdown 渲染 ----
  // 优先用 CDN 的 marked，失败就用纯文本 fallback。
  // 用 jsdelivr + unpkg 双路加载，提高离线成功率。
  var mdReady = false;
  function tryLoadMarked(url, onLoad, onFail) {
    var s = document.createElement('script');
    s.src = url;
    s.onload = onLoad;
    s.onerror = onFail;
    document.head.appendChild(s);
  }
  tryLoadMarked(
    'https://cdn.jsdelivr.net/npm/marked/marked.min.js',
    function() { mdReady = true; flushRender(); },
    function() {
      tryLoadMarked(
        'https://unpkg.com/marked/marked.min.js',
        function() { mdReady = true; flushRender(); },
        function() { /* 纯文本 fallback，flushRender 里自己判断 */ flushRender(); }
      );
    }
  );

  // ---- 状态与内容 ----
  var currentText = '';
  var finalized = false;
  var lastSequence = -1;
  var activeRequest = 0;
  var pendingRender = null;
  var RENDER_THROTTLE_MS = 60; // 60ms throttle 防止高频 chunk 时 CPU 飙高

  function setStatus(kind, text) {
    var el = document.getElementById('status');
    el.className = kind || '';
    document.getElementById('status-text').textContent = text;
  }

  function flushRender() {
    if (pendingRender) { clearTimeout(pendingRender); pendingRender = null; }
    var container = document.getElementById('content');
    if (!currentText) {
      container.innerHTML = '<div id="empty">等待答案推送...</div>';
      return;
    }
    if (mdReady && window.marked) {
      try {
        if (window.marked.setOptions) {
          window.marked.setOptions({ breaks: true, gfm: true });
        }
        var parsed = window.marked.parse ? window.marked.parse(currentText) : window.marked(currentText);
        container.innerHTML = parsed;
        return;
      } catch(e) { /* 降级 */ }
    }
    // 纯文本 fallback
    var pre = document.createElement('pre');
    pre.style.whiteSpace = 'pre-wrap';
    pre.textContent = currentText;
    container.innerHTML = '';
    container.appendChild(pre);
  }

  function scheduleRender() {
    if (pendingRender) return;
    pendingRender = setTimeout(function() {
      pendingRender = null;
      flushRender();
    }, RENDER_THROTTLE_MS);
  }

  // ---- SSE 连接 ----
  var es = null;
  var reconnectDelay = 1000;

  function connect() {
    try { if (es) es.close(); } catch(e) {}
    setStatus('', '连接中...');
    es = new EventSource('stream');
    es.onopen = function() {
      setStatus('live', '已连接');
      reconnectDelay = 1000;
    };
    es.onmessage = function(ev) {
      if (!ev.data) return;
      var msg;
      try { msg = JSON.parse(ev.data); } catch(e) { return; }
      if (msg.kind === 'snapshot') {
        lastSequence = msg.seq;
        activeRequest = msg.requestId;
        currentText = msg.content || '';
      } else {
        if (msg.seq <= lastSequence) return;
        if (msg.kind === 'started') {
          if (msg.requestId <= activeRequest) return;
          activeRequest = msg.requestId;
          currentText = '';
        } else {
          if (msg.requestId !== activeRequest || finalized) return;
          if (msg.kind === 'chunk') currentText += msg.content || '';
          if (msg.kind === 'completed') currentText = msg.content || '';
        }
        lastSequence = msg.seq;
      }
      finalized = ['completed', 'failed', 'cancelled'].indexOf(msg.phase) >= 0;
      document.body.dataset.requestId = String(activeRequest);
      document.body.dataset.phase = msg.phase;
      var labels = {idle: '已连接', capturing: '正在截图', generating: '正在生成', completed: '已完成', cancelled: '已停止', failed: '请求出错'};
      setStatus(msg.phase === 'failed' ? 'err' : 'live', labels[msg.phase] || '已连接');
      scheduleRender();
    };
    es.onerror = function() {
      setStatus('err', '断线，重连中...');
      try { es.close(); } catch(e) {}
      setTimeout(connect, reconnectDelay);
      reconnectDelay = Math.min(reconnectDelay * 1.5, 10000);
    };
  }

  connect();
})();
</script>
</body>
</html>`
