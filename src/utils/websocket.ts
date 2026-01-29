import WebSocket from "ws";

export interface WsMessage {
  type: string;
  [key: string]: any;
}

export function connectWebSocket(
  url: string,
  options: {
    onMessage: (msg: WsMessage) => void;
    onError?: (err: Error) => void;
    onClose?: () => void;
    onOpen?: () => void;
  }
): WebSocket {
  const ws = new WebSocket(url);

  ws.on("open", () => {
    options.onOpen?.();
  });

  ws.on("message", (data) => {
    try {
      const msg = JSON.parse(data.toString());
      options.onMessage(msg);
    } catch (e) {
      console.error("Failed to parse WebSocket message:", e);
    }
  });

  ws.on("error", (err) => {
    options.onError?.(err);
  });

  ws.on("close", () => {
    options.onClose?.();
  });

  return ws;
}

export function buildWsUrl(
  host: string,
  store: string,
  repo: string,
  token: string,
  options: { branch?: string; lastEventId?: number } = {}
): string {
  const wsHost = host.replace(/^http/, "ws");
  const params = new URLSearchParams({ token });
  if (options.branch) params.set("branch", options.branch);
  if (options.lastEventId !== undefined) {
    params.set("lastEventId", options.lastEventId.toString());
  }
  return `${wsHost}/stores/${store}/repos/${repo}/ws?${params}`;
}
