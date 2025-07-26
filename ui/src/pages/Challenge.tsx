import React, { useState, useRef, useEffect } from 'react';
import { useNotificationWebSocket } from '../components/NotificationWebSocketContext';

interface Player {
  id: string;
  name: string;
}

const Challenge: React.FC = () => {
  const [search, setSearch] = useState('');
  const [players, setPlayers] = useState<Player[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sent, setSent] = useState<string | null>(null);
  const [touched, setTouched] = useState(false);
  const player = (() => {
    try {
      return JSON.parse(localStorage.getItem('player') || 'null');
    } catch {
      return null;
    }
  })();
  const ws = useNotificationWebSocket();
  const debounceRef = useRef<number | null>(null);

  // Debounced search
  const handleSearchChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setSearch(e.target.value);
    setTouched(true);
    if (debounceRef.current) window.clearTimeout(debounceRef.current);
    debounceRef.current = window.setTimeout(() => {
      if (e.target.value.trim().length === 0) {
        setPlayers([]);
        setError(null);
        setLoading(false);
        return;
      }
      setLoading(true);
      setError(null);
      fetch(`/api/player/search?name=${encodeURIComponent(e.target.value.trim())}`)
        .then(async (res) => {
          if (!res.ok) throw new Error('Failed to search players');
          const data = await res.json();
          setPlayers(data.filter((p: Player) => p.id !== player?.id));
          setLoading(false);
        })
        .catch(() => {
          setError('Could not search players.');
          setPlayers([]);
          setLoading(false);
        });
    }, 350);
  };

  const handleChallenge = (target: Player) => {
    setSent(target.id);
    const payload = {
      type: 'challenge_invite',
      from: player?.id,
      to: target.id,
      rematch: false,
      from_name: player?.name,
    };
    console.log('[Challenge WS] Sending challenge_invite:', payload);
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(payload));
    } else {
      alert('Challenge connection not available.');
    }
  };

  const handleCancel = (target: Player) => {
    setSent(null);
    const payload = {
      type: 'challenge_cancel',
      from: player?.id,
      to: target.id,
      // rematch and prev_game_id can be added if needed for rematch
    };
    console.log('[Challenge WS] Sending challenge_cancel:', payload);
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(payload));
    }
  };

  return (
    <div style={{ color: 'white', textAlign: 'center', marginTop: '4rem', minHeight: '60vh' }}>
      <h1>Challenge a Player</h1>
      <div style={{ margin: '2rem auto', maxWidth: 400 }}>
        <input
          type="text"
          value={search}
          onChange={handleSearchChange}
          placeholder="Search for a player by name..."
          style={{
            width: '100%',
            padding: '12px 16px',
            borderRadius: 8,
            border: '1.5px solid #333',
            fontSize: 16,
            background: '#181824',
            color: '#fff',
            marginBottom: 18,
            outline: 'none',
          }}
        />
        {loading && <p style={{ color: '#aaa', margin: 0 }}>Searching...</p>}
        {error && <p style={{ color: 'red', margin: 0 }}>{error}</p>}
        {touched && !loading && !error && search.trim().length > 0 && players.length === 0 && (
          <p style={{ color: '#aaa', margin: 0 }}>No players found.</p>
        )}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16, marginTop: 10 }}>
          {players.map((p) => (
            <div key={p.id} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', background: '#23233a', borderRadius: 8, padding: '12px 18px' }}>
              <span style={{ fontWeight: 500, fontSize: 17 }}>{p.name}</span>
              {sent === p.id ? (
                <button
                  onClick={() => handleCancel(p)}
                  style={{
                    background: '#888',
                    color: 'white',
                    border: 'none',
                    borderRadius: 6,
                    padding: '7px 18px',
                    fontWeight: 600,
                    fontSize: 15,
                    cursor: 'pointer',
                    transition: 'background 0.2s',
                  }}
                >
                  Cancel
                </button>
              ) : (
                <button
                  onClick={() => handleChallenge(p)}
                  style={{
                    background: '#538d4e',
                    color: 'white',
                    border: 'none',
                    borderRadius: 6,
                    padding: '7px 18px',
                    fontWeight: 600,
                    fontSize: 15,
                    cursor: 'pointer',
                    transition: 'background 0.2s',
                  }}
                >
                  Challenge
                </button>
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};

export default Challenge; 