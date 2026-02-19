import type { Store } from "../state/store.js";
import type { WSClient } from "../connection/ws-client.js";

// LogViewerController manages log WS subscriptions.
// It subscribes/unsubscribes from log channels as the selected agent
// and active tab change.  Rendering is handled by agent-detail.ts.
export class LogViewerController {
  private store: Store;
  private wsClient: WSClient;
  private subscribedAgentId: string | null = null;
  private followMode = true;

  constructor(store: Store, wsClient: WSClient) {
    this.store = store;
    this.wsClient = wsClient;

    // Watch for agent selection / tab changes
    this.store.subscribe(() => this.onStateChange());
  }

  private onStateChange() {
    const { selectedAgentId, activeTab } = this.store.current;

    if (activeTab === 1 && selectedAgentId) {
      if (selectedAgentId !== this.subscribedAgentId) {
        this.switchSubscription(selectedAgentId);
      }
    } else if (activeTab !== 1 && this.subscribedAgentId) {
      this.unsubscribeCurrent();
    }
  }

  private switchSubscription(agentId: string) {
    this.unsubscribeCurrent();
    this.subscribedAgentId = agentId;
    this.wsClient.subscribe(`logs:${agentId}`);
  }

  private unsubscribeCurrent() {
    if (this.subscribedAgentId) {
      this.wsClient.unsubscribe(`logs:${this.subscribedAgentId}`);
      this.subscribedAgentId = null;
    }
  }

  toggleFollow(): boolean {
    this.followMode = !this.followMode;
    return this.followMode;
  }

  get isFollowing(): boolean {
    return this.followMode;
  }

  destroy() {
    this.unsubscribeCurrent();
  }
}
