import React, { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';

const Matchmaking: React.FC = () => {
  const navigate = useNavigate();
  useEffect(() => {
    const player = JSON.parse(localStorage.getItem('player') || 'null');
    if (!player || !player.id) {
      window.location.href = '/';
      return;
    }
    const ws = new WebSocket(
      window.location.protocol === 'https:'
        ? `wss://${window.location.host}/ws/matchmaking`
        : `ws://${window.location.host}/ws/matchmaking`
    );
    ws.onopen = () => {
      ws.send(JSON.stringify({ type: 'join', player_id: player.id }));
    };
    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        if (msg.type === 'match_found' && msg.game_id) {
          ws.close();
          navigate(`/game/${msg.game_id}`);
        }
      } catch {}
    };
    ws.onerror = () => {
      // Optionally handle error
    };
    return () => {
      ws.close();
    };
  }, [navigate]);

  return (
    <div style={{
      width: '100vw',
      height: '100vh',
      background: '#121213',
      display: 'flex',
      justifyContent: 'center',
      alignItems: 'center',
      fontFamily: `-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, 'Open Sans', 'Helvetica Neue', sans-serif`,
    }}>
      <div style={{
        backgroundColor: 'rgba(0,0,0,0.9)',
        padding: 30,
        borderRadius: 12,
        textAlign: 'center',
        boxShadow: '0 4px 20px rgba(0,0,0,0.3)',
        backdropFilter: 'blur(8px)',
        border: '1px solid rgba(255,255,255,0.1)',
        maxWidth: 500,
      }}>
        <div style={{
          width: 40,
          height: 40,
          border: '3px solid rgba(255,255,255,0.1)',
          borderTopColor: '#538d4e',
          borderRadius: '50%',
          margin: '0 auto',
          animation: 'spin 1s linear infinite',
        }}
        className="queue-spinner"
        />
        <p style={{
          color: 'white',
          margin: '15px 0 0',
          fontSize: '1.2rem',
          fontWeight: 500,
          letterSpacing: 0.5
        }}>
          Finding match...
        </p>
      </div>
      {/* Spinner animation keyframes */}
      {/* TODO: Move spinner CSS to a stylesheet for scalability */}
      <style>{`
        @keyframes spin {
          to { transform: rotate(360deg); }
        }
      `}</style>
    </div>
  );
};

export default Matchmaking; 