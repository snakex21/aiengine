const $ = (s) => document.querySelector(s);
const $$ = (s) => [...document.querySelectorAll(s)];
const status = (msg) => { $('#status').textContent = msg; };

let currentRoot = '';
let currentEntries = [];
let expandedDirs = new Set();
let selectedTile = 'W';
let grid = Array.from({ length: 10 }, () => Array(10).fill('.'));
let currentMapPath = '';
const tiles = [
  ['.', 'Empty', '#30343f'], ['W', 'Wall 1', '#b4643c'], ['S', 'Wall 2', '#8c8c8c'],
  ['M', 'Wall 3', '#a0a0b4'], ['D', 'Door', '#8b5a2b'], ['P', 'Player', '#22c55e'],
  ['E', 'Enemy', '#ef4444'], ['B', 'Boss', '#b91c1c'],
];

function switchTab(id) {
  $$('.tab').forEach(b => b.classList.toggle('active', b.dataset.tab === id));
  $$('.panel').forEach(p => p.classList.toggle('active', p.id === id));
}

function fmtSize(n) {
  if (!n) return '0 B';
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

async function scan() {
  try {
    status('Skanowanie...');
    const dir = $('#projectDir').value.trim();
    const result = await window.scanProject(dir);
    currentRoot = result.root;
    $('#projectDir').value = result.root;
    await renderProjectInfo();
    currentEntries = result.entries || [];
    loadExpandedDirs();
    renderAssetTree();
    status(`Znaleziono ${result.entries.length} plików`);
  } catch (err) {
    status(`Błąd: ${err}`);
  }
}

function storageKey() {
  return `aiengine.expandedDirs:${currentRoot}`;
}

function loadExpandedDirs() {
  try {
    const raw = localStorage.getItem(storageKey());
    if (raw) {
      expandedDirs = new Set(JSON.parse(raw));
    } else {
      expandedDirs = new Set(['assets', 'maps', 'scripts', 'styles', 'entities']);
    }
  } catch {
    expandedDirs = new Set();
  }
}

function saveExpandedDirs() {
  try { localStorage.setItem(storageKey(), JSON.stringify([...expandedDirs])); } catch {}
}

function buildTree(entries) {
  const root = { name: '', path: '', dirs: new Map(), files: [] };
  for (const entry of entries) {
    const parts = entry.path.split('/').filter(Boolean);
    let node = root;
    for (let i = 0; i < parts.length - 1; i++) {
      const name = parts[i];
      const path = parts.slice(0, i + 1).join('/');
      if (!node.dirs.has(name)) node.dirs.set(name, { name, path, dirs: new Map(), files: [] });
      node = node.dirs.get(name);
    }
    node.files.push(entry);
  }
  return root;
}

function renderAssetTree() {
  const list = $('#assetList');
  list.innerHTML = '';
  const tree = buildTree(currentEntries);
  const dirs = [...tree.dirs.values()].sort((a, b) => a.name.localeCompare(b.name));
  const files = [...tree.files].sort((a, b) => a.name.localeCompare(b.name));
  for (const dir of dirs) renderDirNode(list, dir, 0);
  for (const file of files) renderFileNode(list, file, 0);
}

function renderDirNode(parent, dir, depth) {
  const open = expandedDirs.has(dir.path);
  const row = document.createElement('div');
  row.className = 'tree-row dir-row';
  row.style.paddingLeft = `${10 + depth * 18}px`;
  row.title = dir.path;
  const chevron = document.createElement('span');
  chevron.className = 'chevron';
  chevron.textContent = open ? '▾' : '▸';
  const label = document.createElement('span');
  label.className = 'tree-label';
  label.textContent = `📁 ${dir.name}`;
  row.append(chevron, label);
  row.onclick = () => {
    if (expandedDirs.has(dir.path)) expandedDirs.delete(dir.path);
    else expandedDirs.add(dir.path);
    saveExpandedDirs();
    renderAssetTree();
  };
  parent.appendChild(row);

  if (!open) return;
  const dirs = [...dir.dirs.values()].sort((a, b) => a.name.localeCompare(b.name));
  const files = [...dir.files].sort((a, b) => a.name.localeCompare(b.name));
  for (const child of dirs) renderDirNode(parent, child, depth + 1);
  for (const file of files) renderFileNode(parent, file, depth + 1);
}

function renderFileNode(parent, file, depth) {
  const row = document.createElement('div');
  row.className = 'tree-row file-row';
  row.style.paddingLeft = `${28 + depth * 18}px`;
  row.title = file.path;

  const label = document.createElement('span');
  label.className = 'tree-label';
  label.textContent = `${icon(file.kind)} ${file.name}`;
  const size = document.createElement('small');
  size.textContent = fmtSize(file.size);
  row.append(label, size);
  row.onclick = () => preview(file);
  parent.appendChild(row);
}

async function renderProjectInfo() {
  const box = $('#projectInfo');
  if (!box) return;
  try {
    const info = await window.getProjectInfo();
    const rows = [
      ['Project', info.name],
      ['Path', info.path],
      ['Engine', info.engineExe || 'bin/aiengine.exe not found'],
      ['Manifest', info.manifestPath],
      ['Config', info.configPath],
      ['Main map', info.mainMap],
      ['Main script', info.mainScript],
      ['HUD script', info.hudScript],
      ['Player assets', info.playerAssets],
      ['Player atlas', info.playerAtlas],
      ['Enemy atlas', info.enemyAtlas],
    ];
    box.innerHTML = rows.map(([k, v]) => `<div class="kv-row"><div class="kv-key">${k}</div><div class="kv-val" title="${v || ''}">${v || '-'}</div></div>`).join('');
  } catch (err) {
    box.textContent = `Nie można odczytać manifestu: ${err}`;
  }
}

async function runSelectedProject() {
  try {
    status('Uruchamianie projektu...');
    // Ensure Go bridge root matches the currently selected project before launch.
    await window.scanProject($('#projectDir').value.trim());
    const msg = await window.launchEngine();
    status(msg);
    const testerOut = $('#testerOut');
    if (testerOut) testerOut.textContent = msg;
  } catch (err) {
    const msg = `Błąd uruchamiania: ${err}`;
    status(msg);
    const testerOut = $('#testerOut');
    if (testerOut) testerOut.textContent = msg;
  }
}

function icon(kind) {
  return kind === 'image' ? '🖼️' : kind === 'audio' ? '🔊' : kind === 'aie' ? '🧩' : kind === 'text' ? '📄' : '📦';
}

async function preview(e) {
  const box = $('#assetPreview');
  box.textContent = 'Ładowanie...';
  try {
    if (e.kind === 'image') {
      const src = await window.readImageData(e.path);
      box.innerHTML = `<h3>${e.path}</h3><img src="${src}" />`;
    } else if (e.kind === 'text' || e.kind === 'aie') {
      const text = await window.readTextFile(e.path);
      box.innerHTML = `<h3>${e.path}</h3><pre></pre>`;
      box.querySelector('pre').textContent = text.slice(0, 20000);
      $('#sandboxPath').value = e.path;
      $('#sandboxText').value = text;
    } else {
      box.innerHTML = `<h3>${e.path}</h3><p>Typ: ${e.kind}, rozmiar: ${fmtSize(e.size)}</p>`;
    }
  } catch (err) {
    box.textContent = `Błąd podglądu: ${err}`;
  }
}

function renderTiles() {
  const wrap = $('#tiles');
  wrap.innerHTML = '';
  for (const [key, label, color] of tiles) {
    const b = document.createElement('button');
    b.className = 'tile' + (key === selectedTile ? ' active' : '');
    b.textContent = label;
    b.style.background = color;
    b.onclick = () => { selectedTile = key; renderTiles(); };
    wrap.appendChild(b);
  }
}

function renderGrid() {
  const g = $('#mapGrid');
  g.innerHTML = '';
  g.style.gridTemplateColumns = `repeat(${grid[0]?.length || 10}, 34px)`;
  $('#mapWidth').value = grid[0]?.length || 0;
  $('#mapHeight').value = grid.length || 0;
  for (let r = 0; r < grid.length; r++) for (let c = 0; c < grid[r].length; c++) {
    const cell = document.createElement('div');
    const t = tiles.find(x => x[0] === grid[r][c]) || tiles[0];
    cell.className = 'cell';
    cell.textContent = grid[r][c];
    cell.style.background = t[2];
    cell.onclick = () => { grid[r][c] = selectedTile; renderGrid(); };
    g.appendChild(cell);
  }
}

function resizeGrid(width, height, fill = '.') {
  width = Math.max(1, Math.min(80, Number(width) || 10));
  height = Math.max(1, Math.min(80, Number(height) || 10));
  const next = Array.from({ length: height }, (_, r) =>
    Array.from({ length: width }, (_, c) => grid[r]?.[c] ?? fill)
  );
  grid = next;
  renderGrid();
}

function normalizeGridRows(rows) {
  if (!rows || rows.length === 0) return Array.from({ length: 10 }, () => Array(10).fill('.'));
  return rows.map(row => row.trim().split(/\s+/).filter(Boolean));
}

async function loadMapList() {
  const select = $('#mapSelect');
  try {
    const maps = await window.listMaps();
    select.innerHTML = '';
    for (const map of maps) {
      const opt = document.createElement('option');
      opt.value = map.path;
      opt.textContent = map.path;
      select.appendChild(opt);
    }
    if (maps.length > 0 && !currentMapPath) currentMapPath = maps[0].path;
    if (currentMapPath) select.value = currentMapPath;
  } catch (err) {
    select.innerHTML = '<option>no maps</option>';
  }
}

async function loadSelectedMap() {
  try {
    const selected = $('#mapSelect').value;
    if (!selected) return;
    const map = await window.loadMap(selected);
    currentMapPath = map.path;
    $('#mapName').value = map.name || 'untitled';
    $('#mapRaw').value = map.raw || '';
    grid = normalizeGridRows(map.rows);
    renderGrid();
    status(`Załadowano mapę: ${map.path}`);
  } catch (err) {
    status(`Błąd ładowania mapy: ${err}`);
  }
}

async function init() {
  $$('.tab').forEach(b => b.onclick = () => switchTab(b.dataset.tab));
  $('#scanBtn').onclick = scan;
  $('#playProjectBtn').onclick = runSelectedProject;
  $('#projectPlayBtn').onclick = runSelectedProject;
  $('#openFolderBtn').onclick = async () => {
    try { status(await window.openProjectFolder()); }
    catch (err) { status(`Błąd otwierania folderu: ${err}`); }
  };
  $('#calcFrames').onclick = () => {
    const fw = +$('#frameW').value, fh = +$('#frameH').value, cols = +$('#cols').value, rows = +$('#rows').value;
    const frames = [];
    for (let y = 0; y < rows; y++) for (let x = 0; x < cols; x++) frames.push({ index: frames.length, x: x * fw, y: y * fh, w: fw, h: fh });
    $('#framesOut').textContent = JSON.stringify(frames, null, 2);
  };
  $('#exportMap').onclick = async () => {
    const rows = grid.map(r => r.join(' '));
    const res = await window.saveMap(currentMapPath, $('#mapName').value, rows, $('#mapRaw').value);
    currentMapPath = res.path;
    status(`Zapisano mapę: ${res.path}`);
    await loadMapList();
    await scan();
  };
  $('#loadMapBtn').onclick = loadSelectedMap;
  $('#mapSelect').onchange = loadSelectedMap;
  $('#resizeMapBtn').onclick = () => resizeGrid($('#mapWidth').value, $('#mapHeight').value, '.');
  $('#fillMapBtn').onclick = () => { grid = grid.map(row => row.map(() => selectedTile)); renderGrid(); };
  $('#clearMapBtn').onclick = () => { grid = grid.map(row => row.map(() => '.')); renderGrid(); };
  $('#launchEngine').onclick = runSelectedProject;
  $('#loadText').onclick = async () => { $('#sandboxText').value = await window.readTextFile($('#sandboxPath').value); };
  $('#saveText').onclick = async () => { const r = await window.writeTextFile($('#sandboxPath').value, $('#sandboxText').value); status(`Zapisano: ${r.path}`); await scan(); };

  renderTiles(); renderGrid();
  currentRoot = await window.getDefaultProject();
  $('#projectDir').value = currentRoot;
  await loadProjectList();
  await loadMapList();
  await scan();
}

async function loadProjectList() {
  const select = $('#projectSelect');
  try {
    const projects = await window.listProjects();
    select.innerHTML = '';
    for (const p of projects) {
      const opt = document.createElement('option');
      opt.value = p.path;
      opt.textContent = p.name;
      opt.selected = p.path === currentRoot;
      select.appendChild(opt);
    }
    select.onchange = async () => {
      $('#projectDir').value = select.value;
      await scan();
      await renderProjectInfo();
      currentMapPath = '';
      await loadMapList();
    };
  } catch (err) {
    select.innerHTML = '<option>projects unavailable</option>';
  }
}

window.addEventListener('DOMContentLoaded', init);
