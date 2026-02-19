// WebSocket message envelope
export interface Envelope {
  type: string;
  payload: unknown;
}

// Client -> Server
export interface SubscribePayload {
  channel: string;
}

export interface UnsubscribePayload {
  channel: string;
}

export interface CommandPayload {
  id: string;
  action: string;
  args: Record<string, unknown>;
}

// Server -> Client
export interface AgentSnapshot {
  agentID: string;
  vmName: string;
  vmIP: string;
  project: string;
  tool: string;
  branch?: string;
  issue?: string;
  state: string;
  message?: string;
  startedAt: string;
  elapsed: string;
  subdomain?: string;
}

export interface PoolSnapshot {
  warm: number;
  active: number;
  cold: number;
}

export interface StatusSnapshotPayload {
  pool: PoolSnapshot;
  agents: AgentSnapshot[];
}

export interface StatusUpdatePayload {
  agentID: string;
  state: string;
  message?: string;
  branch?: string;
}

export interface AgentEventPayload {
  agentID: string;
  agent?: AgentSnapshot;
}

export interface LogDataPayload {
  agentID: string;
  line: string;
}

export interface CommandResultPayload {
  id: string;
  success: boolean;
  message?: string;
  error?: string;
}

// Message types
export const MSG = {
  SUBSCRIBE: "subscribe",
  UNSUBSCRIBE: "unsubscribe",
  COMMAND: "command",
  STATUS_SNAPSHOT: "status.snapshot",
  STATUS_UPDATE: "status.update",
  AGENT_REGISTERED: "agent.registered",
  AGENT_DEREGISTERED: "agent.deregistered",
  LOGS_DATA: "logs.data",
  COMMAND_RESULT: "command.result",
} as const;
