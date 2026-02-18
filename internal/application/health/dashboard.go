package health

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RenderDashboardHTML returns the full HTML for GET / (same as Express health dashboard).
func RenderDashboardHTML(health CollectResult) string {
	// Build payload that matches what the frontend JS expects (traffic, runtime, dependencies, status).
	payload := map[string]interface{}{
		"status":  health.Status,
		"runtime": health.Runtime,
		"traffic": health.Traffic,
		"dependencies": health.Dependencies,
	}
	b, _ := json.Marshal(payload)
	jsonStr := string(b)
	// Escape for embedding in JS template literal: \ ` $
	jsonStr = strings.ReplaceAll(jsonStr, "\\", "\\\\")
	jsonStr = strings.ReplaceAll(jsonStr, "`", "\\`")
	jsonStr = strings.ReplaceAll(jsonStr, "$", "\\$")

	avgTime := fmt.Sprint(health.Traffic.AvgResponseTime)
	lastReqMethod := "-"
	lastReqPath := "-"
	lastReqIP := "-"
	if health.Traffic.LastRequest != nil {
		if m, ok := health.Traffic.LastRequest.(map[string]interface{}); ok {
			if v, ok := m["method"].(string); ok {
				lastReqMethod = v
			}
			if v, ok := m["path"].(string); ok {
				lastReqPath = v
			}
			if v, ok := m["ip"].(string); ok {
				lastReqIP = v
			}
		}
	}

	load0 := "0.00"
	if len(health.Runtime.CPU.LoadAvg) > 0 {
		load0 = health.Runtime.CPU.LoadAvg[0]
	}

	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Troo Earth · API Status</title>
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <link rel="icon" type="image/svg+xml" href="https://dev.troo.earth/favicon.svg">
  <link href="https://fonts.googleapis.com/css2?family=Nunito+Sans:opsz,wght@6..12,300;400;600;700;800;900&display=swap" rel="stylesheet">
  <style>
    :root { --teal: #007473; --dark: #173E35; --accent: #FFB71B; --bg: #F8F9FA; --muted: #64748b; }
    * { box-sizing: border-box; }
    body { background-color: var(--bg); color: var(--dark); font-family: 'Nunito Sans', sans-serif; margin: 0; height: 100vh; display: flex; align-items: center; justify-content: center; overflow: hidden; }
    .atmosphere { position: fixed; inset: 0; z-index: -1; }
    .blob { position: absolute; border-radius: 50%; opacity: 0.15; filter: blur(100px); }
    .blob-1 { top: -10%; left: -5%; width: 45vw; height: 45vw; background: var(--teal); }
    .blob-2 { top: 15%; right: -5%; width: 40vw; height: 40vw; background: var(--accent); animation: pulse 8s infinite; }
    @keyframes pulse { 0%, 100% { transform: scale(1); opacity: 0.15; } 50% { transform: scale(1.05); opacity: 0.2; } }
    .container { width: 100%; max-width: 1100px; padding: 0 20px; display: flex; flex-direction: column; align-items: center; }
    header { width: 100%; display: flex; justify-content: space-between; align-items: center; margin-bottom: 25px; }
    .brand-logo { height: 46px; }
    .time-badge { font-size: 13px; font-weight: 800; background: rgba(255,255,255,0.7); padding: 8px 18px; border-radius: 99px; border: 1px solid rgba(0,0,0,0.05); }
    .status-headline { font-size: clamp(32px, 5vw, 58px); font-weight: 900; line-height: 1; letter-spacing: -3px; text-align: center; margin: 0; background: linear-gradient(to left, var(--teal), var(--dark)); -webkit-background-clip: text; -webkit-text-fill-color: transparent; }
    .subtext { font-size: 16px; font-weight: 700; color: var(--muted); margin-top: 12px; margin-bottom: 30px; }
    .glass-card { width: 100%; background: rgba(255, 255, 255, 0.85); backdrop-filter: blur(25px); border-radius: 32px; box-shadow: 0 30px 100px -20px rgba(0, 116, 115, 0.1); border: 1px solid rgba(255, 255, 255, 0.6); overflow: hidden; position: relative; }
    .loader { height: 6px; width: 0%; background: var(--accent); position: absolute; top: 0; z-index: 10; transition: width 10s linear; }
    .grid { display: grid; grid-template-columns: repeat(3, 1fr); }
    .col { padding: 45px; border-right: 1px solid rgba(0,0,0,0.04); }
    .col:last-child { border-right: none; }
    .label { text-transform: uppercase; font-size: 11px; font-weight: 900; letter-spacing: 2px; color: #94a3b8; margin-bottom: 25px; }
    .big { font-size: clamp(24px, 3.5vw, 42px); font-weight: 900; color: var(--dark); line-height: 1; letter-spacing: -1.5px; margin-bottom: 10px; white-space: nowrap; }
    .row { display: flex; justify-content: space-between; align-items: center; padding: 8px 0; border-bottom: 1px solid rgba(0,0,0,0.03); font-size: 14px; font-weight: 700; }
    .row:last-child { border-bottom: none; }
    .pill { padding: 5px 12px; border-radius: 10px; font-size: 11px; font-weight: 900; display: flex; align-items: center; gap: 8px; }
    .ok { background: rgba(0, 116, 115, 0.08); color: var(--teal); }
    .err { background: rgba(239, 68, 68, 0.08); color: #EF4444; }
    .dot { width: 7px; height: 7px; border-radius: 50%; background: currentColor; }
    .blink { animation: flash 2s infinite; }
    @keyframes flash { 0%, 100% { opacity: 0.4; } 50% { opacity: 1; } }
    .footer-req { background: rgba(23, 62, 53, 0.03); padding: 18px 45px; display: flex; justify-content: space-between; font-family: monospace; font-size: 13px; font-weight: 700; border-top: 1px solid rgba(0,0,0,0.05); }
    .footer-msg { margin-top: 30px; height: 60px; display: flex; flex-direction: column; align-items: center; justify-content: center; }
    .flex-line { display: flex; align-items: center; gap: 15px; }
    .btn-refresh, .btn-errors { background: var(--teal); color: white; border: none; padding: 8px 20px; border-radius: 10px; cursor: pointer; font-weight: 900; font-size: 12px; transition: 0.2s; }
    .btn-refresh { display: none; }
    .btn-errors { background: transparent; color: var(--muted); border: 1px solid rgba(0,0,0,0.1); }
    .btn-refresh:hover { background: var(--dark); transform: translateY(-1px); }
    .btn-errors:hover { background: rgba(0,0,0,0.02); color: var(--dark); }
    #error-modal { display: none; position: fixed; inset: 0; background: rgba(23, 62, 53, 0.4); backdrop-filter: blur(10px); z-index: 100; align-items: center; justify-content: center; padding: 20px; }
    .modal-content { background: white; width: 100%; max-width: 700px; border-radius: 24px; padding: 40px; max-height: 80vh; overflow-y: auto; box-shadow: 0 40px 100px rgba(0,0,0,0.2); }
    .error-item { border-bottom: 1px solid #f1f5f9; padding: 15px 0; font-size: 13px; }
    .error-item:last-child { border-bottom: none; }
    .err-meta { display: flex; gap: 10px; font-weight: 800; color: var(--teal); margin-bottom: 5px; text-transform: uppercase; font-size: 10px; }
    .err-msg { font-weight: 700; color: #e11d48; margin-bottom: 5px; }
    .err-stack { font-family: monospace; font-size: 11px; color: var(--muted); background: #f8fafc; padding: 10px; border-radius: 8px; white-space: pre-wrap; }
    .money-tag { font-size: 10px; font-weight: 800; opacity: 0.5; margin-top: 8px; text-transform: uppercase; letter-spacing: 0.5px; display: none; }
    @media (max-width: 900px) { body { height: auto; overflow-y: auto; padding: 40px 0; } .brand-logo { height: 32px; } .grid { grid-template-columns: 1fr; } .col { border-right: none; border-bottom: 1px solid rgba(0,0,0,0.04); padding: 35px; } .footer-req { flex-direction: column; gap: 10px; } .modal-content { padding: 25px; } }
  </style>
</head>
<body>
  <div class="atmosphere"><div class="blob blob-1"></div><div class="blob blob-2"></div></div>
  <div id="error-modal" onclick="closeErrors(event)">
    <div class="modal-content" onclick="event.stopPropagation()">
        <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:30px;">
            <h2 style="margin:0; font-weight:900; letter-spacing:-1px;">Internal Server Errors (Last 50)</h2>
            <button onclick="closeErrors()" style="border:none; background:none; cursor:pointer; font-weight:900; color:var(--muted)">CLOSE</button>
        </div>
        <div id="error-list">Loading...</div>
    </div>
  </div>
  <div class="container">
    <header>
      <img src="https://dev.troo.earth/assets/mainLogo-Do2wEJmm.svg" class="brand-logo">
      <div class="time-badge"><span id="time-display"></span></div>
    </header>
    <h1 class="status-headline" id="headline">All Systems Operational</h1>
    <p class="subtext">Real-time monitoring of API performance and dependencies.</p>
    <div class="glass-card">
      <div id="progress-bar" class="loader"></div>
      <div class="grid">
        <div class="col">
          <div class="label">Traffic & Quality</div>
          <div class="big" id="total-req">` + fmt.Sprint(health.Traffic.TotalRequests) + `</div>
          <div class="row"><span>Successful</span><span id="success-count" style="color:var(--teal)">` + fmt.Sprint(health.Traffic.SuccessCount) + `</span></div>
          <div class="row"><span>Failed</span><span id="failed-count" style="color:#EF4444">` + fmt.Sprint(health.Traffic.FailedCount) + `</span></div>
          <div class="row"><span>Success Rate</span><span id="success-rate">` + health.Traffic.SuccessRate + `%</span></div>
          <div class="row"><span>Avg Latency</span><span id="avg-time">` + avgTime + `ms</span></div>
        </div>
        <div class="col">
          <div class="label">Resources</div>
          <div class="big" id="uptime">--h --m --s</div>
          <div class="row"><span>Heap Used</span><span id="mem-heap">` + fmt.Sprint(health.Runtime.Memory.HeapUsed) + ` MB</span></div>
          <div class="row"><span>Memory (RSS)</span><span>` + fmt.Sprint(health.Runtime.Memory.RSS) + ` MB</span></div>
          <div class="row"><span>Load Avg</span><span id="load">` + load0 + `</span></div>
          <div class="row"><span>Platform</span><span style="font-size:10px">` + health.Runtime.Platform + `</span></div>
        </div>
        <div class="col">
          <div class="label">Connectivity</div>
          <div class="row"><span>Database</span><span id="pill-db" class="pill ok"><span class="dot blink"></span><span id="ping-db">-- ms</span></span></div>
          <div class="row"><span>Redis Cache</span><span id="pill-redis" class="pill ok"><span class="dot blink"></span><span id="ping-redis">-- ms</span></span></div>
          <div class="row"><span>Frontend</span><span id="pill-fe" class="pill ok"><span class="dot blink"></span><span id="ping-fe">-- ms</span></span></div>
          <div class="row"><span>Stripe API</span><span id="pill-stripe" class="pill ok"><span class="dot blink"></span><span id="ping-stripe">-- ms</span></span></div>
        </div>
      </div>
      <div class="footer-req">
        <div><span style="opacity:0.5; margin-right:10px;">LAST INBOUND</span> <span id="req-method" style="font-weight:900">` + lastReqMethod + `</span></div>
        <div id="req-path">` + lastReqPath + `</div>
        <div id="req-ip" style="opacity:0.6">` + lastReqIP + `</div>
      </div>
    </div>
    <div class="footer-msg">
      <div class="flex-line">
        <button class="btn-errors" onclick="showErrors()">View Error Log</button>
        <div id="updates-status" style="font-weight:800; color:var(--muted)">Live Updates Active · <span id="count">3</span> refreshes remaining</div>
        <button id="btn-refresh" class="btn-refresh" onclick="tick(true)">Manual Refresh</button>
      </div>
      <div id="money-tag" class="money-tag">Conserving energy today because we're going to be rich as fuck tomorrow. Use refresh sparingly to save server hits.</div>
    </div>
  </div>
  <script>
    let left = 3;
    const bar = document.getElementById('progress-bar');
    const fmt = (s) => { const d = Math.floor(s / 86400); const h = Math.floor((s % 86400) / 3600); const m = Math.floor((s % 3600) / 60); const sec = Math.floor(s % 60); return d > 0 ? d + 'd ' + h + 'h ' + m + 'm' : h + 'h ' + m + 'm ' + sec + 's'; };
    const resetBar = () => { bar.style.transition='none'; bar.style.width='0%'; void bar.offsetWidth; bar.style.transition='width 10s linear'; bar.style.width='100%'; };
    const updateUI = (d) => {
      document.getElementById('time-display').innerText = new Date().toLocaleTimeString();
      document.getElementById('total-req').innerText = d.traffic.totalRequests;
      document.getElementById('success-count').innerText = d.traffic.successCount;
      document.getElementById('failed-count').innerText = d.traffic.failedCount;
      document.getElementById('success-rate').innerText = d.traffic.successRate + '%';
      document.getElementById('avg-time').innerText = d.traffic.avgResponseTime + 'ms';
      document.getElementById('uptime').innerText = fmt(d.runtime.uptimeSeconds);
      document.getElementById('mem-heap').innerText = d.runtime.memory.heapUsed + ' MB';
      document.getElementById('load').innerText = d.runtime.cpu.loadAvg[0];
      if (d.traffic.lastRequest) { document.getElementById('req-method').innerText = d.traffic.lastRequest.method; document.getElementById('req-path').innerText = d.traffic.lastRequest.path; document.getElementById('req-ip').innerText = d.traffic.lastRequest.ip; }
      const setP = (id, s, p) => { const pill=document.getElementById('pill-'+id); const isOk=s==='connected'||s==='reachable'; pill.className='pill '+(isOk?'ok':'err'); document.getElementById('ping-'+id).innerText=(p!=null?p:'?')+' ms'; };
      setP('db', d.dependencies.database.status, d.dependencies.database.pingMs);
      setP('redis', d.dependencies.redis.status, d.dependencies.redis.pingMs);
      setP('fe', d.dependencies.frontend.status, d.dependencies.frontend.pingMs);
      setP('stripe', d.dependencies.stripe.status, d.dependencies.stripe.pingMs);
      const hl = document.getElementById('headline');
      if (d.status === 'ok') { hl.innerText = 'All Systems Operational'; hl.style.background = ''; }
      else { hl.innerText = 'System Issues Detected'; hl.style.background = 'linear-gradient(to right, #EF4444, #B91C1C)'; hl.style.webkitBackgroundClip = 'text'; }
    };
    async function tick(manual) { if (!manual && left <= 0) return; try { const r = await fetch('/health/json'); const d = await r.json(); updateUI(d); if (!manual) { left--; document.getElementById('count').innerText = left; if (left > 0) resetBar(); else { document.getElementById('updates-status').innerHTML = "<span style='color:#B45309; background:#FFFBEB; padding:6px 12px; border-radius:8px;'>Updates Paused</span>"; document.getElementById('btn-refresh').style.display = 'block'; document.getElementById('money-tag').style.display = 'block'; } } } catch(e){} }
    async function showErrors() { const modal = document.getElementById('error-modal'); const list = document.getElementById('error-list'); modal.style.display = 'flex'; list.innerHTML = 'Fetching logs...'; try { const r = await fetch('/health/errors'); const errors = await r.json(); if (errors.length === 0) { list.innerHTML = '<div style="text-align:center; padding:40px; color:var(--muted); font-weight:700;">No internal errors recorded.</div>'; return; } list.innerHTML = errors.map(e => '<div class="error-item"><div class="err-meta"><span>' + new Date(e.time).toLocaleString() + '</span> <span>' + (e.method||'') + ' ' + (e.path||'') + '</span></div><div class="err-msg">' + (e.message||'') + '</div>' + (e.stack ? '<div class="err-stack">' + e.stack + '</div>' : '') + '</div>').join(''); } catch (e) { list.innerHTML = 'Error loading logs.'; } }
    function closeErrors() { document.getElementById('error-modal').style.display = 'none'; }
    setTimeout(() => { const data = JSON.parse(` + "`" + jsonStr + "`" + `); updateUI(data); resetBar(); }, 100);
    setInterval(() => tick(), 10000);
  </script>
</body>
</html>`
}
