/* global require, monaco */
require.config({ paths: { vs: '/vendor/vs' } });
require(['vs/editor/editor.main'], async () => {
  const $ = id => document.getElementById(id);
  const examples = {
    select: 'SELECT u.name, u.email\nFROM users AS u\nWHERE u.id = 1;',
    insert: "INSERT INTO users (name, email)\nVALUES ('Grace', );",
    lint: 'SELECT * FROM users WHERE name = NULL;',
    unicode: "SELECT '😀', u.name FROM users AS u;",
  };
  const model = monaco.editor.createModel(examples.select, 'sql', monaco.Uri.parse('file:///sqls-playground/query.sql'));
  const editor = monaco.editor.create($('editor'), { model, theme: 'vs-dark', automaticLayout: true,
    fontSize: 15, minimap: { enabled: false }, scrollBeyondLastLine: false,
    wordBasedSuggestions: 'off', padding: { top: 18 }, tabSize: 2, insertSpaces: true });
  const textDocument = { uri: model.uri.toString() };
  const toPosition = p => ({ line: p.lineNumber - 1, character: p.column - 1 });
  const toRange = r => ({ start: toPosition(r.getStartPosition()), end: toPosition(r.getEndPosition()) });
  const fromRange = r => new monaco.Range(r.start.line + 1, r.start.character + 1, r.end.line + 1, r.end.character + 1);
  const fromEdit = e => ({ range: fromRange(e.range), text: e.newText });
  const at = p => ({ textDocument, position: toPosition(p) });
  const lines = [];
  function log(direction, message) {
    lines.push(`${direction} ${JSON.stringify(message)}`);
    if (lines.length > 150) lines.shift();
    $('log').textContent = lines.join('\n');
    $('log').scrollTop = $('log').scrollHeight;
  }
  let client;
  let connected = false;
  let generation = 0;
  function setConnected(value, status) {
    connected = value;
    $('status').textContent = status;
    for (const id of ['format', 'range-format', 'execute']) $(id).disabled = !value;
  }
  function showDiagnostics(params) {
    if (params.uri !== textDocument.uri) return;
    const severities = { 1: monaco.MarkerSeverity.Error, 2: monaco.MarkerSeverity.Warning,
      3: monaco.MarkerSeverity.Info, 4: monaco.MarkerSeverity.Hint };
    monaco.editor.setModelMarkers(model, 'sqls', params.diagnostics.map(d => ({
      startLineNumber: d.range.start.line + 1, startColumn: d.range.start.character + 1,
      endLineNumber: d.range.end.line + 1, endColumn: d.range.end.character + 1,
      severity: severities[d.severity] || monaco.MarkerSeverity.Error,
      message: d.message, source: d.source, code: String(d.code || ''),
    })));
    $('diagnostic-count').textContent = params.diagnostics.length;
    $('diagnostics').replaceChildren(...params.diagnostics.map(d => {
      const li = window.document.createElement('li');
      li.textContent = `${d.range.start.line + 1}:${d.range.start.character + 1} ${d.message}`;
      return li;
    }));
  }
  class LSPClient {
    constructor() {
      this.socket = new WebSocket(`ws://${location.host}/lsp`);
      this.pending = new Map();
      this.sequence = 0;
      this.opened = new Promise((resolve, reject) => {
        this.socket.addEventListener('open', resolve, { once: true });
        this.socket.addEventListener('error', () => reject(new Error('WebSocket connection failed')), { once: true });
      });
      this.socket.addEventListener('message', event => {
        const message = JSON.parse(event.data);
        log('←', message);
        if ('id' in message && !message.method) {
          const pending = this.pending.get(message.id);
          if (!pending) return;
          clearTimeout(pending.timer);
          this.pending.delete(message.id);
          if (message.error) pending.reject(new Error(message.error.message));
          else pending.resolve(message.result);
        } else if (this === client && message.method === 'textDocument/publishDiagnostics') {
          showDiagnostics(message.params);
        }
      });
      this.socket.addEventListener('close', () => {
        for (const pending of this.pending.values()) {
          clearTimeout(pending.timer);
          pending.reject(new Error('Language server disconnected'));
        }
        this.pending.clear();
        if (this === client) setConnected(false, 'Disconnected');
      });
    }
    send(message) {
      log('→', message);
      this.socket.send(JSON.stringify(message));
    }
    notify(method, params) { this.send({ jsonrpc: '2.0', method, params }); }
    request(method, params) {
      const id = ++this.sequence;
      return new Promise((resolve, reject) => {
        const timer = setTimeout(() => {
          this.pending.delete(id);
          reject(new Error(`${method} timed out`));
        }, 10000);
        this.pending.set(id, { resolve, reject, timer });
        try { this.send({ jsonrpc: '2.0', id, method, params }); }
        catch (error) { clearTimeout(timer); this.pending.delete(id); reject(error); }
      });
    }
    async stop() {
      if (this.socket.readyState !== WebSocket.OPEN) { this.socket.close(); return; }
      this.notify('textDocument/didClose', { textDocument });
      try {
        await this.request('shutdown');
        this.notify('exit');
      } finally { this.socket.close(); }
    }
  }
  async function connect() {
    const current = ++generation;
    setConnected(false, 'Connecting…');
    if (client) await client.stop();
    monaco.editor.setModelMarkers(model, 'sqls', []);
    $('diagnostics').replaceChildren();
    $('diagnostic-count').textContent = '0';
    const next = new LSPClient();
    client = next;
    await next.opened;
    const initialized = await next.request('initialize', {
      processId: null, rootUri: null,
      capabilities: { workspace: { workspaceEdit: { documentChanges: true } },
        textDocument: { hover: { contentFormat: ['markdown', 'plaintext'] } } },
    });
    if (current !== generation) return;
    next.notify('initialized', {});
    next.notify('textDocument/didOpen', { textDocument: { ...textDocument, languageId: 'sql',
      version: model.getVersionId(), text: model.getValue() } });
    window.playground.capabilities = initialized.capabilities;
    setConnected(true, 'Ready · SQLite');
  }
  model.onDidChangeContent(() => {
    if (connected) client.notify('textDocument/didChange', { textDocument: { ...textDocument, version: model.getVersionId() },
      contentChanges: [{ text: model.getValue() }] });
  });
  const request = (method, params) => connected ? client.request(method, params) : Promise.resolve(null);
  const completionKinds = { 1: 'Text', 2: 'Method', 3: 'Function', 4: 'Constructor', 5: 'Field', 6: 'Variable',
    7: 'Class', 8: 'Interface', 9: 'Module', 10: 'Property', 11: 'Unit', 12: 'Value', 13: 'Enum', 14: 'Keyword',
    15: 'Snippet', 16: 'Color', 17: 'File', 18: 'Reference', 19: 'Folder', 20: 'EnumMember', 21: 'Constant',
    22: 'Struct', 23: 'Event', 24: 'Operator', 25: 'TypeParameter' };
  monaco.languages.registerCompletionItemProvider('sql', {
    triggerCharacters: ['.', '('],
    async provideCompletionItems(model, position) {
      const result = await request('textDocument/completion', at(position));
      const word = model.getWordUntilPosition(position);
      const range = new monaco.Range(position.lineNumber, word.startColumn, position.lineNumber, word.endColumn);
      return { suggestions: (Array.isArray(result) ? result : result?.items || []).map(item => ({
        label: item.label, kind: monaco.languages.CompletionItemKind[completionKinds[item.kind] || 'Text'],
        detail: item.detail, documentation: item.documentation ? { value: item.documentation.value } : undefined,
        insertText: item.textEdit?.newText || item.insertText || item.label,
        insertTextRules: item.insertTextFormat === 2 ? monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet : 0,
        sortText: item.sortText, filterText: item.filterText,
        range: item.textEdit ? fromRange(item.textEdit.range) : range,
      })) };
    },
  });
  monaco.languages.registerHoverProvider('sql', {
    async provideHover(model, position) {
      const result = await request('textDocument/hover', at(position));
      return result ? { contents: [{ value: result.contents.value }], range: fromRange(result.range) } : null;
    },
  });
  monaco.languages.registerDefinitionProvider('sql', {
    async provideDefinition(model, position) {
      const result = await request('textDocument/definition', at(position));
      return (result || []).map(item => ({ uri: monaco.Uri.parse(item.uri), range: fromRange(item.range) }));
    },
  });
  monaco.languages.registerRenameProvider('sql', {
    async provideRenameEdits(model, position, newName) {
      const result = await request('textDocument/rename', { ...at(position), newName });
      if (!result) return { edits: [], rejectReason: 'No identifier at this position' };
      const edits = [];
      for (const change of result.documentChanges || []) {
        for (const edit of change.edits) edits.push({ resource: monaco.Uri.parse(change.textDocument.uri),
          versionId: change.textDocument.version, textEdit: fromEdit(edit) });
      }
      for (const [uri, changes] of Object.entries(result.changes || {})) {
        for (const edit of changes) edits.push({ resource: monaco.Uri.parse(uri), textEdit: fromEdit(edit) });
      }
      return { edits };
    },
  });
  monaco.languages.registerSignatureHelpProvider('sql', {
    signatureHelpTriggerCharacters: ['(', ','], signatureHelpRetriggerCharacters: ['(', ','],
    async provideSignatureHelp(model, position) {
      const result = await request('textDocument/signatureHelp', at(position));
      return result ? { value: result, dispose() {} } : null;
    },
  });
  monaco.languages.registerDocumentFormattingEditProvider('sql', {
    async provideDocumentFormattingEdits(model, options) {
      return (await request('textDocument/formatting', { textDocument, options }) || []).map(fromEdit);
    },
  });
  monaco.languages.registerDocumentRangeFormattingEditProvider('sql', {
    async provideDocumentRangeFormattingEdits(model, range, options) {
      return (await request('textDocument/rangeFormatting', { textDocument, range: toRange(range), options }) || []).map(fromEdit);
    },
  });
  async function action(callback) {
    try { await callback(); }
    catch (error) { $('result').textContent = error.message; log('!', { error: error.message }); }
  }
  $('format').onclick = () => action(() => editor.getAction('editor.action.formatDocument').run());
  $('range-format').onclick = () => action(() => editor.getAction('editor.action.formatSelection').run());
  $('execute').onclick = () => action(async () => {
    const result = await request('workspace/executeCommand', { command: 'executeQuery', arguments: [textDocument.uri],
      range: toRange(editor.getSelection().isEmpty() ? model.getFullModelRange() : editor.getSelection()) });
    $('result').textContent = typeof result === 'string' ? result : JSON.stringify(result, null, 2);
  });
  $('example').onchange = () => { model.setValue(examples[$('example').value]); editor.setPosition({ lineNumber: 1, column: 1 }); editor.focus(); };
  $('clear-log').onclick = () => { lines.length = 0; $('log').textContent = ''; };
  $('restart').onclick = () => action(async () => {
    $('restart').disabled = true;
    try { window.playground.ready = connect(); await window.playground.ready; }
    finally { $('restart').disabled = false; }
  });
  window.playground = { editor, model, request, get connected() { return connected; } };
  window.playground.ready = connect();
  await action(() => window.playground.ready);
});
