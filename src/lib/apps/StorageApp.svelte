<script>
  import { onMount, onDestroy } from 'svelte';
  import { hdrs } from '$lib/stores/auth.js';
  import AppShell from '$lib/components/AppShell.svelte';
  import Card from '$lib/ui/Card.svelte';
  import Badge from '$lib/ui/Badge.svelte';
  import SectionLabel from '$lib/ui/SectionLabel.svelte';
  import Button from '$lib/ui/Button.svelte';
  import StatusHero from '$lib/ui/StatusHero.svelte';
  import PoolCard from '$lib/ui/PoolCard.svelte';
  import ViewToggle from '$lib/ui/ViewToggle.svelte';
  import { prefs, setPref } from '$lib/stores/theme.js';
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';

  // ── Generic dialog state ──
  let dlg = { open:false, variant:'default', title:'', message:'', confirmText:'Confirmar', cancelText:'Cancelar', onConfirm:()=>{}, loading:false, services:[], requireInput:false };
  function showDialog(opts) { dlg = { ...dlg, open:true, loading:false, services:[], requireInput:false, ...opts }; }
  function closeDialog() { dlg = { ...dlg, open:false }; }

  // ── Toast/notification state (replaces alert()) ──
  let toast = { show:false, message:'', variant:'default' };
  function showToast(message, variant='default') { toast = { show:true, message, variant }; setTimeout(()=> toast = { ...toast, show:false }, 4000); }

  let active = 'resumen';
  let loading = true;

// ── View mode (expanded = A1, compact = A2) ──
$: viewModePref = $prefs.poolViewMode || 'auto';
$: viewMode = viewModePref === 'auto'
  ? (pools.length <= 3 ? 'expanded' : 'compact')
  : viewModePref;

  function onViewModeChange(e) {
  setPref('poolViewMode', e.detail);
  }

  // ── Pool activity tracking (scrub, snapshot, export, resilver) ──
  // Shape: { [poolName]: { action, state, progress?, speed?, timeRemaining?, ok?, message? } }
  let poolActivity = {};
  let scrubPollInterval = null;

  function setActivity(name, a) {
    poolActivity = { ...poolActivity, [name]: a };
  }
  function clearActivity(name) {
    const copy = { ...poolActivity };
    delete copy[name];
    poolActivity = copy;
  }
  function completeActivity(name, ok, message) {
    const prev = poolActivity[name];
    if (!prev) return;
    setActivity(name, { ...prev, state: 'completed', ok, message });
    setTimeout(() => {
      // Solo limpia si sigue siendo la misma acción completada (no sobrescribir otra nueva)
      const cur = poolActivity[name];
      if (cur && cur.state === 'completed' && cur.action === prev.action) {
        clearActivity(name);
      }
    }, 4500);
  }

  // Poll de scrub/resilver — se lanza cuando hay al menos una activity de esos tipos en 'running'
  async function pollScrubs() {
    const names = Object.entries(poolActivity)
      .filter(([, a]) => a?.state === 'running' && (a.action === 'scrub' || a.action === 'resilver'))
      .map(([n]) => n);
    if (names.length === 0) {
      if (scrubPollInterval) { clearInterval(scrubPollInterval); scrubPollInterval = null; }
      return;
    }
    for (const name of names) {
      try {
        const r = await fetch(`/api/storage/scrub/status?pool=${encodeURIComponent(name)}`, { headers: hdrs() });
        const d = await r.json();
        const prev = poolActivity[name];
        if (!prev || prev.state !== 'running') continue;
        if (d.status === 'scrubbing') {
          setActivity(name, {
            ...prev,
            progress: typeof d.progress === 'number' ? d.progress : prev.progress,
            speed: d.speed && d.speed !== '—' ? d.speed : undefined,
            timeRemaining: d.timeRemaining && d.timeRemaining !== '—' ? d.timeRemaining : undefined,
          });
        } else if (d.status === 'done') {
          completeActivity(name, true, 'Verificación completada');
          await load(); // refrescar salud del pool
        } else if (d.status === 'canceled') {
          completeActivity(name, false, 'Verificación cancelada');
        } else if (d.status === 'error') {
          completeActivity(name, false, d.error || 'Error en verificación');
        }
      } catch {}
    }
  }

  function ensureScrubPoll() {
    if (scrubPollInterval) return;
    scrubPollInterval = setInterval(pollScrubs, 3000);
  }

  // Detectar scrubs en marcha al cargar (alguien pudo haberlo lanzado antes)
  async function detectOngoingScrubs() {
    for (const p of pools) {
      if (p.type !== 'zfs') continue; // de momento solo ZFS tiene progress fiable
      try {
        const r = await fetch(`/api/storage/scrub/status?pool=${encodeURIComponent(p.name)}`, { headers: hdrs() });
        const d = await r.json();
        if (d.status === 'scrubbing' && !poolActivity[p.name]) {
          setActivity(p.name, {
            action: 'scrub',
            state: 'running',
            progress: typeof d.progress === 'number' ? d.progress : 0,
            speed: d.speed !== '—' ? d.speed : undefined,
            timeRemaining: d.timeRemaining !== '—' ? d.timeRemaining : undefined,
          });
        }
      } catch {}
    }
    if (Object.keys(poolActivity).length > 0) ensureScrubPoll();
  }
  
  let pools = [];
  let shares = [];
  let poolFileStats = {};
  let eligible = [];
  let capabilities = { zfs: false, btrfs: false, mdadm: false, recommended: 'zfs' };
  let expandedPools = {};
  let openMenu = null; // pool name with kebab open

  // Create pool
  let newPool = { name: '', type: 'zfs', profile: 'raidz1', disks: [] };
  let showCreatePool = false;
  let creating = false;
  let poolMsg = '';

  // Destroy
  let showDestroy = false;
  let destroyPool = null;
  let destroyDeps = [];
  let destroyInput = '';
  let destroying = false;
  let stoppingService = {};

  $: allDepsStopped = destroyDeps.every(d => d.status !== 'running' && d.status !== 'starting');
  $: canDestroy = destroyInput === 'ELIMINAR' && allDepsStopped;

  // ── App icon ──
  const appIcon = [
    { tag: 'path', attrs: { d: 'M3 5c0 1.66 4 3 9 3s9-1.34 9-3v14c0 1.66-4 3-9 3s-9-1.34-9-3z', fill: 'currentColor', opacity: '0.12', stroke: 'none' }},
    { tag: 'ellipse', attrs: { cx: 12, cy: 5, rx: 9, ry: 3 }},
    { tag: 'path', attrs: { d: 'M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5', fill: 'none' }},
    { tag: 'path', attrs: { d: 'M3 12c0 1.66 4 3 9 3s9-1.34 9-3', fill: 'none' }},
  ];

  // ── Sidebar ──
  const sections = [
    { label: 'Almacenamiento', items: [
      { id: 'resumen', label: 'Resumen', paths: [
        { tag: 'rect', attrs: { x:3, y:3, width:7, height:7, rx:2, 'data-fill': '' }},
        { tag: 'rect', attrs: { x:14, y:3, width:7, height:7, rx:2, 'data-fill': '' }},
        { tag: 'rect', attrs: { x:3, y:14, width:7, height:7, rx:2, 'data-fill': '' }},
        { tag: 'rect', attrs: { x:14, y:14, width:7, height:7, rx:2, 'data-fill': '' }},
        { tag: 'rect', attrs: { x:3, y:3, width:7, height:7, rx:2 }},
        { tag: 'rect', attrs: { x:14, y:3, width:7, height:7, rx:2 }},
        { tag: 'rect', attrs: { x:3, y:14, width:7, height:7, rx:2 }},
        { tag: 'rect', attrs: { x:14, y:14, width:7, height:7, rx:2 }},
      ]},
      { id: 'disks', label: 'Discos', paths: [
        { tag: 'circle', attrs: { cx:12, cy:12, r:9, 'data-fill': '' }},
        { tag: 'circle', attrs: { cx:12, cy:12, r:9 }},
        { tag: 'circle', attrs: { cx:12, cy:12, r:2.5, 'data-dot': '' }},
      ]},
      { id: 'snapshots', label: 'Puntos de restauración', paths: [
        { tag: 'circle', attrs: { cx:13, cy:14, r:8, 'data-fill': '' }},
        { tag: 'polyline', attrs: { points: '1 4 1 10 7 10' }},
        { tag: 'path', attrs: { d: 'M3.51 15a9 9 0 1 0 2.13-9.36L1 10' }},
      ]},
    ]},
    { label: 'Mantenimiento', items: [
      { id: 'health', label: 'Salud', paths: [
        { tag: 'path', attrs: { d: 'M9 3l-3 9H2l4 9h0l3-9h4l3-9', 'data-fill': '', style: 'opacity:0.08' }},
        { tag: 'path', attrs: { d: 'M22 12h-4l-3 9L9 3l-3 9H2' }},
      ]},
    ]}
  ];

  // ── Data ──
  async function load() {
    loading = true;
    try {
      const [statusRes, sharesRes, disksRes, capRes, restoreRes] = await Promise.all([
        fetch('/api/storage/status', { headers: hdrs() }),
        fetch('/api/shares', { headers: hdrs() }),
        fetch('/api/storage/disks', { headers: hdrs() }),
        fetch('/api/storage/capabilities', { headers: hdrs() }),
        fetch('/api/storage/restorable', { headers: hdrs() }),
      ]);
      pools = (await statusRes.json()).pools || [];
      shares = await sharesRes.json();
      eligible = (await disksRes.json()).eligible || [];
      const caps = await capRes.json();
      capabilities = caps;
      if (caps.recommended) newPool.type = caps.recommended;
      restorable = (await restoreRes.json()).pools || [];

      const agg = {};
      for (const s of shares) {
        const pn = s.pool || s.volume;
        if (!pn || !s.fileStats) continue;
        if (!agg[pn]) agg[pn] = { video:0, image:0, audio:0, document:0, other:0 };
        for (const k of ['video','image','audio','document','other']) agg[pn][k] += s.fileStats[k] || 0;
      }
      poolFileStats = agg;

      // Detectar scrubs/resilvers ya en marcha (al abrir Storage o refrescar)
      detectOngoingScrubs();
    } catch (e) { console.error('[Storage] load failed', e); }
    loading = false;
  }

  // ── Helpers ──
  function poolStatus(p) {
    const ph = p.poolHealth;
    if (ph?.status === 'critical' || p.health === 'FAULTED') return 'crit';
    if (ph?.status === 'degraded' || p.health === 'DEGRADED') return 'warn';
    if (p.disks?.some(d => d.smartStatus === 'critical')) return 'warn';
    return 'ok';
  }
  function diskStatus(d) { return d.smartStatus === 'critical' ? 'crit' : d.smartStatus === 'warning' ? 'warn' : d.smartStatus === 'ok' ? 'ok' : 'unknown'; }
  function diskStatusLabel(d) { const s = diskStatus(d); return s === 'crit' ? 'Crítico' : s === 'warn' ? 'Atención' : s === 'ok' ? 'Sano' : '—'; }
  function fmtH(h) { return !h ? '—' : h >= 1000 ? (h/1000).toFixed(1)+'k h' : h+' h'; }
  function fmtB(b) {
    if (!b) return '0 B';
    if (b >= 1099511627776) return (b/1099511627776).toFixed(1)+' TB';
    if (b >= 1073741824) return (b/1073741824).toFixed(1)+' GB';
    if (b >= 1048576) return Math.round(b/1048576)+' MB';
    if (b >= 1024) return Math.round(b/1024)+' KB';
    return b+' B';
  }
  function vdevLabel(t) { return {raidz1:'RAIDZ1',raidz2:'RAIDZ2',raidz3:'RAIDZ3',mirror:'Espejo',single:'Simple',stripe:'Stripe'}[t]||t||''; }
  function pillSub(p) {
    let s = p.type?.toUpperCase() || '';
    const vl = vdevLabel(p.vdevType);
    if (vl) s += ' · ' + vl;
    s += ' · ' + (p.disks?.length||0) + ' disco' + ((p.disks?.length||0) !== 1 ? 's' : '');
    return s;
  }
  function barColor(pct) { return pct > 90 ? 'crit' : pct > 80 ? 'warn' : 'ok'; }

  const categories = [
    { key:'video', label:'Vídeo', color:'#3b82f6' },
    { key:'audio', label:'Audio', color:'#f59e0b' },
    { key:'image', label:'Imágenes', color:'#10b981' },
    { key:'document', label:'Documentos', color:'#8b5cf6' },
    { key:'other', label:'Otros', color:'#64748b' },
  ];

  function donutSegs(stats) {
    if (!stats) return [];
    const tot = Object.values(stats).reduce((a,b)=>a+b,0);
    if (tot <= 0) return [];
    const C = 2*Math.PI*48;
    let off = 0;
    return categories.filter(c=>(stats[c.key]||0)>0).map(c => {
      const len = (stats[c.key]/tot)*C;
      const seg = { ...c, dash:`${len} ${C}`, off: -off };
      off += len;
      return seg;
    });
  }

  // ── System status ──
  $: totalDisks = pools.reduce((n,p)=>n+(p.disks?.length||0),0);
  $: problemDisks = pools.flatMap(p=>(p.disks||[]).filter(d=>d.smartStatus!=='ok'));
  $: sysStatus = pools.some(p=>poolStatus(p)==='crit')?'crit':pools.some(p=>poolStatus(p)==='warn')?'warn':'ok';
  $: sysTitle = sysStatus==='crit'?'Grupo de almacenamiento en riesgo':sysStatus==='warn'?'Un volumen necesita tu atención':'El sistema funciona correctamente';
  $: sysSub = `${pools.length} volumen${pools.length!==1?'es':''} · ${totalDisks} discos · ${problemDisks.length===0?'sin incidencias':problemDisks.length+' disco'+(problemDisks.length>1?'s':'')+' con avisos'}`;

  // ── Actions ──
  function togglePill(name) { expandedPools[name] = !expandedPools[name]; expandedPools = expandedPools; openMenu = null; }
  function toggleMenu(name, e) { e.stopPropagation(); openMenu = openMenu === name ? null : name; }
  function closeMenus() { openMenu = null; }

  async function startScrub(name) {
    openMenu = null;
    // Marca activity inmediatamente (feedback visual al momento)
    setActivity(name, { action: 'scrub', state: 'running', progress: 0 });
    try {
      const r = await fetch('/api/storage/scrub', { method:'POST', headers:{...hdrs(),'Content-Type':'application/json'}, body:JSON.stringify({pool:name}) });
      const d = await r.json();
      if (!d.ok) {
        completeActivity(name, false, d.error || 'No se pudo iniciar');
        return;
      }
      // La verificación está corriendo — el poll se encargará del progress
      ensureScrubPoll();
    } catch (e) {
      completeActivity(name, false, 'Error de conexión');
    }
  }
  async function createSnapshot(name) {
    openMenu = null;
    setActivity(name, { action: 'snapshot', state: 'running' });
    try {
      const r = await fetch('/api/storage/snapshot', { method:'POST', headers:{...hdrs(),'Content-Type':'application/json'}, body:JSON.stringify({pool:name}) });
      const d = await r.json();
      completeActivity(name, !!d.ok, d.ok ? 'Snapshot creado' : (d.error || 'Error al crear snapshot'));
    } catch (e) {
      completeActivity(name, false, 'Error de conexión');
    }
  }

  async function exportPool(pool) {
    openMenu = null;
    showDialog({
      variant: 'warning',
      title: `¿Desmontar "${pool.name}"?`,
      message: 'Los datos se conservan en los discos. Podrás re-importarlo desde "Restaurar volumen".',
      confirmText: 'Desmontar',
      onConfirm: async () => {
        dlg = { ...dlg, loading: true };
        setActivity(pool.name, { action: 'export', state: 'running' });
        try {
          const d = await (await fetch('/api/storage/pool/export', { method:'POST', headers:{...hdrs(),'Content-Type':'application/json'}, body:JSON.stringify({name:pool.name}) })).json();
          if (d.ok) {
            // Al desmontar, el pool desaparecerá del array tras load() — la activity se va con él
            closeDialog();
            clearActivity(pool.name);
            await load();
          }
          else if (d.error === 'services_active') { closeDialog(); completeActivity(pool.name, false, 'Detén los servicios primero: ' + (d.services?.join(', ')||'')); }
          else { closeDialog(); completeActivity(pool.name, false, d.error || 'Error al desmontar'); }
        } catch(e) { closeDialog(); completeActivity(pool.name, false, 'Error: ' + e.message); }
      }
    });
  }

  async function openDestroyModal(pool) {
    openMenu = null;
    restoreMenu = null;
    destroyPool = pool;
    destroyInput = ''; destroying = false; stoppingService = {};
    // For mounted pools, check service dependencies
    if (pools.find(p => p.name === pool.name)) {
      try {
        const r = await fetch(`/api/services/dependencies?pool=${encodeURIComponent(pool.name)}`, { headers:hdrs() });
        destroyDeps = (await r.json()).dependencies || [];
      } catch { destroyDeps = []; }
    } else {
      // Exported pool — no running services
      destroyDeps = [];
    }
    showDestroy = true;
  }
  async function stopSvcForDestroy(svc) {
    stoppingService = {...stoppingService,[svc.id]:true};
    try { await fetch(`/api/services/${svc.id}/stop`,{method:'POST',headers:hdrs()}); svc.status='stopped'; destroyDeps=[...destroyDeps]; } catch {}
    stoppingService = {...stoppingService,[svc.id]:false};
  }
  async function doDestroy() {
    if (!canDestroy||!destroyPool) return;
    destroying = true;
    try {
      // If pool is exported (not in active pools), restore it first so destroy can find it
      const isMounted = pools.find(p => p.name === destroyPool.name);
      if (!isMounted && destroyPool.zpoolName) {
        await fetch('/api/storage/pool/restore', { method:'POST', headers:{...hdrs(),'Content-Type':'application/json'}, body:JSON.stringify({ zpoolName:destroyPool.zpoolName, name:destroyPool.name, restoreConfig:false }) });
      }
      const d = await (await fetch('/api/storage/pool/destroy',{method:'POST',headers:{...hdrs(),'Content-Type':'application/json'},body:JSON.stringify({name:destroyPool.name})})).json();
      if (d.ok) { showDestroy=false; destroyPool=null; await load(); } else showToast(d.error||'Error', 'warning');
    } catch(e) { showToast('Error: '+e.message, 'warning'); }
    destroying = false;
  }

  // Create pool
  function toggleDisk(path) { newPool.disks = newPool.disks.includes(path) ? newPool.disks.filter(p=>p!==path) : [...newPool.disks,path]; }
  async function createPool() {
    if (!newPool.name.trim()) { poolMsg='Introduce un nombre'; return; }
    if (newPool.disks.length===0) { poolMsg='Selecciona al menos un disco'; return; }
    creating=true; poolMsg='';
    const body = { name:newPool.name.trim(), type:newPool.type, disks:newPool.disks };
    if (newPool.type==='zfs') body.vdevType=newPool.profile;
    else if (newPool.type==='btrfs') body.profile=newPool.profile;
    try {
      const d = await (await fetch('/api/storage/pool',{method:'POST',headers:{...hdrs(),'Content-Type':'application/json'},body:JSON.stringify(body)})).json();
      if (d.ok) { newPool={name:'',type:capabilities.recommended||'zfs',profile:'raidz1',disks:[]}; showCreatePool=false; active='resumen'; await load(); }
      else poolMsg = d.error||'Error';
    } catch { poolMsg='Error de conexión'; }
    creating=false;
  }
  const zfsProfiles = [
    {id:'single',label:'Simple (sin protección)',min:1},
    {id:'mirror',label:'Espejo (mirror)',min:2},
    {id:'raidz1',label:'RAIDZ1 (puede perder 1)',min:3},
    {id:'raidz2',label:'RAIDZ2 (puede perder 2)',min:5},
  ];

  // Filter out disks that belong to restorable (exported) pools
  $: restorableDisks = new Set(restorable.flatMap(r => (r.identity?.disks || []).map(d => d.replace('/dev/',''))));
  $: filteredEligible = eligible.filter(d => !restorableDisks.has(d.name));

  let refreshInterval;
  onMount(()=>{ load(); refreshInterval=setInterval(load,30000); });
  onDestroy(()=>{
    if(refreshInterval) clearInterval(refreshInterval);
    if(scrubPollInterval) clearInterval(scrubPollInterval);
  });

  // Restore
  let restorable = [];
  let scanning = false;
  let restoring = null;
  let restoreMenu = null;

  async function scanRestorable() {
    scanning = true;
    try {
      const d = await (await fetch('/api/storage/restorable', { headers:hdrs() })).json();
      restorable = d.pools || [];
    } catch { restorable = []; }
    scanning = false;
  }

  async function restorePool(pool) {
    restoreMenu = null;
    restoring = pool.name;
    try {
      const d = await (await fetch('/api/storage/pool/restore', { method:'POST', headers:{...hdrs(),'Content-Type':'application/json'}, body:JSON.stringify({ zpoolName:pool.zpoolName, name:pool.name, restoreConfig:pool.hasBackup }) })).json();
      if (d.ok) {
        restorable = restorable.filter(p => p.name !== pool.name);
        await load();
      } else showToast(d.error || 'Error restaurando', 'warning');
    } catch(e) { showToast('Error: '+e.message, 'warning'); }
    restoring = null;
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<AppShell title="Almacenamiento" {appIcon} {sections} bind:active showSearch>

  {#if loading && pools.length===0}
    <div class="state-msg">Cargando...</div>

  {:else if active === 'resumen'}

    <!-- Status hero -->
    {#if pools.length > 0}
      <Card>
        <StatusHero status={sysStatus} title={sysTitle} subtitle={sysSub}
          stats={[{label:'Volúmenes',value:String(pools.length)},{label:'Discos',value:String(totalDisks)}]} />
        {#if problemDisks.length > 0}
          <div class="hero-disks">
            <SectionLabel>Discos con incidencias</SectionLabel>
            {#each problemDisks as disk}
              <div class="hdisk">
                <span class="hdisk-name">{disk.model||disk.name}</span>
                <span class="hdisk-dev">/dev/{disk.name}</span>
                <Badge status={diskStatus(disk)}>{diskStatusLabel(disk)}</Badge>
              </div>
            {/each}
          </div>
        {/if}
      </Card>
      <div style="height:14px"></div>
    {/if}

    <!-- View toggle (siempre visible cuando hay pools) -->
{#if pools.length > 0}
  <div class="view-toggle-row">
    <ViewToggle value={viewMode} on:change={onViewModeChange} />
  </div>
{/if}

    <!-- Pool cards -->
    <div class="pools-container" class:grid={viewMode === 'compact'}>
      {#each pools as pool (pool.name)}
        <PoolCard
          {pool}
          fileStats={poolFileStats[pool.name]}
          variant={viewMode}
          activity={poolActivity[pool.name]}
          on:snapshot={(e) => createSnapshot(e.detail.name)}
          on:scrub={(e) => startScrub(e.detail.name)}
          on:addDisk={() => showToast('Añadir disco próximamente', 'default')}
          on:menu={(e) => toggleMenu(e.detail.pool.name, e.detail.event)}
        />
    
        {#if openMenu === pool.name}
          <div class="pool-menu" on:click|stopPropagation>
            <button on:click={() => createSnapshot(pool.name)}>Crear snapshot</button>
            <button on:click={() => startScrub(pool.name)}>Verificar integridad</button>
            <button on:click={() => exportPool(pool)}>Desmontar volumen</button>
            <button class="destructive" on:click={() => openDestroyModal(pool)}>Destruir volumen</button>
          </div>
        {/if}
      {/each}
    </div>

    {#if pools.length === 0}
      <Card><div class="state-msg"><div style="font-size:18px;font-weight:600;color:var(--text-primary);margin-bottom:8px">Sin volúmenes</div><div>Ve a Discos para crear uno.</div></div></Card>
    {/if}

  {:else if active === 'disks'}
    {#if showCreatePool}
      <Card>
        <div class="create-header">
          <div class="back-btn" on:click={()=>showCreatePool=false}><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" width="14" height="14"><polyline points="15 18 9 12 15 6"/></svg>Volver</div>
          <span style="font-size:18px;font-weight:600">Crear volumen</span>
        </div>
        <div class="create-form">
          <div class="fg"><label class="fl">Nombre</label><input class="fi" bind:value={newPool.name} placeholder="mi-volumen"></div>
          <div class="fg"><label class="fl">Sistema de archivos</label>
            <div class="fo">
              {#if capabilities.zfs}<div class="fopt" class:sel={newPool.type==='zfs'} on:click={()=>{newPool.type='zfs';newPool.profile='raidz1'}}><b>ZFS</b><span>Recomendado</span></div>{/if}
              {#if capabilities.btrfs}<div class="fopt" class:sel={newPool.type==='btrfs'} on:click={()=>{newPool.type='btrfs';newPool.profile='single'}}><b>BTRFS</b><span>Flexible</span></div>{/if}
            </div>
          </div>
          {#if newPool.type==='zfs'}
            <div class="fg"><label class="fl">Protección</label>
              <div class="fo">{#each zfsProfiles as p}<div class="fopt" class:sel={newPool.profile===p.id} class:dis={eligible.length<p.min} on:click={()=>{if(eligible.length>=p.min) newPool.profile=p.id}}><b>{p.label}</b><span>Mín {p.min} disco{p.min>1?'s':''}</span></div>{/each}</div>
            </div>
          {/if}
          <div class="fg"><label class="fl">Discos ({newPool.disks.length})</label>
            {#each eligible as disk}
              <div class="dsel" class:sel={newPool.disks.includes('/dev/'+disk.name)} on:click={()=>toggleDisk('/dev/'+disk.name)}>
                <div class="dchk">{newPool.disks.includes('/dev/'+disk.name)?'✓':''}</div>
                <div class="disk-icon-mini"><svg viewBox="0 0 24 24" fill="currentColor"><path d="M18.84 13.38c1.13 0 2.14.45 2.9 1.18L19.37 5.18C18.84 3.54 17.9 3 16.74 3H7.26C6.1 3 5.16 3.54 4.63 5.18L2.27 14.56c.75-.73 1.76-1.18 2.89-1.18z"/><path d="M5.16 14.4C4 14.4 2.96 15.07 2.41 16.08c-.26.48-.41 1.03-.41 1.62C2 19.55 3.44 21 5.16 21h13.68c1.72 0 3.16-1.45 3.16-3.3 0-.59-.15-1.14-.41-1.62-.55-1.01-1.58-1.68-2.75-1.68z"/></svg></div>
                <div style="flex:1"><div class="disk-name">{disk.model||disk.name}</div><div class="disk-cell">/dev/{disk.name} · {disk.size||'—'}</div></div>
              </div>
            {/each}
            {#if eligible.length===0}<div class="state-msg">No hay discos disponibles</div>{/if}
          </div>
          {#if poolMsg}<div style="color:var(--c-crit);font-size:13px">{poolMsg}</div>{/if}
          <div style="display:flex;gap:10px;margin-top:8px">
            <Button on:click={()=>showCreatePool=false}>Cancelar</Button>
            <Button variant="primary" disabled={creating} on:click={createPool}>{creating?'Creando...':'Crear volumen'}</Button>
          </div>
        </div>
      </Card>
    {:else}
      <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:14px">
        <SectionLabel>Discos disponibles</SectionLabel>
        <Button variant="primary" size="sm" on:click={()=>showCreatePool=true}>+ Crear volumen</Button>
      </div>
      {#if filteredEligible.length > 0}
        <Card>
          <div class="disk-head"><div></div><div>Modelo</div><div>Dispositivo</div><div>Capacidad</div><div>Temp</div><div>Horas</div><div>Rol</div><div>Estado</div></div>
          {#each filteredEligible as disk}
            <div class="disk-row">
              <div class="disk-icon-mini"><svg viewBox="0 0 24 24" fill="currentColor"><path d="M18.84 13.38c1.13 0 2.14.45 2.9 1.18L19.37 5.18C18.84 3.54 17.9 3 16.74 3H7.26C6.1 3 5.16 3.54 4.63 5.18L2.27 14.56c.75-.73 1.76-1.18 2.89-1.18z"/><path d="M5.16 14.4C4 14.4 2.96 15.07 2.41 16.08c-.26.48-.41 1.03-.41 1.62C2 19.55 3.44 21 5.16 21h13.68c1.72 0 3.16-1.45 3.16-3.3 0-.59-.15-1.14-.41-1.62-.55-1.01-1.58-1.68-2.75-1.68z"/></svg></div>
              <div class="disk-name">{disk.model||disk.name}</div>
              <div class="disk-cell">/dev/{disk.name}</div>
              <div class="disk-cell">{disk.size||'—'}</div>
              <div class="disk-cell">{disk.smart?.temperature?disk.smart.temperature+'°C':'—'}</div>
              <div class="disk-cell">{fmtH(disk.smart?.powerOnHours)}</div>
              <div><span class="role-pill">libre</span></div>
              <div class="smart-badge smart-{diskStatus(disk)}"><span class="dot"></span>{diskStatusLabel(disk)}</div>
            </div>
          {/each}
        </Card>
      {:else}
        <Card><div class="state-msg">Todos los discos están en uso.</div></Card>
      {/if}

      <!-- Restaurar volumen section -->
      <div style="height:24px"></div>
      <div class="restore-head">
        <SectionLabel>Restaurar volumen</SectionLabel>
        <button class="btn-scan" on:click={scanRestorable} disabled={scanning}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 12a9 9 0 1 0 3-6.7"/><polyline points="3 4 3 10 9 10"/></svg>
          {scanning ? 'Escaneando...' : 'Escanear discos'}
        </button>
      </div>

      {#if restorable.length > 0}
        <Card>
          <div class="card-label">Pools detectados · pendientes de montar</div>
          {#each restorable as rpool}
            <div class="pool-item">
              <div class="pool-info">
                <div class="pool-item-name">
                  {rpool.name}
                  {#if rpool.health === 'ONLINE' || rpool.state === 'ONLINE'}
                    <span class="status-pill status-ok"><span class="dot"></span>Íntegro</span>
                  {:else}
                    <span class="status-pill status-warn"><span class="dot"></span>{rpool.health || rpool.state || 'Desconocido'}</span>
                  {/if}
                </div>
                <div class="pool-meta">{rpool.type?.toUpperCase() || 'ZFS'}{rpool.vdevType ? ' · '+vdevLabel(rpool.vdevType) : ''} · {rpool.identity?.disks?.length || '?'} discos{rpool.size ? ' · '+rpool.size : ''}{rpool.hasBackup ? ' · Backup disponible' : ''}</div>
                <div class="pool-disks-chips">
                  {#each (rpool.identity?.disks || []) as d}
                    <div class="disk-chip">
                      <svg viewBox="0 0 24 24" fill="currentColor" width="11" height="11"><path d="M18.84 13.38c1.13 0 2.14.45 2.9 1.18L19.37 5.18C18.84 3.54 17.9 3 16.74 3H7.26C6.1 3 5.16 3.54 4.63 5.18L2.27 14.56c.75-.73 1.76-1.18 2.89-1.18z"/><path d="M5.16 14.4C4 14.4 2.96 15.07 2.41 16.08c-.26.48-.41 1.03-.41 1.62C2 19.55 3.44 21 5.16 21h13.68c1.72 0 3.16-1.45 3.16-3.3 0-.59-.15-1.14-.41-1.62-.55-1.01-1.58-1.68-2.75-1.68z"/></svg>
                      {d}
                    </div>
                  {/each}
                </div>
              </div>
              <div class="pool-item-actions">
                <div class="kebab" on:click|stopPropagation={()=> restoreMenu = restoreMenu===rpool.name ? null : rpool.name}>
                  <svg viewBox="0 0 24 24" fill="currentColor"><circle cx="12" cy="5" r="1.8"/><circle cx="12" cy="12" r="1.8"/><circle cx="12" cy="19" r="1.8"/></svg>
                </div>
                {#if restoreMenu === rpool.name}
                  <div class="menu" style="right:0;top:40px">
                    <div class="menu-item" on:click={()=>restorePool(rpool)}>
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v12"/><polyline points="7 8 12 3 17 8"/><path d="M5 21h14"/></svg>
                      {restoring === rpool.name ? 'Montando...' : 'Montar pool'}
                    </div>
                    <div class="menu-divider"></div>
                    <div class="menu-item danger" on:click={()=>openDestroyModal(rpool)}>
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 3h6l1 3h4v2H4V6h4z"/><path d="M6 8v12a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2V8"/></svg>
                      Destruir volumen
                    </div>
                  </div>
                {/if}
              </div>
            </div>
          {/each}
        </Card>
      {:else if scanning}
        <Card><div class="state-msg">Buscando pools en los discos...</div></Card>
      {/if}
    {/if}

  {:else if active === 'snapshots'}
    <Card><SectionLabel>Puntos de restauración</SectionLabel><div class="state-msg">En desarrollo</div></Card>
  {:else if active === 'health'}
    <Card><SectionLabel>Salud del sistema</SectionLabel><div class="state-msg">En desarrollo</div></Card>
  {/if}
</AppShell>

<!-- Destroy modal -->
{#if showDestroy && destroyPool}
  <div class="modal-overlay" on:click|self={()=>showDestroy=false}>
    <div class="modal">
      <div class="modal-header"><span class="modal-title">Destruir {destroyPool.name}</span><span class="modal-close" on:click={()=>showDestroy=false}>✕</span></div>
      <div class="modal-body">
        <div class="destroy-warn">Esta acción eliminará permanentemente todos los datos del volumen.</div>
        {#if destroyDeps.length>0}
          <div class="msec">Servicios dependientes</div>
          {#each destroyDeps as dep}
            <div class="dep-item">
              <span class="dep-dot" style="background:{dep.status==='running'?'var(--c-ok)':'var(--text-muted)'}"></span>
              <span class="dep-name">{dep.app||dep.appId}</span>
              <span class="dep-status">{dep.status==='running'?'activo':dep.status}</span>
              {#if dep.status==='running'||dep.status==='starting'}
                <button class="dep-stop" disabled={stoppingService[dep.id]} on:click={()=>stopSvcForDestroy(dep)}>{stoppingService[dep.id]?'Deteniendo...':'Detener'}</button>
              {:else}<span class="dep-stopped">Detenido</span>{/if}
            </div>
          {/each}
          {#if !allDepsStopped}<div class="dep-hint">Detén todos los servicios primero.</div>{/if}
        {/if}
        <div class="msec" style="margin-top:16px">Confirmar</div>
        <div class="dep-hint" style="margin-bottom:6px">Escribe <strong style="color:var(--c-crit)">ELIMINAR</strong>:</div>
        <input class="confirm-input" bind:value={destroyInput} placeholder="ELIMINAR">
      </div>
      <div class="modal-footer">
        <Button on:click={()=>showDestroy=false}>Cancelar</Button>
        <Button variant="danger" disabled={!canDestroy||destroying} on:click={doDestroy}>{destroying?'Destruyendo...':'Destruir'}</Button>
      </div>
    </div>
  </div>
{/if}

<!-- Generic ConfirmDialog -->
<ConfirmDialog
  open={dlg.open}
  variant={dlg.variant}
  title={dlg.title}
  message={dlg.message}
  confirmText={dlg.confirmText}
  cancelText={dlg.cancelText}
  loading={dlg.loading}
  services={dlg.services}
  requireInput={dlg.requireInput}
  on:confirm={dlg.onConfirm}
  on:cancel={closeDialog}
/>

<!-- Toast -->
{#if toast.show}
  <div class="nimos-toast" class:warn={toast.variant==='warning'} class:err={toast.variant==='error'}>
    {toast.message}
  </div>
{/if}

<style>
  .state-msg{font-size:13px;color:var(--text-muted);padding:20px 0;text-align:center}

  /* ── Hero disks ── */
  .hero-disks{margin-top:18px;padding-top:16px;border-top:1px solid var(--glass-border)}
  .hdisk{display:flex;align-items:center;gap:12px;padding:6px 0;font-size:12px}
  .hdisk-name{font-weight:500;color:var(--text-primary)}
  .hdisk-dev{font-family:var(--font-mono);color:var(--text-muted);font-size:11px}

  /* ── Pool pills — from prototype ── */
  .pool-list{display:flex;flex-direction:column;gap:8px}
  .pool-pill{
    display:grid;grid-template-columns:36px 1.5fr 2.4fr 80px 28px 28px;
    align-items:center;gap:18px;padding:14px 20px 10px;
    background:var(--glass-bg);border:1px solid var(--glass-border);
    border-radius:12px;transition:background .18s;position:relative;
  }
  .pool-pill:hover{background:var(--bg-elev-2)}
  .pool-pill.open{grid-template-columns:1fr;padding:0;gap:0;background:var(--bg-elev-2)}
  .pool-pill.open .pool-head{
    display:grid;grid-template-columns:36px 1.5fr 2.4fr 80px 28px 28px;
    align-items:center;gap:18px;padding:14px 20px 10px;
  }
  .pool-icon{width:36px;height:36px;border-radius:8px;display:flex;align-items:center;justify-content:center;background:var(--bg-elev-2);border:1px solid var(--glass-border);color:var(--text-primary)}
  .pool-icon svg{width:20px;height:20px;display:block}
  .pool-ident{min-width:0;cursor:pointer}
  .pill-name{font-size:16px;font-weight:700;letter-spacing:-0.3px;line-height:1.2;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .pill-sub{font-size:11px;color:var(--text-muted);margin-top:3px;font-family:var(--font-mono);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}

  .bar{position:relative;height:7px;border-radius:4px;background:var(--bg-elev-2);border:1px solid var(--glass-border);margin-top:9px;cursor:pointer}
  .bar-fill{height:100%;border-radius:4px;background:var(--accent);transition:width .3s}
  .bar-fill.warn{background:var(--c-warn)}.bar-fill.crit{background:var(--c-crit)}
  .bar-pct{position:absolute;bottom:100%;transform:translateX(-50%);margin-bottom:3px;font-family:var(--font-mono);font-size:13px;font-weight:700;letter-spacing:-0.3px;color:var(--text-primary);line-height:1;white-space:nowrap;pointer-events:none}
  .bar-pct.warn{color:var(--c-warn)}.bar-pct.crit{color:var(--c-crit)}
  .bar-pct .sym{font-size:10px;font-weight:600;opacity:0.7}
  .cap-total{font-family:var(--font-mono);font-size:12px;color:var(--text-secondary);text-align:right;white-space:nowrap}
  .chev{width:28px;height:28px;border-radius:6px;display:flex;align-items:center;justify-content:center;color:var(--text-muted);cursor:pointer;transition:all .15s;justify-self:end}
  .chev:hover{background:var(--bg-elev-1);color:var(--text-primary)}
  .chev svg{width:13px;height:13px;transition:transform .25s}
  .pool-pill.open .chev svg{transform:rotate(90deg)}.pool-pill.open .chev{color:var(--text-primary)}
  .kebab{width:30px;height:30px;border-radius:7px;display:flex;align-items:center;justify-content:center;color:var(--text-muted);cursor:pointer;transition:all .15s;justify-self:end}
  .kebab:hover{background:var(--bg-elev-1);color:var(--text-primary)}
  .kebab svg{width:15px;height:15px}

  /* Menu */
  .menu{position:absolute;right:18px;top:58px;z-index:1000;background:var(--bg-elev-1);border:1px solid var(--glass-border);border-radius:10px;padding:5px;min-width:220px;box-shadow:0 10px 30px rgba(0,0,0,0.5)}
  .menu-item{display:flex;align-items:center;gap:11px;padding:9px 12px;border-radius:6px;font-size:12px;color:var(--text-primary);cursor:pointer;transition:background .1s}
  .menu-item:hover{background:var(--bg-elev-2)}
  .menu-item svg{width:14px;height:14px;color:var(--text-secondary);flex-shrink:0}
  .menu-item.danger{color:var(--c-crit)}.menu-item.danger svg{color:var(--c-crit)}
  .menu-divider{height:1px;background:var(--glass-border);margin:4px 0}

  /* Expanded body */
  .pool-body{border-top:1px solid var(--glass-border);padding:20px 24px;display:grid;grid-template-columns:1fr auto;gap:24px;align-items:center;animation:fadeUp .3s ease both}
  .legend{display:grid;grid-auto-flow:column;grid-template-rows:repeat(3,auto);gap:8px 24px}
  .legend-row{display:flex;align-items:center;gap:9px;font-size:12px;color:var(--text-primary);white-space:nowrap}
  .legend-val{margin-left:auto;font-family:var(--font-mono);color:var(--text-muted);font-size:11px}
  .legend-dot{width:9px;height:9px;border-radius:50%;flex-shrink:0}
  .donut{position:relative;width:120px;height:120px;flex-shrink:0}
  .donut svg{transform:rotate(-90deg)}
  .donut-center{position:absolute;inset:0;display:flex;flex-direction:column;align-items:center;justify-content:center}
  .dv{font-size:17px;font-weight:600;font-family:var(--font-mono)}
  .dl{font-size:9px;color:var(--text-muted);margin-top:2px;text-transform:uppercase;letter-spacing:0.5px}
  @keyframes fadeUp{from{opacity:0;transform:translateY(6px)}to{opacity:1;transform:translateY(0)}}

  /* Disk table */
  .pool-disks{border-top:1px solid var(--glass-border);padding:18px 24px 20px}
  .disks-label{font-size:10px;color:var(--text-muted);text-transform:uppercase;letter-spacing:1.2px;margin-bottom:10px}
  .disk-head,.disk-row{display:grid;grid-template-columns:36px 1.4fr 1fr 0.7fr 0.55fr 0.65fr 0.55fr 100px;gap:12px;padding:0 10px}
  .disk-head{padding-bottom:8px;font-size:10px;color:var(--text-muted);text-transform:uppercase;letter-spacing:0.8px;align-items:center}
  .disk-row{align-items:center;padding-top:9px;padding-bottom:9px;border-radius:8px;font-size:12px;transition:background .15s}
  .disk-row+.disk-row{margin-top:2px}
  .disk-row:hover{background:var(--bg-elev-1)}
  .disk-icon-mini{width:34px;height:34px;border-radius:7px;background:var(--bg-elev-1);border:1px solid var(--glass-border);display:flex;align-items:center;justify-content:center;color:var(--text-secondary)}
  .disk-icon-mini svg{width:20px;height:20px}
  .disk-name{font-weight:500;color:var(--text-primary);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .disk-cell{font-family:var(--font-mono);color:var(--text-secondary);white-space:nowrap}
  .role-pill{display:inline-block;font-size:10px;padding:3px 7px;border-radius:5px;background:var(--bg-elev-1);color:var(--text-secondary);font-family:var(--font-mono);text-transform:uppercase;letter-spacing:0.5px}
  .smart-badge{display:inline-flex;align-items:center;gap:6px;font-size:11px;font-weight:500;padding:4px 10px;border-radius:13px}
  .smart-badge .dot{width:6px;height:6px;border-radius:50%}
  .smart-ok{background:rgba(16,185,129,0.12);color:var(--c-ok);border:1px solid rgba(16,185,129,0.3)}.smart-ok .dot{background:var(--c-ok)}
  .smart-warn{background:rgba(245,158,11,0.12);color:var(--c-warn);border:1px solid rgba(245,158,11,0.3)}.smart-warn .dot{background:var(--c-warn)}
  .smart-crit{background:rgba(239,68,68,0.12);color:var(--c-crit);border:1px solid rgba(239,68,68,0.3)}.smart-crit .dot{background:var(--c-crit)}
  .smart-unknown{background:var(--bg-elev-2);color:var(--text-muted);border:1px solid var(--glass-border)}.smart-unknown .dot{background:var(--text-muted)}

  /* Create pool */
  .create-header{display:flex;align-items:center;gap:16px;margin-bottom:20px}
  .back-btn{display:inline-flex;align-items:center;gap:6px;font-size:13px;color:var(--text-secondary);cursor:pointer;padding:6px 10px;border-radius:6px;transition:all .15s}
  .back-btn:hover{color:var(--text-primary);background:var(--bg-elev-2)}
  .create-form{display:flex;flex-direction:column;gap:18px}
  .fg{display:flex;flex-direction:column;gap:8px}
  .fl{font-size:10px;color:var(--text-muted);text-transform:uppercase;letter-spacing:1.2px;font-weight:500}
  .fi{padding:10px 14px;border-radius:8px;border:1px solid var(--glass-border);background:var(--bg-elev-2);color:var(--text-primary);font-family:var(--font-sans);font-size:14px;outline:none;max-width:400px}
  .fi:focus{border-color:var(--accent)}
  .fo{display:flex;flex-wrap:wrap;gap:8px}
  .fopt{padding:12px 16px;border-radius:8px;border:1px solid var(--glass-border);background:var(--bg-elev-2);cursor:pointer;transition:all .15s;min-width:160px}
  .fopt:hover{border-color:var(--accent)}.fopt.sel{border-color:var(--accent);background:rgba(59,130,246,0.1)}.fopt.dis{opacity:0.35;cursor:not-allowed}
  .fopt b{display:block;font-size:13px;color:var(--text-primary)}.fopt span{display:block;font-size:11px;color:var(--text-muted);margin-top:2px}
  .dsel{display:flex;align-items:center;gap:12px;padding:10px 14px;border-radius:8px;border:1px solid var(--glass-border);background:var(--bg-elev-2);cursor:pointer;transition:all .15s}
  .dsel:hover{border-color:var(--accent)}.dsel.sel{border-color:var(--accent);background:rgba(59,130,246,0.1)}.dsel+.dsel{margin-top:6px}
  .dchk{width:22px;height:22px;border-radius:6px;border:2px solid var(--glass-border);background:var(--bg-app);display:flex;align-items:center;justify-content:center;font-size:13px;font-weight:700;color:var(--accent);flex-shrink:0}
  .dsel.sel .dchk{border-color:var(--accent);background:rgba(59,130,246,0.1)}

  /* Destroy modal */
  .modal-overlay{position:fixed;inset:0;z-index:9999;background:rgba(0,0,0,0.6);display:flex;align-items:center;justify-content:center}
  .modal{background:var(--bg-elev-1);border:1px solid var(--glass-border);border-radius:14px;width:520px;max-width:90vw;box-shadow:0 20px 60px rgba(0,0,0,0.5)}
  .modal-header{display:flex;justify-content:space-between;align-items:center;padding:18px 22px;border-bottom:1px solid var(--glass-border)}
  .modal-title{font-size:16px;font-weight:600;color:var(--text-primary)}
  .modal-close{font-size:18px;color:var(--text-muted);cursor:pointer;padding:4px 8px;border-radius:4px}.modal-close:hover{color:var(--text-primary);background:var(--bg-elev-2)}
  .modal-body{padding:20px 22px}
  .modal-footer{display:flex;justify-content:flex-end;gap:10px;padding:16px 22px;border-top:1px solid var(--glass-border)}
  .destroy-warn{font-size:13px;color:var(--c-crit);background:rgba(239,68,68,0.08);border:1px solid rgba(239,68,68,0.25);border-radius:10px;padding:14px 16px;line-height:1.5}
  .msec{font-size:10px;color:var(--text-muted);text-transform:uppercase;letter-spacing:1.2px;margin:12px 0 8px}
  .dep-item{display:flex;align-items:center;gap:10px;padding:8px 12px;border-radius:6px;font-size:13px;background:var(--bg-elev-2);margin-bottom:4px}
  .dep-dot{width:8px;height:8px;border-radius:50%;flex-shrink:0}
  .dep-name{font-weight:500;flex:1}.dep-status{font-size:11px;color:var(--text-muted);font-family:var(--font-mono)}
  .dep-stop{font-family:var(--font-sans);font-size:11px;font-weight:500;padding:4px 10px;border-radius:5px;cursor:pointer;border:1px solid rgba(245,158,11,0.3);background:rgba(245,158,11,0.1);color:var(--c-warn)}.dep-stop:disabled{opacity:0.5;cursor:not-allowed}
  .dep-stopped{font-size:11px;color:var(--text-muted);font-style:italic}
  .dep-hint{font-size:11px;color:var(--text-muted)}
  .confirm-input{width:100%;padding:10px 14px;border-radius:8px;border:1px solid var(--glass-border);background:var(--bg-elev-2);color:var(--text-primary);font-family:var(--font-mono);font-size:14px;outline:none}.confirm-input:focus{border-color:var(--c-crit)}

  /* Restore section */
  .restore-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:14px}
  .btn-scan{
    font-family:var(--font-sans);font-size:13px;font-weight:500;
    padding:10px 16px;border-radius:9px;cursor:pointer;
    background:var(--bg-elev-2);border:1px solid var(--glass-border);color:var(--text-primary);
    display:inline-flex;align-items:center;gap:8px;transition:all .15s;
  }
  .btn-scan:hover{background:var(--bg-elev-1)}
  .btn-scan:disabled{opacity:0.5;cursor:not-allowed}
  .btn-scan svg{width:14px;height:14px;color:var(--text-secondary)}
  .card-label{font-size:10px;color:var(--text-muted);text-transform:uppercase;letter-spacing:1.2px;margin-bottom:16px}
  .pool-item{
    display:grid;grid-template-columns:1fr auto;gap:20px;
    padding:16px;border-radius:10px;
    border:1px solid var(--glass-border);background:var(--bg-elev-2);
    align-items:center;position:relative;
  }
  .pool-item+.pool-item{margin-top:10px}
  .pool-info{min-width:0}
  .pool-item-name{font-size:17px;font-weight:700;letter-spacing:-0.3px;margin-bottom:4px;display:flex;align-items:center;gap:8px}
  .pool-meta{font-size:12px;color:var(--text-secondary);margin-bottom:4px}
  .pool-disks-chips{display:flex;flex-wrap:wrap;gap:6px;margin-top:10px}
  .disk-chip{
    display:inline-flex;align-items:center;gap:7px;
    font-size:11px;padding:4px 10px;border-radius:6px;
    background:var(--bg-elev-1);border:1px solid var(--glass-border);
    color:var(--text-secondary);font-family:var(--font-mono);
  }
  .disk-chip svg{width:11px;height:11px;color:var(--text-muted)}
  .pool-item-actions{position:relative}
  .status-pill{display:inline-flex;align-items:center;gap:7px;font-size:11px;font-weight:500;padding:4px 11px;border-radius:13px}
  .status-pill .dot{width:6px;height:6px;border-radius:50%}
  .status-ok{background:rgba(16,185,129,0.12);color:var(--c-ok);border:1px solid rgba(16,185,129,0.3)}.status-ok .dot{background:var(--c-ok)}
  .status-warn{background:rgba(245,158,11,0.12);color:var(--c-warn);border:1px solid rgba(245,158,11,0.3)}.status-warn .dot{background:var(--c-warn)}

  /* Toast */
  .nimos-toast{
    position:fixed;bottom:24px;left:50%;transform:translateX(-50%);z-index:10001;
    padding:10px 20px;border-radius:10px;font-size:13px;font-weight:500;
    background:var(--bg-elev-1);border:1px solid var(--glass-border);
    color:var(--text-primary);box-shadow:0 8px 24px rgba(0,0,0,0.4);
    animation:toastIn .25s ease both;pointer-events:none;
  }
  .nimos-toast.warn{border-color:var(--c-warn-border);color:var(--c-warn)}
  .nimos-toast.err{border-color:var(--c-crit-border);color:var(--c-crit)}
  @keyframes toastIn{from{opacity:0;transform:translateX(-50%) translateY(10px)}to{opacity:1;transform:translateX(-50%) translateY(0)}}

  /* ── Pools container (fluido, pegado al sidebar) ── */
.view-toggle-row {
  margin: 0 0 14px;
  display: flex;
  justify-content: flex-end;
}
.pools-container {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 700px), 1fr));
  gap: 14px;
}
.pools-container.grid {
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 340px), 1fr));
}

/* Kebab menu (reutiliza el comportamiento existente) */
.pool-menu {
  position: absolute;
  right: 28px;
  margin-top: 4px;
  background: var(--bg-elev-1);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-md);
  padding: 6px;
  min-width: 200px;
  box-shadow: 0 8px 24px rgba(0,0,0,0.4);
  z-index: 100;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.pool-menu button {
  background: transparent;
  border: none;
  color: var(--text-primary);
  font-family: var(--font-sans);
  font-size: 12px;
  padding: 8px 12px;
  border-radius: 6px;
  text-align: left;
  cursor: pointer;
  transition: background 0.12s;
}
.pool-menu button:hover { background: var(--bg-elev-2); }
.pool-menu button.destructive { color: var(--c-crit); }
.pool-menu button.destructive:hover { background: var(--c-crit-dim); }
</style>
