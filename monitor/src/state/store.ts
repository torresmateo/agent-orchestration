import type {
  AgentSnapshot,
  PoolSnapshot,
  Envelope,
  StatusSnapshotPayload,
  StatusUpdatePayload,
  AgentEventPayload,
  LogDataPayload,
} from "../connection/protocol.js";
import { MSG } from "../connection/protocol.js";

export type StoreListener = () => void;

export interface AppState {
  connected: boolean;
  pool: PoolSnapshot;
  agents: AgentSnapshot[];
  selectedAgentId: string | null;
  activeTab: number; // 0=Info, 1=Logs, 2=Files, 3=Diff
  logs: Map<string, string[]>; // agentID -> log lines
}

const MAX_LOG_LINES = 5000;

export class Store {
  private state: AppState = {
    connected: false,
    pool: { warm: 0, active: 0, cold: 0 },
    agents: [],
    selectedAgentId: null,
    activeTab: 0,
    logs: new Map(),
  };

  private listeners: StoreListener[] = [];

  get current(): Readonly<AppState> {
    return this.state;
  }

  subscribe(listener: StoreListener): () => void {
    this.listeners.push(listener);
    return () => {
      this.listeners = this.listeners.filter((l) => l !== listener);
    };
  }

  private emit(): void {
    for (const listener of this.listeners) {
      listener();
    }
  }

  // Connection state
  setConnected(connected: boolean): void {
    if (this.state.connected === connected) return;
    this.state.connected = connected;
    this.emit();
  }

  // Selection
  selectAgent(agentID: string | null): void {
    if (this.state.selectedAgentId === agentID) return;
    this.state.selectedAgentId = agentID;
    this.emit();
  }

  setActiveTab(tab: number): void {
    const clamped = Math.max(0, Math.min(3, tab));
    if (this.state.activeTab === clamped) return;
    this.state.activeTab = clamped;
    this.emit();
  }

  // Get currently selected agent
  get selectedAgent(): AgentSnapshot | undefined {
    return this.state.agents.find(
      (a) => a.agentID === this.state.selectedAgentId
    );
  }

  // Get agents grouped by project
  get agentsByProject(): Map<string, AgentSnapshot[]> {
    const grouped = new Map<string, AgentSnapshot[]>();
    for (const agent of this.state.agents) {
      const key = agent.project || "unknown";
      const list = grouped.get(key) || [];
      list.push(agent);
      grouped.set(key, list);
    }
    return grouped;
  }

  // Get log lines for an agent
  getAgentLogs(agentID: string): string[] {
    return this.state.logs.get(agentID) || [];
  }

  // Handle incoming WebSocket messages
  handleMessage(env: Envelope): void {
    switch (env.type) {
      case MSG.STATUS_SNAPSHOT: {
        const payload = env.payload as StatusSnapshotPayload;
        this.state.pool = payload.pool;
        this.state.agents = payload.agents;
        // Auto-select first agent if none selected
        if (
          !this.state.selectedAgentId &&
          payload.agents.length > 0
        ) {
          this.state.selectedAgentId = payload.agents[0].agentID;
        }
        this.emit();
        break;
      }

      case MSG.STATUS_UPDATE: {
        const payload = env.payload as StatusUpdatePayload;
        const agent = this.state.agents.find(
          (a) => a.agentID === payload.agentID
        );
        if (agent) {
          agent.state = payload.state;
          if (payload.message) agent.message = payload.message;
          if (payload.branch) agent.branch = payload.branch;
          this.emit();
        }
        break;
      }

      case MSG.AGENT_REGISTERED: {
        const payload = env.payload as AgentEventPayload;
        if (payload.agent) {
          // Remove existing if present
          this.state.agents = this.state.agents.filter(
            (a) => a.agentID !== payload.agentID
          );
          this.state.agents.push(payload.agent);
          // Auto-select if first agent
          if (!this.state.selectedAgentId) {
            this.state.selectedAgentId = payload.agentID;
          }
          this.emit();
        }
        break;
      }

      case MSG.AGENT_DEREGISTERED: {
        const payload = env.payload as AgentEventPayload;
        this.state.agents = this.state.agents.filter(
          (a) => a.agentID !== payload.agentID
        );
        // Clear selection if the deregistered agent was selected
        if (this.state.selectedAgentId === payload.agentID) {
          this.state.selectedAgentId =
            this.state.agents[0]?.agentID || null;
        }
        // Clean up logs
        this.state.logs.delete(payload.agentID);
        this.emit();
        break;
      }

      case MSG.LOGS_DATA: {
        const payload = env.payload as LogDataPayload;
        let lines = this.state.logs.get(payload.agentID);
        if (!lines) {
          lines = [];
          this.state.logs.set(payload.agentID, lines);
        }
        lines.push(payload.line);
        // Cap log lines
        if (lines.length > MAX_LOG_LINES) {
          lines.splice(0, lines.length - MAX_LOG_LINES);
        }
        this.emit();
        break;
      }
    }
  }
}
