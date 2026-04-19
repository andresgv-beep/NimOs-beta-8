<script>
  import { onMount, onDestroy } from 'svelte';
  import { getToken, hdrs } from '$lib/stores/auth.js';
  import AppShell from '$lib/components/AppShell.svelte';

  let torrents = [];
  let activeTab = 'all';
  let loading = true;
  let search = '';
  let pollInterval;
  let selectedTorrent = null;
  let detailTab = 'general';

  // Add torrent
  let showAddModal = false;
  let addMode = 'file';
  let magnetLink = '';
  let selectedFile = null;
  let savePath = '';
  let shares = [];
  let addMsg = '';
  let addMsgError = false;
  let adding = false;

  // Delete
  let showDeleteConfirm = null;
  let deleteWithFiles = false;

  const appIcon = [
    { tag: 'path', attrs: { d: 'M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4' }},
    { tag: 'polyline', attrs: { points: '7 10 12 15 17 10' }},
    { tag: 'line', attrs: { x1: 12, y1: 15, x2: 12, y2: 3 }},
  ];

  // Reactive sections with badges
  $: activeCount  = torrents.filter(t => isDownloading(t)).length;
  $: seedCount    = torrents.filter(t => t.status === 'seeding' && t.progress >= 100).length;
  $: doneCount    = torrents.filter(t => isDone(t) && t.status !== 'seeding').length;
  $: pausedCount  = torrents.filter(t => isPaused(t)).length;
  $: errorCount   = torrents.filter(t => t.status === 'error').length;

  $: sections = [
    { label: 'Descargar', items: [
      { id: 'all', label: 'Todas las descargas', badge: torrents.length || null, paths: [
        { tag: 'path', attrs: { d: 'M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4' }},
        { tag: 'polyline', attrs: { points: '7 10 12 15 17 10' }},
        { tag: 'line', attrs: { x1:12, y1:15, x2:12, y2:3 }},
      ]},
      { id: 'active', label: 'Descargando', badge: activeCount || null, badgeColor: 'accent', paths: [
        { tag: 'line', attrs: { x1:12, y1:5, x2:12, y2:19 }},
        { tag: 'polyline', attrs: { points: '6 13 12 19 18 13' }},
      ]},
      { id: 'seeding', label: 'Compartiendo', badge: seedCount || null, badgeColor: 'ok', paths: [
        { tag: 'line', attrs: { x1:12, y1:19, x2:12, y2:5 }},
        { tag: 'polyline', attrs: { points: '6 11 12 5 18 11' }},
      ]},
      { id: 'done', label: 'Finalizado', badge: doneCount || null, paths: [
        { tag: 'polyline', attrs: { points: '20 6 9 17 4 12' }},
      ]},
      { id: 'paused', label: 'Pausado', badge: pausedCount || null, paths: [
        { tag: 'rect', attrs: { x:6, y:5, width:4, height:14, rx:1 }},
        { tag: 'rect', attrs: { x:14, y:5, width:4, height:14, rx:1 }},
      ]},
    ]},
  ];

  async function fetchTorrents() {
    try {
      const r = await fetch('/api/torrent/torrents', { headers: hdrs() });
      const d = await r.json();
      const raw = Array.isArray(d) ? d : (d.torrents || []);
      torrents = raw.map(t => ({
        ...t,
        progress:   (t.progress != null && t.progress <= 1) ? t.progress * 100 : (t.progress || 0),
        downloaded: t.total_done ?? t.downloaded ?? 0,
        size:       t.total_wanted ?? t.size ?? t.totalSize ?? 0,
        dlSpeed:    t.download_rate ?? t.dlSpeed ?? t.downloadSpeed ?? 0,
        ulSpeed:    t.upload_rate ?? t.ulSpeed ?? t.uploadSpeed ?? 0,
        numPeers:   t.peers ?? t.numPeers ?? 0,
        numSeeds:   t.seeds ?? t.numSeeds ?? 0,
        status:     t.paused ? 'paused' : (t.state || t.status || 'unknown'),
        savePath:   t.save_path ?? t.savePath ?? '',
      }));
    } catch { torrents = []; }
    loading = false;
  }

  async function fetchShares() {
    try {
      const r = await fetch('/api/shares', { headers: hdrs() });
      const d = await r.json();
      shares = d.shares || d || [];
    } catch { shares = []; }
  }

  onMount(() => { fetchTorrents(); fetchShares(); pollInterval = setInterval(fetchTorrents, 4000); });
  onDestroy(() => clearInterval(pollInterval));

  $: filtered = (() => {
    let list = torrents;
    if (activeTab === 'active')  list = list.filter(t => isDownloading(t));
    else if (activeTab === 'seeding') list = list.filter(t => t.status === 'seeding' && t.progress >= 100);
    else if (activeTab === 'done')    list = list.filter(t => isDone(t) && t.status !== 'seeding');
    else if (activeTab === 'paused')  list = list.filter(t => isPaused(t));
    if (search) list = list.filter(t => t.name?.toLowerCase().includes(search.toLowerCase()));
    return list;
  })();

  // Add torrent
  function openAddModal() { showAddModal = true; addMode = 'file'; magnetLink = ''; selectedFile = null; savePath = ''; addMsg = ''; addMsgError = false; }
  function pickFile() { const input = document.createElement('input'); input.type = 'file'; input.accept = '.torrent'; input.onchange = (e) => { selectedFile = e.target.files[0] || null; }; input.click(); }

  async function doAdd() {
    if (addMode === 'file' && !selectedFile) { addMsg = 'Selecciona un archivo .torrent'; addMsgError = true; return; }
    if (addMode === 'magnet' && !magnetLink.trim()) { addMsg = 'Introduce un magnet link'; addMsgError = true; return; }
    adding = true; addMsg = '';
    try {
      if (addMode === 'file') {
        const fd = new FormData(); fd.append('torrent', selectedFile); fd.append('save_path', savePath);
        const r = await fetch('/api/torrent/upload', { method: 'POST', headers: { 'Authorization': `Bearer ${getToken()}` }, body: fd });
        const d = await r.json(); if (d.error) { addMsg = d.error; addMsgError = true; adding = false; return; }
      } else {
        const r = await fetch('/api/torrent/add', { method: 'POST', headers: { ...hdrs(), 'Content-Type': 'application/json' }, body: JSON.stringify({ magnet: magnetLink.trim(), save_path: savePath }) });
        const d = await r.json(); if (d.error) { addMsg = d.error; addMsgError = true; adding = false; return; }
      }
      showAddModal = false; fetchTorrents();
    } catch { addMsg = 'Error de conexión'; addMsgError = true; }
    adding = false;
  }

  // Actions
  async function pauseTorrent(hash)  { await fetch('/api/torrent/pause',  { method:'POST', headers:{ ...hdrs(), 'Content-Type':'application/json' }, body:JSON.stringify({ hash }) }); fetchTorrents(); }
  async function resumeTorrent(hash) { await fetch('/api/torrent/resume', { method:'POST', headers:{ ...hdrs(), 'Content-Type':'application/json' }, body:JSON.stringify({ hash }) }); fetchTorrents(); }
  async function deleteTorrent(hash) { await fetch('/api/torrent/remove', { method:'POST', headers:{ ...hdrs(), 'Content-Type':'application/json' }, body:JSON.stringify({ hash, delete_files:deleteWithFiles }) }); showDeleteConfirm = null; deleteWithFiles = false; if ((selectedTorrent?.hash || selectedTorrent?.id) === hash) selectedTorrent = null; fetchTorrents(); }
  async function pauseAll()  { for (const t of torrents.filter(t => isDownloading(t))) await pauseTorrent(t.hash||t.id); }
  async function resumeAll() { for (const t of torrents.filter(t => isPaused(t))) await resumeTorrent(t.hash||t.id); }

  // Formatting
  function fmtSize(bytes) { if (!bytes) return '—'; if (bytes >= 1e12) return (bytes/1e12).toFixed(1)+' TB'; if (bytes >= 1e9) return (bytes/1e9).toFixed(1)+' GB'; if (bytes >= 1e6) return (bytes/1e6).toFixed(1)+' MB'; return (bytes/1e3).toFixed(0)+' KB'; }
  function fmtSpeed(bytes) { if (!bytes || bytes < 100) return '—'; if (bytes >= 1e6) return (bytes/1e6).toFixed(1)+' MB/s'; return (bytes/1e3).toFixed(0)+' KB/s'; }

  function isDownloading(t) { return t.status === 'downloading' || (t.progress < 100 && t.status !== 'paused' && t.status !== 'stopped' && t.status !== 'seeding'); }
  function isPaused(t) { return t.status === 'paused' || t.status === 'stopped'; }
  function isDone(t) { return t.progress >= 100; }
  function selectTorrent(t) { selectedTorrent = (selectedTorrent?.hash === t.hash) ? null : t; detailTab = 'general'; }

  function stateClass(t) { if (isDownloading(t)) return 'dl'; if (t.status === 'seeding') return 'sd'; if (isPaused(t)) return 'ps'; if (isDone(t)) return 'dn'; return 'ps'; }
  function stateLabel(t) { if (isDownloading(t)) return 'Descargando'; if (t.status === 'seeding') return 'Compartiendo'; if (isPaused(t)) return 'En pausa'; if (isDone(t)) return 'Completado'; return t.status; }

  $: dlSpeed = torrents.reduce((a, t) => a + (t.dlSpeed || 0), 0);
  $: ulSpeed = torrents.reduce((a, t) => a + (t.ulSpeed || 0), 0);
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<AppShell title="NimTorrent" {appIcon} {sections} bind:active={activeTab} showSearch>
  <svelte:fragment slot="toolbar">
    <div class="toolbar">
      <button class="tb-btn primary" title="Añadir torrent" on:click={openAddModal}><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg></button>
      <div class="tb-sep"></div>
      <button class="tb-btn" title="Reanudar todo" on:click={resumeAll}><svg viewBox="0 0 24 24" fill="currentColor"><polygon points="6,4 20,12 6,20"/></svg></button>
      <button class="tb-btn" title="Pausar todo" on:click={pauseAll}><svg viewBox="0 0 24 24" fill="currentColor"><rect x="6" y="5" width="4" height="14" rx="1"/><rect x="14" y="5" width="4" height="14" rx="1"/></svg></button>
      <div class="tb-sep"></div>
      <div class="tb-filter">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="7"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
        <input type="text" placeholder="Filtrar..." bind:value={search}>
      </div>
    </div>
  </svelte:fragment>

  {#if loading}
    <div class="t-empty"><div class="spinner"></div></div>
  {:else if filtered.length === 0}
    <div class="t-empty">
      <div class="t-empty-icon">⬇</div>
      <div>Sin torrents</div>
      <button class="btn-accent" style="margin-top:10px" on:click={openAddModal}>Añadir torrent</button>
    </div>
  {:else}
    <div class="nt-layout" class:has-detail={selectedTorrent}>
      <div class="table-wrap">
        <table>
          <thead><tr>
            <th>Nombre de archivo</th>
            <th class="num">Tamaño</th>
            <th class="num">Progreso</th>
            <th class="num">↓ Vel.</th>
            <th class="num">↑ Vel.</th>
            <th>Estado</th>
            <th></th>
          </tr></thead>
          <tbody>
            {#each filtered as t (t.hash || t.id)}
              <tr class:selected={(selectedTorrent?.hash||selectedTorrent?.id)===(t.hash||t.id)} on:click={() => selectTorrent(t)}>
                <td class="name"><div class="name-cell">
                  <div class="arrow {stateClass(t)}">
                    {#if isDownloading(t)}<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="5" x2="12" y2="19"/><polyline points="6 13 12 19 18 13"/></svg>
                    {:else if t.status === 'seeding'}<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="19" x2="12" y2="5"/><polyline points="6 11 12 5 18 11"/></svg>
                    {:else if isPaused(t)}<svg viewBox="0 0 24 24" fill="currentColor"><rect x="6" y="5" width="4" height="14" rx="1"/><rect x="14" y="5" width="4" height="14" rx="1"/></svg>
                    {:else}<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>
                    {/if}
                  </div>
                  <span class="name-text">{t.name}</span>
                </div></td>
                <td class="num">{fmtSize(t.size)}</td>
                <td class="num">{(t.progress ?? 0).toFixed(1)}%</td>
                <td class="num">{fmtSpeed(t.dlSpeed)}</td>
                <td class="num">{fmtSpeed(t.ulSpeed)}</td>
                <td><span class="state-text {stateClass(t)}">{stateLabel(t)}</span></td>
                <td><div class="row-actions" on:click|stopPropagation>
                  {#if isPaused(t)}
                    <button class="act-btn start" on:click={() => resumeTorrent(t.hash||t.id)}><svg viewBox="0 0 24 24" fill="currentColor"><polygon points="6,4 20,12 6,20"/></svg></button>
                  {:else if !isDone(t)}
                    <button class="act-btn" on:click={() => pauseTorrent(t.hash||t.id)}><svg viewBox="0 0 24 24" fill="currentColor"><rect x="6" y="5" width="4" height="14" rx="1"/><rect x="14" y="5" width="4" height="14" rx="1"/></svg></button>
                  {/if}
                  <button class="act-btn stop" on:click={() => { showDeleteConfirm = t.hash||t.id; deleteWithFiles = false; }}><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/></svg></button>
                </div></td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>

      <!-- DETAIL BAR -->
      <div class="detail-bar">
        <div class="st dl">↓ <b>{fmtSpeed(dlSpeed)}</b></div>
        <div class="sep"></div>
        <div class="st ul">↑ <b>{fmtSpeed(ulSpeed)}</b></div>
        <div class="sep"></div>
        <div class="st"><b>{torrents.length}</b> elementos</div>
      </div>

      <!-- DETAIL PANEL -->
      {#if selectedTorrent}
        <div class="tabs-bar">
          {#each [['general','General'],['transfer','Transferir'],['trackers','Trackers'],['files','Archivos']] as [id,label]}
            <!-- svelte-ignore a11y_click_events_have_key_events --><!-- svelte-ignore a11y_no_static_element_interactions -->
            <button class="tab" class:active={detailTab===id} on:click={() => detailTab=id}>{label}</button>
          {/each}
        </div>
        <div class="tab-panel">
          {#if detailTab === 'general'}
            <div class="kv-grid">
              <div class="kv-k">Nombre:</div><div class="kv-v">{selectedTorrent.name}</div>
              <div class="kv-k">Destino:</div><div class="kv-v dim">{selectedTorrent.savePath || '—'}</div>
              <div class="kv-k">Tamaño:</div><div class="kv-v">{fmtSize(selectedTorrent.size)}</div>
              <div class="kv-k">Descargado:</div><div class="kv-v">{fmtSize(selectedTorrent.downloaded)}</div>
              <div class="kv-k">Progreso:</div><div class="kv-v">{(selectedTorrent.progress ?? 0).toFixed(1)}%</div>
              <div class="kv-k">Peers:</div><div class="kv-v">{selectedTorrent.numPeers || 0}</div>
              <div class="kv-k">Seeds:</div><div class="kv-v">{selectedTorrent.numSeeds || 0}</div>
            </div>
          {:else}
            <div class="kv-empty">Sin datos disponibles</div>
          {/if}
        </div>
      {/if}
    </div>
  {/if}
</AppShell>

<!-- ADD MODAL -->
{#if showAddModal}
  <!-- svelte-ignore a11y_click_events_have_key_events --><!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="modal-overlay" on:click|self={() => showAddModal = false}></div>
  <div class="modal">
    <div class="modal-header">
      <div class="modal-title">Añadir torrent</div>
      <!-- svelte-ignore a11y_click_events_have_key_events --><!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="modal-close" on:click={() => showAddModal = false}>✕</div>
    </div>
    <div class="modal-body">
      <div class="add-tabs">
        <!-- svelte-ignore a11y_click_events_have_key_events --><!-- svelte-ignore a11y_no_static_element_interactions -->
        <div class="add-tab" class:active={addMode==='file'} on:click={() => addMode='file'}>Archivo .torrent</div>
        <!-- svelte-ignore a11y_click_events_have_key_events --><!-- svelte-ignore a11y_no_static_element_interactions -->
        <div class="add-tab" class:active={addMode==='magnet'} on:click={() => addMode='magnet'}>Magnet link</div>
      </div>
      {#if addMode === 'file'}
        <div class="form-field"><label class="form-label">Archivo</label>
          <!-- svelte-ignore a11y_click_events_have_key_events --><!-- svelte-ignore a11y_no_static_element_interactions -->
          <div class="file-picker" on:click={pickFile}>{#if selectedFile}<span class="file-name">{selectedFile.name}</span><span class="file-size">{fmtSize(selectedFile.size)}</span>{:else}<span class="file-placeholder">Seleccionar archivo .torrent</span>{/if}</div>
        </div>
      {:else}
        <div class="form-field"><label class="form-label">Magnet link</label><input class="form-input" type="text" placeholder="magnet:?xt=urn:btih:..." bind:value={magnetLink} /></div>
      {/if}
      <div class="form-field"><label class="form-label">Carpeta de destino</label>
        <div class="dest-options">
          {#each shares as share}
            <!-- svelte-ignore a11y_click_events_have_key_events --><!-- svelte-ignore a11y_no_static_element_interactions -->
            <div class="dest-option" class:active={savePath===(share.path||`/pool/${share.pool}/${share.name}`)} on:click={() => savePath = share.path||`/pool/${share.pool}/${share.name}`}>
              <div class="dest-name">{share.displayName||share.name}</div>
              <div class="dest-path">{share.path||`/pool/${share.pool}/${share.name}`}</div>
            </div>
          {/each}
        </div>
        <input class="form-input" style="margin-top:8px" type="text" placeholder="/ruta/personalizada" bind:value={savePath} />
      </div>
      {#if addMsg}<div class="add-msg" class:error={addMsgError}>{addMsg}</div>{/if}
    </div>
    <div class="modal-footer">
      <button class="btn-secondary" on:click={() => showAddModal = false}>Cancelar</button>
      <button class="btn-accent" on:click={doAdd} disabled={adding}>{adding ? 'Añadiendo...' : 'Añadir torrent'}</button>
    </div>
  </div>
{/if}

<!-- DELETE CONFIRM -->
{#if showDeleteConfirm}
  <!-- svelte-ignore a11y_click_events_have_key_events --><!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="modal-overlay" on:click|self={() => showDeleteConfirm = null}></div>
  <div class="modal modal-sm">
    <div class="modal-header"><div class="modal-title">Eliminar torrent</div><!-- svelte-ignore a11y_click_events_have_key_events --><!-- svelte-ignore a11y_no_static_element_interactions --><div class="modal-close" on:click={() => showDeleteConfirm = null}>✕</div></div>
    <div class="modal-body">
      <p style="font-size:12px;color:var(--text-secondary);margin:0">¿Eliminar este torrent de la lista?</p>
      <label class="delete-check"><input type="checkbox" bind:checked={deleteWithFiles} /><span>También eliminar archivos</span></label>
    </div>
    <div class="modal-footer">
      <button class="btn-secondary" on:click={() => showDeleteConfirm = null}>Cancelar</button>
      <button class="btn-accent btn-danger" on:click={() => deleteTorrent(showDeleteConfirm)}>Eliminar</button>
    </div>
  </div>
{/if}

<style>
  /* Toolbar */
  .toolbar{display:flex;align-items:center;gap:4px;padding:8px 18px;border-bottom:1px solid var(--glass-border);flex-shrink:0}
  .tb-btn{width:30px;height:30px;border-radius:6px;background:transparent;border:none;cursor:pointer;display:flex;align-items:center;justify-content:center;color:var(--text-secondary);transition:all .12s}
  .tb-btn:hover{background:var(--bg-elev-2);color:var(--text-primary)}
  .tb-btn.primary{color:var(--accent)}
  .tb-btn svg{width:15px;height:15px}
  .tb-sep{width:1px;height:20px;background:var(--glass-border);margin:0 4px}
  .tb-filter{display:flex;align-items:center;gap:7px;padding:5px 10px;border-radius:6px;background:var(--bg-elev-1);border:1px solid var(--glass-border);max-width:160px}
  .tb-filter svg{width:12px;height:12px;color:var(--text-muted)}
  .tb-filter input{background:transparent;border:none;outline:none;font-family:inherit;font-size:11px;color:var(--text-primary);width:100%}
  .tb-filter input::placeholder{color:var(--text-muted)}

  /* Layout */
  .nt-layout{display:flex;flex-direction:column;height:100%}
  .table-wrap{flex:1;overflow:auto}
  .table-wrap::-webkit-scrollbar{width:3px}
  .table-wrap::-webkit-scrollbar-thumb{background:var(--glass-border);border-radius:3px}

  /* Table */
  table{width:100%;border-collapse:collapse;font-size:12px}
  thead th{text-align:left;font-size:10px;font-weight:600;color:var(--text-muted);padding:10px 12px;border-bottom:1px solid var(--glass-border);background:var(--bg-app);position:sticky;top:0;z-index:5;text-transform:uppercase;letter-spacing:0.8px}
  thead th.num{text-align:right}
  tbody tr{border-bottom:1px solid var(--glass-border);transition:background .1s;cursor:pointer}
  tbody tr:hover{background:var(--bg-elev-1)}
  tbody tr.selected{background:rgba(59,130,246,0.08)}
  tbody td{padding:8px 12px;vertical-align:middle}
  td.name{max-width:0}
  .name-cell{display:flex;align-items:center;gap:10px;min-width:0}
  .arrow{width:16px;height:16px;flex-shrink:0;display:flex;align-items:center;justify-content:center}
  .arrow svg{width:13px;height:13px}
  .arrow.dl{color:var(--accent)}.arrow.sd{color:#8b5cf6}.arrow.ps{color:var(--text-muted)}.arrow.dn{color:var(--c-ok)}
  .name-text{min-width:0;overflow:hidden;white-space:nowrap;text-overflow:ellipsis;color:var(--text-primary)}
  td.num{text-align:right;font-family:var(--font-mono);color:var(--text-secondary);white-space:nowrap}
  .state-text{font-size:12px;font-weight:500}
  .state-text.dl{color:var(--accent)}.state-text.sd{color:#8b5cf6}.state-text.ps{color:var(--text-muted)}.state-text.dn{color:var(--c-ok)}

  /* Row actions */
  .row-actions{display:flex;gap:4px;justify-content:flex-end}
  .act-btn{width:24px;height:24px;border-radius:5px;background:transparent;border:1px solid var(--glass-border);color:var(--text-muted);cursor:pointer;display:inline-flex;align-items:center;justify-content:center;transition:all .12s}
  .act-btn:hover{background:var(--bg-elev-2);color:var(--text-primary)}
  .act-btn.start:hover{color:var(--c-ok);border-color:rgba(16,185,129,0.4)}
  .act-btn.stop:hover{color:var(--c-crit);border-color:rgba(239,68,68,0.4)}
  .act-btn svg{width:11px;height:11px}

  /* Detail bar */
  .detail-bar{display:flex;align-items:center;gap:14px;padding:8px 18px;border-top:1px solid var(--glass-border);background:var(--bg-elev-1);font-size:11px;font-family:var(--font-mono);color:var(--text-secondary)}
  .detail-bar .st{color:var(--text-muted)}.detail-bar .st b{color:var(--text-primary);font-weight:600}
  .detail-bar .st.dl b{color:var(--c-ok)}.detail-bar .st.ul b{color:var(--accent)}
  .detail-bar .sep{width:1px;height:12px;background:var(--glass-border)}

  /* Tabs */
  .tabs-bar{display:flex;gap:2px;padding:0 18px;background:var(--bg-elev-1);border-top:1px solid var(--glass-border)}
  .tab{padding:8px 14px;font-size:11px;font-weight:500;color:var(--text-muted);background:transparent;border:none;cursor:pointer;border-bottom:2px solid transparent;transition:all .12s;font-family:inherit}
  .tab:hover{color:var(--text-secondary)}
  .tab.active{color:var(--accent);border-bottom-color:var(--accent)}

  /* Tab panel */
  .tab-panel{padding:14px 18px;background:var(--bg-elev-1);border-top:1px solid var(--glass-border);max-height:160px;overflow-y:auto;flex-shrink:0}
  .tab-panel::-webkit-scrollbar{width:3px}.tab-panel::-webkit-scrollbar-thumb{background:var(--glass-border);border-radius:3px}
  .kv-grid{display:grid;grid-template-columns:110px 1fr;gap:6px 14px;font-size:12px}
  .kv-k{color:var(--text-muted)}.kv-v{color:var(--text-primary);font-family:var(--font-mono);word-break:break-all}.kv-v.dim{color:var(--text-secondary)}
  .kv-empty{font-size:12px;color:var(--text-muted);text-align:center;padding:12px}

  /* Empty */
  .t-empty{height:100%;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:8px;color:var(--text-muted);font-size:12px}
  .t-empty-icon{font-size:28px;opacity:.3}
  .spinner{width:24px;height:24px;border-radius:50%;border:2px solid var(--glass-border);border-top-color:var(--accent);animation:spin .7s linear infinite}
  @keyframes spin{to{transform:rotate(360deg)}}

  /* Modal */
  .modal-overlay{position:fixed;inset:0;z-index:10000;background:rgba(0,0,0,0.55)}
  .modal{position:fixed;top:50%;left:50%;transform:translate(-50%,-50%);z-index:10001;width:480px;max-width:90%;background:var(--bg-elev-1);border-radius:12px;border:1px solid var(--glass-border);box-shadow:0 24px 60px rgba(0,0,0,0.5);display:flex;flex-direction:column;overflow:hidden;animation:modalIn .2s cubic-bezier(0.16,1,0.3,1) both}
  .modal.modal-sm{width:360px}
  @keyframes modalIn{from{opacity:0;transform:translate(-50%,-48%) scale(0.97)}to{opacity:1;transform:translate(-50%,-50%) scale(1)}}
  .modal-header{display:flex;align-items:center;gap:12px;padding:14px 18px;border-bottom:1px solid var(--glass-border)}
  .modal-title{font-size:13px;font-weight:600;color:var(--text-primary);flex:1}
  .modal-close{width:24px;height:24px;border-radius:6px;cursor:pointer;display:flex;align-items:center;justify-content:center;color:var(--text-muted);font-size:11px;background:var(--bg-elev-2);transition:all .15s}
  .modal-close:hover{color:var(--text-primary)}
  .modal-body{padding:18px 20px;display:flex;flex-direction:column;gap:14px}
  .modal-footer{display:flex;align-items:center;justify-content:flex-end;gap:8px;padding:12px 18px;border-top:1px solid var(--glass-border)}

  .add-tabs{display:flex;gap:6px}
  .add-tab{padding:8px 14px;border-radius:8px;cursor:pointer;font-size:11px;font-weight:500;color:var(--text-muted);border:1px solid var(--glass-border);background:var(--bg-elev-2);transition:all .15s}
  .add-tab:hover{color:var(--text-secondary)}
  .add-tab.active{color:var(--accent);border-color:var(--accent);background:rgba(59,130,246,0.06)}
  .form-field{display:flex;flex-direction:column;gap:4px}
  .form-label{font-size:10px;font-weight:600;color:var(--text-muted);text-transform:uppercase;letter-spacing:.06em}
  .form-input{padding:8px 12px;border-radius:8px;background:var(--bg-elev-2);border:1px solid var(--glass-border);color:var(--text-primary);font-size:12px;font-family:inherit;outline:none;transition:border-color .2s}
  .form-input:focus{border-color:var(--accent)}.form-input::placeholder{color:var(--text-muted)}
  .file-picker{display:flex;align-items:center;justify-content:space-between;padding:10px 14px;border-radius:8px;cursor:pointer;border:1px dashed var(--glass-border);background:var(--bg-elev-2);transition:all .15s}
  .file-picker:hover{border-color:var(--accent)}
  .file-name{font-size:12px;color:var(--text-primary);font-weight:500}.file-size{font-size:10px;color:var(--text-muted);font-family:var(--font-mono)}
  .file-placeholder{font-size:12px;color:var(--text-muted)}
  .dest-options{display:flex;flex-direction:column;gap:4px}
  .dest-option{padding:8px 12px;border-radius:8px;cursor:pointer;border:1px solid var(--glass-border);background:var(--bg-elev-2);transition:all .15s}
  .dest-option:hover{border-color:var(--accent)}.dest-option.active{border-color:var(--accent);background:rgba(59,130,246,0.06)}
  .dest-name{font-size:11px;font-weight:600;color:var(--text-primary)}.dest-path{font-size:9px;color:var(--text-muted);font-family:var(--font-mono)}
  .add-msg{font-size:11px;color:var(--c-ok)}.add-msg.error{color:var(--c-crit)}
  .delete-check{display:flex;align-items:center;gap:8px;margin-top:10px;font-size:11px;color:var(--text-secondary);cursor:pointer}
  .delete-check input{accent-color:var(--c-crit)}
  .btn-secondary{padding:7px 13px;border-radius:8px;border:1px solid var(--glass-border);background:var(--bg-elev-2);color:var(--text-secondary);font-size:11px;font-weight:500;cursor:pointer;font-family:inherit;transition:all .15s}
  .btn-secondary:hover{color:var(--text-primary)}
  .btn-accent{padding:7px 14px;border-radius:8px;border:none;cursor:pointer;background:var(--accent);color:#fff;font-size:11px;font-weight:600;font-family:inherit;transition:all .15s}
  .btn-accent:hover{filter:brightness(1.1)}.btn-accent:disabled{opacity:.5;cursor:not-allowed}
  .btn-danger{background:var(--c-crit)}
</style>
