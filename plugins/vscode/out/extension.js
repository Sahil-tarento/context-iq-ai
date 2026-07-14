"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.activate = activate;
exports.deactivate = deactivate;
const vscode = require("vscode");
const axios_1 = require("axios");
// Default URL for the local ContextIQ daemon
const CONTEXTIQ_API_URL = 'http://localhost:9009/v1';
function activate(context) {
    console.log('ContextIQ Extension Activated (Zero Config Mode)');
    // Ask Command
    let askCommand = vscode.commands.registerCommand('contextiq.ask', async () => {
        const editor = vscode.window.activeTextEditor;
        if (!editor) {
            vscode.window.showInformationMessage('Open a file to use ContextIQ');
            return;
        }
        const query = await vscode.window.showInputBox({
            prompt: 'Ask ContextIQ AI about this code:',
            placeHolder: 'e.g., How does this function calculate the area?'
        });
        if (!query)
            return;
        // Gather open files context
        const openFiles = vscode.workspace.textDocuments
            .filter(doc => !doc.isUntitled)
            .map(doc => doc.fileName);
        const cursorFile = editor.document.fileName;
        const cursorLine = editor.selection.active.line + 1; // 1-indexed for backend
        let workspaceFolder = '';
        if (vscode.workspace.workspaceFolders && vscode.workspace.workspaceFolders.length > 0) {
            workspaceFolder = vscode.workspace.workspaceFolders[0].uri.fsPath;
        }
        vscode.window.withProgress({
            location: vscode.ProgressLocation.Notification,
            title: "ContextIQ: Analyzing and querying AI...",
            cancellable: false
        }, async (progress) => {
            try {
                // Post to local ContextIQ REST Server
                const response = await axios_1.default.post(`${CONTEXTIQ_API_URL}/chat`, {
                    query: query,
                    provider: 'mock', // Zero-config default: uses mock or ollama if configured
                    model: 'mock-model',
                    open_files: openFiles,
                    cursor_file: cursorFile,
                    cursor_line: cursorLine,
                    repo_path: workspaceFolder,
                    max_tokens: 2048
                });
                const data = response.data;
                const panel = vscode.window.createWebviewPanel('contextiqResponse', 'ContextIQ Response', vscode.ViewColumn.Beside, {});
                panel.webview.html = getWebviewContent(query, data);
            }
            catch (error) {
                vscode.window.showErrorMessage(`ContextIQ Error: Ensure the daemon is running on :8080. ${error.message}`);
            }
        });
    });
    // Index Command
    let indexCommand = vscode.commands.registerCommand('contextiq.index', async () => {
        if (!vscode.workspace.workspaceFolders) {
            vscode.window.showWarningMessage('No workspace open to index.');
            return;
        }
        const workspaceFolder = vscode.workspace.workspaceFolders[0].uri.fsPath;
        vscode.window.withProgress({
            location: vscode.ProgressLocation.Notification,
            title: "ContextIQ: Indexing workspace...",
            cancellable: false
        }, async (progress) => {
            try {
                const response = await axios_1.default.post(`${CONTEXTIQ_API_URL}/index`, {
                    repo_path: workspaceFolder
                });
                vscode.window.showInformationMessage(`ContextIQ: Indexed ${response.data.files_indexed} files and ${response.data.symbols_indexed} symbols.`);
            }
            catch (error) {
                vscode.window.showErrorMessage(`ContextIQ Indexing Error: ${error.message}`);
            }
        });
    });
    context.subscriptions.push(askCommand);
    context.subscriptions.push(indexCommand);
}
function deactivate() { }
function getWebviewContent(query, data) {
    return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: var(--vscode-font-family); padding: 20px; line-height: 1.6; }
        .stats { background: var(--vscode-editor-inactiveSelectionBackground); padding: 10px; border-radius: 5px; margin-bottom: 20px; font-size: 0.9em; }
        .success { color: #4CAF50; font-weight: bold; }
        pre { background: var(--vscode-textCodeBlock-background); padding: 10px; border-radius: 4px; overflow-x: auto; }
    </style>
</head>
<body>
    <h2>Query: ${query}</h2>
    
    <div class="stats">
        <div><strong>Tokens Sent:</strong> ${data.optimized_tokens} (reduced from ${data.raw_tokens})</div>
        <div><strong>Savings:</strong> <span class="success">${data.token_savings.toFixed(1)}%</span></div>
        <div><strong>Source:</strong> ${data.from_cache ? '⚡ Semantic Cache Hit' : '🤖 Live LLM Inference'}</div>
    </div>

    <h3>Response:</h3>
    <pre>${data.response}</pre>
</body>
</html>`;
}
//# sourceMappingURL=extension.js.map