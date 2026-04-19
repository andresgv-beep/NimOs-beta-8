<script>
  /**
   * NetworkApp · Remote Access (v3 · Fase A)
   * ──────────────────────────────────────────────────
   * Migración desde Beta 7 NetworkPanel.svelte (1109 líneas).
   *
   * Scope Fase A: solo Remote Access para acceso HTTPS desde fuera.
   *   - DuckDNS (configuración + estado)
   *   - Router / UPnP (abrir puerto 443)
   *   - SSL / HTTPS (Let's Encrypt + activar nginx HTTPS)
   *   - Setup Steps didácticos
   *
   * Backend endpoints reutilizados (sin cambios):
   *   GET    /api/ddns/status
   *   POST   /api/ddns/config
   *   POST   /api/ddns/test
   *   GET    /api/remote-access/status
   *   POST   /api/remote-access/request-ssl
   *   POST   /api/remote-access/enable-https
   *   GET    /api/router/status
   *   GET    /api/router/ports
   *   POST   /api/router/port
   *   DELETE /api/router/port
   *   POST   /api/router/test
   *
   * Fase B (pendiente): Interfaces, DNS, SMB, SSH, FTP, NFS, WebDAV, Firewall, Fail2ban.
   */
  import { onMount, onDestroy } from 'svelte';
  import { token, hdrs } from '$lib/stores/auth.js';
  import AppShell from '$lib/components/AppShell.svelte';
  import {
    KPICard, SectionHead, BevelButton, IconButton, TextInput,
    Tab, Badge, LED, EmptyState, Spinner
  } from '$lib/ui';

  // ─── State ───
  let active = 'duckdns';  // sidebar section: 'duckdns' | 'router' | 'ssl'

  // DDNS data
  let ddnsData = {};
  let ddnsForm = { provider: 'duckdns', domain: '', token: '', username: '', password: '' };
  let ddnsEditing = false;
  let ddnsSaving = false;
  let ddnsTesting = false;
  let ddnsMsg = '';
  let ddnsMsgError = false;
  let tokenVisible = false;

  // Router (UPnP)
  let routerStatus = {};
  let routerPorts = [];
  let routerLoading = false;
  let routerMsg = '';
  let routerMsgError = false;
  let newPort = '';
  let newPortProto = 'TCP';
  let newPortDesc = '';
  let routerTesting = {};

  // Certs / HTTPS
  let certData = {};
  let certEmail = '';
  let certRequesting = false;
  let certMsg = '';
  let certMsgError = false;
  let httpsSaving = false;
  let httpsPort = 443;

  // Polling
  let pollInterval;
  let loading = true;

  // ─── Derived ───
  $: httpsEnabled  = certData.https?.running || false;
  $: sslValid      = certData.ssl?.valid || false;
  $: certDomain    = ddnsData.config?.domain || certData.config?.ddns?.domain || '';
  $: externalIp    = certData.ddns?.externalIp || ddnsData.externalIp || '';
  $: localIp       = certData.localIp || '';
  $: sslExpiryDays = certData.ssl?.expiryDays || 0;

  // Port 443 status: ¿hay regla UPnP activa en el puerto 443?
  $: port443Open = (routerPorts || []).some(p => parseInt(p.port) === 443 || parseInt(p.externalPort) === 443);

  // Setup step completion
  $: stepDdnsDone = !!(ddnsData.config?.enabled && ddnsData.config?.domain);
  $: stepSslDone  = sslValid;
  $: stepPortDone = port443Open;
  $: stepHttpsDone = httpsEnabled;

  // Determinar el paso actual (primer no completado)
  $: currentStep = !stepDdnsDone ? 1 : !stepSslDone ? 2 : !stepPortDone ? 3 : !stepHttpsDone ? 4 : 5;

  $: stepsCompleted = [stepDdnsDone, stepSslDone, stepPortDone, stepHttpsDone].filter(Boolean).length;

  $: ddnsActive = ddnsData.config?.enabled;
  $: autoUpdate = ddnsData.config?.autoUpdate !== false;

  // Sync ddnsForm from loaded config
  $: if (ddnsData.config) {
    if (!ddnsForm.provider && ddnsData.config.provider) ddnsForm.provider = ddnsData.config.provider;
    if (!ddnsForm.domain && ddnsData.config.domain)     ddnsForm.domain = ddnsData.config.domain;
    if (!ddnsForm.token && ddnsData.config.token)       ddnsForm.token = ddnsData.config.token;
  }

  // ─── API calls ───
  async function loadAll() {
    try {
      const [ddns, certs, routerS, routerP] = await Promise.all([
        fetch('/api/ddns/status',           { headers: hdrs() }).then(r => r.json()).catch(() => ({})),
        fetch('/api/remote-access/status',  { headers: hdrs() }).then(r => r.json()).catch(() => ({})),
        fetch('/api/router/status',         { headers: hdrs() }).then(r => r.json()).catch(() => ({})),
        fetch('/api/router/ports',          { headers: hdrs() }).then(r => r.json()).catch(() => ({ ports: [] })),
      ]);
      ddnsData     = ddns || {};
      certData     = certs || {};
      routerStatus = routerS || {};
      routerPorts  = routerP?.ports || [];
    } catch (e) {
      console.error('[NetworkApp] loadAll failed', e);
    }
    loading = false;
  }

  // ─── DDNS handlers ───
  async function saveDdns() {
    const p = ddnsForm.provider;
    if (!p) { ddnsMsg = 'Selecciona un proveedor'; ddnsMsgError = true; return; }
    if (p === 'freedns' && !ddnsForm.token) {
      ddnsMsg = 'Introduce el update key'; ddnsMsgError = true; return;
    }
    if (p !== 'freedns' && !ddnsForm.domain) {
      ddnsMsg = 'Introduce el dominio'; ddnsMsgError = true; return;
    }
    if (p === 'noip' && (!ddnsForm.username || !ddnsForm.password)) {
      ddnsMsg = 'Introduce email y contraseña'; ddnsMsgError = true; return;
    }
    if ((p === 'duckdns' || p === 'dynu') && !ddnsForm.token) {
      ddnsMsg = 'Introduce el token'; ddnsMsgError = true; return;
    }
    ddnsSaving = true;
    ddnsMsg = '';
    try {
      const res = await fetch('/api/ddns/config', {
        method: 'POST',
        headers: { ...hdrs(), 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...ddnsForm, enabled: true }),
      });
      const data = await res.json();
      if (data.ok) {
        ddnsMsg = 'Guardado correctamente';
        ddnsMsgError = false;
        ddnsEditing = false;
        await loadAll();
      } else {
        ddnsMsg = data.error || 'Error al guardar';
        ddnsMsgError = true;
      }
    } catch {
      ddnsMsg = 'Error de conexión';
      ddnsMsgError = true;
    }
    ddnsSaving = false;
  }

  async function testDdns() {
    ddnsTesting = true;
    ddnsMsg = '';
    try {
      const res = await fetch('/api/ddns/test', {
        method: 'POST',
        headers: { ...hdrs(), 'Content-Type': 'application/json' },
        body: JSON.stringify(ddnsForm),
      });
      const data = await res.json();
      if (data.ok) {
        ddnsMsg = 'Conexión exitosa: ' + (data.result || 'OK');
        ddnsMsgError = false;
      } else {
        ddnsMsg = data.error || 'Falló la prueba';
        ddnsMsgError = true;
      }
    } catch {
      ddnsMsg = 'Error de conexión';
      ddnsMsgError = true;
    }
    ddnsTesting = false;
  }

  async function disableDdns() {
    if (!confirm('¿Desactivar DuckDNS? El dominio dejará de actualizarse.')) return;
    try {
      await fetch('/api/ddns/config', {
        method: 'POST',
        headers: { ...hdrs(), 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled: false }),
      });
      ddnsMsg = 'DDNS desactivado';
      ddnsMsgError = false;
      await loadAll();
    } catch {
      ddnsMsg = 'Error';
      ddnsMsgError = true;
    }
  }

  async function toggleAutoUpdate() {
    const current = ddnsData.config?.autoUpdate !== false;
    try {
      await fetch('/api/ddns/config', {
        method: 'POST',
        headers: { ...hdrs(), 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...ddnsData.config, autoUpdate: !current }),
      });
      await loadAll();
    } catch (e) {
      console.error('Toggle auto-update failed', e);
    }
  }

  // ─── Router (UPnP) handlers ───
  async function addPort() {
    if (!newPort) return;
    routerMsg = '';
    try {
      const res = await fetch('/api/router/port', {
        method: 'POST',
        headers: { ...hdrs(), 'Content-Type': 'application/json' },
        body: JSON.stringify({
          port: parseInt(newPort),
          protocol: newPortProto,
          description: newPortDesc || 'NimOS',
        }),
      });
      const d = await res.json();
      if (d.ok) {
        routerMsg = `Puerto ${newPort}/${newPortProto} abierto`;
        routerMsgError = false;
        newPort = '';
        newPortDesc = '';
        await loadAll();
      } else {
        routerMsg = d.error || 'Error';
        routerMsgError = true;
      }
    } catch {
      routerMsg = 'Error de conexión';
      routerMsgError = true;
    }
  }

  async function addPort443() {
    newPort = '443';
    newPortProto = 'TCP';
    newPortDesc = 'NimOS HTTPS';
    await addPort();
  }

  async function removePort(port, protocol) {
    if (!confirm(`¿Cerrar puerto ${port}/${protocol}?`)) return;
    try {
      const res = await fetch('/api/router/port', {
        method: 'DELETE',
        headers: { ...hdrs(), 'Content-Type': 'application/json' },
        body: JSON.stringify({ port: parseInt(port), protocol }),
      });
      const d = await res.json();
      if (d.ok) {
        await loadAll();
      } else {
        routerMsg = d.error || 'Error';
        routerMsgError = true;
      }
    } catch {
      routerMsg = 'Error';
      routerMsgError = true;
    }
  }

  async function testPort(port) {
    routerTesting = { ...routerTesting, [port]: 'testing' };
    try {
      const res = await fetch('/api/router/test', {
        method: 'POST',
        headers: { ...hdrs(), 'Content-Type': 'application/json' },
        body: JSON.stringify({ port: parseInt(port) }),
      });
      const d = await res.json();
      routerTesting = { ...routerTesting, [port]: d.reachable ? 'ok' : 'fail' };
    } catch {
      routerTesting = { ...routerTesting, [port]: 'fail' };
    }
    setTimeout(() => {
      routerTesting = { ...routerTesting, [port]: false };
    }, 5000);
  }

  // ─── SSL / HTTPS handlers ───
  async function requestCert() {
    const domain = ddnsData.config?.domain || certData.config?.ddns?.domain || '';
    if (!domain) {
      certMsg = 'Configura un dominio DDNS primero';
      certMsgError = true;
      return;
    }
    if (!certEmail) {
      certMsg = "Introduce un email para Let's Encrypt";
      certMsgError = true;
      return;
    }
    certRequesting = true;
    certMsg = '';
    try {
      const provider = ddnsData.config?.provider || '';
      const dnsToken = ddnsData.config?.token || '';
      const useDns = provider === 'duckdns' && dnsToken;

      const res = await fetch('/api/remote-access/request-ssl', {
        method: 'POST',
        headers: { ...hdrs(), 'Content-Type': 'application/json' },
        body: JSON.stringify({
          domain,
          email: certEmail,
          method: useDns ? 'dns' : 'standalone',
          provider: useDns ? 'duckdns' : '',
          dnsToken: useDns ? dnsToken : '',
        }),
      });
      const data = await res.json();
      if (data.ok) {
        certMsg = 'Certificado obtenido correctamente';
        certMsgError = false;
        await loadAll();
      } else {
        certMsg = data.error || 'Error al solicitar certificado';
        certMsgError = true;
      }
    } catch {
      certMsg = 'Error de conexión';
      certMsgError = true;
    }
    certRequesting = false;
  }

  async function toggleHttps(enable) {
    httpsSaving = true;
    try {
      await fetch('/api/remote-access/enable-https', {
        method: 'POST',
        headers: { ...hdrs(), 'Content-Type': 'application/json' },
        body: JSON.stringify({ domain: certDomain, port: httpsPort, enabled: enable }),
      });
      await loadAll();
    } catch (e) {
      console.error('HTTPS toggle failed', e);
    }
    httpsSaving = false;
  }

  // ─── Helpers ───
  function fmtRelative(ts) {
    if (!ts) return '—';
    const diff = (Date.now() - new Date(ts).getTime()) / 1000;
    if (diff < 60) return 'hace ' + Math.floor(diff) + 's';
    if (diff < 3600) return 'hace ' + Math.floor(diff / 60) + 'm';
    if (diff < 86400) return 'hace ' + Math.floor(diff / 3600) + 'h';
    return 'hace ' + Math.floor(diff / 86400) + 'd';
  }

  function copyUrl() {
    const url = `https://${certDomain}`;
    navigator.clipboard?.writeText(url);
  }

  function openUrl() {
    const url = `https://${certDomain}`;
    window.open(url, '_blank');
  }

  // ─── Lifecycle ───
  onMount(async () => {
    let attempts = 0;
    while (!$token && attempts < 10) { await new Promise(r => setTimeout(r, 200)); attempts++; }
    await loadAll();
    pollInterval = setInterval(loadAll, 15000); // 15s polling (DDNS no cambia tan rápido)
  });

  onDestroy(() => { if (pollInterval) clearInterval(pollInterval); });
</script>

<AppShell
  appId="network"
  title="Network"
  headerIcon="⚡"
  pathSegments={['network', 'remote-access', active]}
  sections={[
    {
      label: 'Remote Access',
      items: [
        { id: 'duckdns', label: 'DuckDNS',       keyHint: '1' },
        { id: 'router',  label: 'Router / UPnP', keyHint: '2' },
        { id: 'ssl',     label: 'SSL / HTTPS',   keyHint: '3' },
      ],
    },
    {
      label: 'Próximamente',
      items: [
        { id: '_iface',    label: 'Interfaces',  keyHint: 'I', disabled: true },
        { id: '_dns',      label: 'DNS',         keyHint: 'D', disabled: true },
        { id: '_firewall', label: 'Firewall',    keyHint: 'F', disabled: true },
        { id: '_services', label: 'Servicios',   disabled: true },
      ],
    },
  ]}
  bind:active
>

  <!-- KPIs globales -->
  <div class="na-kpis">
    <KPICard
      label="IP pública"
      value={externalIp || '—'}
      unit=""
      state={externalIp ? 'detectada' : 'desconocida'}
      stateVariant={externalIp ? 'ok' : 'warn'}
      valueVariant={externalIp ? 'accent' : 'default'}
      bracketVariant={externalIp ? 'accent' : 'warn'}
    />
    <KPICard
      label="DuckDNS"
      value={certDomain ? certDomain.split('.')[0] : '—'}
      unit={certDomain ? '.' + certDomain.split('.').slice(1).join('.') : ''}
      state={ddnsActive ? 'online' : 'inactivo'}
      stateVariant={ddnsActive ? 'ok' : 'warn'}
      valueVariant={ddnsActive ? 'accent' : 'default'}
      bracketVariant={ddnsActive ? 'accent' : 'warn'}
    />
    <KPICard
      label="SSL / HTTPS"
      value={sslValid ? "Let's Encrypt" : 'Sin cert'}
      unit=""
      state={sslValid ? 'válido' : 'falta emitir'}
      stateVariant={sslValid ? 'ok' : 'warn'}
      valueVariant={sslValid ? 'accent' : 'default'}
      bracketVariant={sslValid ? 'accent' : 'warn'}
    />
    <KPICard
      label="Puerto 443"
      value={port443Open ? 'Abierto' : 'Cerrado'}
      unit=""
      state={port443Open ? 'ok' : 'pendiente'}
      stateVariant={port443Open ? 'ok' : 'warn'}
      valueVariant={port443Open ? 'accent' : 'warn'}
      bracketVariant={port443Open ? 'accent' : 'warn'}
    />
  </div>

  <!-- Loading state -->
  {#if loading}
    <div class="na-loading">
      <Spinner label="Cargando configuración de red..." />
    </div>
  {:else}

  <div class="na-scroll">

    <!-- URL final destacada -->
    {#if certDomain}
      <div class="url-box" class:ready={stepHttpsDone}>
        <span class="url-box-label">
          ▸ {stepHttpsDone ? 'Tu URL de acceso remoto' : 'URL objetivo (aún no funcional)'}
        </span>
        <div class="url-row">
          <span class="url">
            <span class="proto">https://</span><span class="host">{certDomain}</span>
          </span>
          <IconButton size="sm" title="Copiar" onClick={copyUrl}>⎘</IconButton>
          <IconButton size="sm" title="Abrir" onClick={openUrl} disabled={!stepHttpsDone}>↗</IconButton>
        </div>
      </div>
    {/if}

    <!-- Setup Steps -->
    <div class="na-section">
      <SectionHead count="· {stepsCompleted}/4 pasos">Setup acceso remoto</SectionHead>

      <div class="steps">

        <!-- Paso 1: DDNS -->
        <div class="step" class:done={stepDdnsDone} class:current={currentStep === 1}>
          <div class="step-num">
            {#if stepDdnsDone}✓{:else}1{/if}
          </div>
          <div class="step-body">
            <div class="step-title">Configurar DuckDNS</div>
            <div class="step-hint">
              {#if stepDdnsDone}
                Dominio <b class="tc-accent">{certDomain}</b> apuntando a tu IP pública.
              {:else}
                Configura un dominio DuckDNS gratuito que apunte a tu IP pública dinámica.
              {/if}
            </div>
            <div class="step-status">
              <LED size={6} variant={stepDdnsDone ? 'ok' : currentStep === 1 ? 'warn' : 'off'} />
              <span>
                {#if stepDdnsDone}
                  completado{ddnsData.lastUpdate ? ` · ${fmtRelative(ddnsData.lastUpdate)}` : ''}
                {:else if currentStep === 1}
                  acción requerida
                {:else}
                  pendiente
                {/if}
              </span>
            </div>
            {#if currentStep === 1}
              <div class="step-actions">
                <BevelButton variant="primary" size="sm" onClick={() => active = 'duckdns'}>
                  ▸ Configurar ahora
                </BevelButton>
              </div>
            {/if}
          </div>
        </div>

        <!-- Paso 2: SSL -->
        <div class="step" class:done={stepSslDone} class:current={currentStep === 2}>
          <div class="step-num">
            {#if stepSslDone}✓{:else}2{/if}
          </div>
          <div class="step-body">
            <div class="step-title">Certificado SSL Let's Encrypt</div>
            <div class="step-hint">
              {#if stepSslDone}
                Certificado emitido para <b class="tc-accent">{certDomain}</b>. Renovación automática.
              {:else}
                Solicita un certificado SSL gratuito para poder servir el panel vía HTTPS.
              {/if}
            </div>
            <div class="step-status">
              <LED size={6} variant={stepSslDone ? 'ok' : currentStep === 2 ? 'warn' : 'off'} />
              <span>
                {#if stepSslDone}
                  válido{sslExpiryDays ? ` · expira en ${sslExpiryDays} días` : ''}
                {:else if currentStep === 2}
                  acción requerida
                {:else}
                  pendiente del paso {currentStep < 2 ? 1 : '—'}
                {/if}
              </span>
            </div>
            {#if currentStep === 2}
              <div class="step-actions">
                <BevelButton variant="primary" size="sm" onClick={() => active = 'ssl'}>
                  ▸ Emitir certificado
                </BevelButton>
              </div>
            {/if}
          </div>
        </div>

        <!-- Paso 3: Puerto 443 -->
        <div class="step" class:done={stepPortDone} class:current={currentStep === 3}>
          <div class="step-num">
            {#if stepPortDone}✓{:else}3{/if}
          </div>
          <div class="step-body">
            <div class="step-title">Abrir puerto 443 en el router</div>
            <div class="step-hint">
              {#if stepPortDone}
                Puerto 443 abierto vía UPnP. Tu NAS es alcanzable desde internet.
              {:else}
                Abre el puerto 443 (HTTPS) en tu router, preferiblemente con UPnP o manualmente.
              {/if}
            </div>
            <div class="step-status">
              <LED size={6} variant={stepPortDone ? 'ok' : currentStep === 3 ? 'warn' : 'off'} />
              <span>
                {#if stepPortDone}
                  completado · vía {routerStatus.method || 'UPnP'}
                {:else if currentStep === 3}
                  acción requerida
                {:else}
                  pendiente
                {/if}
              </span>
            </div>
            {#if currentStep === 3}
              <div class="step-actions">
                <BevelButton variant="primary" size="sm" onClick={addPort443} disabled={!routerStatus.upnpAvailable}>
                  ▸ Abrir con UPnP
                </BevelButton>
                <BevelButton size="sm" onClick={() => active = 'router'}>
                  Ver router
                </BevelButton>
              </div>
              {#if !routerStatus.upnpAvailable}
                <div class="step-warn">
                  UPnP no disponible · abre el puerto manualmente desde la interfaz de tu router
                </div>
              {/if}
            {/if}
          </div>
        </div>

        <!-- Paso 4: HTTPS nginx -->
        <div class="step" class:done={stepHttpsDone} class:current={currentStep === 4}>
          <div class="step-num">
            {#if stepHttpsDone}✓{:else}4{/if}
          </div>
          <div class="step-body">
            <div class="step-title">Activar HTTPS en nginx</div>
            <div class="step-hint">
              {#if stepHttpsDone}
                Panel NimOS sirviendo en <b class="tc-accent">https://{certDomain}</b>
              {:else}
                Configurar nginx para servir el panel vía HTTPS usando el certificado de Let's Encrypt.
              {/if}
            </div>
            <div class="step-status">
              <LED size={6} variant={stepHttpsDone ? 'ok' : currentStep === 4 ? 'warn' : 'off'} />
              <span>
                {#if stepHttpsDone}
                  activo · puerto {httpsPort}
                {:else if currentStep === 4}
                  acción requerida
                {:else}
                  pendiente
                {/if}
              </span>
            </div>
            {#if currentStep === 4}
              <div class="step-actions">
                <BevelButton variant="primary" size="sm" onClick={() => toggleHttps(true)} disabled={httpsSaving}>
                  {httpsSaving ? '▸ Activando...' : '▸ Activar HTTPS'}
                </BevelButton>
              </div>
            {/if}
          </div>
        </div>

      </div>
    </div>

    <!-- ═══════ SECCIÓN: DUCKDNS ═══════ -->
    {#if active === 'duckdns'}
      <div class="na-section">
        <SectionHead count={ddnsActive ? '· activo' : ''}>Configuración DDNS</SectionHead>

        {#if ddnsActive && !ddnsEditing}
          <!-- Estado activo: mostrar info + controls -->
          <div class="panel">
            <div class="panel-head">
              <div class="panel-title">
                <Badge variant="accent">DuckDNS</Badge>
                <span class="domain-big">{ddnsData.config.domain}</span>
              </div>
              <div class="panel-status">
                <LED size={7} variant="ok" />
                <span>activo</span>
              </div>
            </div>

            <div class="info-grid">
              <div class="info-row"><span class="k">proveedor</span><span class="v">{ddnsData.config.provider}</span></div>
              <div class="info-row"><span class="k">dominio</span><span class="v tc-accent">{ddnsData.config.domain}</span></div>
              <div class="info-row"><span class="k">ip externa</span><span class="v">{ddnsData.externalIp || '—'}</span></div>
              <div class="info-row"><span class="k">último update</span><span class="v">{fmtRelative(ddnsData.lastUpdate) || '—'}</span></div>
              <div class="info-row"><span class="k">auto-update</span><span class="v" class:tc-accent={autoUpdate}>{autoUpdate ? 'sí · cada 5 min' : 'desactivado'}</span></div>
            </div>

            <div class="actions">
              <BevelButton size="sm" onClick={testDdns} disabled={ddnsTesting}>
                {ddnsTesting ? '▸ Probando...' : '↻ Probar ahora'}
              </BevelButton>
              <BevelButton size="sm" onClick={toggleAutoUpdate}>
                {autoUpdate ? 'Desactivar auto' : 'Activar auto'}
              </BevelButton>
              <BevelButton size="sm" onClick={() => ddnsEditing = true}>
                ✎ Editar
              </BevelButton>
              <div style="flex:1"></div>
              <BevelButton variant="danger" size="sm" onClick={disableDdns}>
                Desactivar
              </BevelButton>
            </div>

            {#if ddnsMsg}
              <div class="msg" class:error={ddnsMsgError}>{ddnsMsg}</div>
            {/if}
          </div>
        {:else}
          <!-- Formulario -->
          <div class="panel">
            <div class="panel-head">
              <div class="panel-title">
                <Badge>Configurar</Badge>
                <span>{ddnsEditing ? 'Editar dominio DDNS' : 'Añadir nuevo dominio'}</span>
              </div>
            </div>

            <div class="form-row">
              <label class="form-label">Proveedor</label>
              <div class="input-wrap">
                <select bind:value={ddnsForm.provider}>
                  <option value="">Seleccionar...</option>
                  <option value="duckdns">DuckDNS</option>
                  <option value="noip">No-IP</option>
                  <option value="dynu">Dynu</option>
                  <option value="freedns">FreeDNS</option>
                </select>
                <span class="caret">▾</span>
              </div>
            </div>

            {#if ddnsForm.provider === 'duckdns'}
              <div class="form-row">
                <div>
                  <label class="form-label">Subdominio</label>
                  <div class="form-hint">tu-nombre.duckdns.org</div>
                </div>
                <TextInput bind:value={ddnsForm.domain} placeholder="midominio.duckdns.org" size="sm" />
              </div>
              <div class="form-row">
                <div>
                  <label class="form-label">Token</label>
                  <div class="form-hint">duckdns.org tras login</div>
                </div>
                <div class="input-with-eye">
                  <TextInput
                    bind:value={ddnsForm.token}
                    placeholder="Token de DuckDNS"
                    type={tokenVisible ? 'text' : 'password'}
                    size="sm"
                  />
                  <IconButton size="sm" title="Mostrar/ocultar" onClick={() => tokenVisible = !tokenVisible}>
                    {tokenVisible ? '◉' : '○'}
                  </IconButton>
                </div>
              </div>
            {:else if ddnsForm.provider === 'noip'}
              <div class="form-row">
                <label class="form-label">Hostname</label>
                <TextInput bind:value={ddnsForm.domain} placeholder="midominio.ddns.net" size="sm" />
              </div>
              <div class="form-row">
                <label class="form-label">Email</label>
                <TextInput bind:value={ddnsForm.username} placeholder="tu@email.com" size="sm" />
              </div>
              <div class="form-row">
                <label class="form-label">Contraseña</label>
                <TextInput bind:value={ddnsForm.password} type="password" size="sm" />
              </div>
            {:else if ddnsForm.provider === 'dynu'}
              <div class="form-row">
                <label class="form-label">Hostname</label>
                <TextInput bind:value={ddnsForm.domain} placeholder="midominio.dynu.net" size="sm" />
              </div>
              <div class="form-row">
                <label class="form-label">Password</label>
                <TextInput bind:value={ddnsForm.token} type="password" size="sm" />
              </div>
            {:else if ddnsForm.provider === 'freedns'}
              <div class="form-row">
                <label class="form-label">Update Key</label>
                <TextInput bind:value={ddnsForm.token} size="sm" />
              </div>
            {/if}

            {#if ddnsForm.provider}
              <div class="actions">
                <BevelButton size="sm" onClick={testDdns} disabled={ddnsTesting}>
                  {ddnsTesting ? '▸ Probando...' : '↻ Probar'}
                </BevelButton>
                <BevelButton variant="primary" size="sm" onClick={saveDdns} disabled={ddnsSaving}>
                  {ddnsSaving ? '▸ Guardando...' : '▸ Guardar'}
                </BevelButton>
                {#if ddnsEditing}
                  <BevelButton size="sm" onClick={() => { ddnsEditing = false; ddnsMsg = ''; }}>
                    Cancelar
                  </BevelButton>
                {/if}
              </div>
            {/if}

            {#if ddnsMsg}
              <div class="msg" class:error={ddnsMsgError}>{ddnsMsg}</div>
            {/if}
          </div>
        {/if}
      </div>
    {/if}

    <!-- ═══════ SECCIÓN: ROUTER / UPNP ═══════ -->
    {#if active === 'router'}
      <div class="na-section">
        <SectionHead count={routerStatus.upnpAvailable ? '· UPnP activo' : '· sin UPnP'}>
          Router / UPnP
        </SectionHead>

        <div class="panel">
          <div class="panel-head">
            <div class="panel-title">
              <Badge variant={routerStatus.upnpAvailable ? 'accent' : 'warn'}>
                {routerStatus.upnpAvailable ? 'UPnP' : 'manual'}
              </Badge>
              <span>{routerStatus.model || routerStatus.manufacturer || 'Router'}</span>
            </div>
            <div class="panel-status">
              <LED size={7} variant={routerStatus.upnpAvailable ? 'ok' : 'warn'} />
              <span>{routerStatus.upnpAvailable ? 'disponible' : 'no disponible'}</span>
            </div>
          </div>

          <div class="info-grid">
            <div class="info-row"><span class="k">fabricante</span><span class="v">{routerStatus.manufacturer || '—'}</span></div>
            <div class="info-row"><span class="k">modelo</span><span class="v">{routerStatus.model || '—'}</span></div>
            <div class="info-row"><span class="k">ip gateway</span><span class="v">{routerStatus.gateway || '—'}</span></div>
            <div class="info-row"><span class="k">ip externa</span><span class="v tc-accent">{routerStatus.externalIp || externalIp || '—'}</span></div>
          </div>
        </div>

        <!-- Puertos abiertos -->
        <div class="panel">
          <div class="panel-head">
            <div class="panel-title">
              <span>Puertos abiertos</span>
              <Badge size="sm">{routerPorts.length}</Badge>
            </div>
          </div>

          {#if routerPorts.length === 0}
            <EmptyState icon="◌" title="Sin puertos abiertos" hint="Usa el formulario de abajo para abrir uno" />
          {:else}
            <div class="ports-list">
              {#each routerPorts as p}
                <div class="port-row">
                  <span class="port-num">{p.externalPort || p.port}</span>
                  <Badge size="sm" variant={p.protocol === 'TCP' ? 'info' : 'warn'}>{p.protocol}</Badge>
                  <span class="port-desc">{p.description || '—'}</span>
                  <span class="port-internal">→ {p.internalIp || localIp}:{p.internalPort || p.port}</span>
                  <div class="port-actions">
                    {#if routerTesting[p.externalPort || p.port] === 'testing'}
                      <Badge size="sm" variant="warn">probando...</Badge>
                    {:else if routerTesting[p.externalPort || p.port] === 'ok'}
                      <Badge size="sm" variant="accent">accesible</Badge>
                    {:else if routerTesting[p.externalPort || p.port] === 'fail'}
                      <Badge size="sm" variant="crit">no accesible</Badge>
                    {/if}
                    <IconButton size="sm" title="Probar" onClick={() => testPort(p.externalPort || p.port)}>↻</IconButton>
                    <IconButton size="sm" variant="danger" title="Cerrar" onClick={() => removePort(p.externalPort || p.port, p.protocol)}>×</IconButton>
                  </div>
                </div>
              {/each}
            </div>
          {/if}

          <!-- Formulario abrir puerto -->
          <div class="port-form">
            <div class="pf-field">
              <label class="form-label">Puerto</label>
              <TextInput bind:value={newPort} placeholder="443" size="sm" />
            </div>
            <div class="pf-field">
              <label class="form-label">Protocolo</label>
              <div class="input-wrap">
                <select bind:value={newPortProto}>
                  <option value="TCP">TCP</option>
                  <option value="UDP">UDP</option>
                </select>
                <span class="caret">▾</span>
              </div>
            </div>
            <div class="pf-field wide">
              <label class="form-label">Descripción</label>
              <TextInput bind:value={newPortDesc} placeholder="NimOS HTTPS" size="sm" />
            </div>
            <div class="pf-field">
              <BevelButton variant="primary" size="sm" onClick={addPort} disabled={!newPort}>
                ▸ Abrir
              </BevelButton>
            </div>
          </div>

          {#if routerMsg}
            <div class="msg" class:error={routerMsgError}>{routerMsg}</div>
          {/if}
        </div>
      </div>
    {/if}

    <!-- ═══════ SECCIÓN: SSL / HTTPS ═══════ -->
    {#if active === 'ssl'}
      <div class="na-section">
        <SectionHead count={sslValid ? '· válido' : '· falta emitir'}>Certificado SSL</SectionHead>

        <div class="panel">
          <div class="panel-head">
            <div class="panel-title">
              <Badge variant={sslValid ? 'accent' : 'warn'}>Let's Encrypt</Badge>
              <span>{certDomain || 'Sin dominio configurado'}</span>
            </div>
            <div class="panel-status">
              <LED size={7} variant={sslValid ? 'ok' : 'warn'} />
              <span>{sslValid ? 'válido' : 'no emitido'}</span>
            </div>
          </div>

          {#if sslValid}
            <div class="info-grid">
              <div class="info-row"><span class="k">dominio</span><span class="v tc-accent">{certDomain}</span></div>
              <div class="info-row"><span class="k">emisor</span><span class="v">Let's Encrypt</span></div>
              <div class="info-row"><span class="k">expira</span><span class="v">{sslExpiryDays} días</span></div>
              <div class="info-row"><span class="k">renovación</span><span class="v tc-accent">automática</span></div>
            </div>
          {:else}
            <div class="form-row">
              <label class="form-label">Email de contacto</label>
              <TextInput bind:value={certEmail} placeholder="tu@email.com" size="sm" />
            </div>
            <div class="form-hint" style="margin-top:-8px">
              Let's Encrypt usará este email para avisos de expiración. No se comparte.
            </div>
            {#if !certDomain}
              <div class="msg warn">⚠ Configura un dominio DDNS antes de solicitar un certificado SSL</div>
            {/if}
            <div class="actions">
              <BevelButton
                variant="primary"
                size="sm"
                onClick={requestCert}
                disabled={certRequesting || !certDomain || !certEmail}
              >
                {certRequesting ? '▸ Emitiendo...' : '▸ Emitir certificado'}
              </BevelButton>
            </div>
          {/if}

          {#if certMsg}
            <div class="msg" class:error={certMsgError}>{certMsg}</div>
          {/if}
        </div>

        <!-- HTTPS nginx -->
        <div class="panel">
          <div class="panel-head">
            <div class="panel-title">
              <span>HTTPS en nginx</span>
            </div>
            <div class="panel-status">
              <LED size={7} variant={httpsEnabled ? 'ok' : 'off'} />
              <span>{httpsEnabled ? 'activo' : 'inactivo'}</span>
            </div>
          </div>

          <div class="info-grid">
            <div class="info-row"><span class="k">puerto</span><span class="v">{httpsPort}</span></div>
            <div class="info-row"><span class="k">dominio</span><span class="v">{certDomain || '—'}</span></div>
            <div class="info-row"><span class="k">certificado</span><span class="v" class:tc-accent={sslValid}>{sslValid ? 'Let\'s Encrypt' : 'ninguno'}</span></div>
            <div class="info-row"><span class="k">puerto 443 router</span><span class="v" class:tc-accent={port443Open}>{port443Open ? 'abierto' : 'cerrado'}</span></div>
          </div>

          <div class="actions">
            {#if httpsEnabled}
              <BevelButton variant="danger" size="sm" onClick={() => toggleHttps(false)} disabled={httpsSaving}>
                {httpsSaving ? '▸ Desactivando...' : '■ Desactivar HTTPS'}
              </BevelButton>
            {:else}
              <BevelButton
                variant="primary"
                size="sm"
                onClick={() => toggleHttps(true)}
                disabled={httpsSaving || !sslValid || !port443Open}
              >
                {httpsSaving ? '▸ Activando...' : '▸ Activar HTTPS'}
              </BevelButton>
            {/if}
          </div>

          {#if !sslValid}
            <div class="msg warn">Emite primero un certificado SSL para poder activar HTTPS</div>
          {:else if !port443Open}
            <div class="msg warn">Abre el puerto 443 del router antes de activar HTTPS</div>
          {/if}
        </div>
      </div>
    {/if}

  </div>
  {/if}

  <!-- Footer -->
  <svelte:fragment slot="footer">
    <span><span class="k">domain</span> <span class="v tc-accent">{certDomain || 'no configurado'}</span></span>
    <span class="sep">·</span>
    <span><span class="k">external</span> <span class="v">{externalIp || '—'}</span></span>
    <span class="sep">·</span>
    <span><span class="k">setup</span> <span class="v" class:tc-accent={stepsCompleted === 4}>{stepsCompleted}/4</span></span>
  </svelte:fragment>

  <svelte:fragment slot="footer-right">
    <span><span class="k">poll</span> <span class="v">15s</span></span>
  </svelte:fragment>

</AppShell>

<style>
  /* ─── KPIs row ─── */
  .na-kpis {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    border-bottom: 1px solid var(--border);
    background: var(--bg-1);
    flex-shrink: 0;
  }
  .na-kpis :global(.kpi) {
    border-right: 1px solid var(--border);
  }
  .na-kpis :global(.kpi:last-child) {
    border-right: none;
  }

  /* ─── Loading ─── */
  .na-loading {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 200px;
  }

  /* ─── Main scroll ─── */
  .na-scroll {
    flex: 1;
    overflow-y: auto;
    padding: 22px 28px 24px;
    display: flex;
    flex-direction: column;
    gap: 28px;
  }

  .na-section {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  /* ─── URL box destacado ─── */
  .url-box {
    background: var(--bg);
    border: 1px solid var(--accent);
    padding: 14px 18px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    box-shadow: 0 0 12px rgba(0, 255, 159, 0.08);
    clip-path: polygon(
      0 0, 100% 0, 100% calc(100% - 10px),
      calc(100% - 10px) 100%, 0 100%
    );
  }
  .url-box:not(.ready) {
    border-color: var(--warn);
    box-shadow: 0 0 12px rgba(255, 184, 0, 0.06);
  }
  .url-box-label {
    font-family: var(--font-mono);
    font-size: 9px;
    color: var(--accent);
    text-transform: uppercase;
    letter-spacing: 1.8px;
  }
  .url-box:not(.ready) .url-box-label { color: var(--warn); }
  .url-row {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .url {
    flex: 1;
    font-family: var(--font-mono);
    font-size: 14px;
    color: var(--fg);
    letter-spacing: 0.3px;
    word-break: break-all;
  }
  .url .proto { color: var(--accent); }
  .url .host  { color: var(--fg); font-weight: 500; }

  /* ─── Setup Steps ─── */
  .steps {
    display: flex;
    flex-direction: column;
  }
  .step {
    display: grid;
    grid-template-columns: 40px 1fr;
    gap: 14px;
    padding: 14px 0;
    border-bottom: 1px dashed var(--border);
  }
  .step:last-child { border-bottom: none; }

  .step-num {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    background: var(--bg);
    border: 1px solid var(--border-bright);
    color: var(--fg-mute);
    font-family: var(--font-mono);
    font-size: 11px;
    font-weight: 700;
    clip-path: polygon(
      0 0, calc(100% - 5px) 0, 100% 5px,
      100% 100%, 5px 100%, 0 calc(100% - 5px)
    );
  }
  .step.done .step-num {
    border-color: var(--accent);
    color: var(--accent);
    background: var(--accent-dim);
  }
  .step.current .step-num {
    border-color: var(--warn);
    color: var(--warn);
    background: rgba(255, 184, 0, 0.06);
    animation: pulse-warn-box 1.5s ease-in-out infinite;
  }
  @keyframes pulse-warn-box {
    0%, 100% { box-shadow: 0 0 4px rgba(255, 184, 0, 0.3); }
    50%      { box-shadow: 0 0 10px rgba(255, 184, 0, 0.5); }
  }

  .step-body {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding-top: 5px;
  }
  .step-title {
    font-size: 12px;
    color: var(--fg);
    font-weight: 600;
    letter-spacing: 0.3px;
  }
  .step.done .step-title { color: var(--fg-dim); }
  .step-hint {
    font-size: 10px;
    color: var(--fg-mute);
    letter-spacing: 0.3px;
    line-height: 1.5;
    font-family: var(--font-sans);
  }
  .step-status {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-family: var(--font-mono);
    font-size: 9px;
    text-transform: uppercase;
    letter-spacing: 1px;
    margin-top: 4px;
    color: var(--fg-dim);
  }
  .step-actions {
    display: flex;
    gap: 8px;
    margin-top: 10px;
  }
  .step-warn {
    margin-top: 8px;
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--warn);
    background: rgba(255, 184, 0, 0.04);
    border-left: 2px solid var(--warn);
    padding: 6px 10px;
  }

  /* ─── Panel ─── */
  .panel {
    background: var(--bg-1);
    border: 1px solid var(--border);
    padding: 16px 20px;
    display: flex;
    flex-direction: column;
    gap: 14px;
    font-family: var(--font-mono);
  }
  .panel-head {
    display: flex;
    align-items: center;
    gap: 10px;
    padding-bottom: 12px;
    border-bottom: 1px solid var(--border);
  }
  .panel-title {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 11px;
    color: var(--fg);
    letter-spacing: 1.3px;
    text-transform: uppercase;
    font-weight: 600;
  }
  .panel-status {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 9px;
    color: var(--fg-dim);
    text-transform: uppercase;
    letter-spacing: 1px;
  }
  .domain-big {
    text-transform: none;
    color: var(--accent);
    font-size: 13px;
    letter-spacing: 0.3px;
    font-weight: 500;
  }

  /* ─── Form ─── */
  .form-row {
    display: grid;
    grid-template-columns: 140px 1fr;
    gap: 14px;
    align-items: center;
  }
  .form-label {
    font-size: 10px;
    color: var(--fg-mute);
    text-transform: uppercase;
    letter-spacing: 1.3px;
    font-family: var(--font-mono);
    display: block;
  }
  .form-hint {
    font-size: 9px;
    color: var(--fg-faint);
    letter-spacing: 0.3px;
    margin-top: 2px;
    font-family: var(--font-mono);
  }

  .input-wrap {
    display: flex;
    align-items: center;
    gap: 8px;
    height: 28px;
    padding: 0 10px;
    border: 1px solid var(--border);
    background: var(--bg);
    clip-path: polygon(
      0 0, calc(100% - 6px) 0, 100% 6px,
      100% 100%, 6px 100%, 0 calc(100% - 6px)
    );
  }
  .input-wrap:focus-within { border-color: var(--accent); }
  .input-wrap select {
    flex: 1;
    min-width: 0;
    background: transparent;
    border: none;
    outline: none;
    color: var(--fg);
    font-family: var(--font-mono);
    font-size: 11px;
    letter-spacing: 0.5px;
    appearance: none;
    cursor: pointer;
  }
  .input-wrap .caret { color: var(--fg-mute); font-size: 10px; }

  .input-with-eye {
    display: flex;
    gap: 6px;
    align-items: stretch;
  }
  .input-with-eye :global(.nimos-text-input) {
    flex: 1;
  }

  /* ─── Actions ─── */
  .actions {
    display: flex;
    gap: 8px;
    padding-top: 10px;
    border-top: 1px solid var(--border);
    align-items: center;
  }

  /* ─── Info grid ─── */
  .info-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 6px 24px;
    padding: 4px 0;
  }
  .info-row {
    display: grid;
    grid-template-columns: 130px 1fr;
    gap: 10px;
    font-size: 11px;
  }
  .info-row .k {
    color: var(--fg-mute);
    text-transform: uppercase;
    letter-spacing: 1px;
    font-size: 9px;
  }
  .info-row .v {
    color: var(--fg);
    font-feature-settings: "tnum";
  }

  /* ─── Ports list ─── */
  .ports-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
    background: var(--bg);
    border: 1px solid var(--border);
    padding: 6px;
  }
  .port-row {
    display: grid;
    grid-template-columns: 60px 60px 1fr 1fr auto;
    gap: 12px;
    align-items: center;
    padding: 6px 10px;
    font-family: var(--font-mono);
    font-size: 11px;
    background: var(--bg-1);
    border-left: 2px solid var(--accent);
  }
  .port-num {
    color: var(--accent);
    font-weight: 600;
    font-feature-settings: "tnum";
  }
  .port-desc {
    color: var(--fg-dim);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .port-internal {
    color: var(--fg-mute);
    font-size: 10px;
  }
  .port-actions {
    display: flex;
    gap: 4px;
    align-items: center;
  }

  /* ─── Port form ─── */
  .port-form {
    display: grid;
    grid-template-columns: 100px 110px 1fr auto;
    gap: 10px;
    align-items: end;
    padding: 12px;
    background: var(--bg);
    border: 1px dashed var(--border);
  }
  .pf-field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .pf-field.wide { grid-column: span 1; }

  /* ─── Mensajes ─── */
  .msg {
    font-family: var(--font-mono);
    font-size: 10px;
    padding: 6px 10px;
    background: rgba(0, 255, 159, 0.04);
    border-left: 2px solid var(--accent);
    color: var(--accent);
  }
  .msg.error {
    background: rgba(255, 90, 90, 0.04);
    border-left-color: var(--crit);
    color: var(--crit);
  }
  .msg.warn {
    background: rgba(255, 184, 0, 0.04);
    border-left-color: var(--warn);
    color: var(--warn);
  }

  /* ─── Utility ─── */
  .tc-accent { color: var(--accent); }
  .tc-crit { color: var(--crit); }
  .k { color: var(--fg-faint); }
  .v { color: var(--fg-dim); font-feature-settings: "tnum"; }
  .sep { color: var(--fg-faint); }
</style>
