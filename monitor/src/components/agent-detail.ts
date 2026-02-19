import blessed from "blessed";
import type { Store } from "../state/store.js";
import type { WSClient } from "../connection/ws-client.js";
import { colors, stateColor, toolColor } from "../utils/colors.js";
import { formatElapsed, formatTime } from "../utils/format.js";

const TAB_NAMES = ["Info", "Logs", "Files", "Diff"];

export interface DetailPanel {
  box: blessed.Widgets.BoxElement;
  contentBox: blessed.Widgets.BoxElement;
}

export function createAgentDetail(
  parent: blessed.Widgets.Screen,
  store: Store,
  _wsClient: WSClient
): DetailPanel {
  // Container — uses left/right positioning, no percentage width
  const box = blessed.box({
    parent,
    top: 3,
    left: Math.floor((parent.width as number) * 0.4),
    right: 0,
    bottom: 3,
    label: " Agent Detail ",
    border: { type: "line" },
    style: {
      fg: colors.fg,
      bg: colors.bg,
      border: { fg: colors.border },
      label: { fg: colors.border },
    },
    tags: true,
  });

  // Tab bar — fixed height, no width property
  const tabBar = blessed.box({
    parent: box,
    top: 0,
    left: 0,
    right: 0,
    height: 1,
    tags: true,
    style: { fg: colors.fg, bg: colors.bg },
  });

  // Single content area — all tabs render into this one box
  const contentBox = blessed.box({
    parent: box,
    top: 1,
    left: 0,
    right: 0,
    bottom: 0,
    tags: true,
    scrollable: true,
    alwaysScroll: true,
    keys: true,
    vi: true,
    mouse: true,
    scrollbar: { ch: " ", style: { bg: "cyan" } },
    style: { fg: colors.fg, bg: colors.bg },
  });

  function renderTabBar() {
    const { activeTab } = store.current;
    const tabs = TAB_NAMES.map((name, i) => {
      if (i === activeTab) {
        return `{inverse} ${i + 1}:${name} {/inverse}`;
      }
      return ` ${i + 1}:${name} `;
    });
    tabBar.setContent(tabs.join("|"));
  }

  function renderInfo() {
    const agent = store.selectedAgent;
    if (!agent) {
      contentBox.setContent("  No agent selected");
      return;
    }

    const sc = stateColor(agent.state);
    const tc = toolColor(agent.tool);

    const lines = [
      ``,
      `  {bold}Agent ID:{/bold}    ${agent.agentID}`,
      `  {bold}VM:{/bold}          ${agent.vmName} (${agent.vmIP || "?"})`,
      `  {bold}Project:{/bold}     ${agent.project}`,
      `  {bold}Tool:{/bold}        {${tc}-fg}${agent.tool || "-"}{/}`,
      `  {bold}State:{/bold}       {${sc}-fg}${agent.state}{/}`,
      `  {bold}Branch:{/bold}      ${agent.branch || "-"}`,
      `  {bold}Issue:{/bold}       ${agent.issue || "-"}`,
      `  {bold}Started:{/bold}     ${formatTime(agent.startedAt)}`,
      `  {bold}Elapsed:{/bold}     ${formatElapsed(agent.elapsed)}`,
      `  {bold}Subdomain:{/bold}   ${agent.subdomain || "-"}`,
    ];

    if (agent.message) {
      lines.push(``);
      lines.push(`  {bold}Message:{/bold}`);
      lines.push(`  ${agent.message}`);
    }

    contentBox.setContent(lines.join("\n"));
  }

  function renderLogs() {
    const agentId = store.current.selectedAgentId;
    if (!agentId) {
      contentBox.setContent("  No agent selected");
      return;
    }

    const lines = store.getAgentLogs(agentId);
    if (lines.length === 0) {
      contentBox.setContent("  No logs yet. Subscribing to log stream...");
    } else {
      contentBox.setContent(lines.join("\n"));
      contentBox.setScrollPerc(100);
    }
  }

  function renderFiles() {
    const agent = store.selectedAgent;
    if (!agent) {
      contentBox.setContent("  No agent selected");
    } else {
      contentBox.setContent(
        `  File browser for ${agent.agentID}\n\n` +
          `  Mount the agent's workspace first (press 'm'),\n` +
          `  then browse files here.\n\n` +
          `  REST API: GET /agents/${agent.agentID}/files?path=.`
      );
    }
  }

  // Track which agent's diff we last loaded to avoid re-fetching
  let diffLoadedFor: string | null = null;

  function renderDiff() {
    const agent = store.selectedAgent;
    if (!agent) {
      contentBox.setContent("  No agent selected");
      return;
    }
    if (agent.agentID === diffLoadedFor) return; // already loaded
    diffLoadedFor = agent.agentID;

    contentBox.setContent("  Loading diff...");
    loadDiff(agent.agentID);
  }

  async function loadDiff(agentId: string) {
    try {
      const resp = await fetch(
        `http://127.0.0.1:8091/agents/${agentId}/diff`
      );
      if (!resp.ok) {
        contentBox.setContent(`  Error loading diff: HTTP ${resp.status}`);
        parent.render();
        return;
      }
      const data = (await resp.json()) as { diff: string; stat: string };
      const content: string[] = [];

      if (data.stat) {
        content.push(`{bold}${blessed.escape(data.stat)}{/bold}`);
        content.push("");
      }

      if (data.diff && data.diff.trim() !== "no changes") {
        for (const line of data.diff.split("\n")) {
          if (line.startsWith("+++") || line.startsWith("---")) {
            content.push(`{bold}${blessed.escape(line)}{/bold}`);
          } else if (line.startsWith("+")) {
            content.push(`{green-fg}${blessed.escape(line)}{/green-fg}`);
          } else if (line.startsWith("-")) {
            content.push(`{red-fg}${blessed.escape(line)}{/red-fg}`);
          } else if (line.startsWith("@@")) {
            content.push(`{cyan-fg}${blessed.escape(line)}{/cyan-fg}`);
          } else if (line.startsWith("diff ")) {
            content.push(`{yellow-fg}${blessed.escape(line)}{/yellow-fg}`);
          } else {
            content.push(blessed.escape(line));
          }
        }
      } else {
        content.push("  No changes detected");
      }

      // Only update if we're still on the diff tab for this agent
      if (store.current.activeTab === 3 && store.current.selectedAgentId === agentId) {
        contentBox.setContent(content.join("\n"));
        parent.render();
      }
    } catch (e: any) {
      contentBox.setContent(`  Error loading diff: ${e.message}`);
      parent.render();
    }
  }

  let prevTab = -1;
  let prevAgentId: string | null = null;

  function render() {
    const { selectedAgentId, activeTab } = store.current;
    const agent = store.selectedAgent;

    box.setLabel(agent ? ` ${agent.agentID} ` : " Agent Detail ");
    renderTabBar();

    // Reset diff cache when agent changes
    if (selectedAgentId !== prevAgentId) {
      diffLoadedFor = null;
      prevAgentId = selectedAgentId;
    }

    switch (activeTab) {
      case 0: renderInfo(); break;
      case 1: renderLogs(); break;
      case 2: renderFiles(); break;
      case 3: renderDiff(); break;
    }

    prevTab = activeTab;
  }

  store.subscribe(render);
  render();

  return { box, contentBox };
}
