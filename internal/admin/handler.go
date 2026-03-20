// Package admin provides a minimal web dashboard for Tentaserve.
// The dashboard is served at /-/admin and shows upstream health,
// metrics summary, active configuration, and MCP tools.
package admin

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// UpstreamStatus represents the status of an upstream service.
type UpstreamStatus struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	URL     string `json:"url"`
	Status  string `json:"status"`
	Latency int64  `json:"latency_ms"`
}

// ToolInfo represents an MCP tool.
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Upstream    string `json:"upstream"`
}

// DashboardData represents the data displayed on the dashboard.
type DashboardData struct {
	Version      string           `json:"version"`
	Uptime       string           `json:"uptime"`
	UptimeMs     int64            `json:"uptime_ms"`
	Upstreams    []UpstreamStatus `json:"upstreams"`
	Tools        []ToolInfo       `json:"tools"`
	Requests     int64            `json:"requests_total"`
	CacheHits    int64            `json:"cache_hits"`
	CacheMisses  int64            `json:"cache_misses"`
	CacheHitRate float64          `json:"cache_hit_rate"`
	ActiveConns  int64            `json:"active_connections"`
	Timestamp    time.Time        `json:"timestamp"`
}

// DataProvider provides dashboard data.
type DataProvider interface {
	GetDashboardData() *DashboardData
}

// Handler serves the admin dashboard.
type Handler struct {
	mu        sync.RWMutex
	provider  DataProvider
	basicAuth *BasicAuth
}

// BasicAuth provides optional basic authentication.
type BasicAuth struct {
	Username string
	Password string
}

// Options configures the admin handler.
type Options struct {
	Provider  DataProvider
	BasicAuth *BasicAuth
}

// NewHandler creates a new admin handler.
func NewHandler(opts Options) *Handler {
	return &Handler{
		provider:  opts.Provider,
		basicAuth: opts.BasicAuth,
	}
}

// ServeHTTP implements the http.Handler interface.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check basic auth if configured
	if h.basicAuth != nil {
		user, pass, ok := r.BasicAuth()
		if !ok || user != h.basicAuth.Username || pass != h.basicAuth.Password {
			w.Header().Set("WWW-Authenticate", `Basic realm="Tentaserve Admin"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	switch {
	case r.URL.Path == "/-/admin" || r.URL.Path == "/-/admin/":
		h.serveDashboard(w, r)
	case r.URL.Path == "/-/admin/api/data":
		h.serveAPIData(w, r)
	default:
		http.NotFound(w, r)
	}
}

// serveDashboard serves the HTML dashboard page.
func (h *Handler) serveDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(dashboardHTML))
}

// serveAPIData serves the dashboard data as JSON.
func (h *Handler) serveAPIData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")

	if h.provider == nil {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(&DashboardData{
			Timestamp: time.Now(),
		})
		return
	}

	data := h.provider.GetDashboardData()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

// dashboardHTML is the embedded HTML for the admin dashboard.
// Single-page dashboard with no external dependencies (inline CSS/JS).
// All dynamic content uses safe DOM methods (textContent, createElement)
// instead of innerHTML to prevent XSS.
const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Tentaserve Admin</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;background:#0f172a;color:#e2e8f0;min-height:100vh}
.header{background:#1e293b;border-bottom:1px solid #334155;padding:16px 24px;display:flex;align-items:center;justify-content:space-between}
.header h1{font-size:20px;font-weight:600;color:#f8fafc}
.header .meta{color:#94a3b8;font-size:13px}
.container{max-width:1200px;margin:0 auto;padding:24px}
.grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(280px,1fr));gap:16px;margin-bottom:24px}
.card{background:#1e293b;border:1px solid #334155;border-radius:8px;padding:20px}
.card h2{font-size:14px;text-transform:uppercase;color:#94a3b8;margin-bottom:12px;letter-spacing:0.05em}
.stat{font-size:32px;font-weight:700;color:#f8fafc}
.stat-label{font-size:13px;color:#64748b;margin-top:4px}
.status-dot{display:inline-block;width:8px;height:8px;border-radius:50%;margin-right:8px}
.status-dot.healthy{background:#22c55e}
.status-dot.degraded{background:#eab308}
.status-dot.unhealthy{background:#ef4444}
.upstream-list{list-style:none}
.upstream-item{display:flex;align-items:center;justify-content:space-between;padding:10px 0;border-bottom:1px solid #334155}
.upstream-item:last-child{border-bottom:none}
.upstream-name{font-weight:500;color:#f8fafc}
.upstream-url{font-size:12px;color:#64748b}
.upstream-latency{font-size:13px;color:#94a3b8}
.tools-table{width:100%;border-collapse:collapse}
.tools-table th{text-align:left;padding:8px 12px;border-bottom:1px solid #334155;color:#94a3b8;font-size:12px;text-transform:uppercase}
.tools-table td{padding:8px 12px;border-bottom:1px solid #1e293b;font-size:14px}
.tools-table tr:hover{background:#1e293b}
.section{margin-bottom:24px}
.section-title{font-size:16px;font-weight:600;margin-bottom:12px;color:#f8fafc}
.badge{display:inline-block;padding:2px 8px;border-radius:4px;font-size:12px;font-weight:500}
.badge-rest{background:#1d4ed8;color:#bfdbfe}
.badge-graphql{background:#7c3aed;color:#ddd6fe}
.badge-mcp{background:#0f766e;color:#ccfbf1}
.refresh{color:#94a3b8;font-size:12px;text-align:right;margin-top:8px}
.empty{padding:12px;color:#64748b}
</style>
</head>
<body>
<div class="header">
  <div style="display:flex;align-items:center">
    <span style="font-size:24px;margin-right:8px">&#x1F419;</span>
    <h1>Tentaserve</h1>
  </div>
  <div class="meta">
    <span id="version"></span> &middot; <span id="uptime"></span>
  </div>
</div>
<div class="container">
  <div class="grid" id="stats"></div>
  <div class="section">
    <div class="section-title">Upstreams</div>
    <div class="card">
      <ul class="upstream-list" id="upstreams"></ul>
    </div>
  </div>
  <div class="section">
    <div class="section-title">MCP Tools</div>
    <div class="card" style="overflow-x:auto">
      <table class="tools-table">
        <thead><tr><th>Name</th><th>Description</th><th>Upstream</th></tr></thead>
        <tbody id="tools"></tbody>
      </table>
    </div>
  </div>
  <div class="refresh" id="refresh"></div>
</div>
<script>
(function(){
  function formatUptime(ms){
    var s=Math.floor(ms/1000),m=Math.floor(s/60),h=Math.floor(m/60),d=Math.floor(h/24);
    if(d>0)return d+'d '+h%24+'h '+m%60+'m';
    if(h>0)return h+'h '+m%60+'m '+s%60+'s';
    if(m>0)return m+'m '+s%60+'s';
    return s+'s';
  }

  function el(tag,cls,text){
    var e=document.createElement(tag);
    if(cls)e.className=cls;
    if(text!==undefined)e.textContent=String(text);
    return e;
  }

  function clearChildren(node){while(node.firstChild)node.removeChild(node.firstChild);}

  function buildStatCard(title,value,label){
    var card=el('div','card');
    card.appendChild(el('h2','',title));
    card.appendChild(el('div','stat',value));
    card.appendChild(el('div','stat-label',label));
    return card;
  }

  function buildUpstreamItem(u){
    var li=el('li','upstream-item');
    var left=el('div');
    var dot=el('span','status-dot '+(u.status==='healthy'?'healthy':(u.status==='degraded'?'degraded':'unhealthy')));
    left.appendChild(dot);
    left.appendChild(el('span','upstream-name',u.name));
    var badge=el('span','badge badge-'+u.type,u.type);
    badge.style.marginLeft='8px';
    left.appendChild(badge);
    left.appendChild(el('div','upstream-url',u.url));
    li.appendChild(left);
    li.appendChild(el('span','upstream-latency',u.latency_ms+'ms'));
    return li;
  }

  function buildToolRow(t){
    var tr=el('tr');
    var td1=el('td');
    td1.appendChild(el('code','',t.name));
    tr.appendChild(td1);
    tr.appendChild(el('td','',t.description));
    tr.appendChild(el('td','',t.upstream));
    return tr;
  }

  function refresh(){
    fetch('/-/admin/api/data').then(function(r){return r.json()}).then(function(d){
      document.getElementById('version').textContent='v'+(d.version||'0.1.0');
      document.getElementById('uptime').textContent='Uptime: '+formatUptime(d.uptime_ms||0);

      var total=(d.cache_hits||0)+(d.cache_misses||0);
      var rate=total>0?((d.cache_hits/total)*100).toFixed(1):'-';

      var stats=document.getElementById('stats');
      clearChildren(stats);
      stats.appendChild(buildStatCard('Requests',String(d.requests_total||0),'Total requests processed'));
      stats.appendChild(buildStatCard('Cache Hit Rate',rate+'%',(d.cache_hits||0)+' hits / '+(d.cache_misses||0)+' misses'));
      stats.appendChild(buildStatCard('Active Connections',String(d.active_connections||0),'Current active connections'));

      var ul=document.getElementById('upstreams');
      clearChildren(ul);
      if(d.upstreams&&d.upstreams.length>0){
        d.upstreams.forEach(function(u){ul.appendChild(buildUpstreamItem(u));});
      }else{
        ul.appendChild(el('li','empty','No upstreams configured'));
      }

      var tb=document.getElementById('tools');
      clearChildren(tb);
      if(d.tools&&d.tools.length>0){
        d.tools.forEach(function(t){tb.appendChild(buildToolRow(t));});
      }else{
        var tr=el('tr');var td=el('td','empty','No MCP tools generated');td.colSpan=3;tr.appendChild(td);tb.appendChild(tr);
      }

      document.getElementById('refresh').textContent='Last updated: '+new Date().toLocaleTimeString();
    }).catch(function(e){
      document.getElementById('refresh').textContent='Error: '+e.message;
    });
  }

  refresh();
  setInterval(refresh,5000);
})();
</script>
</body>
</html>`
