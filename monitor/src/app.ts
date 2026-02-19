import blessed from "blessed";
import { WSClient } from "./connection/ws-client.js";
import type { Envelope } from "./connection/protocol.js";
import { Store } from "./state/store.js";
import { createHeader } from "./components/header.js";
import { createAgentList } from "./components/agent-list.js";
import { createAgentDetail } from "./components/agent-detail.js";
import { LogViewerController } from "./components/log-viewer.js";
import { createStatusBar } from "./components/status-bar.js";
import { showDispatchDialog } from "./components/dispatch-dialog.js";
import { showConfirmDialog } from "./components/confirm-dialog.js";
import { openShell } from "./actions/shell.js";
import { toggleMount, openInVSCode } from "./actions/mount.js";
import { killAgent } from "./actions/kill.js";

export interface AppConfig {
  wsUrl: string;
}

export function createApp(config: AppConfig) {
  // State store
  const store = new Store();

  // Screen
  const screen = blessed.screen({
    smartCSR: true,
    title: "agentvm-monitor",
    fullUnicode: true,
  });

  // Monkey-patch screen.render to prevent re-entrancy.
  // Blessed's layout engine can emit resize events during reposition,
  // which triggers another render(), causing infinite recursion.
  const _origRender = screen.render.bind(screen);
  let _inRender = false;
  (screen as any).render = function () {
    if (_inRender) return;
    _inRender = true;
    try {
      _origRender();
    } finally {
      _inRender = false;
    }
  };

  // WebSocket client
  const wsClient = new WSClient({
    url: config.wsUrl,
    onMessage: (env: Envelope) => store.handleMessage(env),
    onConnect: () => {
      store.setConnected(true);
      wsClient.subscribe("status");
    },
    onDisconnect: () => {
      store.setConnected(false);
    },
  });

  // Create UI components
  createHeader(screen, store);
  const agentList = createAgentList(screen, store);
  const detail = createAgentDetail(screen, store, wsClient);
  createStatusBar(screen, store);

  // Re-render on every store change (re-entrancy handled by monkey-patch)
  store.subscribe(() => {
    screen.render();
  });

  // Handle terminal resize — recalculate layout positions
  screen.on("resize", () => {
    const listWidth = Math.floor((screen.width as number) * 0.4);
    agentList.width = listWidth;
    (detail.box as any).left = listWidth;
    screen.render();
  });

  // Log viewer controller (manages WS subscriptions only)
  const logController = new LogViewerController(store, wsClient);

  // Track last mount path for VS Code opening
  let lastMountPath: string | null = null;

  // Focus management
  let focusedPanel: "list" | "detail" = "list";

  function setFocus(panel: "list" | "detail") {
    focusedPanel = panel;
    if (panel === "list") {
      agentList.focus();
      (agentList.style.border as any).fg = "green";
      (detail.box.style.border as any).fg = "cyan";
    } else {
      detail.contentBox.focus();
      (agentList.style.border as any).fg = "cyan";
      (detail.box.style.border as any).fg = "green";
    }
    screen.render();
  }

  // --- Global keybindings ---

  screen.key(["q", "C-c"], () => {
    logController.destroy();
    wsClient.disconnect();
    return process.exit(0);
  });

  screen.key(["tab"], () => {
    setFocus(focusedPanel === "list" ? "detail" : "list");
  });

  screen.key(["1"], () => { store.setActiveTab(0); });
  screen.key(["2"], () => { store.setActiveTab(1); });
  screen.key(["3"], () => { store.setActiveTab(2); });
  screen.key(["4"], () => { store.setActiveTab(3); });

  screen.key(["d"], () => {
    showDispatchDialog(screen, wsClient);
  });

  screen.key(["S-k"], () => {
    const agent = store.selectedAgent;
    if (!agent) return;
    showConfirmDialog(
      screen,
      `Kill agent {bold}${agent.agentID}{/bold}?`,
      async () => {
        const err = await killAgent(store, wsClient);
        if (err) {
          showMessage(screen, `Kill failed: ${err}`, "red");
        }
      }
    );
  });

  screen.key(["m"], async () => {
    const result = await toggleMount(store, wsClient);
    if (result.error) {
      showMessage(screen, `Mount error: ${result.error}`, "red");
    } else if (result.mounted && result.path) {
      lastMountPath = result.path;
      showMessage(screen, `Mounted at ${result.path}`, "green");
    } else {
      lastMountPath = null;
      showMessage(screen, "Unmounted", "yellow");
    }
  });

  screen.key(["s"], async () => {
    const err = await openShell(store, wsClient);
    if (err) {
      showMessage(screen, err, "red");
    }
  });

  screen.key(["v"], async () => {
    if (!lastMountPath) {
      showMessage(screen, "Mount the workspace first (press m)", "yellow");
      return;
    }
    const err = await openInVSCode(lastMountPath);
    if (err) {
      showMessage(screen, err, "red");
    }
  });

  screen.key(["f"], () => {
    if (store.current.activeTab === 1) {
      const following = logController.toggleFollow();
      showMessage(
        screen,
        following ? "Log follow: ON" : "Log follow: OFF",
        "cyan"
      );
    }
  });

  // Start with focus on agent list
  setFocus("list");

  // Connect
  wsClient.connect();

  // Initial render
  screen.render();

  return { screen, store, wsClient };
}

function showMessage(
  screen: blessed.Widgets.Screen,
  text: string,
  color: string
) {
  const msg = blessed.message({
    parent: screen,
    top: "center",
    left: "center",
    width: Math.min(text.length + 8, 60),
    height: 5,
    border: { type: "line" },
    style: {
      fg: "white",
      bg: "black",
      border: { fg: color },
    },
    tags: true,
  });
  msg.display(text, 3, () => {
    msg.destroy();
    screen.render();
  });
}
