(function() {
  'use strict';

  // State
  let state = {
    mode: null,       // 'trace' or 'navigate'
    trace: null,      // trace tree data
    callgraph: null,  // call graph data
    flatSteps: [],    // flattened trace steps for stepping
    currentStep: 0,   // current step index
    sourceCache: {},  // file -> lines cache
    selectedNode: null,
  };

  // DOM refs
  const dom = {
    modeBadge: document.getElementById('mode-badge'),
    funcName: document.getElementById('func-name'),
    stepCounter: document.getElementById('step-counter'),
    callTree: document.getElementById('call-tree'),
    sourceCode: document.getElementById('source-code'),
    sourceFile: document.getElementById('source-file'),
    variables: document.getElementById('variables'),
    varContext: document.getElementById('var-context'),
    stepSlider: document.getElementById('step-slider'),
    btnPrev: document.getElementById('btn-prev'),
    btnNext: document.getElementById('btn-next'),
    btnStepInto: document.getElementById('btn-step-into'),
    btnStepOver: document.getElementById('btn-step-over'),
    btnRestart: document.getElementById('btn-restart'),
    btnExpandAll: document.getElementById('btn-expand-all'),
    btnCollapseAll: document.getElementById('btn-collapse-all'),
    controls: document.getElementById('controls'),
  };

  // Init
  async function init() {
    const info = await fetchJSON('/api/info');
    state.mode = info.mode;

    if (info.mode === 'trace') {
      dom.modeBadge.textContent = 'TRACE';
      dom.modeBadge.className = 'badge badge-trace';
      dom.funcName.textContent = info.func_name || '';
      await loadTrace();
    } else if (info.mode === 'navigate') {
      dom.modeBadge.textContent = 'NAVIGATE';
      dom.modeBadge.className = 'badge badge-navigate';
      dom.controls.style.display = 'none';
      await loadCallGraph();
    }

    bindEvents();
  }

  // Load trace data
  async function loadTrace() {
    state.trace = await fetchJSON('/api/trace');
    if (!state.trace) return;

    // Flatten steps for linear navigation
    state.flatSteps = [];
    flattenTrace(state.trace, state.flatSteps);

    dom.stepSlider.max = Math.max(0, state.flatSteps.length - 1);
    dom.stepSlider.value = 0;

    renderCallTree(state.trace);
    if (state.flatSteps.length > 0) {
      selectStep(0);
    }
  }

  // Load call graph data
  async function loadCallGraph() {
    state.callgraph = await fetchJSON('/api/callgraph');
    if (!state.callgraph) return;
    renderCallGraph(state.callgraph);
  }

  // Flatten trace tree into linear steps
  function flattenTrace(node, result) {
    result.push(node);
    if (node.children) {
      for (const child of node.children) {
        flattenTrace(child, result);
      }
    }
  }

  // Render call tree (trace mode)
  function renderCallTree(node) {
    dom.callTree.innerHTML = '';
    dom.callTree.appendChild(createTreeNode(node, true));
  }

  function createTreeNode(node, expanded) {
    const container = document.createElement('div');
    container.className = 'tree-node';

    const item = document.createElement('div');
    item.className = 'tree-item ' + stepTypeClass(node.step_type);
    item.dataset.nodeId = node.id;

    const hasChildren = node.children && node.children.length > 0;

    // Toggle
    const toggle = document.createElement('span');
    toggle.className = 'tree-toggle';
    toggle.textContent = hasChildren ? (expanded ? '▼' : '▶') : '·';
    item.appendChild(toggle);

    // Icon
    const icon = document.createElement('span');
    icon.textContent = stepIcon(node.step_type);
    item.appendChild(icon);

    // Label
    const label = document.createElement('span');
    label.textContent = ' ' + truncate(node.description || node.func_name || '?', 40);
    item.appendChild(label);

    // Return value badge
    if (node.return_value && node.step_type === 'func_enter') {
      const ret = document.createElement('span');
      ret.style.cssText = 'margin-left:auto;color:var(--text-muted);font-size:0.7rem;';
      ret.textContent = '→ ' + truncate(node.return_value, 20);
      item.appendChild(ret);
    }

    container.appendChild(item);

    // Children
    if (hasChildren) {
      const childContainer = document.createElement('div');
      childContainer.className = 'tree-children' + (expanded ? ' expanded' : '');
      for (const child of node.children) {
        const isFunc = child.step_type === 'func_enter';
        childContainer.appendChild(createTreeNode(child, isFunc && expanded));
      }
      container.appendChild(childContainer);

      // Toggle click
      toggle.addEventListener('click', (e) => {
        e.stopPropagation();
        const isExpanded = childContainer.classList.contains('expanded');
        childContainer.classList.toggle('expanded');
        toggle.textContent = isExpanded ? '▶' : '▼';
      });
    }

    // Select click
    item.addEventListener('click', () => {
      const idx = state.flatSteps.findIndex(s => s.id === node.id);
      if (idx >= 0) selectStep(idx);
    });

    return container;
  }

  // Render call graph (navigate mode)
  function renderCallGraph(graph) {
    dom.callTree.innerHTML = '';

    // Find root nodes (no callers)
    const calleeSet = new Set(graph.edges.map(e => e.callee_id));
    const callerSet = new Set(graph.edges.map(e => e.caller_id));

    // Build adjacency list
    const children = {};
    for (const edge of graph.edges) {
      if (!children[edge.caller_id]) children[edge.caller_id] = [];
      children[edge.caller_id].push(edge);
    }

    // All root nodes (callers that are not callees, or nodes with no callers)
    const roots = Object.keys(graph.nodes).filter(id => !calleeSet.has(id) || !callerSet.has(id));
    if (roots.length === 0) {
      // fallback: show all nodes
      for (const id of Object.keys(graph.nodes)) {
        roots.push(id);
      }
    }

    // Remove duplicates
    const seen = new Set();
    for (const rootId of roots) {
      if (seen.has(rootId)) continue;
      seen.add(rootId);
      const node = graph.nodes[rootId];
      if (!node) continue;
      dom.callTree.appendChild(createGraphNode(node, children, graph.nodes, new Set()));
    }

    // Show source for first node
    if (roots.length > 0 && graph.nodes[roots[0]]) {
      showSource(graph.nodes[roots[0]].file, graph.nodes[roots[0]].line);
    }
  }

  function createGraphNode(node, children, allNodes, visited) {
    const div = document.createElement('div');
    div.className = 'graph-node' + (node.is_method ? ' method' : '');

    const name = document.createElement('div');
    name.className = 'node-name';
    name.textContent = (node.recv ? node.recv + '.' : '') + node.name;
    div.appendChild(name);

    const file = document.createElement('div');
    file.className = 'node-file';
    const shortFile = node.file.split('/').slice(-2).join('/');
    file.textContent = shortFile + ':' + node.line;
    div.appendChild(file);

    div.addEventListener('click', (e) => {
      e.stopPropagation();
      showSource(node.file, node.line);
      // Highlight
      document.querySelectorAll('.graph-node').forEach(n => n.style.borderColor = '');
      div.style.borderColor = 'var(--accent)';
    });

    // Show callees
    const edges = children[node.id] || [];
    if (edges.length > 0 && !visited.has(node.id)) {
      visited.add(node.id);
      const edgeContainer = document.createElement('div');
      edgeContainer.className = 'graph-edges';
      for (const edge of edges) {
        const callee = allNodes[edge.callee_id];
        if (callee && !visited.has(edge.callee_id)) {
          edgeContainer.appendChild(createGraphNode(callee, children, allNodes, visited));
        }
      }
      if (edgeContainer.children.length > 0) {
        div.appendChild(edgeContainer);
      }
    }

    return div;
  }

  // Select a step
  function selectStep(idx) {
    if (idx < 0 || idx >= state.flatSteps.length) return;

    state.currentStep = idx;
    const step = state.flatSteps[idx];

    // Update counter
    dom.stepCounter.textContent = `Step ${idx + 1} / ${state.flatSteps.length}`;
    dom.stepSlider.value = idx;

    // Highlight in tree
    document.querySelectorAll('.tree-item.selected').forEach(el => el.classList.remove('selected'));
    const treeItem = document.querySelector(`.tree-item[data-node-id="${step.id}"]`);
    if (treeItem) {
      treeItem.classList.add('selected');
      treeItem.scrollIntoView({ block: 'nearest', behavior: 'smooth' });

      // Expand parents
      let parent = treeItem.closest('.tree-children');
      while (parent) {
        parent.classList.add('expanded');
        const toggle = parent.previousElementSibling?.querySelector('.tree-toggle');
        if (toggle) toggle.textContent = '▼';
        parent = parent.parentElement?.closest('.tree-children');
      }
    }

    // Show source
    if (step.file && step.line) {
      showSource(step.file, step.line);
    }

    // Show variables
    renderVariables(step);

    // Update context
    dom.varContext.textContent = step.func_name || step.description || '';
  }

  // Show source file with highlighted line
  async function showSource(file, highlightLine) {
    if (!file) return;

    dom.sourceFile.textContent = file.split('/').slice(-2).join('/');

    let lines;
    if (state.sourceCache[file]) {
      lines = state.sourceCache[file];
    } else {
      const data = await fetchJSON('/api/source?file=' + encodeURIComponent(file));
      if (!data || !data.lines) return;
      lines = data.lines;
      state.sourceCache[file] = lines;
    }

    dom.sourceCode.innerHTML = '';
    for (let i = 0; i < lines.length; i++) {
      const lineNum = i + 1;
      const div = document.createElement('div');
      div.className = 'source-line';
      if (lineNum === highlightLine) div.classList.add('current');

      const num = document.createElement('span');
      num.className = 'line-number';
      num.textContent = lineNum;
      div.appendChild(num);

      const content = document.createElement('span');
      content.className = 'line-content';
      content.textContent = lines[i];
      div.appendChild(content);

      dom.sourceCode.appendChild(div);
    }

    // Scroll to highlighted line
    const currentLine = dom.sourceCode.querySelector('.source-line.current');
    if (currentLine) {
      currentLine.scrollIntoView({ block: 'center', behavior: 'smooth' });
    }
  }

  // Render variables panel
  function renderVariables(step) {
    dom.variables.innerHTML = '';

    if (!step.variables || step.variables.length === 0) {
      const empty = document.createElement('div');
      empty.className = 'empty-state';
      empty.innerHTML = '<span class="emoji">📭</span><span>No variables at this step</span>';
      dom.variables.appendChild(empty);
      return;
    }

    // Group header
    const header = document.createElement('div');
    header.className = 'var-group-header';
    header.textContent = step.description || 'Variables';
    dom.variables.appendChild(header);

    for (const v of step.variables) {
      const item = document.createElement('div');
      item.className = 'var-item';

      const nameSpan = document.createElement('span');
      nameSpan.className = 'var-name';
      nameSpan.textContent = v.name;
      item.appendChild(nameSpan);

      const valueSpan = document.createElement('span');
      valueSpan.className = 'var-value';
      valueSpan.textContent = v.value;
      valueSpan.title = v.type + ': ' + v.value;
      item.appendChild(valueSpan);

      dom.variables.appendChild(item);
    }
  }

  // Bind events
  function bindEvents() {
    dom.btnPrev.addEventListener('click', () => selectStep(state.currentStep - 1));
    dom.btnNext.addEventListener('click', () => selectStep(state.currentStep + 1));
    dom.btnRestart.addEventListener('click', () => selectStep(0));

    dom.btnStepInto.addEventListener('click', () => {
      // Step into: go to next step (which might be inside a function)
      selectStep(state.currentStep + 1);
    });

    dom.btnStepOver.addEventListener('click', () => {
      // Step over: skip to next sibling step
      const current = state.flatSteps[state.currentStep];
      if (current && current.step_type === 'func_enter') {
        // Find the matching func_exit or next sibling
        let depth = 0;
        for (let i = state.currentStep + 1; i < state.flatSteps.length; i++) {
          const s = state.flatSteps[i];
          if (s.step_type === 'func_enter') depth++;
          if (s.step_type === 'func_exit') {
            if (depth === 0) {
              selectStep(i + 1);
              return;
            }
            depth--;
          }
        }
      }
      selectStep(state.currentStep + 1);
    });

    dom.stepSlider.addEventListener('input', (e) => {
      selectStep(parseInt(e.target.value));
    });

    dom.btnExpandAll.addEventListener('click', () => {
      document.querySelectorAll('.tree-children').forEach(el => el.classList.add('expanded'));
      document.querySelectorAll('.tree-toggle').forEach(el => { if (el.textContent === '▶') el.textContent = '▼'; });
    });

    dom.btnCollapseAll.addEventListener('click', () => {
      document.querySelectorAll('.tree-children').forEach(el => el.classList.remove('expanded'));
      document.querySelectorAll('.tree-toggle').forEach(el => { if (el.textContent === '▼') el.textContent = '▶'; });
    });

    // Keyboard shortcuts
    document.addEventListener('keydown', (e) => {
      if (e.key === 'ArrowLeft' || e.key === 'h') selectStep(state.currentStep - 1);
      if (e.key === 'ArrowRight' || e.key === 'l') selectStep(state.currentStep + 1);
      if (e.key === 'ArrowUp' || e.key === 'k') selectStep(state.currentStep - 1);
      if (e.key === 'ArrowDown' || e.key === 'j') selectStep(state.currentStep + 1);
      if (e.key === 'Home' || e.key === 'r') selectStep(0);
      if (e.key === 'End') selectStep(state.flatSteps.length - 1);
    });

    // Panel resizing
    setupResizer('resizer-left', 'panel-tree', true);
    setupResizer('resizer-right', 'panel-vars', false);
  }

  // Panel resizing
  function setupResizer(resizerId, panelId, isLeft) {
    const resizer = document.getElementById(resizerId);
    const panel = document.getElementById(panelId);
    let startX, startWidth;

    resizer.addEventListener('mousedown', (e) => {
      startX = e.clientX;
      startWidth = panel.offsetWidth;
      document.addEventListener('mousemove', onMouseMove);
      document.addEventListener('mouseup', onMouseUp);
      e.preventDefault();
    });

    function onMouseMove(e) {
      const diff = e.clientX - startX;
      const newWidth = isLeft ? startWidth + diff : startWidth - diff;
      panel.style.width = Math.max(150, newWidth) + 'px';
    }

    function onMouseUp() {
      document.removeEventListener('mousemove', onMouseMove);
      document.removeEventListener('mouseup', onMouseUp);
    }
  }

  // Helpers
  function stepTypeClass(type) {
    const map = {
      'func_enter': 'func-enter',
      'func_exit': 'func-exit',
      'assign': 'assign',
      'branch': 'branch',
      'loop_iter': 'loop',
      'return': 'return-step',
    };
    return map[type] || '';
  }

  function stepIcon(type) {
    const map = {
      'func_enter': '→',
      'func_exit': '←',
      'assign': '=',
      'branch': '?',
      'loop_iter': '↻',
      'return': '↩',
      'call': '→',
    };
    return map[type] || '·';
  }

  function truncate(str, len) {
    if (!str) return '';
    return str.length > len ? str.substring(0, len) + '...' : str;
  }

  async function fetchJSON(url) {
    try {
      const res = await fetch(url);
      return await res.json();
    } catch (e) {
      console.error('Fetch error:', e);
      return null;
    }
  }

  // Start
  init();
})();
