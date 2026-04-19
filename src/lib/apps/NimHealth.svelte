<script>
  import { onMount, onDestroy, getContext } from 'svelte';
  import { token, hdrs } from '$lib/stores/auth.js';
  import { APP_META } from '$lib/apps.js';

  const wc = getContext('windowControls');

  let view = 'dashboard';
  let services = [];
  let selectedService = null;
  let filter = 'all';
  let search = '';
  let stopping = {};

  let cpu = { percent: 0, cores: 0, load: 0 };
  let ram = { used: 0, total: 0, percent: 0 };
  let diskIO = { read: 0, write: 0 };
  let netIO = { rx: 0, tx: 0 };
  let cpuHistory = Array(12).fill(0);
  let ramHistory = Array(12).fill(0);
  let diskHistory = Array(12).fill(0);
  let netHistory = Array(12).fill(0);

  let detailLogs = [];
  let pollInterval;

  async function loadServices() {
    try {
      const r = await fetch('/api/services', { headers: hdrs() });
      const d = await r.json();
      const raw = d.services || [];
      // Flatten: keep parent services + inject Docker children as individual rows
      let flat = [];
      for (const svc of raw) {
        flat.push(svc);
        if (svc.children && svc.children.length > 0) {
          for (const child of svc.children) {
            flat.push({
              ...child,
              _isChild: true,
              _parentId: svc.id,
              poolName: svc.poolName,
              owner: svc.owner,
            });
          }
        }
      }
      services = flat;
    } catch { services = []; }
  }

  async function loadMetrics() {
    try {
      const r = await fetch('/api/hardware/stats', { headers: hdrs() });
      const d = await r.json();
      if (d.cpu) {
        cpu = { percent: Math.round(d.cpu.percent || 0), cores: d.cpu.cores || 0, load: (d.cpu.load1 || 0).toFixed(1) };
        cpuHistory = [...cpuHistory.slice(1), cpu.percent];
      }
      if (d.memory) {
        ram = { used: d.memory.used || 0, total: d.memory.total || 0, percent: Math.round((d.memory.used / d.memory.total) * 100) || 0 };
        ramHistory = [...ramHistory.slice(1), ram.percent];
      }
      if (d.disk) {
        diskIO = { read: d.disk.readSpeed || 0, write: d.disk.writeSpeed || 0 };
        diskHistory = [...diskHistory.slice(1), Math.min(100, Math.round((diskIO.read + diskIO.write) / 1048576))];
      }
      if (d.network) {
        netIO = { rx: d.network.rxSpeed || 0, tx: d.network.txSpeed || 0 };
        netHistory = [...netHistory.slice(1), Math.min(100, Math.round((netIO.rx + netIO.tx) / 1048576))];
      }
    } catch {}
  }

  async function loadDetail(svc) { selectedService = svc; view = 'detail'; detailLogs = []; await loadLogs(svc); }
  function goBack() { view = 'dashboard'; selectedService = null; detailLogs = []; }

  async function loadLogs(svc) {
    try { const r = await fetch(`/api/services/${svc.id}/logs?n=50`, { headers: hdrs() }); const d = await r.json(); detailLogs = d.logs || []; } catch { detailLogs = []; }
  }

  async function doAction(svc, action) {
    const key = svc.id + ':' + action;
    stopping = { ...stopping, [key]: true };
    try {
      await fetch(`/api/services/${svc.id}/${action}`, { method: 'POST', headers: hdrs() });
      await loadServices();
      if (selectedService?.id === svc.id) { selectedService = services.find(s => s.id === svc.id) || selectedService; await loadLogs(selectedService); }
    } catch {}
    stopping = { ...stopping, [key]: false };
  }

  function fmtBytes(b) {
    if (!b || b === 0) return '0 B';
    if (b >= 1e12) return (b / 1e12).toFixed(1) + ' TB';
    if (b >= 1e9) return (b / 1e9).toFixed(1) + ' GB';
    if (b >= 1e6) return (b / 1e6).toFixed(1) + ' MB';
    if (b >= 1e3) return (b / 1e3).toFixed(0) + ' KB';
    return b + ' B';
  }
  function fmtSpeed(b) { if (!b) return '0'; if (b >= 1e6) return (b / 1e6).toFixed(1); if (b >= 1e3) return (b / 1e3).toFixed(1); return '0'; }
  function fmtUptime(svc) {
    if (svc.status !== 'running') return '—';
    if (svc.uptime) return svc.uptime; // Docker children ya traen uptime formateado
    if (!svc.startedAt) return '—';
    const ms = Date.now() - new Date(svc.startedAt).getTime();
    const h = Math.floor(ms / 3600000);
    if (h >= 24) return Math.floor(h / 24) + 'd ' + (h % 24).toString().padStart(2, '0') + 'h';
    return h + 'h ' + Math.floor((ms % 3600000) / 60000).toString().padStart(2, '0') + 'm';
  }
  function appInitials(svc) { return (svc.name || svc.appName || svc.appId || '?').slice(0, 2).toUpperCase(); }
  function svcDisplayName(svc) { return svc.name || svc.appName || svc.appId || '?'; }
  function svcIcon(svc) {
    // Docker children traen icon del backend
    if (svc.icon) return svc.icon;
    // Servicios del sistema: buscar en APP_META por appId
    const appId = svc.appId || svc.id?.split('@')[0] || '';
    if (APP_META[appId]?.icon) return APP_META[appId].icon;
    // containers → tiene su propio icono
    if (appId === 'containers' && APP_META['appstore']?.icon) return '/icons/containers.png';
    return '';
  }
  function svcVersion(svc) {
    if (svc.image) { const parts = svc.image.split(':'); return parts[1] || ''; }
    if (svc.containerImage) { const parts = svc.containerImage.split(':'); return parts[1] || ''; }
    return '';
  }

  function sparkPoints(history) {
    if (!history || history.length === 0) return '';
    const max = Math.max(...history, 1);
    return history.map((v, i) => `${(i / (history.length - 1) * 100).toFixed(1)},${(28 - (v / max) * 24 - 2).toFixed(1)}`).join(' ');
  }

  $: filteredServices = services.filter(s => {
    const mf = filter === 'all' || (filter === 'running' ? s.status === 'running' : filter === 'stopped' ? s.status === 'stopped' : filter === 'error' ? (s.status === 'error' || s.status === 'failed') : true);
    return mf && (!search || (s.appName || s.appId || '').toLowerCase().includes(search.toLowerCase()));
  });
  $: runningCount = services.filter(s => s.status === 'running').length;

  onMount(async () => {
    let attempts = 0;
    while (!$token && attempts < 10) { await new Promise(r => setTimeout(r, 200)); attempts++; }
    await loadServices(); await loadMetrics();
    pollInterval = setInterval(() => { loadServices(); loadMetrics(); }, 5000);
  });
  onDestroy(() => { if (pollInterval) clearInterval(pollInterval); });
</script>

<div class="nh-root">
  {#if view === 'dashboard'}
    <!-- TITLEBAR -->
    <div class="nh-titlebar">
      <span class="nh-title">NimHealth</span>
      <span class="nh-sub">Task Manager</span>
      {#if wc}
        <div class="wf-controls">
          <button class="wf-btn" on:click={wc.minimize} title="Minimizar"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><line x1="5" y1="12" x2="19" y2="12"/></svg></button>
          <button class="wf-btn" on:click={wc.maximize} title="Maximizar"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="6" y="6" width="12" height="12" rx="2"/></svg></button>
          <button class="wf-btn wf-close" on:click={wc.close} title="Cerrar"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>
        </div>
      {/if}
    </div>

    <div class="hw-bar">
      {#each [
        { label:'CPU', value:`${cpu.percent}%`, sub:`${cpu.cores} cores · load ${cpu.load}`, hist:cpuHistory, color:'var(--accent)' },
        { label:'RAM', value:fmtBytes(ram.used), sub:`de ${fmtBytes(ram.total)} · ${ram.percent}%`, hist:ramHistory, color:'var(--c-info)' },
        { label:'Disco I/O', value:`${fmtSpeed(diskIO.read+diskIO.write)} MB/s`, sub:`↓ ${fmtSpeed(diskIO.read)} ↑ ${fmtSpeed(diskIO.write)}`, hist:diskHistory, color:'var(--c-warn)' },
        { label:'Red', value:`${fmtSpeed(netIO.rx+netIO.tx)} MB/s`, sub:`↓ ${fmtSpeed(netIO.rx)} ↑ ${fmtSpeed(netIO.tx)}`, hist:netHistory, color:'var(--c-ok)' },
      ] as m}
        <div class="hw-stat">
          <div class="hw-info"><div class="hw-lbl">{m.label}</div><div class="hw-val">{m.value}</div><div class="hw-sub">{m.sub}</div></div>
          <div class="hw-spark"><svg viewBox="0 0 100 28" preserveAspectRatio="none"><polyline points="{sparkPoints(m.hist)}" fill="none" stroke="{m.color}" stroke-width="1.5" vector-effect="non-scaling-stroke"/></svg></div>
        </div>
      {/each}
    </div>

    <div class="toolbar">
      <div class="filters">
        {#each [['all','Todos'],['running','Activos'],['stopped','Detenidos'],['error','Error']] as [f,label]}
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <span class="chip" class:active={filter===f} on:click={() => filter=f}>{label}</span>
        {/each}
      </div>
      <div class="tb-counter"><b>{filteredServices.length}</b> apps · <b>{runningCount}</b> activas</div>
      <div class="search-box">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="7"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
        <input type="text" placeholder="Buscar..." bind:value={search}>
      </div>
    </div>

    <div class="table-wrap">
      <table>
        <thead><tr><th>Nombre</th><th>Estado</th><th class="num">CPU</th><th class="num">RAM</th><th class="num">Uptime</th><th></th></tr></thead>
        <tbody>
          {#each filteredServices as svc}
            <!-- svelte-ignore a11y_click_events_have_key_events -->
            <tr class:selected={selectedService?.id===svc.id} on:click={() => loadDetail(svc)}>
              <td class="name-cell">
                {#if svcIcon(svc)}
                  <img class="app-icon-img" src="{svcIcon(svc)}" alt="" />
                {:else}
                  <div class="app-icon">{appInitials(svc)}</div>
                {/if}
                <span class="app-name">{svcDisplayName(svc)}</span>
                {#if svcVersion(svc)}<span class="app-ver">{svcVersion(svc)}</span>{/if}
                {#if svc._isChild}<span class="child-badge">docker</span>{/if}
              </td>
              <td><span class="state" class:run={svc.status==='running'} class:stop={svc.status==='stopped'} class:err={svc.status==='error'||svc.status==='failed'} class:starting={svc.status==='starting'||svc.status==='stopping'}><span class="dot"></span>{svc.status}</span></td>
              <td class="num" class:warn={svc.cpuPercent>50} class:crit={svc.cpuPercent>80}>{#if svc.status==='running'}<b>{(svc.cpuPercent||0).toFixed(1)}</b>%{:else}—{/if}</td>
              <td class="num">{#if svc.status==='running'}<b>{fmtBytes(svc.memoryUsage||0)}</b>{:else}—{/if}</td>
              <td class="num">{fmtUptime(svc)}</td>
              <td><div class="row-actions" on:click|stopPropagation>
                {#if svc.status==='running'||svc.status==='starting'}
                  <button class="act-btn stop" title="Detener" disabled={stopping[svc.id+':stop']} on:click={() => doAction(svc,'stop')}><svg viewBox="0 0 24 24" fill="currentColor"><rect x="6" y="6" width="12" height="12" rx="1"/></svg></button>
                  <button class="act-btn" title="Reiniciar" disabled={stopping[svc.id+':restart']} on:click={() => doAction(svc,'restart')}><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M3 12a9 9 0 1 0 3-6.7"/><polyline points="3 4 3 10 9 10"/></svg></button>
                {:else}
                  <button class="act-btn start" title="Iniciar" disabled={stopping[svc.id+':start']||svc.status==='error'} on:click={() => doAction(svc,'start')}><svg viewBox="0 0 24 24" fill="currentColor"><polygon points="6,4 20,12 6,20"/></svg></button>
                {/if}
              </div></td>
            </tr>
          {:else}
            <tr><td colspan="6" class="empty-row">{search ? 'Sin resultados' : 'No hay servicios registrados'}</td></tr>
          {/each}
        </tbody>
      </table>
    </div>

    {#if selectedService}
      <div class="footer"><span class="f-lbl">Seleccionado:</span><span class="f-val">{svcDisplayName(selectedService)}</span><span class="f-sep">·</span><span class="f-val" style="color:{selectedService.status==='running'?'var(--c-ok)':selectedService.status==='error'?'var(--c-crit)':'var(--text-muted)'}">{selectedService.status}</span>{#if selectedService.poolName}<span class="f-sep">·</span><span class="f-lbl">pool</span><span class="f-val">{selectedService.poolName}</span>{/if}<span class="f-sep">·</span><span class="f-lbl">id</span><span class="f-val">{selectedService.id}</span></div>
    {/if}

  {:else if view==='detail' && selectedService}
    <div class="nh-titlebar">
      <span class="nh-title">NimHealth</span>
      {#if wc}
        <div class="wf-controls">
          <button class="wf-btn" on:click={wc.minimize} title="Minimizar"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><line x1="5" y1="12" x2="19" y2="12"/></svg></button>
          <button class="wf-btn" on:click={wc.maximize} title="Maximizar"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="6" y="6" width="12" height="12" rx="2"/></svg></button>
          <button class="wf-btn wf-close" on:click={wc.close} title="Cerrar"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg></button>
        </div>
      {/if}
    </div>
    <div class="detail-nav">
      <!-- svelte-ignore a11y_click_events_have_key_events --><!-- svelte-ignore a11y_no_static_element_interactions -->
      <span class="back-btn" on:click={goBack}><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 18 9 12 15 6"/></svg>Volver</span>
      <span class="detail-name">{svcDisplayName(selectedService)}</span>
      <span class="state" class:run={selectedService.status==='running'} class:stop={selectedService.status==='stopped'} class:err={selectedService.status==='error'}><span class="dot"></span>{selectedService.status}</span>
    </div>
    <div class="detail-content">
      <div class="d-card"><div class="d-card-label">Información</div><div class="d-grid">
        <span class="d-key">ID</span><span class="d-val">{selectedService.id}</span>
        <span class="d-key">Pool</span><span class="d-val">{selectedService.poolName||'—'}</span>
        <span class="d-key">Path</span><span class="d-val" style="font-size:10px;word-break:break-all">{selectedService.path||'—'}</span>
        <span class="d-key">Owner</span><span class="d-val">{selectedService.owner||'system'}</span>
        <span class="d-key">Health</span><span class="d-val">{selectedService.health||'unknown'}</span>
      </div></div>
      <div class="detail-actions">
        {#if selectedService.status==='running'||selectedService.status==='starting'}
          <button class="d-btn d-stop" disabled={stopping[selectedService.id+':stop']} on:click={() => doAction(selectedService,'stop')}>{stopping[selectedService.id+':stop']?'Deteniendo...':'Detener'}</button>
          <button class="d-btn d-restart" disabled={stopping[selectedService.id+':restart']} on:click={() => doAction(selectedService,'restart')}>{stopping[selectedService.id+':restart']?'Reiniciando...':'Reiniciar'}</button>
        {:else}
          <button class="d-btn d-start" disabled={stopping[selectedService.id+':start']||selectedService.status==='error'} on:click={() => doAction(selectedService,'start')}>{stopping[selectedService.id+':start']?'Iniciando...':'Iniciar'}</button>
        {/if}
      </div>
      {#if selectedService.dependencies?.length>0}
        <div class="d-card"><div class="d-card-label">Dependencias</div>
          {#each selectedService.dependencies as dep}
            <div class="dep-row"><span class="dep-type">{dep.depType}</span><span class="dep-target">{dep.target}</span><span class="dep-req">{dep.required}</span></div>
          {/each}
        </div>
      {/if}
      {#if detailLogs.length>0}
        <div class="d-card log-card"><div class="d-card-label">Logs recientes<button class="log-refresh" on:click={() => loadLogs(selectedService)}><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" style="width:10px;height:10px"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-.18-5.4"/></svg></button></div>
          <div class="log-wrap">{#each detailLogs as log}<div class="log-line"><span class="log-ts">{log.timestamp}</span>{log.message}</div>{/each}</div>
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .nh-root{width:100%;height:100%;display:flex;flex-direction:column;overflow:hidden;font-family:var(--font-sans,'IBM Plex Sans',sans-serif);color:var(--text-primary);background:var(--bg-app)}
  .nh-titlebar{display:flex;align-items:center;gap:14px;padding:0 24px;height:var(--titlebar-height,40px);background:var(--bg-elev-1);border-bottom:1px solid var(--glass-border);flex-shrink:0}
  .nh-title{font-size:15px;font-weight:600;color:var(--text-primary);letter-spacing:-0.2px}
  .nh-sub{font-size:13px;color:var(--text-muted)}
  .wf-controls{display:flex;align-items:center;margin-left:auto;flex-shrink:0}
  .wf-btn{width:36px;height:30px;border:none;background:transparent;padding:0;color:var(--text-muted);cursor:pointer;display:flex;align-items:center;justify-content:center;transition:background .12s,color .12s;border-radius:6px}
  .wf-btn svg{width:14px;height:14px}
  .wf-btn:hover{background:rgba(255,255,255,0.06);color:var(--text-primary)}
  .wf-close:hover{background:rgba(239,68,68,0.8);color:#fff}
  .back-btn{display:inline-flex;align-items:center;gap:6px;font-size:13px;color:var(--text-secondary);cursor:pointer;padding:6px 10px;border-radius:6px;transition:all .15s}
  .back-btn:hover{color:var(--text-primary);background:var(--bg-elev-2)}.back-btn svg{width:14px;height:14px}
  .hw-bar{display:grid;grid-template-columns:repeat(4,1fr);background:var(--bg-elev-1);border-bottom:1px solid var(--glass-border);padding:10px 24px;flex-shrink:0}
  .hw-stat{display:flex;align-items:center;gap:14px;padding:0 18px;border-right:1px solid var(--glass-border)}
  .hw-stat:last-child{border-right:none}.hw-stat:first-child{padding-left:0}
  .hw-info{display:flex;flex-direction:column;gap:2px;min-width:0}
  .hw-lbl{font-size:9px;color:var(--text-muted);text-transform:uppercase;letter-spacing:1px;font-weight:500}
  .hw-val{font-family:var(--font-mono);font-size:15px;font-weight:600;color:var(--text-primary);line-height:1.1}
  .hw-sub{font-family:var(--font-mono);font-size:10px;color:var(--text-muted)}
  .hw-spark{flex:1;height:28px;min-width:60px}.hw-spark svg{width:100%;height:100%;display:block}
  .toolbar{display:flex;align-items:center;gap:14px;padding:10px 24px;flex-shrink:0;border-bottom:1px solid var(--glass-border)}
  .filters{display:flex;gap:4px}
  .chip{font-size:11px;font-weight:500;padding:6px 12px;border-radius:6px;background:transparent;border:1px solid transparent;color:var(--text-secondary);cursor:pointer;transition:all .12s}
  .chip:hover{background:var(--bg-elev-2);color:var(--text-primary)}
  .chip.active{background:rgba(59,130,246,0.12);color:var(--accent);border-color:rgba(59,130,246,0.3)}
  .search-box{max-width:180px;display:flex;align-items:center;gap:6px;padding:5px 10px;border-radius:6px;background:var(--bg-elev-1);border:1px solid var(--glass-border)}
  .search-box svg{width:12px;height:12px;color:var(--text-muted);flex-shrink:0}
  .search-box input{background:transparent;border:none;outline:none;font-family:inherit;font-size:11px;color:var(--text-primary);width:100%}
  .search-box input::placeholder{color:var(--text-muted)}
  .tb-counter{margin-left:auto;font-size:11px;color:var(--text-muted);font-family:var(--font-mono)}.tb-counter b{color:var(--text-primary);font-weight:600}
  .table-wrap{flex:1;overflow-y:auto;padding:0 24px}.table-wrap::-webkit-scrollbar{width:3px}.table-wrap::-webkit-scrollbar-thumb{background:var(--glass-border);border-radius:3px}
  table{width:100%;border-collapse:collapse;font-size:12px}
  thead th{text-align:left;font-size:10px;font-weight:600;color:var(--text-muted);text-transform:uppercase;letter-spacing:0.8px;padding:11px 10px 9px;border-bottom:1px solid var(--glass-border);background:var(--bg-app);position:sticky;top:0}
  thead th.num{text-align:right}
  tbody tr{border-bottom:1px solid var(--glass-border);transition:background .1s;cursor:pointer}
  tbody tr:hover{background:var(--bg-elev-1)}
  tbody tr.selected{background:rgba(59,130,246,0.08)}tbody tr.selected:hover{background:rgba(59,130,246,0.12)}
  tbody td{padding:9px 10px;vertical-align:middle}
  td.name-cell{display:flex;align-items:center;gap:10px}
  td.num{text-align:right;font-family:var(--font-mono);font-feature-settings:'tnum';color:var(--text-secondary)}
  td.num b{color:var(--text-primary);font-weight:500}
  td.num.warn{color:var(--c-warn)}td.num.warn b{color:var(--c-warn)}
  td.num.crit{color:var(--c-crit)}td.num.crit b{color:var(--c-crit)}
  .app-icon{width:22px;height:22px;border-radius:5px;background:var(--bg-elev-2);border:1px solid var(--glass-border);display:flex;align-items:center;justify-content:center;flex-shrink:0;font-size:10px;font-weight:700;color:var(--text-secondary);font-family:var(--font-mono)}
  .app-icon-img{width:22px;height:22px;border-radius:5px;object-fit:contain;flex-shrink:0}
  .app-name{font-weight:500;color:var(--text-primary)}.app-ver{font-family:var(--font-mono);font-size:10px;color:var(--text-muted);margin-left:6px}
  .child-badge{font-size:8px;font-weight:600;padding:1px 5px;border-radius:3px;background:rgba(59,130,246,0.1);color:var(--accent);margin-left:4px;text-transform:uppercase;letter-spacing:0.5px}
  .state{display:inline-flex;align-items:center;gap:6px;font-size:11px;font-weight:500}.state .dot{width:6px;height:6px;border-radius:50%}
  .state.run{color:var(--c-ok)}.state.run .dot{background:var(--c-ok);box-shadow:0 0 0 2px rgba(16,185,129,0.2)}
  .state.stop{color:var(--text-muted)}.state.stop .dot{background:var(--text-muted)}
  .state.err{color:var(--c-crit)}.state.err .dot{background:var(--c-crit);box-shadow:0 0 0 2px rgba(239,68,68,0.2)}
  .state.starting{color:var(--c-warn)}.state.starting .dot{background:var(--c-warn);animation:pulse .8s ease-in-out infinite}
  @keyframes pulse{0%,100%{opacity:1}50%{opacity:.35}}
  .row-actions{display:flex;gap:4px;justify-content:flex-end}
  .act-btn{width:24px;height:24px;border-radius:5px;background:transparent;border:1px solid var(--glass-border);color:var(--text-muted);cursor:pointer;display:inline-flex;align-items:center;justify-content:center;transition:all .12s}
  .act-btn:hover{background:var(--bg-elev-2);color:var(--text-primary)}.act-btn.start:hover{color:var(--c-ok);border-color:rgba(16,185,129,0.4)}.act-btn.stop:hover{color:var(--c-crit);border-color:rgba(239,68,68,0.4)}
  .act-btn:disabled{opacity:0.3;cursor:not-allowed}.act-btn svg{width:11px;height:11px}
  .empty-row{text-align:center;padding:28px;color:var(--text-muted);font-size:12px}
  .footer{display:flex;align-items:center;gap:8px;padding:8px 24px;flex-shrink:0;border-top:1px solid var(--glass-border);background:var(--bg-elev-1);font-size:11px;font-family:var(--font-mono)}
  .f-lbl{color:var(--text-muted)}.f-val{color:var(--text-primary);font-weight:500}.f-sep{color:var(--glass-border)}
  .detail-nav{display:flex;align-items:center;gap:14px;padding:10px 24px;flex-shrink:0;border-bottom:1px solid var(--glass-border)}
  .detail-name{font-size:16px;font-weight:700;color:var(--text-primary);letter-spacing:-0.3px}
  .detail-content{flex:1;overflow-y:auto;padding:20px 24px;display:flex;flex-direction:column;gap:14px}.detail-content::-webkit-scrollbar{width:3px}.detail-content::-webkit-scrollbar-thumb{background:var(--glass-border);border-radius:3px}
  .d-card{background:var(--glass-bg,rgba(30,34,48,0.55));border:1px solid var(--glass-border);border-radius:12px;padding:16px 18px}
  .d-card-label{font-size:10px;font-weight:600;color:var(--text-muted);text-transform:uppercase;letter-spacing:1px;margin-bottom:12px;display:flex;align-items:center;gap:6px}
  .d-grid{display:grid;grid-template-columns:100px 1fr;gap:10px 14px;font-size:12px}.d-key{color:var(--text-secondary)}.d-val{color:var(--text-primary);font-family:var(--font-mono);font-weight:500}
  .detail-actions{display:flex;gap:8px}
  .d-btn{flex:1;padding:10px;border:none;border-radius:8px;font-family:inherit;font-size:12px;font-weight:600;cursor:pointer;transition:opacity .15s}.d-btn:hover{opacity:.82}.d-btn:disabled{opacity:.3;cursor:not-allowed}
  .d-stop{background:var(--c-crit-dim);color:var(--c-crit)}.d-restart{background:var(--bg-elev-2);color:var(--text-secondary)}.d-start{background:var(--c-ok-dim,rgba(16,185,129,0.12));color:var(--c-ok)}
  .dep-row{display:flex;align-items:center;gap:8px;padding:7px 10px;background:var(--bg-elev-2);border:1px solid var(--glass-border);border-radius:7px;margin-top:4px}
  .dep-type{font-size:9px;font-weight:700;padding:2px 6px;border-radius:4px;background:var(--accent-dim);color:var(--accent);min-width:36px;text-align:center}
  .dep-target{font-size:10px;color:var(--text-secondary);font-family:var(--font-mono);flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.dep-req{font-size:9px;color:var(--text-muted);margin-left:auto}
  .log-card{flex:1;min-height:0;display:flex;flex-direction:column}
  .log-wrap{background:var(--bg-elev-2);border-radius:7px;padding:10px;font-family:var(--font-mono);font-size:9px;color:var(--text-secondary);line-height:1.7;flex:1;min-height:80px;overflow-y:auto}.log-wrap::-webkit-scrollbar{width:2px}.log-wrap::-webkit-scrollbar-thumb{background:var(--glass-border);border-radius:2px}
  .log-line{white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.log-ts{color:var(--text-muted);margin-right:8px}
  .log-refresh{background:none;border:none;cursor:pointer;color:var(--text-muted);padding:2px;transition:color .15s}.log-refresh:hover{color:var(--text-primary)}
</style>
