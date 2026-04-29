"use strict";

const vscode = require("vscode");
const cp = require("child_process");
const fs = require("fs");
const path = require("path");

let statusBar;
let output;

function activate(context) {
    output = vscode.window.createOutputChannel("mdcompress");
    statusBar = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 100);
    statusBar.command = "mdcompress.viewCompressed";

    context.subscriptions.push(
        output,
        statusBar,
        vscode.workspace.onDidSaveTextDocument(handleSave),
        vscode.window.onDidChangeActiveTextEditor(() => updateStatusBar()),
        vscode.commands.registerCommand("mdcompress.viewCompressed", viewCompressed),
        vscode.commands.registerCommand("mdcompress.runCurrentFile", runCurrentFile)
    );

    updateStatusBar();
}

function deactivate() {}

function getConfig(key, fallback) {
    return vscode.workspace.getConfiguration("mdcompress").get(key, fallback);
}

function workspaceFor(doc) {
    return vscode.workspace.getWorkspaceFolder(doc.uri);
}

function relSlashPath(folder, doc) {
    const rel = path.relative(folder.uri.fsPath, doc.uri.fsPath);
    return rel.split(path.sep).join("/");
}

function isTrackable(doc) {
    if (!doc) return false;
    if (doc.languageId !== "markdown") return false;
    if (doc.uri.scheme !== "file") return false;
    const fsPath = doc.uri.fsPath;
    const cacheSegment = path.sep + ".mdcompress" + path.sep + "cache" + path.sep;
    if (fsPath.includes(cacheSegment)) return false;
    return true;
}

function repoConfigured(folder) {
    if (!folder) return false;
    return fs.existsSync(path.join(folder.uri.fsPath, ".mdcompress"));
}

function readManifest(workspaceRoot) {
    try {
        const raw = fs.readFileSync(path.join(workspaceRoot, ".mdcompress", "manifest.json"), "utf-8");
        return JSON.parse(raw);
    } catch (_) {
        return null;
    }
}

async function handleSave(doc) {
    if (!getConfig("runOnSave", true)) return;
    if (!isTrackable(doc)) return;
    const folder = workspaceFor(doc);
    if (!repoConfigured(folder)) return;

    const rel = relSlashPath(folder, doc);
    await runMdcompress(folder.uri.fsPath, rel);
    updateStatusBar();
}

function runMdcompress(cwd, relPath) {
    const bin = getConfig("binaryPath", "mdcompress");
    return new Promise((resolve) => {
        cp.execFile(bin, ["run", relPath], { cwd }, (err, stdout, stderr) => {
            if (stdout) output.append(stdout);
            if (stderr) output.append(stderr);
            if (err) {
                output.appendLine(`mdcompress failed: ${err.message}`);
                if (err.code === "ENOENT") {
                    vscode.window.showWarningMessage(
                        `mdcompress binary not found (looked for "${bin}"). Set "mdcompress.binaryPath" in settings.`
                    );
                }
            }
            resolve();
        });
    });
}

function updateStatusBar() {
    if (!getConfig("showStatusBar", true)) {
        statusBar.hide();
        return;
    }
    const editor = vscode.window.activeTextEditor;
    if (!editor || !isTrackable(editor.document)) {
        statusBar.hide();
        return;
    }
    const folder = workspaceFor(editor.document);
    if (!repoConfigured(folder)) {
        statusBar.hide();
        return;
    }

    const rel = relSlashPath(folder, editor.document);
    const manifest = readManifest(folder.uri.fsPath);
    if (!manifest || !manifest.entries) {
        statusBar.text = "$(zap) mdcompress: no manifest";
        statusBar.tooltip = "Run `mdcompress run --all` to populate the cache.";
        statusBar.show();
        return;
    }

    const entry = manifest.entries[rel];
    if (!entry) {
        statusBar.text = "$(zap) mdcompress: not tracked";
        statusBar.tooltip = `${rel} is not in the manifest yet. Save the file or run \`mdcompress run\`.`;
        statusBar.show();
        return;
    }

    const before = entry.tokens_before || 0;
    const after = entry.tokens_after || 0;
    const saved = before - after;
    const pct = before > 0 ? (saved / before) * 100 : 0;
    statusBar.text = `$(zap) mdcompress: -${saved} tok (${pct.toFixed(1)}%)`;
    statusBar.tooltip = `tokens: ${before} -> ${after}\nClick to diff against the compressed mirror.`;
    statusBar.show();
}

async function viewCompressed() {
    const editor = vscode.window.activeTextEditor;
    if (!editor) {
        vscode.window.showInformationMessage("mdcompress: open a markdown file first.");
        return;
    }
    const doc = editor.document;
    if (!isTrackable(doc)) {
        vscode.window.showInformationMessage("mdcompress: active file is not a tracked markdown file.");
        return;
    }
    const folder = workspaceFor(doc);
    if (!repoConfigured(folder)) {
        vscode.window.showInformationMessage("mdcompress: this workspace does not have a .mdcompress/ directory.");
        return;
    }

    const rel = relSlashPath(folder, doc);
    const cachePath = path.join(folder.uri.fsPath, ".mdcompress", "cache", ...rel.split("/"));
    if (!fs.existsSync(cachePath)) {
        const choice = await vscode.window.showWarningMessage(
            `No compressed mirror at ${path.relative(folder.uri.fsPath, cachePath)}.`,
            "Refresh now"
        );
        if (choice === "Refresh now") {
            await runMdcompress(folder.uri.fsPath, rel);
            if (!fs.existsSync(cachePath)) return;
        } else {
            return;
        }
    }

    const cacheUri = vscode.Uri.file(cachePath);
    await vscode.commands.executeCommand("vscode.diff", doc.uri, cacheUri, `${path.basename(rel)} ↔ compressed`);
}

async function runCurrentFile() {
    const editor = vscode.window.activeTextEditor;
    if (!editor) return;
    const doc = editor.document;
    if (!isTrackable(doc)) {
        vscode.window.showInformationMessage("mdcompress: active file is not a tracked markdown file.");
        return;
    }
    const folder = workspaceFor(doc);
    if (!repoConfigured(folder)) {
        vscode.window.showInformationMessage("mdcompress: this workspace does not have a .mdcompress/ directory.");
        return;
    }
    const rel = relSlashPath(folder, doc);
    await runMdcompress(folder.uri.fsPath, rel);
    updateStatusBar();
}

module.exports = { activate, deactivate };
