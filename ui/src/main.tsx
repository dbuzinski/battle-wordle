import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { BrowserRouter as Router } from 'react-router-dom';
import { NotificationWebSocketProvider } from './components/NotificationWebSocketContext';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <Router>
      <NotificationWebSocketProvider ws={null}>
        <App />
      </NotificationWebSocketProvider>
    </Router>
  </StrictMode>,
)
