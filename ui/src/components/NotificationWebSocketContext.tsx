import React, { createContext, useContext } from 'react';

export const NotificationWebSocketContext = createContext<WebSocket | null>(null);

export const NotificationWebSocketProvider: React.FC<{ ws: WebSocket | null; children: React.ReactNode }> = ({ ws, children }) => (
  <NotificationWebSocketContext.Provider value={ws}>
    {children}
  </NotificationWebSocketContext.Provider>
);

export function useNotificationWebSocket() {
  return useContext(NotificationWebSocketContext);
} 